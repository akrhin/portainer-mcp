package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/portainer"
)

// Limits of the one-shot exec tool. The command is short by nature; the output
// is capped so a verbose command cannot flood the context, and the deadline is
// hard because the session stays open as long as the container's shell does.
const (
	execMaxCommand  = 4 << 10
	execMaxOutput   = 64 << 10
	execDefaultSecs = 30
	execMaxSecs     = 300
	// execFrameLimit caps one websocket frame; the library default is 32 KiB and
	// a busy command prints more than that between reads.
	execFrameLimit = 1 << 20
	// execLogCommand is how much of the command the audit line carries.
	execLogCommand = 200
	// execCols keeps the TTY wide, so commands that wrap to the terminal width
	// (docker ps, ls) do not cut their own output.
	execCols = 200
	execRows = 50
)

// execTool is registered outside the catalog: Arcane exposes exec only as a
// terminal websocket, and a websocket is not an operation the catalog can
// describe.
func execTool() *mcp.Tool {
	yes := true
	return &mcp.Tool{
		Name: "container_exec",
		Description: "Run one command inside a running container and return what the terminal printed. " +
			"The command runs as a shell line in /bin/sh inside the container, with exactly the rights of the " +
			"container's own process: it can read, change and delete anything that process can reach. Nothing is " +
			"quoted or interpolated on this side, the text reaches the shell as it was sent. The session is a TTY, " +
			"so the answer carries the echoed command, the shell prompt and ANSI escapes. There is no stdin: the " +
			"session ends after the command. Arcane's terminal protocol reports no exit status, so a command that " +
			"fails is only visible in its output. Enabled by the operator with --allow-exec or ARCANE_ALLOW_EXEC=true.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Environment (agent) ID: the Arcane agent whose container runs the command.",
				},
				"container": map[string]any{
					"type":        "string",
					"description": "Container ID or name, as the agent knows it.",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "Shell line sent to /bin/sh in the container, at most 4096 bytes. Run as is.",
				},
				"timeout_sec": map[string]any{
					"type":        "integer",
					"description": "Hard deadline in seconds, 1 to 300; the session is closed when it passes.",
					"minimum":     1,
					"maximum":     execMaxSecs,
					"default":     execDefaultSecs,
				},
				"instance": map[string]any{
					"type":        "string",
					"description": "Name of a configured Arcane server; defaults to the first one.",
				},
			},
			"required": []any{"id", "container", "command"},
		},
		Annotations: &mcp.ToolAnnotations{
			Title:           "Run a command in a container",
			ReadOnlyHint:    false,
			DestructiveHint: &yes,
			IdempotentHint:  false,
			OpenWorldHint:   &yes,
		},
	}
}

// execResult is what one terminal session produced.
type execResult struct {
	// Output is what the terminal printed: command echo and prompt included.
	Output    []byte
	Truncated bool
	TimedOut  bool
	// CloseCode and CloseReason come from the server's close frame.
	CloseCode   websocket.StatusCode
	CloseReason string
}

// containerExecWS is the one-shot exec behind --allow-exec. It sends the command
// as the session's whole input, reads until the server closes the session, and
// closes it in every path.
func (e *env) containerExecWS() handler {
	return func(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
		agent, _ := args["id"].(string)
		container, _ := args["container"].(string)
		command, _ := args["command"].(string)
		switch {
		case strings.TrimSpace(agent) == "":
			return nil, errors.New("id is required: the environment (agent) ID")
		case strings.TrimSpace(container) == "":
			return nil, errors.New("container is required")
		case strings.TrimSpace(command) == "":
			return nil, errors.New("command is empty: the tool sends command text to the container's shell")
		case len(command) > execMaxCommand:
			return nil, fmt.Errorf("command is %d bytes, the limit is %d", len(command), execMaxCommand)
		}
		secs, err := execTimeout(args)
		if err != nil {
			return nil, err
		}
		c, err := e.client(args)
		if err != nil {
			return nil, err
		}

		started := time.Now()
		runCtx, cancel := context.WithTimeout(ctx, time.Duration(secs)*time.Second)
		defer cancel()
		res, err := execOnce(runCtx, c, agent, container, command)
		auditExec(agent, container, command, started, res, err)
		if err != nil {
			return nil, err
		}

		text := string(res.Output)
		if res.Truncated {
			text += fmt.Sprintf("\n…[truncated at %d bytes]", execMaxOutput)
		}
		if res.TimedOut {
			text += fmt.Sprintf("\n…[timed out after %ds: the session was closed, the output above is partial]", secs)
		}
		if res.CloseCode != websocket.StatusNormalClosure && res.CloseReason != "" {
			text += fmt.Sprintf("\n…[session closed %d: %s]", res.CloseCode, res.CloseReason)
		}
		if text == "" {
			text = "no output"
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
	}
}

// execOnce is the protocol: the Arcane terminal route starts a shell with a TTY,
// so the command is typed into it followed by exit, and the server closes the
// session when the shell is gone. Frames from the server are binary and carry
// raw terminal bytes, stdout and stderr merged; a text frame is an error message.
func execOnce(ctx context.Context, c *portainer.Client, agent, container, command string) (execResult, error) {
	path := "/api/agents/" + url.PathEscape(agent) + "/ws/containers/" + url.PathEscape(container) + "/terminal"
	query := url.Values{
		"shell": {"/bin/sh"},
		"cols":  {strconv.Itoa(execCols)},
		"rows":  {strconv.Itoa(execRows)},
	}
	conn, err := c.DialWS(ctx, path, query, execFrameLimit)
	if err != nil {
		return execResult{}, err
	}
	defer conn.CloseNow() // the session must not outlive the call, on any path

	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A text frame that is not a resize control also reaches stdin, but binary
	// cannot be mistaken for one.
	if err := conn.Write(writeCtx, websocket.MessageBinary, []byte(command+"\nexit\n")); err != nil {
		return execResult{}, fmt.Errorf("send command: %w", err)
	}

	var res execResult
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			var closed websocket.CloseError
			switch {
			case ctx.Err() != nil:
				res.TimedOut = true
			case errors.As(err, &closed):
				res.CloseCode, res.CloseReason = closed.Code, closed.Reason
			default:
				res.CloseReason = err.Error()
			}
			break
		}
		if room := execMaxOutput - len(res.Output); len(data) > 0 {
			if len(data) > room {
				res.Output = append(res.Output, data[:max(room, 0)]...)
				res.Truncated = true
			} else {
				res.Output = append(res.Output, data...)
			}
		}
	}
	// A graceful close keeps the server's log free of an abnormal-close warning.
	_ = conn.Close(websocket.StatusNormalClosure, "command finished")
	return res, nil
}

func execTimeout(args map[string]any) (int, error) {
	v, ok := args["timeout_sec"]
	if !ok || v == nil {
		return execDefaultSecs, nil
	}
	secs, ok := v.(float64)
	if !ok || secs != float64(int(secs)) {
		return 0, errors.New("timeout_sec must be a whole number of seconds")
	}
	if secs < 1 || secs > execMaxSecs {
		return 0, fmt.Errorf("timeout_sec must be between 1 and %d", execMaxSecs)
	}
	return int(secs), nil
}

// auditExec writes one line per command to the server's log: the only trace of
// what an agent did inside a container. The protocol carries no exit status, so
// the line says so instead of inventing one.
func auditExec(agent, container, command string, started time.Time, res execResult, err error) {
	status := "ok"
	switch {
	case err != nil:
		status = err.Error()
	case res.TimedOut:
		status = "timeout"
	case res.CloseCode != websocket.StatusNormalClosure && res.CloseReason != "":
		status = fmt.Sprintf("closed %d: %s", res.CloseCode, res.CloseReason)
	}
	fmt.Fprintf(os.Stderr, "arcane-mcp: container_exec agent=%s container=%s exit=unavailable status=%s duration=%s command=%q\n",
		agent, container, status, time.Since(started).Round(time.Millisecond), cutRunes(command, execLogCommand))
}

// cutRunes shortens s to n characters, marking the cut.
func cutRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
