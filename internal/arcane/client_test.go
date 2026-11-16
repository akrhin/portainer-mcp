package arcane

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akrhin/portainer-mcp/internal/config"
	"github.com/akrhin/portainer-mcp/internal/portainer"
)

func testClient(t *testing.T, url string) *portainer.Client {
	t.Helper()
	c, err := New(config.Instance{Name: "arcane", URL: url, APIKey: "k-123"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// TestNewSendsArcaneKey checks the header Arcane reads and the path prefix.
func TestNewSendsArcaneKey(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("X-API-Key"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	resp, err := c.Do(context.Background(), "GET", "/api/health", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotKey != "k-123" {
		t.Fatalf("X-API-Key = %q, want k-123", gotKey)
	}
	if gotPath != "/api/health" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(string(resp.Body), `"status":"ok"`) {
		t.Fatalf("body = %q", resp.Body)
	}
}

// TestErrorCarriesArcaneLabel checks that a refusal says which server refused.
func TestErrorCarriesArcaneLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	_, err := testClient(t, srv.URL).Do(context.Background(), "GET", "/api/health", nil, nil)
	if err == nil {
		t.Fatal("no error for 401")
	}
	var apiErr *portainer.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not an APIError", err)
	}
	if want := "arcane: 401 unauthorized"; !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("error = %q, want prefix %q", err, want)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error lost the server message: %q", err)
	}
}
