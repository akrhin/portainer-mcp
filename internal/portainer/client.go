// Package portainer is a small read client for the Portainer HTTP API.
package portainer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Options configure one instance client.
type Options struct {
	BaseURL   string
	APIKey    string
	TLSVerify bool
	CAFile    string
}

// Client talks to one Portainer instance.
type Client struct {
	base   string
	apiKey string
	http   *http.Client
}

// New builds a client. Self-signed certificates are accepted only when
// TLSVerify is false.
func New(opts Options) (*Client, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: !opts.TLSVerify} //nolint:gosec // opt-in for self-signed homelab instances
	if opts.CAFile != "" {
		pem, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca_file %s contains no certificates", opts.CAFile)
		}
		tlsCfg.RootCAs = pool
	}
	return &Client{
		base:   strings.TrimRight(opts.BaseURL, "/"),
		apiKey: opts.APIKey,
		http: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
	}, nil
}

// APIError is a non-2xx response with its body.
type APIError struct {
	Status  int
	Message string
}

// Error implements error with readable text for the statuses a user hits.
func (e *APIError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return fmt.Sprintf("portainer: 401 unauthorized: check the api_key (server said: %s)", e.Message)
	case http.StatusForbidden:
		return fmt.Sprintf("portainer: 403 forbidden: this api key may not perform the operation (server said: %s)", e.Message)
	case http.StatusNotFound:
		return fmt.Sprintf("portainer: 404 not found: %s", e.Message)
	}
	return fmt.Sprintf("portainer: %d %s: %s", e.Status, http.StatusText(e.Status), e.Message)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", path, err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(body))}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// SystemVersion returns GET /api/system/version.
func (c *Client) SystemVersion(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, "/api/system/version", &out)
}

// SystemStatus returns GET /api/system/status.
func (c *Client) SystemStatus(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, "/api/system/status", &out)
}

// SystemInfo returns GET /api/system/info.
func (c *Client) SystemInfo(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, "/api/system/info", &out)
}

// Settings returns GET /api/settings.
func (c *Client) Settings(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, "/api/settings", &out)
}

// CurrentUser returns GET /api/users/me.
func (c *Client) CurrentUser(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, "/api/users/me", &out)
}

// Endpoint is the subset of a Portainer environment used for listing.
type Endpoint struct {
	ID     int    `json:"Id"`
	Name   string `json:"Name"`
	Type   int    `json:"Type"`
	Status int    `json:"Status"`
	URL    string `json:"URL"`
}

// Endpoints returns GET /api/endpoints.
func (c *Client) Endpoints(ctx context.Context) ([]Endpoint, error) {
	var out []Endpoint
	return out, c.get(ctx, "/api/endpoints", &out)
}

// Endpoint returns GET /api/endpoints/{id}, including snapshots.
func (c *Client) Endpoint(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.get(ctx, fmt.Sprintf("/api/endpoints/%d", id), &out)
}
