package catalog

import (
	"strings"
	"testing"
)

func TestEmbeddedCatalogValidates(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Toolsets) != 11 {
		t.Fatalf("want 11 toolsets, got %d", len(c.Toolsets))
	}
	total, defaults := 0, 0
	for _, ts := range c.Toolsets {
		if len(ts.Tools) == 0 {
			t.Fatalf("toolset %q has no tools", ts.Name)
		}
		total += len(ts.Tools)
		for _, tool := range ts.Tools {
			if tool.Default {
				defaults++
			}
		}
	}
	// One tool per operation of both documents (288 + 108) minus the websocket
	// operations, plus the three raw extras.
	if total != 394 {
		t.Fatalf("want 394 tools, got %d", total)
	}
	if defaults != 87 {
		t.Fatalf("want 87 default tools, got %d", defaults)
	}
	core, ok := c.Toolset("core")
	if !ok {
		t.Fatal("core toolset missing")
	}
	if len(core.Tools) != 29 {
		t.Fatalf("want 29 core tools, got %d", len(core.Tools))
	}
	if _, ok := c.Toolset("docker"); !ok {
		t.Fatal("docker toolset missing")
	}
}

// TestGeneratedPaths keeps the decisions the generator makes about request
// paths honest: Portainer paths stay under /api, Docker paths go through the
// environment proxy without a pinned API version, and no parameter keeps the
// old environmentId name.
func TestGeneratedPaths(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, ts := range c.Toolsets {
		for _, tool := range ts.Tools {
			path := tool.Operation.Path
			if strings.Contains(path, "{environmentId}") {
				t.Fatalf("tool %q: path still names environmentId: %s", tool.Name, path)
			}
			if strings.Contains(path, "/v1.5") {
				t.Fatalf("tool %q: path pins a Docker API version: %s", tool.Name, path)
			}
			if !tool.Operation.Raw && !strings.HasPrefix(path, "/api") {
				t.Fatalf("tool %q: path is outside /api: %s", tool.Name, path)
			}
		}
	}
}

func TestParseErrors(t *testing.T) {
	const valid = `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/system/status}
`
	if _, err := Parse([]byte(valid)); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}

	cases := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "duplicate tool names",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/a}
  - name: docker
    depends_on: [core]
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/b}
`,
			want: `duplicate tool "t1"`,
		},
		{
			name: "write tool without destructive or idempotent hint",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: false}
        operation: {method: POST, path: /api/a}
`,
			want: `write tool "t1": annotations.destructiveHint or idempotentHint is required`,
		},
		{
			name: "tool without operation path",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET}
`,
			want: `tool "t1": operation.path is required`,
		},
		{
			name: "relative operation path",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: api/a}
`,
			want: `operation.path must start with /`,
		},
		{
			name: "operation without method",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {path: /api/a}
`,
			want: `tool "t1": operation.method is required`,
		},
		{
			name: "tool without toolset",
			doc: `
version: 1
toolsets:
  - name: ""
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/a}
`,
			want: `tool "t1" has no toolset`,
		},
		{
			name: "unknown dependency",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/a}
  - name: helm
    depends_on: [k8s]
`,
			want: `toolset "helm" depends on unknown toolset "k8s"`,
		},
		{
			name: "missing readOnlyHint",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {}
        operation: {method: GET, path: /api/a}
`,
			want: `tool "t1": annotations.readOnlyHint is required`,
		},
		{
			name: "unsupported version",
			doc: `
version: 2
toolsets: []
`,
			want: `unsupported version 2`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc))
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	const meta = `
version: 1
toolsets:
  - name: core
    description: Core.
    depends_on: []
`
	const tools = `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/a}
`
	c, err := Merge([]byte(meta), []byte(tools))
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	core, _ := c.Toolset("core")
	if core.Description != "Core." || len(core.Tools) != 1 {
		t.Fatalf("merged toolset = %+v", core)
	}

	_, err = Merge([]byte(meta), []byte(`
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/a}
  - name: ghost
    tools:
      - name: t2
        annotations: {readOnlyHint: true}
        operation: {method: GET, path: /api/b}
`))
	if err == nil || !strings.Contains(err.Error(), `generated toolset "ghost" is not declared in catalog.yaml`) {
		t.Fatalf("stray toolset error missing: %v", err)
	}
}

func TestSchemaDescriptionsAreStrings(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// YAML flow mappings split on an unquoted comma, which turned
	// `description: up, down or unknown` into two schema entries.
	for _, ts := range c.Toolsets {
		for _, tool := range ts.Tools {
			checkSchema(t, tool.Name, tool.Input)
			checkSchema(t, tool.Name, tool.Output)
		}
	}
}

func checkSchema(t *testing.T, tool string, schema map[string]any) {
	t.Helper()
	if v, ok := schema["description"]; ok {
		if _, isString := v.(string); !isString {
			t.Fatalf("tool %q: description is %T, want string: %v", tool, v, schema)
		}
	}
	if v, ok := schema["type"]; ok {
		if _, isString := v.(string); !isString {
			t.Fatalf("tool %q: schema type is %T, want string", tool, v)
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, sub := range props {
			if m, ok := sub.(map[string]any); ok {
				checkSchema(t, tool, m)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		checkSchema(t, tool, items)
	}
}

func TestCheckToolsets(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CheckToolsets([]string{"core"}); err != nil {
		t.Fatalf("core alone must be valid: %v", err)
	}
	if err := c.CheckToolsets(nil); err == nil {
		t.Fatal("empty toolset list must fail")
	}
	if err := c.CheckToolsets([]string{"core", "nope"}); err == nil || !strings.Contains(err.Error(), `unknown toolset "nope"`) {
		t.Fatalf("unknown toolset error missing: %v", err)
	}
	err = c.CheckToolsets([]string{"core", "helm"})
	if err == nil || !strings.Contains(err.Error(), `toolset "helm" requires "k8s"`) {
		t.Fatalf("dependency error missing: %v", err)
	}
}
