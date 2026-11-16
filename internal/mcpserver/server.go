// Package mcpserver registers the catalog tools on the official MCP SDK.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/config"
	"github.com/akrhin/portainer-mcp/internal/portainer"
)

// Version is reported to MCP clients.
const Version = "0.1.0"

// Options are the command-line switches that override the config.
type Options struct {
	Toolsets   []string
	ReadOnly   bool
	Exhaustive bool
}

// New builds the MCP server for the selected toolsets.
func New(cfg *config.Config, cat *catalog.Catalog, opts Options) (*mcp.Server, error) {
	enabled := cfg.Toolsets
	if len(opts.Toolsets) > 0 {
		enabled = opts.Toolsets
	}
	readOnly := cfg.Server.ReadOnly || opts.ReadOnly
	exhaustive := cfg.Server.Exhaustive || opts.Exhaustive
	if err := cat.CheckToolsets(enabled); err != nil {
		return nil, err
	}

	env, err := newEnv(cfg.Instances)
	if err != nil {
		return nil, err
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: Version}, nil)
	registered := 0
	for _, tool := range selectTools(cat, enabled, readOnly, exhaustive) {
		h, err := handlerFor(env, cat, tool)
		if err != nil {
			return nil, err
		}
		srv.AddTool(toTool(tool), adapt(h))
		registered++
	}
	if registered == 0 {
		return nil, fmt.Errorf("no tools selected for toolsets %s", strings.Join(enabled, ", "))
	}
	return srv, nil
}

// handlerFor picks the implementation behind a catalog tool. Every tool is one
// request described by the catalog except the exec tool, which creates the exec
// instance and starts it, so it takes its own handler and reads the second
// operation's path from the catalog entry that declares it.
func handlerFor(e *env, cat *catalog.Catalog, t catalog.Tool) (handler, error) {
	if t.Name != "container_exec" {
		return e.call(t), nil
	}
	start, ok := findTool(cat, "exec_start")
	if !ok {
		return nil, fmt.Errorf("container_exec needs the exec_start operation in the catalog")
	}
	return e.containerExec(t, start.Operation.Path), nil
}

func findTool(cat *catalog.Catalog, name string) (catalog.Tool, bool) {
	for _, ts := range cat.Toolsets {
		for _, t := range ts.Tools {
			if t.Name == name {
				return t, true
			}
		}
	}
	return catalog.Tool{}, false
}

// selectTools flattens the enabled toolsets. Without exhaustive it keeps the
// curated default tools; readOnly drops write tools either way, so the client
// never sees what it may not call.
func selectTools(cat *catalog.Catalog, enabled []string, readOnly, exhaustive bool) []catalog.Tool {
	var out []catalog.Tool
	for _, name := range enabled {
		ts, ok := cat.Toolset(name)
		if !ok {
			continue
		}
		for _, t := range ts.Tools {
			if !exhaustive && !t.Default {
				continue
			}
			if readOnly && !t.Annotations.IsReadOnly() {
				continue
			}
			out = append(out, t)
		}
	}
	return out
}

func toTool(t catalog.Tool) *mcp.Tool {
	a := t.Annotations
	return &mcp.Tool{
		Name:         t.Name,
		Description:  t.Description,
		InputSchema:  t.Input,
		OutputSchema: t.Output,
		Annotations: &mcp.ToolAnnotations{
			Title:           a.Title,
			ReadOnlyHint:    a.IsReadOnly(),
			DestructiveHint: a.DestructiveHint,
			IdempotentHint:  a.IsIdempotent(),
			OpenWorldHint:   a.OpenWorldHint,
		},
	}
}

// handler performs one tool call. It returns the MCP result itself because the
// API answer decides what goes into context.
type handler func(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error)

// adapt turns a handler into a low-level MCP handler so API failures surface
// as isError results instead of protocol errors or panics.
func adapt(h handler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return toolError(fmt.Errorf("arguments must be a JSON object: %w", err)), nil
			}
		}
		out, err := h(ctx, args)
		if err != nil {
			return toolError(err), nil
		}
		return out, nil
	}
}

func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

// env resolves the instance argument to a client.
type env struct {
	clients map[string]*portainer.Client
	order   []string
}

func newEnv(instances []config.Instance) (*env, error) {
	e := &env{clients: make(map[string]*portainer.Client, len(instances))}
	for _, inst := range instances {
		c, err := portainer.New(portainer.Options{
			BaseURL:   inst.URL,
			APIKey:    inst.APIKey,
			TLSVerify: bool(inst.TLSVerify),
			CAFile:    inst.CAFile,
		})
		if err != nil {
			return nil, fmt.Errorf("instance %q: %w", inst.Name, err)
		}
		e.clients[inst.Name] = c
		e.order = append(e.order, inst.Name)
	}
	return e, nil
}

func (e *env) client(args map[string]any) (*portainer.Client, error) {
	name := e.order[0]
	if v, ok := args["instance"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("instance must be a string")
		}
		if s != "" {
			name = s
		}
	}
	c, ok := e.clients[name]
	if !ok {
		return nil, fmt.Errorf("unknown instance %q (configured: %s)", name, strings.Join(e.order, ", "))
	}
	return c, nil
}
