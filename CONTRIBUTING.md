[Documentation index](docs/index.md) · [Русский](CONTRIBUTING.ru.md)

# Contributing

Thanks for taking the time. This page covers what a change has to include to be merged.

## Requirements

- Go 1.27 or newer (`go.mod` pins `go 1.27.4`).
- A Portainer instance, and an Arcane manager for `arcane-mcp`, if your change touches
  tool behaviour. Integration checks run against a real server; unit tests do not need
  one.
- No other tooling: the build, the checks and the tests are plain `go` commands behind a Makefile.

## Build and gate

```sh
make build        # go build -o portainer-mcp ./cmd/portainer-mcp
make build-arcane # go build -o arcane-mcp ./cmd/arcane-mcp
make check        # gofmt -l ., go vet ./..., go test ./...
```

`make check` is the gate. A pull request that does not pass it is not reviewed.

Before opening a pull request, also run:

```sh
go test -race ./...
bash scripts/docs-check.sh
```

The race detector is required for anything that touches goroutines, the HTTP transport,
streaming output or shared caches. The docs gate checks links, language parity, personal
paths and the facts pinned in `docs/anchors.txt`.

## Workflow

1. Branch from `main`. One branch, one logical change.
2. Commit with a Conventional Commit message: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`. An optional scope helps: `feat(catalog): ...`.
3. Open a pull request against `main` and describe what changes for the user, not which files you edited.

Keep the diff reviewable. A refactor and a behaviour change in one pull request are two pull requests.

## What a pull request must include

- A test for new or changed behaviour. Table-driven tests are the house style; a golden test keeps the catalog and the registered tool list in sync.
- A [CHANGELOG.md](CHANGELOG.md) entry under a `## [Unreleased]` heading at the top of
  the file — create that heading if it is absent — in the section that matches the
  change (`Added`, `Changed`, `Fixed`, `Removed`).
- Documentation updates when behaviour, configuration keys, tool names or defaults change. A fact in a page must match the code in the same pull request.
- No secrets, no personal paths. CI runs gitleaks; `.env`, `*.key` and `config.local.yaml` are ignored by git and must stay out of commits.
- No internal hostnames, IP addresses or personal home directories in code, tests, fixtures or docs. The repository is public.

## Style

Ordinary Go, written the way the standard library is written.

- Wrap errors with context: `fmt.Errorf("inspect container %s: %w", id, err)`. Never drop the original error.
- Do not introduce an interface before there is a second implementation. A concrete type and a function are enough.
- No options struct, no wrapper and no abstraction that only one caller uses.
- Keep the catalog declarative. Tool names, descriptions, annotations and schemas live
  in `api/`, and the generated catalog carries them; code reads them and does not
  duplicate them.
- Comment the reason, not the mechanics.

## Adding a tool

The tool surface is data. Do not edit `internal/catalog/generated.yaml` and do not
write a handler: one shared handler executes every operation.

1. Add or move the operation in `api/toolsets.yaml` — the tag map, an
   `operation_toolsets` override, a name rewrite, or an `extras` entry. The Arcane
   profile has the same file at `api/arcane-toolsets.yaml`.
2. Decide whether it joins the default set in `api/default-tools.yaml` (or
   `api/arcane-default-tools.yaml`).
3. Run `make catalog`, then `make catalog && git diff --exit-code internal/catalog/generated.yaml`
   — the second run must change nothing.
4. Add the CHANGELOG entry and update the toolset or configuration page in the same
   pull request.

Gates afterwards: `make check`, `go test ./tools/cataloggen`, `bash scripts/docs-check.sh`.
`ReadOnly` and `Exhaustive` both change which tools are registered, so a new tool needs
a case in the registration test.

Toolset names, descriptions and `depends_on` live in `internal/catalog/catalog.yaml`
(`internal/catalog/arcane/catalog.yaml` for Arcane). That file holds no tool lists.

## License

The project is MIT-licensed. By contributing you agree that your contribution is licensed under the same terms.
