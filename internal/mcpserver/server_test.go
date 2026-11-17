package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
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

func testConfig(url string) *config.Config {
	return &config.Config{
		Instances: []config.Instance{{Name: "homelab", URL: url, APIKey: "test-key"}},
		Toolsets:  []string{"core"},
	}
}

func testServer(t *testing.T, cat *catalog.Catalog, opts Options) *mcp.Server {
	t.Helper()
	srv, err := New(testConfig("https://127.0.0.1:9443"), cat, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func connect(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func listTools(t *testing.T, srv *mcp.Server) []*mcp.Tool {
	t.Helper()
	res, err := connect(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	return res.Tools
}

func TestServerReportsBuildVersion(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	cs := connect(t, testServer(t, cat, Options{Version: "9.9.9"}))
	info := cs.InitializeResult().ServerInfo
	if info == nil || info.Version != "9.9.9" {
		t.Fatalf("serverInfo = %+v, want version 9.9.9", info)
	}
}

func TestToolsListGolden(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	tools := listTools(t, testServer(t, cat, Options{Toolsets: []string{"core", "docker"}}))
	if want := 7 + 13; len(tools) != want {
		t.Fatalf("core,docker exposes %d default tools, want %d", len(tools), want)
	}
	got, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "tools_core_docker.golden.json")
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

// TestExhaustiveCounts checks that exhaustive registration exposes every
// operation of the enabled toolsets, exactly once.
func TestExhaustiveCounts(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, sets := range [][]string{{"core", "docker"}, {"core", "docker", "swarm", "k8s", "helm", "stacks", "gitops", "edge", "admin", "observability", "raw"}} {
		want := 0
		for _, name := range sets {
			ts, ok := cat.Toolset(name)
			if !ok {
				t.Fatalf("missing toolset %q", name)
			}
			want += len(ts.Tools)
		}
		tools := listTools(t, testServer(t, cat, Options{Toolsets: sets, Exhaustive: true}))
		if len(tools) != want {
			t.Fatalf("--exhaustive %v exposes %d tools, want %d", sets, len(tools), want)
		}
		seen := map[string]bool{}
		for _, tool := range tools {
			if seen[tool.Name] {
				t.Fatalf("tool %q registered twice", tool.Name)
			}
			seen[tool.Name] = true
		}
	}
}

func TestReadOnlyDropsWriteTools(t *testing.T) {
	cat, err := catalog.Parse([]byte(`
version: 1
toolsets:
  - name: core
    tools:
      - {name: read_probe, annotations: {readOnlyHint: true}, operation: {method: GET, path: /api/a}}
      - {name: write_probe, annotations: {readOnlyHint: false, destructiveHint: true}, operation: {method: POST, path: /api/b}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := names(selectTools(cat, []string{"core"}, true, true)); len(got) != 1 || got[0] != "read_probe" {
		t.Fatalf("read_only selection = %v, want only read_probe", got)
	}
	if got := names(selectTools(cat, []string{"core"}, false, true)); len(got) != 2 {
		t.Fatalf("full selection = %v, want both tools", got)
	}

	// Core is read-only in the curated set: seven tools, all read-only.
	embedded, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	tools := listTools(t, testServer(t, embedded, Options{ReadOnly: true, Toolsets: []string{"core"}}))
	if len(tools) != 7 {
		t.Fatalf("read_only core exposes %d tools, want 7", len(tools))
	}
	for _, tool := range tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Fatalf("read_only exposed write tool %q", tool.Name)
		}
	}
}

// TestReadOnlyExhaustiveHasNoWriteTool covers every set: read-only registration
// must expose only tools the catalog marks read-only.
func TestReadOnlyExhaustiveHasNoWriteTool(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	all := cat.Names()
	want := 0
	for _, name := range all {
		ts, _ := cat.Toolset(name)
		for _, tool := range ts.Tools {
			if tool.Annotations.IsReadOnly() {
				want++
			}
		}
	}
	tools := listTools(t, testServer(t, cat, Options{Toolsets: all, ReadOnly: true, Exhaustive: true}))
	if len(tools) != want {
		t.Fatalf("read-only exhaustive exposes %d tools, want %d", len(tools), want)
	}
	for _, tool := range tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Fatalf("read_only exposed write tool %q", tool.Name)
		}
	}
}

// TestCallThroughMCP drives a real request end to end: the arguments reach the
// path, the query and the body, and a JSON answer comes back structured.
func TestCallThroughMCP(t *testing.T) {
	type captured struct {
		method, path, query, body, apiKey string
	}
	got := &captured{}
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		got.method, got.path, got.query, got.body = r.Method, r.URL.Path, r.URL.RawQuery, buf.String()
		got.apiKey = r.Header.Get("X-API-KEY")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/system/status":
			_, _ = w.Write([]byte(`{"Version":"2.45.0"}`))
		default:
			_, _ = w.Write([]byte(`[{"Id":"abc"}]`))
		}
	}))
	defer srvHTTP.Close()

	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(testConfig(srvHTTP.URL), cat, Options{Toolsets: []string{"core", "docker"}})
	if err != nil {
		t.Fatal(err)
	}
	cs := connect(t, srv)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "system_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call system_status: %v", err)
	}
	if res.IsError {
		t.Fatalf("system_status failed: %v", res.Content)
	}
	if got.method != "GET" || got.path != "/api/system/status" || got.apiKey != "test-key" {
		t.Fatalf("request = %s %s (key %q)", got.method, got.path, got.apiKey)
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "2.45.0") {
		t.Fatalf("answer lost: %v", res.Content)
	}

	got.body, got.query = "", ""
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "container_list",
		Arguments: map[string]any{
			"environment": float64(7),
			"all":         true,
		},
	})
	if err != nil {
		t.Fatalf("call container_list: %v", err)
	}
	if res.IsError {
		t.Fatalf("container_list failed: %v", res.Content)
	}
	if want := "/api/endpoints/7/docker/containers/json"; got.path != want {
		t.Fatalf("path = %q, want %q", got.path, want)
	}
	if want := "all=true"; got.query != want {
		t.Fatalf("query = %q, want %q", got.query, want)
	}
	if got.body != "" {
		t.Fatalf("GET sent a body: %q", got.body)
	}
}

// TestCallErrorIsToolError checks that an API failure reaches the client as an
// isError result carrying the server's message.
func TestCallErrorIsToolError(t *testing.T) {
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid API key"}`))
	}))
	defer srvHTTP.Close()

	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(testConfig(srvHTTP.URL), cat, Options{Toolsets: []string{"core"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := connect(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "system_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("want isError result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "401") || !strings.Contains(text, "invalid API key") {
		t.Fatalf("error text = %q", text)
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
