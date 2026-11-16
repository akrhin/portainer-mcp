// Package catalog loads the declarative tool catalog that drives tool
// registration, read-only filtering and validation of the toolset selection.
//
// Two documents make up the catalog: catalog.yaml is curated by hand (toolset
// names, descriptions, dependencies) and generated.yaml is written by
// tools/cataloggen from the OpenAPI documents in api/. Load merges them.
package catalog

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed catalog.yaml
var embeddedMeta []byte

//go:embed generated.yaml
var embeddedTools []byte

//go:embed arcane/catalog.yaml
var embeddedArcaneMeta []byte

//go:embed arcane/generated.yaml
var embeddedArcaneTools []byte

// Catalog is the tool catalog as data.
type Catalog struct {
	Version  int       `yaml:"version"`
	Toolsets []Toolset `yaml:"toolsets"`

	byName map[string]*Toolset
}

// Toolset is one switchable group of tools.
type Toolset struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
	Tools       []Tool   `yaml:"tools"`
}

// Tool is one MCP tool declared in the catalog.
type Tool struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Annotations Annotations    `yaml:"annotations"`
	Input       map[string]any `yaml:"input,omitempty"`
	Output      map[string]any `yaml:"output,omitempty"`
	Operation   Operation      `yaml:"operation"`
	// StreamingParams are input properties that keep the response open when
	// set true; a call that sets one is refused before the request.
	StreamingParams []string `yaml:"streaming_params,omitempty"`
	// StreamingPath marks an operation that streams whatever the arguments
	// are, so only a bounded window ("until") makes the call answer.
	StreamingPath bool `yaml:"streaming_path,omitempty"`
	// Stream marks a server-sent-event or long-poll operation that the handler
	// reads until its timeout_sec deadline instead of refusing: the bytes that
	// arrived are the answer.
	Stream bool `yaml:"stream,omitempty"`
	// StripManagedFields drops metadata.managedFields from the answer before
	// it reaches the model; Kubernetes lists repeat it in every object.
	StripManagedFields bool `yaml:"strip_managed_fields,omitempty"`
	Default            bool `yaml:"default,omitempty"`
}

// Operation is the HTTP call a tool performs.
type Operation struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
	// Body names the input property holding the request body.
	Body string `yaml:"body,omitempty"`
	// Form sends Body as multipart/form-data instead of JSON: a property that
	// is an object with a filename becomes a file part, the rest become text.
	Form bool `yaml:"form,omitempty"`
	// Binary sends Body as raw bytes with the media type the document names;
	// the property holds {content} or {base64}.
	Binary bool `yaml:"binary,omitempty"`
	// CSV lists query parameters encoded as one comma-joined value.
	CSV []string `yaml:"csv,omitempty"`
	// Raw tools take method and path from their arguments and append the path
	// to Path.
	Raw bool `yaml:"raw,omitempty"`
}

// Annotations mirror the MCP tool annotations. Pointers distinguish a missing
// hint from an explicit false.
type Annotations struct {
	Title           string `yaml:"title,omitempty"`
	ReadOnlyHint    *bool  `yaml:"readOnlyHint"`
	DestructiveHint *bool  `yaml:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `yaml:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `yaml:"openWorldHint,omitempty"`
}

// IsReadOnly reports whether the tool is explicitly declared read-only.
func (a Annotations) IsReadOnly() bool {
	return a.ReadOnlyHint != nil && *a.ReadOnlyHint
}

// IsIdempotent reports whether the tool is declared idempotent.
func (a Annotations) IsIdempotent() bool {
	return a.IdempotentHint != nil && *a.IdempotentHint
}

// Load parses and validates the embedded catalog.
func Load() (*Catalog, error) {
	return Merge(embeddedMeta, embeddedTools)
}

// LoadArcane parses and validates the embedded Arcane catalog.
func LoadArcane() (*Catalog, error) {
	return Merge(embeddedArcaneMeta, embeddedArcaneTools)
}

// Merge attaches the generated tools to the curated toolset metadata.
func Merge(meta, tools []byte) (*Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(meta, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	var gen Catalog
	if err := yaml.Unmarshal(tools, &gen); err != nil {
		return nil, fmt.Errorf("parse generated catalog: %w", err)
	}
	if gen.Version != c.Version {
		return nil, fmt.Errorf("catalog: curated version %d does not match generated version %d", c.Version, gen.Version)
	}
	generated := make(map[string][]Tool, len(gen.Toolsets))
	for _, ts := range gen.Toolsets {
		generated[ts.Name] = ts.Tools
	}
	for i := range c.Toolsets {
		ts := &c.Toolsets[i]
		found, ok := generated[ts.Name]
		if !ok {
			return nil, fmt.Errorf("catalog: toolset %q has no generated tools", ts.Name)
		}
		if len(ts.Tools) > 0 {
			return nil, fmt.Errorf("catalog: toolset %q declares tools in catalog.yaml; tools come from generated.yaml", ts.Name)
		}
		ts.Tools = found
		delete(generated, ts.Name)
	}
	for name := range generated {
		return nil, fmt.Errorf("catalog: generated toolset %q is not declared in catalog.yaml", name)
	}
	return initCatalog(&c)
}

// Parse parses and validates a catalog document. Toolsets carry their tools
// inline, so a single document is enough.
func Parse(data []byte) (*Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	return initCatalog(&c)
}

func initCatalog(c *Catalog) (*Catalog, error) {
	c.byName = make(map[string]*Toolset, len(c.Toolsets))
	for i := range c.Toolsets {
		c.byName[c.Toolsets[i].Name] = &c.Toolsets[i]
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks the catalog invariants that the golden test and registration
// rely on.
func (c *Catalog) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("catalog: unsupported version %d (want 1)", c.Version)
	}
	sets := make(map[string]bool, len(c.Toolsets))
	for i, ts := range c.Toolsets {
		if ts.Name == "" {
			if len(ts.Tools) > 0 {
				return fmt.Errorf("catalog: tool %q has no toolset", ts.Tools[0].Name)
			}
			return fmt.Errorf("catalog: toolset at index %d has no name", i)
		}
		if sets[ts.Name] {
			return fmt.Errorf("catalog: duplicate toolset %q", ts.Name)
		}
		sets[ts.Name] = true
	}

	owner := make(map[string]string)
	for _, ts := range c.Toolsets {
		for _, t := range ts.Tools {
			if t.Name == "" {
				return fmt.Errorf("catalog: toolset %q has a tool without a name", ts.Name)
			}
			if prev, ok := owner[t.Name]; ok {
				return fmt.Errorf("catalog: duplicate tool %q in toolsets %q and %q", t.Name, prev, ts.Name)
			}
			owner[t.Name] = ts.Name
			if t.Annotations.ReadOnlyHint == nil {
				return fmt.Errorf("catalog: tool %q: annotations.readOnlyHint is required", t.Name)
			}
			if !t.Annotations.IsReadOnly() &&
				t.Annotations.DestructiveHint == nil && t.Annotations.IdempotentHint == nil {
				return fmt.Errorf("catalog: write tool %q: annotations.destructiveHint or idempotentHint is required", t.Name)
			}
			if t.Input != nil && t.Input["type"] != "object" {
				return fmt.Errorf("catalog: tool %q: input schema must have type object", t.Name)
			}
			props, _ := t.Input["properties"].(map[string]any)
			for _, name := range t.StreamingParams {
				if _, ok := props[name]; !ok {
					return fmt.Errorf("catalog: tool %q: streaming param %q is not an input property", t.Name, name)
				}
			}
			if t.Output != nil && t.Output["type"] != "object" && t.Output["type"] != "array" {
				return fmt.Errorf("catalog: tool %q: output schema must have type object or array", t.Name)
			}
			if t.Operation.Path == "" {
				return fmt.Errorf("catalog: tool %q: operation.path is required", t.Name)
			}
			if !t.Operation.Raw && t.Operation.Method == "" {
				return fmt.Errorf("catalog: tool %q: operation.method is required", t.Name)
			}
			if !strings.HasPrefix(t.Operation.Path, "/") {
				return fmt.Errorf("catalog: tool %q: operation.path must start with /", t.Name)
			}
			if t.Operation.Form && t.Operation.Body == "" {
				return fmt.Errorf("catalog: tool %q: operation.form needs operation.body to name the form object", t.Name)
			}
		}
	}

	for _, ts := range c.Toolsets {
		for _, dep := range ts.DependsOn {
			if !sets[dep] {
				return fmt.Errorf("catalog: toolset %q depends on unknown toolset %q", ts.Name, dep)
			}
		}
	}
	return nil
}

// Names returns toolset names in declaration order.
func (c *Catalog) Names() []string {
	out := make([]string, len(c.Toolsets))
	for i, ts := range c.Toolsets {
		out[i] = ts.Name
	}
	return out
}

// Toolset returns the named toolset.
func (c *Catalog) Toolset(name string) (*Toolset, bool) {
	ts, ok := c.byName[name]
	return ts, ok
}

// CheckToolsets verifies that every requested toolset exists and that all of
// its dependencies are enabled too.
func (c *Catalog) CheckToolsets(enabled []string) error {
	if len(enabled) == 0 {
		return fmt.Errorf("no toolsets enabled")
	}
	on := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		if _, ok := c.byName[name]; !ok {
			return fmt.Errorf("unknown toolset %q (known: %s)", name, strings.Join(c.Names(), ", "))
		}
		on[name] = true
	}
	for _, name := range enabled {
		for _, dep := range c.byName[name].DependsOn {
			if !on[dep] {
				return fmt.Errorf("toolset %q requires %q, which is not enabled", name, dep)
			}
		}
	}
	return nil
}
