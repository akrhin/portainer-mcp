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
	core, ok := c.Toolset("core")
	if !ok {
		t.Fatal("core toolset missing")
	}
	if len(core.Tools) != 7 {
		t.Fatalf("want 7 core tools, got %d", len(core.Tools))
	}
	if _, ok := c.Toolset("docker"); !ok {
		t.Fatal("docker toolset missing")
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
  - name: docker
    depends_on: [core]
    tools:
      - name: t1
        annotations: {readOnlyHint: true}
`,
			want: `duplicate tool "t1"`,
		},
		{
			name: "write tool without destructive hint",
			doc: `
version: 1
toolsets:
  - name: core
    tools:
      - name: t1
        annotations: {readOnlyHint: false}
`,
			want: `write tool "t1": annotations.destructiveHint is required`,
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
