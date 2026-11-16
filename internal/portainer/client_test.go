package portainer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendsAPIKey(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-API-KEY")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"Status":"up"}`)
	}))
	defer srv.Close()

	c, err := New(Options{BaseURL: srv.URL, APIKey: "k-123"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := c.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("SystemStatus: %v", err)
	}
	if gotKey != "k-123" {
		t.Fatalf("X-API-KEY = %q, want k-123", gotKey)
	}
	if gotPath != "/api/system/status" {
		t.Fatalf("path = %q", gotPath)
	}
	if string(raw) != `{"Status":"up"}` {
		t.Fatalf("body = %s", raw)
	}
}

func TestClientParses401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Unauthorized","details":"invalid api key"}`)
	}))
	defer srv.Close()

	c, err := New(Options{BaseURL: srv.URL, APIKey: "bad"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.SystemVersion(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %T is not *APIError: %v", err, err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiErr.Status)
	}
	if !strings.Contains(apiErr.Message, "invalid api key") {
		t.Fatalf("message = %q", apiErr.Message)
	}
	if !strings.Contains(err.Error(), "401 unauthorized") {
		t.Fatalf("readable text missing: %v", err)
	}
}

func TestEndpointsDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"Id":16,"Name":"local","Type":1,"Status":1,"URL":"unix:///var/run/docker.sock"}]`)
	}))
	defer srv.Close()

	c, err := New(Options{BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	eps, err := c.Endpoints(context.Background())
	if err != nil {
		t.Fatalf("Endpoints: %v", err)
	}
	if len(eps) != 1 || eps[0].ID != 16 || eps[0].Name != "local" || eps[0].Type != 1 {
		t.Fatalf("endpoints = %+v", eps)
	}
}
