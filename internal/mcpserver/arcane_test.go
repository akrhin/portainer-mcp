package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/arcane"
	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/config"
)

// TestStreamDeadline drives the event stream with a server that sends one
// event and then goes silent: the tool must answer by timeout_sec with what
// arrived, not hang.
func TestStreamDeadline(t *testing.T) {
	done := make(chan struct{})
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"channel\":\"version\"}\n\n"))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer srvHTTP.Close()
	defer close(done)

	cat, err := catalog.LoadArcane()
	if err != nil {
		t.Fatalf("LoadArcane: %v", err)
	}
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "arcane", URL: srvHTTP.URL, APIKey: "k-1"}},
		Toolsets:  []string{"core", "events"},
	}
	srv, err := New(cfg, cat, Options{Toolsets: []string{"core", "events"}, Name: "arcane-mcp", Open: arcane.New, Exhaustive: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cs := connect(t, srv)

	start := time.Now()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "stream_client",
		Arguments: map[string]any{"timeout_sec": 1},
	})
	if err != nil {
		t.Fatalf("call stream_client: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("stream_client held for %v, want about a second", elapsed)
	}
	if res.IsError {
		t.Fatalf("stream_client failed: %v", res.Content)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"channel":"version"`) {
		t.Fatalf("answer lost the event: %q", text)
	}
	if !strings.Contains(text, "stream read stopped at timeout_sec") {
		t.Fatalf("answer does not say it was cut at the deadline: %q", text)
	}
}

// TestArcaneCallThroughMCP drives the Arcane catalog end to end: the key
// travels in the Arcane header and an agent id reaches the path.
func TestArcaneCallThroughMCP(t *testing.T) {
	var gotKey, gotPath string
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("X-API-Key"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"debian-docker","name":"debian-docker"}}`))
	}))
	defer srvHTTP.Close()

	cat, err := catalog.LoadArcane()
	if err != nil {
		t.Fatalf("LoadArcane: %v", err)
	}
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "arcane", URL: srvHTTP.URL, APIKey: "k-1"}},
		Toolsets:  []string{"core"},
	}
	srv, err := New(cfg, cat, Options{Toolsets: []string{"core"}, Name: "arcane-mcp", Open: arcane.New})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cs := connect(t, srv)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "environment_inspect",
		Arguments: map[string]any{"id": "debian-docker"},
	})
	if err != nil {
		t.Fatalf("call environment_inspect: %v", err)
	}
	if res.IsError {
		t.Fatalf("environment_inspect failed: %v", res.Content)
	}
	if want := "/api/agents/debian-docker"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if gotKey != "k-1" {
		t.Fatalf("X-API-Key = %q, want k-1", gotKey)
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "debian-docker") {
		t.Fatalf("answer lost: %v", res.Content)
	}
}
