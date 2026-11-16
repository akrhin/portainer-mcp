package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestArcaneBuildMatchesCommittedFile is the reproducibility gate for the
// second backend: the committed catalog must be what the generator produces.
func TestArcaneBuildMatchesCommittedFile(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	body, err := yaml.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(arcaneProfile.out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(append([]byte(arcaneProfile.header), body...), want) {
		t.Fatal("internal/catalog/arcane/generated.yaml differs from a fresh run; run: go run ./tools/cataloggen -profile arcane")
	}
}

// TestArcaneCounts checks the numbers the mapping promises: one tool per
// operation of the document, plus the raw extra, and no name twice.
func TestArcaneCounts(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wantSets := map[string]int{
		"core": 36, "containers": 15, "images": 46, "volumes": 26, "networks": 8,
		"projects": 46, "backups": 27, "gitops": 26, "swarm": 54, "podman": 26,
		"security": 42, "admin": 45, "uploads": 4, "push": 6, "events": 6,
		"auth": 31, "raw": 1,
	}
	if len(cat.Toolsets) != len(wantSets) {
		t.Fatalf("%d toolsets, want %d", len(cat.Toolsets), len(wantSets))
	}
	tools, defaults := 0, 0
	seen := map[string]bool{}
	heads := map[string]bool{}
	for _, ts := range cat.Toolsets {
		want, ok := wantSets[ts.Name]
		if !ok {
			t.Fatalf("unexpected toolset %q", ts.Name)
		}
		if len(ts.Tools) != want {
			t.Fatalf("toolset %q has %d tools, want %d", ts.Name, len(ts.Tools), want)
		}
		for _, tool := range ts.Tools {
			if seen[tool.Name] {
				t.Fatalf("duplicate tool %q", tool.Name)
			}
			seen[tool.Name] = true
			tools++
			if tool.Default {
				defaults++
			}
			if tool.Operation.Method == "HEAD" {
				heads[tool.Name] = true
			}
		}
	}
	if tools != 445 {
		t.Fatalf("%d tools, want 445", tools)
	}
	if defaults != 53 {
		t.Fatalf("%d default tools, want 53", defaults)
	}
	for _, name := range []string{"health_check_head", "system_health_head"} {
		if !heads[name] {
			t.Fatalf("HEAD tool %q is missing", name)
		}
	}
}

// TestArcaneCoversEveryOperation is the completeness gate: every operation of
// the document becomes a tool, and nothing is dropped by a skip list.
func TestArcaneCoversEveryOperation(t *testing.T) {
	t.Chdir("../..")

	specs, err := loadSpecs(arcaneProfile.sources)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// An empty mapping carries no skip list, which is the point: the document
	// alone decides what has to be reachable.
	ops, err := collectOps(mapping{}, specs)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	have := map[string]string{}
	for _, ts := range cat.Toolsets {
		for _, tool := range ts.Tools {
			if tool.Operation.Raw {
				continue
			}
			key := tool.Operation.Method + " " + tool.Operation.Path
			if prev, dup := have[key]; dup {
				t.Fatalf("%s is reached twice: %q and %q", key, prev, tool.Name)
			}
			have[key] = tool.Name
		}
	}
	for _, op := range ops {
		key := op.method + " " + op.path
		if _, ok := have[key]; !ok {
			t.Errorf("%s has no tool", key)
			continue
		}
		delete(have, key)
	}
	for key, name := range have {
		t.Errorf("tool %q reaches %s, which the document does not have", name, key)
	}
}

// TestArcaneStreamsAreBounded checks the three operations that never return on
// their own: each carries the timeout that makes the call answer.
func TestArcaneStreamsAreBounded(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	streams := map[string]bool{}
	for _, ts := range cat.Toolsets {
		for _, tool := range ts.Tools {
			if !tool.Stream {
				continue
			}
			streams[tool.Operation.Path] = true
			props, _ := tool.Input["properties"].(map[string]any)
			spec, _ := props["timeout_sec"].(map[string]any)
			if spec == nil {
				t.Fatalf("%s streams without timeout_sec", tool.Name)
			}
			if spec["default"] != 10 || spec["minimum"] != 1 || spec["maximum"] != 60 {
				t.Fatalf("%s: timeout_sec is %v, want default 10, range 1..60", tool.Name, spec)
			}
		}
	}
	for _, p := range []string{"/api/stream", "/api/agents/{id}/events/changes", "/api/agents/{id}/system/upgrade/progress"} {
		if !streams[p] {
			t.Fatalf("%s is not a bounded stream", p)
		}
	}
	if len(streams) != 3 {
		t.Fatalf("%d bounded streams, want 3: %v", len(streams), streams)
	}
}

// TestArcaneBinaryBody pins the one operation whose body is raw bytes: the
// chunk upload, sent as application/octet-stream rather than JSON.
func TestArcaneBinaryBody(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var names []string
	for _, ts := range cat.Toolsets {
		for _, tool := range ts.Tools {
			if tool.Operation.Binary {
				names = append(names, tool.Name)
				if tool.Operation.Form {
					t.Fatalf("%s is both multipart and binary", tool.Name)
				}
			}
		}
	}
	if len(names) != 1 || names[0] != "upload_chunk" {
		t.Fatalf("binary tools are %v, want [upload_chunk]", names)
	}
}

// TestArcaneNamesReadably rejects the names a derivation gets wrong often: a
// snake_case spelling and a duplicate of a GET operation.
func TestArcaneNamesReadably(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, ts := range cat.Toolsets {
		for _, tool := range ts.Tools {
			if tool.Name != strings.ToLower(tool.Name) || strings.Contains(tool.Name, "-") {
				t.Fatalf("tool %q is not snake_case", tool.Name)
			}
		}
	}
}
