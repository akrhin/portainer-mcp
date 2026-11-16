package portainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"slices"
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
	var reader io.Reader
	contentType := ""
	if body != nil {
		reader = bytes.NewReader(body)
		contentType = "application/json"
	}
	return c.roundTrip(ctx, method, path, query, contentType, reader)
}

// FilePart is one file of a multipart form body.
type FilePart struct {
	Filename string
	Content  []byte
}

// DoForm performs one request whose body is a multipart form: fields travel as
// text parts, files as file parts. Portainer parses some operations only from
// the form, so a JSON body reaches them empty and is rejected.
func (c *Client) DoForm(ctx context.Context, method, path string, query url.Values, fields map[string]string, files map[string]FilePart) (*Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		if err := w.WriteField(name, fields[name]); err != nil {
			return nil, fmt.Errorf("%s %s: form field %q: %w", method, path, name, err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		f := files[name]
		part, err := w.CreateFormFile(name, f.Filename)
		if err != nil {
			return nil, fmt.Errorf("%s %s: form file %q: %w", method, path, name, err)
		}
		if _, err := part.Write(f.Content); err != nil {
			return nil, fmt.Errorf("%s %s: form file %q: %w", method, path, name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("%s %s: close form: %w", method, path, err)
	}
	return c.roundTrip(ctx, method, path, query, w.FormDataContentType(), &buf)
}

// roundTrip is the one place that sends and reads a request: auth header,
// status mapping and the MaxResponse cut are the same for every body kind.
func (c *Client) roundTrip(ctx context.Context, method, path string, query url.Values, contentType string, body io.Reader) (*Response, error) {
	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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

// ArrayPrefix decodes the elements of a JSON array answer that Do cut at
// MaxResponse. It returns at most n of them, the number of elements that were
// complete before the cut, and ok=false when the body is not an array or holds
// no complete element.
func ArrayPrefix(body []byte, n int) (items []any, total int, ok bool) {
	if len(bytes.TrimLeft(body, " \t\r\n")) == 0 || bytes.TrimLeft(body, " \t\r\n")[0] != '[' {
		return nil, 0, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber() // keep numbers as they arrived: ids and sizes must not round
	if _, err := dec.Token(); err != nil {
		return nil, 0, false
	}
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			break // the cut landed inside this element
		}
		total++
		if len(items) < n {
			items = append(items, v)
		}
	}
	if total == 0 {
		return nil, 0, false
	}
	return items, total, true
}

// StripField removes one field name from every object of a JSON document,
// including nested ones, and returns the re-encoded document. A body that is
// not JSON, or that carries no such field, comes back unchanged.
func StripField(body []byte, field string) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return body
	}
	if !stripField(v, field) {
		return body
	}
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}

func stripField(v any, field string) bool {
	switch val := v.(type) {
	case map[string]any:
		changed := false
		if _, ok := val[field]; ok {
			delete(val, field)
			changed = true
		}
		for _, sub := range val {
			if stripField(sub, field) {
				changed = true
			}
		}
		return changed
	case []any:
		changed := false
		for _, sub := range val {
			if stripField(sub, field) {
				changed = true
			}
		}
		return changed
	}
	return false
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
