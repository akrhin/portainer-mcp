BINARY := portainer-mcp
ARCANE_BINARY := arcane-mcp

.PHONY: build build-arcane check test catalog spec-arcane

build:
	go build -o $(BINARY) ./cmd/portainer-mcp

build-arcane:
	go build -o $(ARCANE_BINARY) ./cmd/arcane-mcp

check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go test ./...

# Regenerates both catalogs from the API specs and the curated files in api/.
# The output is deterministic: a second run is a no-op.
catalog:
	go run ./tools/cataloggen

# Rebuilds api/arcane-3.1.yaml from an Arcane OpenAPI document. The document is
# not committed: fetch it from a running manager, /api/openapi.json.
spec-arcane:
	go run ./tools/specgen -in arcane-openapi.json -out api/arcane-3.1.yaml
