package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
)

// testCall drives one tool over MCP against a stub API.
func testCall(t *testing.T, h http.HandlerFunc, toolsets []string, name string, args map[string]any) (*mcp.CallToolResult, int) {
	t.Helper()
	hits := 0
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		h(w, r)
	}))
	t.Cleanup(srvHTTP.Close)

	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(testConfig(srvHTTP.URL), cat, Options{Toolsets: toolsets})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := connect(t, srv).CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res, hits
}

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("result carries no content")
	}
	return res.Content[0].(*mcp.TextContent).Text
}

func jsonBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// TestStreamingRefusal covers the first defect: a call that would stay open
// until the client times out is refused before anything is sent.
func TestStreamingRefusal(t *testing.T) {
	sets := []string{"core", "docker", "observability"}

	res, hits := testCall(t, jsonBody(`[]`), sets, "container_logs", map[string]any{
		"environment": float64(7), "id": "abc", "follow": true,
	})
	if !res.IsError {
		t.Fatal("follow=true must be refused")
	}
	if msg := text(t, res); !strings.Contains(msg, "streams") || !strings.Contains(msg, "follow=false") {
		t.Fatalf("refusal text = %q", msg)
	}
	if hits != 0 {
		t.Fatalf("a refused call still reached the API (%d requests)", hits)
	}

	// system_events has no follow parameter: the path streams by itself.
	res, hits = testCall(t, jsonBody(`[]`), sets, "system_events", map[string]any{
		"environment": float64(7),
	})
	if !res.IsError || hits != 0 {
		t.Fatalf("unbounded system_events = error %v after %d requests", res.IsError, hits)
	}
	if msg := text(t, res); !strings.Contains(msg, "until") {
		t.Fatalf("refusal text = %q, want the way to bound it", msg)
	}

	// Bounded or non-streaming calls go through.
	cases := []struct {
		name string
		args map[string]any
	}{
		{"system_events", map[string]any{"environment": float64(7), "until": "2026-01-01T00:00:00Z"}},
		{"container_logs", map[string]any{"environment": float64(7), "id": "abc", "follow": false}},
	}
	for _, tc := range cases {
		res, hits = testCall(t, jsonBody(`[]`), sets, tc.name, tc.args)
		if res.IsError {
			t.Fatalf("%s with a bound = %q", tc.name, text(t, res))
		}
		if hits != 1 {
			t.Fatalf("%s sent %d requests, want 1", tc.name, hits)
		}
	}
}

// TestManagedFieldsStripped covers the second defect: Kubernetes answers lose
// metadata.managedFields at every depth, and only the tools the catalog marks.
func TestManagedFieldsStripped(t *testing.T) {
	const pod = `{
      "metadata": {"name": "web", "managedFields": [{"manager": "kubectl"}, {"manager": "go"}]},
      "spec": {"template": {"metadata": {"managedFields": [{"manager": "helm"}]}, "spec": {"containers": []}}},
      "containers": [{"image": "nginx", "metadata": {"managedFields": [{"manager": "nested"}]}}],
      "status": {"phase": "Running"}
    }`
	body := "[" + pod + "]"

	res, _ := testCall(t, jsonBody(body), []string{"core", "k8s"}, "kubernetes_pods_list", map[string]any{"id": float64(7)})
	if res.IsError {
		t.Fatalf("kubernetes_pods_list = %q", text(t, res))
	}
	out := text(t, res)
	if strings.Contains(out, "managedFields") {
		t.Fatalf("answer still carries managedFields: %s", out)
	}
	var pods []map[string]any
	if err := json.Unmarshal([]byte(out), &pods); err != nil {
		t.Fatalf("answer is not valid JSON: %v\n%s", err, out)
	}
	if len(pods) != 1 || pods[0]["status"].(map[string]any)["phase"] != "Running" {
		t.Fatalf("stripping changed the object: %s", out)
	}
	if len(res.StructuredContent.([]any)) != 1 {
		t.Fatalf("structured content = %v", res.StructuredContent)
	}

	// The same answer through a tool the catalog does not mark keeps the field.
	res, _ = testCall(t, jsonBody(body), []string{"core", "raw"}, "portainer_raw", map[string]any{
		"method": "GET", "path": "/kubernetes/7/pods",
	})
	if res.IsError || !strings.Contains(text(t, res), "managedFields") {
		t.Fatalf("unmarked tool lost the field: %q", text(t, res))
	}
}

// TestTruncatedArrayPage covers the third defect: a cut array answer becomes a
// page with counts instead of a broken prefix.
func TestTruncatedArrayPage(t *testing.T) {
	blob := strings.Repeat("x", 2000)
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 700; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"Id":"c%d","Blob":"%s"}`, i, blob)
	}
	sb.WriteString("]")

	res, _ := testCall(t, jsonBody(sb.String()), []string{"core", "docker"}, "container_list", map[string]any{
		"environment": float64(7),
	})
	if res.IsError {
		t.Fatalf("container_list = %q", text(t, res))
	}
	var page struct {
		Items     []any  `json:"items"`
		Truncated bool   `json:"_truncated"`
		Total     int    `json:"_total"`
		Hint      string `json:"_hint"`
	}
	if err := json.Unmarshal([]byte(text(t, res)), &page); err != nil {
		t.Fatalf("answer is not the page: %v", err)
	}
	if !page.Truncated || len(page.Items) != arrayPage || page.Total <= arrayPage {
		t.Fatalf("page = %d items, total %d, truncated %v", len(page.Items), page.Total, page.Truncated)
	}
	if !strings.Contains(page.Hint, "container_list") {
		t.Fatalf("hint %q does not name the way to narrow the list", page.Hint)
	}

	// A cut object keeps the old marker: there are no elements to page.
	res, _ = testCall(t, jsonBody(`{"blob":"`+strings.Repeat("x", 1<<20+64)+`"}`), []string{"core", "docker"}, "container_list", map[string]any{
		"environment": float64(7),
	})
	if res.IsError || !strings.Contains(text(t, res), "truncated at") {
		t.Fatalf("object answer = %q", text(t, res))
	}
}
