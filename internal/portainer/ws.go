package portainer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// DialWS opens a websocket on the instance with the same auth header and TLS
// settings as the JSON calls. The connection carries no deadline of its own:
// the caller bounds the conversation with its context. readLimit caps one
// frame; 0 leaves the library default.
func (c *Client) DialWS(ctx context.Context, path string, query url.Values, readLimit int64) (*websocket.Conn, error) {
	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", path, err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, fmt.Errorf("dial %s: unsupported scheme %q", path, u.Scheme)
	}
	// c.http.Timeout would cut a long-lived session as if it were one request.
	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
		HTTPClient: &http.Client{Transport: c.http.Transport},
		HTTPHeader: http.Header{c.auth: {c.apiKey}},
	})
	if err != nil {
		// The transport quotes the whole URL in its error; the reader gets the path,
		// because a tool answer must not disclose the manager address.
		msg := err.Error()
		for _, s := range []string{target, c.base} {
			msg = strings.ReplaceAll(msg, s, path)
		}
		msg = strings.ReplaceAll(msg, u.Host, "<manager>")
		return nil, fmt.Errorf("dial %s: %w", path, errors.New(msg))
	}
	if readLimit > 0 {
		conn.SetReadLimit(readLimit)
	}
	return conn, nil
}
