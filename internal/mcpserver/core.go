package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

func (e *env) handlers() map[string]handler {
	return map[string]handler{
		"system_version":      e.systemVersion,
		"system_status":       e.systemStatus,
		"system_info":         e.systemInfo,
		"settings_get":        e.settingsGet,
		"whoami":              e.whoami,
		"environment_list":    e.environmentList,
		"environment_inspect": e.environmentInspect,
	}
}

func (e *env) systemVersion(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.SystemVersion(ctx)
}

func (e *env) systemStatus(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.SystemStatus(ctx)
}

func (e *env) systemInfo(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.SystemInfo(ctx)
}

func (e *env) settingsGet(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.Settings(ctx)
}

func (e *env) whoami(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.CurrentUser(ctx)
}

type environment struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

type environmentList struct {
	Environments []environment `json:"environments"`
}

func (e *env) environmentList(ctx context.Context, args map[string]any) (any, error) {
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	eps, err := c.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	out := environmentList{Environments: make([]environment, 0, len(eps))}
	for _, ep := range eps {
		out.Environments = append(out.Environments, environment{
			ID:     ep.ID,
			Name:   ep.Name,
			Type:   endpointType(ep.Type),
			Status: endpointStatus(ep.Status),
			URL:    ep.URL,
		})
	}
	return out, nil
}

func (e *env) environmentInspect(ctx context.Context, args map[string]any) (any, error) {
	id, err := intArg(args, "id")
	if err != nil {
		return nil, err
	}
	c, err := e.client(args)
	if err != nil {
		return nil, err
	}
	return c.Endpoint(ctx, id)
}

func intArg(args map[string]any, key string) (int, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch n := v.(type) {
	case float64:
		if n != math.Trunc(n) {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		return int(n), nil
	}
	return 0, fmt.Errorf("%s must be an integer", key)
}

// endpointType names Portainer's numeric environment type.
func endpointType(t int) string {
	switch t {
	case 1:
		return "docker"
	case 2:
		return "agent"
	case 3:
		return "azure"
	case 4:
		return "edge-agent"
	case 5:
		return "kubernetes-local"
	case 6:
		return "kubernetes"
	case 7:
		return "edge-kubernetes"
	}
	return "unknown(" + strconv.Itoa(t) + ")"
}

// endpointStatus names Portainer's numeric environment status.
func endpointStatus(s int) string {
	switch s {
	case 1:
		return "up"
	case 2:
		return "down"
	}
	return "unknown(" + strconv.Itoa(s) + ")"
}
