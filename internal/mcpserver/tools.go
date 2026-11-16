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
	sort.Strings(query)

	return func(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
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
		return apiResult(resp), nil
	}
}

// apiResult turns an API answer into a tool result: parsed JSON as structured
// content, everything else as text.
func apiResult(resp *portainer.Response) *mcp.CallToolResult {
	text := string(resp.Body)
	var structured any
	if resp.JSON() && len(resp.Body) > 0 {
		var v any
		if json.Unmarshal(resp.Body, &v) == nil {
			structured = v
		}
	}
	if len(resp.Body) == 0 {
		text = fmt.Sprintf("HTTP %d: empty body", resp.Status)
	}
	if resp.Truncated {
		text += fmt.Sprintf("\n…[truncated at %d bytes: ask for a narrower filter or a smaller range]", portainer.MaxResponse)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: structured,
	}
}
