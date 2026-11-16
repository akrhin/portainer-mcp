package mcpserver

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/portainer"
)

// containerExec is the one tool that needs two requests: the catalog entry
// names the create call, and the handler then starts the instance to read what
// the command printed. The start call is bounded by the command itself, so it
// does not take the streaming guard every other path-shaped call does.
func (e *env) containerExec(create catalog.Tool, startPath string) handler {
	return func(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
		body, _ := args["body"].(map[string]any)
		if body == nil {
			return nil, fmt.Errorf("%s is required", create.Operation.Body)
		}
		if attach, _ := body["AttachStdin"].(bool); attach {
			return nil, errors.New("AttachStdin is not supported: the tool sends no stdin")
		}
		tty, _ := body["Tty"].(bool)
		c, err := e.client(args)
		if err != nil {
			return nil, err
		}
		path, err := portainer.ExpandPath(create.Operation.Path, args)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
		resp, err := c.Do(ctx, create.Operation.Method, path, nil, payload)
		if err != nil {
			return nil, err
		}
		var created struct{ Id string }
		if err := json.Unmarshal(resp.Body, &created); err != nil || created.Id == "" {
			return nil, fmt.Errorf("exec create returned no id: %s", cut(resp.Body, 300))
		}

		run, err := portainer.ExpandPath(startPath, map[string]any{
			"environment": args["environment"], "id": created.Id,
		})
		if err != nil {
			return nil, err
		}
		payload, err = json.Marshal(map[string]any{"Detach": false, "Tty": tty})
		if err != nil {
			return nil, fmt.Errorf("encode start: %w", err)
		}
		out, err := c.Do(ctx, http.MethodPost, run, nil, payload)
		if err != nil {
			return nil, err
		}

		text := streamText(out.Body, tty)
		if text == "" {
			text = fmt.Sprintf("HTTP %d: no output", out.Status)
		}
		if out.Truncated {
			text += fmt.Sprintf("\n…[truncated at %d bytes]", portainer.MaxResponse)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
	}
}

// streamText turns an exec start answer into the command's output. Without a
// TTY the daemon multiplexes stdout and stderr into 8-byte framed chunks. A
// non-zero exit code is not an error: the output is what the caller asked for.
func streamText(body []byte, tty bool) string {
	if tty {
		return string(body)
	}
	var out []byte
	rest := body
	for len(rest) >= 8 {
		size := int(binary.BigEndian.Uint32(rest[4:8]))
		if 8+size > len(rest) {
			break // the answer was cut inside a frame
		}
		out = append(out, rest[8:8+size]...)
		rest = rest[8+size:]
	}
	if len(out) == 0 && len(body) > 0 && body[0] != 1 && body[0] != 2 {
		return string(body) // not a multiplexed stream at all
	}
	return string(out)
}

func cut(body []byte, n int) string {
	if len(body) <= n {
		return string(body)
	}
	return string(body[:n]) + "…"
}
