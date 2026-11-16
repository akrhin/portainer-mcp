[Documentation index](docs/index.md) · [Русский](CONTRIBUTING.ru.md)

# Contributing

Thanks for taking the time. This page covers what a change has to include to be merged.

## Requirements

- Go 1.27 or newer (`go.mod` pins `go 1.27.4`).
- A Portainer instance if your change touches tool behaviour. Integration checks run against a real server; unit tests do not need one.
- No other tooling: the build, the checks and the tests are plain `go` commands behind a Makefile.

## Build and gate

```sh
make build   # go build -o portainer-mcp ./cmd/portainer-mcp
make check   # gofmt -l ., go vet ./..., go test ./...
```

`make check` is the gate. A pull request that does not pass it is not reviewed.

Before opening a pull request, also run:

```sh
go test -race ./...
```

The race detector is required for anything that touches goroutines, the HTTP transport, streaming output or shared caches.

## Workflow

1. Branch from `main`. One branch, one logical change.
2. Commit with a Conventional Commit message: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`. An optional scope helps: `feat(catalog): ...`.
3. Open a pull request against `main` and describe what changes for the user, not which files you edited.

Keep the diff reviewable. A refactor and a behaviour change in one pull request are two pull requests.

## What a pull request must include

- A test for new or changed behaviour. Table-driven tests are the house style; a golden test keeps the catalog and the registered tool list in sync.
- A [CHANGELOG.md](CHANGELOG.md) entry under `## [Unreleased]`, in the section that matches the change (`Added`, `Changed`, `Fixed`, `Removed`).
- Documentation updates when behaviour, configuration keys, tool names or defaults change. A fact in a page must match the code in the same pull request.
- No secrets, no personal paths. CI runs gitleaks; `.env`, `*.key` and `config.local.yaml` are ignored by git and must stay out of commits.
- No internal hostnames, IP addresses or personal home directories in code, tests, fixtures or docs. The repository is public.

## Style

Ordinary Go, written the way the standard library is written.

- Wrap errors with context: `fmt.Errorf("inspect container %s: %w", id, err)`. Never drop the original error.
- Do not introduce an interface before there is a second implementation. A concrete type and a function are enough.
- No options struct, no wrapper and no abstraction that only one caller uses.
- Keep the catalog declarative. Tool names, descriptions, annotations and schemas live in `internal/catalog/catalog.yaml`; handlers read from it and do not duplicate it.
- Comment the reason, not the mechanics.

## Adding a tool

1. Add the tool to the owning toolset in `internal/catalog/catalog.yaml`:
   - `name` in `snake_case`, prefixed with its domain (`docker_container_list`, not `list`).
   - `description` says when to use the tool, not what it wraps.
   - `annotations.readOnlyHint` is required on every tool. A write tool also requires `annotations.destructiveHint`.
   - `input` and `output` are JSON Schema objects with `type: object`. Declare `output` whenever the response has a stable shape.
2. Implement the handler and register it from the catalog entry. Read-only filtering follows from the annotations, so a write tool never has to check `read_only` by hand.
3. Add the handler test. If the tool reaches a real Portainer API, cover the request it builds and record one integration run against a live instance in the pull request.
4. Add the CHANGELOG entry.

`ReadOnly` and `Exhaustive` both change which tools are registered, so a new tool needs a case in the registration test.

## License

The project is MIT-licensed. By contributing you agree that your contribution is licensed under the same terms.
