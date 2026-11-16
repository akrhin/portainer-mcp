// Package config loads and validates the YAML server configuration.
package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/akrhin/portainer-mcp/internal/catalog"
)

// Config is the server configuration file.
type Config struct {
	Server    Server     `yaml:"server"`
	Instances []Instance `yaml:"instances"`
	Toolsets  []string   `yaml:"toolsets"`
}

// Server holds transport and catalog switches.
type Server struct {
	Transport  string `yaml:"transport"`
	Listen     string `yaml:"listen"`
	ReadOnly   bool   `yaml:"read_only"`
	Exhaustive bool   `yaml:"exhaustive"`
}

// Instance is one Portainer server the tools can talk to.
type Instance struct {
	Name      string   `yaml:"name"`
	URL       string   `yaml:"url"`
	APIKey    string   `yaml:"api_key"`
	TLSVerify flexBool `yaml:"tls_verify"`
	CAFile    string   `yaml:"ca_file"`

	apiKeyEnv string // ${VAR} the key came from, for the empty-key error
}

// Load reads the config file, substitutes ${VAR} references and validates the
// result against the catalog.
func Load(path string, cat *catalog.Catalog) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the operator's --config flag
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.expand(); err != nil {
		return nil, err
	}
	if err := cfg.validate(cat); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Single builds the config of one instance from command-line flags. The Arcane
// server takes no config file: one manager is all it talks to.
func Single(cat *catalog.Catalog, name, rawURL, apiKey string, toolsets []string) (*Config, error) {
	u, err := expandInto(rawURL, 0)
	if err != nil {
		return nil, fmt.Errorf("instance %q: url: %w", name, err)
	}
	inst := Instance{Name: name, URL: u}
	inst.APIKey, inst.apiKeyEnv = ExpandEnv(apiKey)
	cfg := &Config{Instances: []Instance{inst}, Toolsets: toolsets}
	if err := cfg.validate(cat); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) expand() error {
	var err error
	if c.Server.Transport, err = expandInto(c.Server.Transport, 0); err != nil {
		return fmt.Errorf("server.transport: %w", err)
	}
	for i := range c.Instances {
		inst := &c.Instances[i]
		if inst.URL, err = expandInto(inst.URL, 0); err != nil {
			return fmt.Errorf("instance %q: url: %w", inst.Name, err)
		}
		inst.APIKey, inst.apiKeyEnv = ExpandEnv(inst.APIKey)
		if inst.CAFile, err = expandInto(inst.CAFile, 0); err != nil {
			return fmt.Errorf("instance %q: ca_file: %w", inst.Name, err)
		}
	}
	return nil
}

func (c *Config) validate(cat *catalog.Catalog) error {
	if len(c.Instances) == 0 {
		return fmt.Errorf("config: at least one instance is required")
	}
	seen := make(map[string]bool, len(c.Instances))
	for _, inst := range c.Instances {
		if inst.Name == "" {
			return fmt.Errorf("config: instance url %q has no name", inst.URL)
		}
		if seen[inst.Name] {
			return fmt.Errorf("config: duplicate instance name %q", inst.Name)
		}
		seen[inst.Name] = true
		u, err := url.Parse(inst.URL)
		if err != nil {
			return fmt.Errorf("config: instance %q: bad url: %w", inst.Name, err)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("config: instance %q: url must be absolute http(s), got %q", inst.Name, inst.URL)
		}
	}
	if err := cat.CheckToolsets(c.Toolsets); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	for _, inst := range c.Instances {
		if inst.APIKey != "" {
			continue
		}
		if inst.apiKeyEnv != "" {
			return fmt.Errorf("instance %q: api_key is empty; set %s", inst.Name, inst.apiKeyEnv)
		}
		return fmt.Errorf("instance %q: api_key is required", inst.Name)
	}
	switch c.Server.Transport {
	case "":
		c.Server.Transport = "stdio"
	case "stdio":
	case "http":
		if c.Server.Listen == "" {
			return fmt.Errorf("config: server.listen is required for http transport")
		}
	default:
		return fmt.Errorf("config: server.transport must be stdio or http, got %q", c.Server.Transport)
	}
	return nil
}

// ExpandEnv substitutes a whole-value ${VAR} reference. It returns the value
// and the variable name, or an empty name when s is a literal.
func ExpandEnv(s string) (value, envName string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "${") || !strings.HasSuffix(s, "}") {
		return s, ""
	}
	name := s[2 : len(s)-1]
	if name == "" || strings.ContainsAny(name, "{}") {
		return s, ""
	}
	return os.Getenv(name), name
}

// expandInto resolves ${VAR} for a string field; depth guards against a
// variable that points at another reference.
func expandInto(s string, depth int) (string, error) {
	value, name := ExpandEnv(s)
	if name == "" {
		return s, nil
	}
	if value == "" {
		return "", fmt.Errorf("${%s} is empty", name)
	}
	if depth > 4 {
		return "", fmt.Errorf("${%s} nests too deep", name)
	}
	return expandInto(value, depth+1)
}

// flexBool accepts a YAML boolean or a string, so tls_verify can come from an
// environment variable.
type flexBool bool

// UnmarshalYAML implements yaml.Unmarshaler.
func (b *flexBool) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!bool" {
		var v bool
		if err := node.Decode(&v); err != nil {
			return err
		}
		*b = flexBool(v)
		return nil
	}
	value, name := ExpandEnv(node.Value)
	if name != "" && value == "" {
		return fmt.Errorf("%s is empty", name)
	}
	v, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("expected a boolean, got %q", node.Value)
	}
	*b = flexBool(v)
	return nil
}
