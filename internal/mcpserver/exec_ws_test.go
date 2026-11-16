package mcpserver

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/arcane"
	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/config"
)

// execStub serves one WebSocket endpoint and counts the requests it got.
func execStub(t *testing.T, serve func(conn *websocket.Conn, r *http.Request)) (*httptest.Server, *int) {
	t.Helper()
	hits := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		serve(conn, r)
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

// execServer builds the Arcane server with the exec tool registered as main.go
// does when --allow-exec is set.
func execServer(t *testing.T, url string, opts Options) *mcp.Server {
	t.Helper()
	cat, err := catalog.LoadArcane()
	if err != nil {
		t.Fatalf("LoadArcane: %v", err)
	}
	cfg := &config.Config{
		Instances: []config.Instance{{Name: "arcane", URL: url, APIKey: "k-1"}},
		Toolsets:  []string{"core"},
	}
	opts.Toolsets = []string{"core"}
	opts.Name = "arcane-mcp"
	opts.Open = arcane.New
	srv, err := New(cfg, cat, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

// TestContainerExecWS covers the protocol: the command travels as the session's
// input, the answer comes back from binary frames, and the auth header is the
// Arcane one.
func TestContainerExecWS(t *testing.T) {
	var path, query, key, input string
	stub, _ := execStub(t, func(conn *websocket.Conn, r *http.Request) {
		path, query, key = r.URL.Path, r.URL.RawQuery, r.Header.Get("X-API-Key")
		_, data, err := conn.Read(r.Context())
		if err != nil {
			t.Errorf("read input: %v", err)
			return
		}
		input = string(data)
		if err := conn.Write(r.Context(), websocket.MessageBinary, []byte("ok\r\n")); err != nil {
			t.Errorf("write output: %v", err)
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "shell exited")
	})

	res, err := connect(t, execServer(t, stub.URL, Options{AllowExec: true})).
		CallTool(context.Background(), &mcp.CallToolParams{Name: "container_exec", Arguments: map[string]any{
			"id": "agent-1", "container": "web", "command": "echo ok",
		}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("container_exec = %q", text(t, res))
	}
	if want := "/api/agents/agent-1/ws/containers/web/terminal"; path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	for _, want := range []string{"shell=%2Fbin%2Fsh", "cols=200", "rows=50"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query = %q, want %s", query, want)
		}
	}
	if key != "k-1" {
		t.Fatalf("X-API-Key = %q, want k-1", key)
	}
	if want := "echo ok\nexit\n"; input != want {
		t.Fatalf("session input = %q, want %q", input, want)
	}
	if got := text(t, res); got != "ok\r\n" {
		t.Fatalf("output = %q, want %q", got, "ok\r\n")
	}
	if strings.Contains(text(t, res), "k-1") {
		t.Fatal("the answer leaked the API key")
	}
}

// TestContainerExecErrorHidesManager: a refused dial must not disclose the
// manager address or the key in the tool answer.
func TestContainerExecErrorHidesManager(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens here now, so the dial is refused

	res, err := connect(t, execServer(t, "http://"+addr, Options{AllowExec: true})).
		CallTool(context.Background(), &mcp.CallToolParams{Name: "container_exec", Arguments: map[string]any{
			"id": "agent-1", "container": "web", "command": "echo ok",
		}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	got := text(t, res)
	if !res.IsError {
		t.Fatalf("call succeeded: %q", got)
	}
	for _, leak := range []string{addr, "k-1", "http://"} {
		if strings.Contains(got, leak) {
			t.Fatalf("answer leaks %q: %q", leak, got)
		}
	}
	if want := "/api/agents/agent-1/ws/containers/web/terminal"; !strings.Contains(got, want) {
		t.Fatalf("answer lacks the path %q: %q", want, got)
	}
}

// TestContainerExecWSOutputCut: a chatty command cannot flood the context.
func TestContainerExecWSOutputCut(t *testing.T) {
	chunk := strings.Repeat("x", 40000)
	stub, _ := execStub(t, func(conn *websocket.Conn, r *http.Request) {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		for range 3 {
			if err := conn.Write(r.Context(), websocket.MessageBinary, []byte(chunk)); err != nil {
				return
			}
		}
		_ = conn.Close(websocket.StatusNormalClosure, "shell exited")
	})

	res, err := connect(t, execServer(t, stub.URL, Options{AllowExec: true})).
		CallTool(context.Background(), &mcp.CallToolParams{Name: "container_exec", Arguments: map[string]any{
			"id": "agent-1", "container": "web", "command": "yes x",
		}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	got := text(t, res)
	if res.IsError {
		t.Fatalf("container_exec = %q", got)
	}
	const marker = "\n…[truncated at 65536 bytes]"
	if idx := strings.Index(got, marker); idx != execMaxOutput {
		t.Fatalf("output before the marker = %d bytes, want %d (%q)", idx, execMaxOutput, got[len(got)-120:])
	}
}

// TestContainerExecWSTimeout: the deadline closes the session and what was read
// is returned as partial output.
func TestContainerExecWSTimeout(t *testing.T) {
	stub, _ := execStub(t, func(conn *websocket.Conn, r *http.Request) {
		if err := conn.Write(r.Context(), websocket.MessageBinary, []byte("partial\n")); err != nil {
			return
		}
		for { // hold the session open until the client gives up
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})

	started := time.Now()
	res, err := connect(t, execServer(t, stub.URL, Options{AllowExec: true})).
		CallTool(context.Background(), &mcp.CallToolParams{Name: "container_exec", Arguments: map[string]any{
			"id": "agent-1", "container": "web", "command": "sleep 60", "timeout_sec": float64(1),
		}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	got := text(t, res)
	if res.IsError {
		t.Fatalf("container_exec = %q", got)
	}
	if !strings.Contains(got, "partial\n") || !strings.Contains(got, "timed out after 1s") {
		t.Fatalf("timeout answer = %q", got)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the deadline did not fire: %s", elapsed)
	}
}

// TestContainerExecNeedsOptIn: the tool is absent unless --allow-exec, and
// read-only mode registers nothing but read tools.
func TestContainerExecNeedsOptIn(t *testing.T) {
	stub, _ := execStub(t, func(*websocket.Conn, *http.Request) {})
	cases := []struct {
		name string
		opts Options
		want bool
	}{
		{"default", Options{}, false},
		{"read-only wins", Options{AllowExec: true, ReadOnly: true}, false},
		{"opt-in", Options{AllowExec: true}, true},
	}
	for _, tc := range cases {
		found := false
		for _, tool := range listTools(t, execServer(t, stub.URL, tc.opts)) {
			if tool.Name == "container_exec" {
				found = true
			}
		}
		if found != tc.want {
			t.Fatalf("%s: container_exec registered = %v, want %v", tc.name, found, tc.want)
		}
	}
}

// TestContainerExecRefusals: bad arguments never open a session.
func TestContainerExecRefusals(t *testing.T) {
	stub, hits := execStub(t, func(*websocket.Conn, *http.Request) {})
	srv := execServer(t, stub.URL, Options{AllowExec: true})
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"empty command", map[string]any{"id": "a", "container": "c", "command": "   "}, "command is empty"},
		{"long command", map[string]any{"id": "a", "container": "c", "command": strings.Repeat("x", execMaxCommand+1)}, "the limit is 4096"},
		{"no container", map[string]any{"id": "a", "command": "ls"}, "container is required"},
		{"timeout too small", map[string]any{"id": "a", "container": "c", "command": "ls", "timeout_sec": float64(0)}, "between 1 and 300"},
		{"timeout too big", map[string]any{"id": "a", "container": "c", "command": "ls", "timeout_sec": float64(301)}, "between 1 and 300"},
		{"timeout fractional", map[string]any{"id": "a", "container": "c", "command": "ls", "timeout_sec": 1.5}, "whole number"},
	}
	for _, tc := range cases {
		res, err := connect(t, srv).CallTool(context.Background(), &mcp.CallToolParams{Name: "container_exec", Arguments: tc.args})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !res.IsError || !strings.Contains(text(t, res), tc.want) {
			t.Fatalf("%s = %q, want %q", tc.name, text(t, res), tc.want)
		}
	}
	if *hits != 0 {
		t.Fatalf("a refused call opened %d sessions", *hits)
	}
}

// TestAuditCommandCut keeps the log line readable.
func TestAuditCommandCut(t *testing.T) {
	if got := cutRunes("short", 10); got != "short" {
		t.Fatalf("short = %q", got)
	}
	if got := cutRunes(strings.Repeat("я", 210), 200); len([]rune(got)) != 201 || !strings.HasSuffix(got, "…") {
		t.Fatalf("cut = %d runes", len([]rune(got)))
	}
}
