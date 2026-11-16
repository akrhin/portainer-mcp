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
// operation that survives the skip list, plus the raw extra, and no name twice.
func TestArcaneCounts(t *testing.T) {
	t.Chdir("../..")

	cat, err := buildProfile(arcaneProfile)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(cat.Toolsets) != 13 {
		t.Fatalf("%d toolsets, want 13", len(cat.Toolsets))
	}
	tools, defaults := 0, 0
	seen := map[string]bool{}
	var paths []string
	for _, ts := range cat.Toolsets {
		if len(ts.Tools) == 0 {
			t.Fatalf("toolset %q is empty", ts.Name)
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
				t.Fatalf("HEAD tool %q registered", tool.Name)
			}
			paths = append(paths, tool.Operation.Path)
		}
	}
	if tools != 399 {
		t.Fatalf("%d tools, want 399", tools)
	}
	if defaults != 53 {
		t.Fatalf("%d default tools, want 53", defaults)
	}
	for _, p := range paths {
		for _, skip := range []string{"/oidc", "/passkeys", "/mfa", "/apns", "/stream", "/events/changes", "/upgrade/progress"} {
			if strings.Contains(p, skip) {
				t.Fatalf("path %q belongs to an excluded topic", p)
			}
		}
	}
}
