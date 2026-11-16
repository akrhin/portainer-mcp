package portainer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	path, err := ExpandPath("/api/endpoints/{environment}/docker/containers/{id}/json", map[string]any{
		"environment": float64(3),
		"id":          "a/b",
	})
	if err != nil {
		t.Fatalf("ExpandPath: %v", err)
	}
	if want := "/api/endpoints/3/docker/containers/a%2Fb/json"; path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}

	if _, err := ExpandPath("/api/endpoints/{id}", map[string]any{}); err == nil || !strings.Contains(err.Error(), "missing path parameter: id") {
		t.Fatalf("missing parameter error = %v", err)
	}
}

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Options{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestDoSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/stacks" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("type") != "2" || r.URL.Query().Get("ids") != "1,2" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-API-KEY") != "test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":1,"Name":"web"}`))
	})

	query := url.Values{}
	AddQuery(query, "type", float64(2), false)
	AddQuery(query, "ids", []any{float64(1), float64(2)}, true)
	resp, err := c.Do(context.Background(), http.MethodPost, "/api/stacks", query, []byte(`{"Name":"web"}`))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != 200 || string(resp.Body) != `{"Id":1,"Name":"web"}` || !resp.JSON() {
		t.Fatalf("response = %+v", resp)
	}
}

func TestDoErrors(t *testing.T) {
	cases := []struct {
		status  int
		body    string
		wantErr string
	}{
		{http.StatusUnauthorized, `{"message":"invalid API key"}`, "401 unauthorized"},
		{http.StatusNotFound, `{"message":"environment not found"}`, "404 not found"},
		{http.StatusBadRequest, `{"message":"unable to find a stack with the specified id"}`, "400 Bad Request"},
	}
	for _, tc := range cases {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})
		_, err := c.Do(context.Background(), http.MethodGet, "/api/stacks/9", nil, nil)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: want APIError, got %v", tc.status, err)
		}
		if apiErr.Status != tc.status || !strings.Contains(apiErr.Message, "message") {
			t.Fatalf("status %d: APIError = %+v", tc.status, apiErr)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("status %d: error text %q lacks %q", tc.status, err, tc.wantErr)
		}
	}
}

func TestDoTruncatesLargeAnswer(t *testing.T) {
	big := strings.Repeat("x", MaxResponse+1024)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"blob":"` + big + `"}`))
	})
	resp, err := c.Do(context.Background(), http.MethodGet, "/api/logs", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !resp.Truncated {
		t.Fatal("answer was not marked truncated")
	}
	if len(resp.Body) != MaxResponse {
		t.Fatalf("body = %d bytes, want %d", len(resp.Body), MaxResponse)
	}
	if resp.JSON() {
		t.Fatal("a truncated body must not claim to be JSON")
	}
}

func TestDoNonJSONAnswer(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-tar")
		_, _ = w.Write([]byte("tar bytes"))
	})
	resp, err := c.Do(context.Background(), http.MethodGet, "/api/images/export", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.JSON() || string(resp.Body) != "tar bytes" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestAddQuery(t *testing.T) {
	q := url.Values{}
	AddQuery(q, "a", true, false)
	AddQuery(q, "b", []any{"x", "y"}, false)
	AddQuery(q, "c", []any{"x", "y"}, true)
	AddQuery(q, "d", nil, false)
	want := "a=true&b=x&b=y&c=x%2Cy"
	if got := q.Encode(); got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}
