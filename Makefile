BINARY := portainer-mcp

.PHONY: build check test catalog

build:
	go build -o $(BINARY) ./cmd/portainer-mcp

check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go test ./...

# Regenerates internal/catalog/generated.yaml from the API specs and the two
# curated files in api/. The output is deterministic: a second run is a no-op.
catalog:
	go run ./tools/cataloggen
