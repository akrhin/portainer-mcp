// Command specgen converts an OpenAPI 3.1 document into the YAML form
// tools/cataloggen reads.
//
// A 3.1 document uses two constructs the generator does not understand: a JSON
// Schema type written as an array ("type": ["string", "null"]) and the $schema
// keyword. This command rewrites the first to a single type and drops the
// second, leaving every $ref, allOf and description untouched.
//
//	go run ./tools/specgen -in spec.json -out api/arcane-3.1.yaml
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	in := flag.String("in", "", "OpenAPI 3.1 JSON document")
	out := flag.String("out", "", "YAML file to write")
	flag.Parse()
	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "specgen: -in and -out are required")
		os.Exit(2)
	}
	if err := run(*in, *out); err != nil {
		fmt.Fprintln(os.Stderr, "specgen:", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	raw, err := os.ReadFile(in) // #nosec G304 -- path is the operator's -in flag
	if err != nil {
		return err
	}
	doc, err := decode(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	root, ok := normalize(doc).(map[string]any)
	if !ok {
		return fmt.Errorf("%s: document is not a JSON object", in)
	}
	// The document names the server it was captured from; the catalog carries
	// the base path instead, so nothing environment-specific is committed.
	root["servers"] = []any{map[string]any{"url": "/api"}}

	body, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	const header = "# Converted from the Arcane 3.0.1 OpenAPI 3.1 document by tools/specgen; DO NOT EDIT.\n" +
		"# Edit the document or the tool, then run: go run ./tools/specgen -in <spec.json> -out api/arcane-3.1.yaml\n"
	// #nosec G306 -- a generated repository file must stay group/world readable.
	return os.WriteFile(out, append([]byte(header), body...), 0o644)
}

// decode reads JSON keeping numbers as they were written: an identifier must
// not become 1.0 on the way through float64.
func decode(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	return convertNumbers(doc), nil
}

// convertNumbers turns json.Number into int64 or float64 so the YAML writer
// emits 3 rather than 3.0.
func convertNumbers(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, sub := range val {
			val[k] = convertNumbers(sub)
		}
		return val
	case []any:
		for i, sub := range val {
			val[i] = convertNumbers(sub)
		}
		return val
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val.String()
	}
	return v
}

// normalize rewrites the 3.1 constructs in place and returns the same tree, so
// a second run over the output changes nothing.
func normalize(v any) any {
	switch val := v.(type) {
	case map[string]any:
		delete(val, "$schema")
		if t, ok := val["type"].([]any); ok {
			val["type"] = singleType(t)
		}
		for k, sub := range val {
			val[k] = normalize(sub)
		}
		return val
	case []any:
		for i, sub := range val {
			val[i] = normalize(sub)
		}
		return val
	}
	return v
}

// singleType keeps the first named type of a 3.1 type array. "null" marks the
// value optional, which the generator already expresses by leaving the
// property out of the required list.
func singleType(types []any) string {
	for _, t := range types {
		if s, ok := t.(string); ok && s != "null" {
			return s
		}
	}
	return "string"
}
