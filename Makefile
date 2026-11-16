BINARY := portainer-mcp

.PHONY: build check test

build:
	go build -o $(BINARY) ./cmd/portainer-mcp

check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go test ./...
