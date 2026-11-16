package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/portainer"
)

// call binds one catalog tool to the HTTP layer. The catalog says what to call;
// this builds the request from the arguments and formats the answer.
func (e *env) call(t catalog.Tool) handler {
	pathParams := make(map[string]bool)
	for _, name := range portainer.PathParams(t.Operation.Path) {
		pathParams[name] = true
	}
	var query []string
	props, _ := t.Input["properties"].(map[string]any)
	for name := range props {
		if name == "instance" || name == t.Operation.Body || pathParams[name] {
			continue
		}
		if t.Operation.Raw && (name == "method" || name == "path" || name == "query") {
			continue
		}
		query = append(query, name)
	}
	sort.Strings(query)
	csv := make(map[string]bool, len(t.Operation.CSV))
	for _, name := range t.Operation.CSV {
		csv[name] = true
	}
	required := map[string]bool{}
	if list, ok := t.Input["required"].([]any); ok {
		for _, v := range list {
			if name, ok := v.(string); ok {
				required[name] = true
			}
		}
	}

	return func(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
		if err := unbounded(t, args); err != nil {
			return nil, err
		}
		c, err := e.client(args)
		if err != nil {
			return nil, err
		}

		method := t.Operation.Method
		path := t.Operation.Path
		values := url.Values{}
		if t.Operation.Raw {
			m, _ := args["method"].(string)
			if m == "" {
				return nil, fmt.Errorf("method is required")
			}
			method = strings.ToUpper(m)
			p, _ := args["path"].(string)
			if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
				return nil, errors.New("path must start with / and must not contain parent references")
			}
			path += p
			if q, ok := args["query"].(map[string]any); ok {
				for name, v := range q {
					portainer.AddQuery(values, name, v, false)
				}
			}
		}
		path, err = portainer.ExpandPath(path, args)
		if err != nil {
			return nil, err
		}
		for _, name := range query {
			portainer.AddQuery(values, name, args[name], csv[name])
		}

		var body []byte
		if t.Operation.Body != "" {
			v, ok := args[t.Operation.Body]
			if !ok || v == nil {
				if required[t.Operation.Body] {
					return nil, fmt.Errorf("%s is required", t.Operation.Body)
				}
			} else {
				body, err = json.Marshal(v)
				if err != nil {
					return nil, fmt.Errorf("encode %s: %w", t.Operation.Body, err)
				}
			}
		}

		resp, err := c.Do(ctx, method, path, values, body)
		if err != nil {
			return nil, err
		}
		return apiResult(t, resp), nil
	}
}

// arrayPage is how many elements of a cut JSON array answer a caller sees.
const arrayPage = 50

// unbounded refuses a streaming call before anything is sent: such a request
// stays open until the client times out, and the model learns nothing from
// waiting. The flags come from the catalog, not from a list kept here.
func unbounded(t catalog.Tool, args map[string]any) error {
	for _, name := range t.StreamingParams {
		if v, ok := args[name].(bool); ok && v {
			return fmt.Errorf("this operation streams while %s=true and would not return until the client times out: pass %s=false, or bound the window with until", name, name)
		}
	}
	if !t.StreamingPath {
		return nil
	}
	if until, ok := args["until"].(string); ok && until != "" {
		return nil
	}
	return errors.New("this operation streams and does not return on its own: set until to bound the window, or use a list or log tool instead")
}

// apiResult turns an API answer into a tool result: parsed JSON as structured
// content, everything else as text. A cut array answer becomes a readable page
// instead of a broken prefix.
func apiResult(t catalog.Tool, resp *portainer.Response) *mcp.CallToolResult {
	body := resp.Body
	if t.StripManagedFields {
		body = portainer.StripField(body, "managedFields")
	}
	var structured any
	if resp.JSON() {
		var v any
		if json.Unmarshal(body, &v) == nil {
			structured = v
		}
	}
	text := string(body)
	if len(body) == 0 {
		text = fmt.Sprintf("HTTP %d: empty body", resp.Status)
	}
	if resp.Truncated {
		if page, ok := arrayPageResult(t, body); ok {
			return page
		}
		text += fmt.Sprintf("\n…[truncated at %d bytes: ask for a narrower filter or a smaller range]", portainer.MaxResponse)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: structured,
	}
}

// arrayPageResult answers the first elements of a cut array with the counts
// and the way to get the rest.
func arrayPageResult(t catalog.Tool, body []byte) (*mcp.CallToolResult, bool) {
	items, total, ok := portainer.ArrayPrefix(body, arrayPage)
	if !ok {
		return nil, false
	}
	page := map[string]any{
		"items":      items,
		"_truncated": true,
		"_total":     total,
		"_hint": fmt.Sprintf("%d items arrived before the %d-byte cut, the first %d are shown: use %s with a narrower filter or pagination",
			total, portainer.MaxResponse, len(items), t.Name),
	}
	text, err := json.Marshal(page)
	if err != nil {
		return nil, false
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
		StructuredContent: page,
	}, true
}
