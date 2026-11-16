package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrhin/portainer-mcp/internal/catalog"
)

func load(t *testing.T, body string) (*Config, error) {
	t.Helper()
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path, cat)
}

const base = `
server:
  transport: stdio
instances:
  - name: homelab
    url: https://127.0.0.1:9443
    api_key: KEY
    tls_verify: false
toolsets: [core]
`

func TestLoadExpandsEnv(t *testing.T) {
	t.Setenv("PMCP_TEST_KEY", "s3cret")
	cfg, err := load(t, strings.Replace(base, "KEY", "${PMCP_TEST_KEY}", 1))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Instances[0].APIKey; got != "s3cret" {
		t.Fatalf("api_key = %q, want s3cret", got)
	}
	if cfg.Server.Transport != "stdio" {
		t.Fatalf("transport = %q, want stdio", cfg.Server.Transport)
	}
}

func TestLoadExpandsTLSVerify(t *testing.T) {
	t.Setenv("PMCP_TEST_TLS", "0")
	cfg, err := load(t, strings.Replace(base, "tls_verify: false", "tls_verify: ${PMCP_TEST_TLS}", 1))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bool(cfg.Instances[0].TLSVerify) {
		t.Fatal("tls_verify should be false")
	}
}

func TestLoadUnknownToolset(t *testing.T) {
	_, err := load(t, strings.Replace(base, "toolsets: [core]", "toolsets: [core, nope]", 1))
	if err == nil || !strings.Contains(err.Error(), `unknown toolset "nope"`) {
		t.Fatalf("want unknown toolset error, got %v", err)
	}
}

func TestLoadBrokenDependency(t *testing.T) {
	_, err := load(t, strings.Replace(base, "toolsets: [core]", "toolsets: [core, helm]", 1))
	if err == nil || !strings.Contains(err.Error(), `toolset "helm" requires "k8s"`) {
		t.Fatalf("want dependency error, got %v", err)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	t.Setenv("PMCP_TEST_EMPTY", "")
	_, err := load(t, strings.Replace(base, "KEY", "${PMCP_TEST_EMPTY}", 1))
	if err == nil || !strings.Contains(err.Error(), "PMCP_TEST_EMPTY") {
		t.Fatalf("want error naming the env var, got %v", err)
	}
}

func TestLoadNoInstances(t *testing.T) {
	body := `
server: {transport: stdio}
instances: []
toolsets: [core]
`
	_, err := load(t, body)
	if err == nil || !strings.Contains(err.Error(), "at least one instance") {
		t.Fatalf("want instance error, got %v", err)
	}
}
