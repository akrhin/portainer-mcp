package portainer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

// TestDoForm sends a multipart body: text fields as text parts, files as file
// parts, and the auth header and status mapping of Do still apply.
func TestDoForm(t *testing.T) {
	var values map[string]string
	var filename, content string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/stacks/create/standalone/file" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("endpointId") != "7" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Errorf("X-API-KEY = %q", r.Header.Get("X-API-KEY"))
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			return
		}
		values = map[string]string{}
		for name, v := range r.MultipartForm.Value {
			values[name] = v[0]
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file part: %v", err)
			return
		}
		defer f.Close()
		raw, _ := io.ReadAll(f)
		filename, content = hdr.Filename, string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":1}`))
	})

	query := url.Values{}
	AddQuery(query, "endpointId", float64(7), false)
	resp, err := c.DoForm(context.Background(), http.MethodPost, "/api/stacks/create/standalone/file", query,
		map[string]string{"Name": "web", "TagIds": "[1,2]"},
		map[string]FilePart{"file": {Filename: "stack.yml", Content: []byte("services: {}\n")}})
	if err != nil {
		t.Fatalf("DoForm: %v", err)
	}
	if resp.Status != 200 || string(resp.Body) != `{"Id":1}` {
		t.Fatalf("response = %+v", resp)
	}
	if values["Name"] != "web" || values["TagIds"] != "[1,2]" {
		t.Fatalf("fields = %v", values)
	}
	if filename != "stack.yml" || content != "services: {}\n" {
		t.Fatalf("file part = %q %q", filename, content)
	}
}

// TestDoFormError keeps the status mapping: a rejected form is an APIError like
// any other call.
func TestDoFormError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Invalid stack name"}`))
	})
	_, err := c.DoForm(context.Background(), http.MethodPost, "/api/stacks/create/standalone/file", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid stack name") {
		t.Fatalf("error text = %v", err)
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

// TestArrayPrefix reads a JSON array that Do cut at MaxResponse: complete
// elements count, a half-received one does not, and numbers keep their digits.
func TestArrayPrefix(t *testing.T) {
	body := []byte(`[{"a":123456789012345678},{"b":2},{"c":`)
	items, total, ok := ArrayPrefix(body, 1)
	if !ok || total != 2 || len(items) != 1 {
		t.Fatalf("ArrayPrefix = %v, total %d, ok %v", items, total, ok)
	}
	first, _ := items[0].(map[string]any)
	if got := first["a"].(json.Number).String(); got != "123456789012345678" {
		t.Fatalf("number = %s, want the digits as they arrived", got)
	}

	for _, body := range []string{`{"a":1}`, `[]`, `[{"a":`, ``, `not json`} {
		if _, _, ok := ArrayPrefix([]byte(body), 10); ok {
			t.Fatalf("%q reported an array prefix", body)
		}
	}
}

// TestStripField drops one field name at every depth, keeps numbers exact, and
// leaves a body without that field byte for byte.
func TestStripField(t *testing.T) {
	in := `[{"metadata":{"name":"p","managedFields":[{"manager":"k"}]},"n":123456789012345678,"nested":{"deep":[{"managedFields":[]}]}}]`
	out := StripField([]byte(in), "managedFields")
	if strings.Contains(string(out), "managedFields") {
		t.Fatalf("field survived: %s", out)
	}
	var doc []map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("stripped answer is not valid JSON: %v", err)
	}
	if doc[0]["metadata"].(map[string]any)["name"] != "p" {
		t.Fatalf("sibling field lost: %s", out)
	}
	if got := doc[0]["n"].(float64); got != 123456789012345678 {
		t.Fatalf("number = %v", got)
	}

	untouched := `{"Id":1,"Name":"web"}`
	if got := StripField([]byte(untouched), "managedFields"); string(got) != untouched {
		t.Fatalf("re-encoded a body without the field: %s", got)
	}
	if got := StripField([]byte("not json"), "managedFields"); string(got) != "not json" {
		t.Fatalf("changed a non-JSON body: %s", got)
	}
}
