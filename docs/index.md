[Русский](index.ru.md) · [README](../README.md) · [Contributing](../CONTRIBUTING.md) · [Security](../SECURITY.md) · [Changelog](../CHANGELOG.md)

# Portainer MCP documentation

An MCP server that exposes Portainer, Docker, Kubernetes and Helm to MCP clients. It
is written in Go against the official `github.com/modelcontextprotocol/go-sdk`
(v1.8.0), which speaks the MCP revision `2026-07-28`, and runs over stdio or
Streamable HTTP at `/mcp`.

The project is at stage 0: the catalog is complete and validated, the `core`
read-only tools are implemented, and every other toolset is declared but not yet
implemented. Selecting one is a startup error with `not implemented yet`, so a client
never sees a tool that cannot answer.

## Pages

| Page | Answers |
|---|---|
| [README](../README.md) | Install, first run, flags, client setup |
| [Configuration](configuration.md) | Every YAML key, default, `${VAR}` rule, flag precedence and startup error |
| [Toolsets](toolsets.md) | The eleven toolsets, dependencies, status, and the tools inside each |
| [Architecture](architecture.md) | Packages, startup order, request flow, Portainer client, transports |
| [Development](development.md) | Tests, golden files, smoke run, docs gate, CI, release |

## Product

One process serves one or more configured Portainer instances. Every tool takes an
optional `instance` argument and defaults to the first configured instance.

The tool surface is declarative: `internal/catalog/catalog.yaml` is embedded in the
binary and drives registration, read-only filtering and startup validation. A tool
that is not in the catalog cannot be called, and a handler that has no catalog entry
is never registered.

Eleven toolsets and 113 operations are declared today; `core` is the only one with
handlers. The catalog is validated at startup: duplicate names, a tool without a
toolset, a missing `readOnlyHint`, a write tool without `destructiveHint`, or a
dependency on an unknown toolset all fail before the server serves anything.
[Toolsets](toolsets.md) has the full list.

## Configuration in brief

A YAML file with three top-level keys — `server`, `instances`, `toolsets` — plus
flags that override where they overlap. String fields accept a whole-value `${VAR}`
reference, and an unset variable is a startup error rather than an empty value.

The HTTP transport serves Streamable HTTP at `/mcp` and has no default address; the
example configuration binds `127.0.0.1:17717`. The full reference, including every
validation message, is in [Configuration](configuration.md).

## Development in brief

| Need | Command |
|---|---|
| Build the binary | `make build` |
| Format, vet and test | `make check` |
| Race detector | `go test -race ./...` |
| Live stdio run against a real instance | `bash scripts/smoke.sh config.local.yaml` |
| Documentation gate | `bash scripts/docs-check.sh` |

Package layout: `cmd/portainer-mcp` for the entry point, `internal/catalog` for the
embedded catalog and its validation, `internal/config` for the YAML configuration,
`internal/mcpserver` for registration and handlers, `internal/portainer` for the HTTP
client. Handlers sit next to the domain they serve. The details are in
[Development](development.md) and [Architecture](architecture.md).

## History

- [Current stage goal](CURRENT-GOAL.md) — the stage-0 plan, what it gates on and where
  it stops. Russian only.
- [Market research, 2026-11-16](history/2026-11-16-market-research.ru.md) — the state
  of Portainer MCP servers, the vendor deprecation, and the decisions this project
  took from it. Russian only, by policy.

## See also

- [Contributing](../CONTRIBUTING.md) — build gate, commit style, adding a tool.
- [Security policy](../SECURITY.md) — private reporting and what counts as a
  vulnerability in an MCP server.
- [Changelog](../CHANGELOG.md) — what has landed so far.
- [AGENTS.md](../AGENTS.md) — instructions for coding agents and the documentation
  policy.
