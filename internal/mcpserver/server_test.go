package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/config"
)

var update = flag.Bool("update", false, "rewrite golden files")

func testServer(t *testing.T, cat *catalog.Catalog, opts Options) *mcp.Server {
	t.Helper()
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "homelab", URL: "https://127.0.0.1:9443", APIKey: "test-key"}},
		Toolsets:  []string{"core"},
	}
	srv, err := New(cfg, cat, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func listTools(t *testing.T, srv *mcp.Server) []*mcp.Tool {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "golden", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	return res.Tools
}

func TestToolsListGolden(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	got, err := json.MarshalIndent(listTools(t, testServer(t, cat, Options{})), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "tools_core.golden.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/mcpserver -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("tools/list differs from %s; first mismatch:\n%s", path, firstDiff(got, want))
	}
}

func TestReadOnlyDropsWriteTools(t *testing.T) {
	cat, err := catalog.Parse([]byte(`
version: 1
toolsets:
  - name: core
    tools:
      - {name: read_probe, annotations: {readOnlyHint: true}}
      - {name: write_probe, annotations: {readOnlyHint: false, destructiveHint: true}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	readOnly := selectTools(cat, []string{"core"}, true)
	if len(readOnly) != 1 || readOnly[0].Name != "read_probe" {
		t.Fatalf("read_only selection = %v, want only read_probe", names(readOnly))
	}
	if all := selectTools(cat, []string{"core"}, false); len(all) != 2 {
		t.Fatalf("full selection = %v, want both tools", names(all))
	}

	embedded, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	tools := listTools(t, testServer(t, embedded, Options{ReadOnly: true}))
	if len(tools) != 7 {
		t.Fatalf("read_only core exposes %d tools, want 7", len(tools))
	}
	for _, tool := range tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Fatalf("read_only exposed write tool %q", tool.Name)
		}
	}
}

func TestUnknownToolsetFails(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(&config.Config{Toolsets: []string{"core"}}, cat, Options{Toolsets: []string{"core", "nope"}})
	if err == nil || !strings.Contains(err.Error(), `unknown toolset "nope"`) {
		t.Fatalf("want unknown toolset error, got %v", err)
	}
}

func TestUnimplementedToolsetFails(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "homelab", URL: "https://127.0.0.1:9443", APIKey: "k"}},
		Toolsets:  []string{"core"},
	}
	_, err = New(cfg, cat, Options{Toolsets: []string{"core", "docker"}})
	if err == nil || !strings.Contains(err.Error(), `toolset "docker" not implemented yet`) {
		t.Fatalf("want not-implemented error, got %v", err)
	}
}

func TestExhaustiveFails(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "homelab", URL: "https://127.0.0.1:9443", APIKey: "k"}},
		Toolsets:  []string{"core"},
	}
	_, err = New(cfg, cat, Options{Exhaustive: true})
	if err == nil || !strings.Contains(err.Error(), "exhaustive catalog requires generated client") {
		t.Fatalf("want exhaustive error, got %v", err)
	}
}

func names(tools []catalog.Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

func firstDiff(got, want []byte) string {
	gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			return "line " + strconv.Itoa(i+1) + ":\n got: " + gl[i] + "\nwant: " + wl[i]
		}
	}
	return "length differs: got " + strconv.Itoa(len(gl)) + " lines, want " + strconv.Itoa(len(wl))
}
