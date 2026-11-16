# AGENTS.md

Instructions for coding agents working in this repository.

## What this project is

An MCP server for Portainer, written in Go. Module `github.com/akrhin/portainer-mcp`.
It exposes Portainer, Docker, Kubernetes and Helm operations to MCP clients over
stdio or Streamable HTTP, using the official `github.com/modelcontextprotocol/go-sdk`.

The tool surface is data: `internal/catalog/catalog.yaml` is embedded and drives
registration, read-only filtering and validation. A tool that is not in the
catalog cannot be called.

## Commands

```sh
make build            # go build -o portainer-mcp ./cmd/portainer-mcp
make check            # gofmt -l ., go vet ./..., go test ./...
go test ./...         # the suite alone
go build ./...        # compile everything without producing a binary
go test -race ./...   # required before a pull request that touches concurrency
bash scripts/docs-check.sh   # documentation gate
```

`make check` is the gate. Report its real output; a green build you did not run is
a failed task.

## Layout

| path | contents |
|---|---|
| `cmd/portainer-mcp/` | entry point, flag parsing, wiring |
| `internal/catalog/` | catalog types, YAML loading and validation, embedded `catalog.yaml` |
| `internal/config/` | server and instance configuration, `${VAR}` expansion, toolset checks |
| `docs/` | documentation; `docs/history/` is a dated archive |
| `scripts/` | gate scripts |

Handlers live next to the domain they serve. The catalog declares operations; it is
not the place for logic.

## Conventions

- Errors are wrapped with context and `%w`. Never swallow or stringify the cause.
- No interface before a second implementation. No option struct for a single caller.
- `internal/catalog/catalog.yaml` is the single source of tool names, descriptions,
  annotations and schemas. Handlers must not restate them.
- Every tool declares `annotations.readOnlyHint`. Write tools also declare
  `annotations.destructiveHint`. Validation rejects the catalog otherwise.
- Read-only mode filters at registration time. A tool checking `read_only` inside
  its handler is a defect: the client must never see what it cannot call.
- Tests are table-driven. A golden test keeps the catalog and the registered tool
  list in agreement.
- Conventional Commits. One logical change per pull request.

## Documentation rules

**A fact in a page is a claim about the code.** Numbers, flag names, environment
variables, defaults and tool names are copied from source in the same change that
alters them, never from memory. If a value cannot be found in the code, it does not
go into a document — ask instead of guessing.

**Language policy for this repository.** English is canonical.

- Russian mirrors exist for `CONTRIBUTING.md`, `SECURITY.md` and every page under
  `docs/` except `docs/history/`, as a sibling file with the `.ru.md` suffix
  (`SECURITY.ru.md`, `docs/index.ru.md`). Each mirror starts with the line
  `Русский перевод \`<file>\`; английский текст — авторитетный.`
- No mirror for `AGENTS.md` (agent-facing instructions), `CHANGELOG.md` (release
  history), `LICENSE` (legal text fixed by its own wording) and
  `docs/CURRENT-GOAL.md` (a live plan rewritten every stage, not a page for readers). This is a deliberate
  deviation from the default two-directory layout: mirrors are file siblings, not
  `docs/en/` and `docs/ru/` trees.
- Mirrors keep heading structure 1:1. Flags, environment variables, tool names and
  code identifiers are never translated. A lagging mirror is a defect.
- Russian text follows the `ru-text` typography: «ёлочки», spaced em dashes,
  non-breaking space in «11 наборов».

**De-personalization.** The repository is public. No internal hostnames, no IP
addresses, no names of private infrastructure services, no personal home
directories, no sandbox paths. Use neutral wording: "a self-hosted Portainer
instance", "the probed deployment". `docs/forbidden.txt` enforces a subset of this
in the gate.

**Process artifacts are history.** Plans, audits, research notes and proposals go to
`docs/history/` with a status and a date in the header, not next to product pages.

## Before you claim done

Run `make check` and `bash scripts/docs-check.sh`, and paste their output. If the
gate fails, fix the cause instead of disabling the check.
