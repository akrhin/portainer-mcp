package portainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// MaxResponse caps what one tool call hands back to the model. A larger answer
// is cut and marked truncated, so a log dump cannot flood the context.
const MaxResponse = 1 << 20

// Response is one API answer.
type Response struct {
	Status      int
	Body        []byte
	ContentType string
	// Truncated reports that Body is the first MaxResponse bytes of a larger
	// answer.
	Truncated bool
}

// Do performs one request against the instance. Path is the already expanded
// API path; query is encoded as given; body is sent as JSON when not nil.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body []byte) (*Response, error) {
	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(msg))}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("%s %s: read body: %w", method, path, err)
	}
	out := &Response{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}
	if len(data) > MaxResponse {
		out.Body = data[:MaxResponse]
		out.Truncated = true
		return out, nil
	}
	out.Body = data
	return out, nil
}

// JSON reports whether the body holds a complete JSON document.
func (r *Response) JSON() bool {
	return !r.Truncated && json.Valid(r.Body)
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// ExpandPath substitutes {name} placeholders with the argument values, escaping
// each one so a value cannot change the path structure.
func ExpandPath(path string, args map[string]any) (string, error) {
	var missing []string
	out := pathParam.ReplaceAllStringFunc(path, func(placeholder string) string {
		name := placeholder[1 : len(placeholder)-1]
		v, ok := args[name]
		if !ok || v == nil {
			missing = append(missing, name)
			return placeholder
		}
		return url.PathEscape(valueString(v))
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("missing path parameter: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// PathParams lists the {name} placeholders of a path template.
func PathParams(path string) []string {
	var out []string
	for _, p := range pathParam.FindAllString(path, -1) {
		out = append(out, p[1:len(p)-1])
	}
	return out
}

// AddQuery appends one argument to the query. An array becomes repeated keys,
// or one comma-joined value when csv is set.
func AddQuery(query url.Values, name string, v any, csv bool) {
	switch val := v.(type) {
	case nil:
		return
	case []any:
		if len(val) == 0 {
			return
		}
		if csv {
			parts := make([]string, len(val))
			for i, item := range val {
				parts[i] = valueString(item)
			}
			query.Set(name, strings.Join(parts, ","))
			return
		}
		for _, item := range val {
			query.Add(name, valueString(item))
		}
	default:
		query.Set(name, valueString(v))
	}
}

func valueString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		// JSON numbers arrive as float64; keep integers integral.
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case json.Number:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
