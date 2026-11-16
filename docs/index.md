[Русский](index.ru.md) · [README](../README.md) · [Contributing](../CONTRIBUTING.md) · [Security](../SECURITY.md) · [Changelog](../CHANGELOG.md)

# Portainer MCP documentation

An MCP server that exposes Portainer, Docker, Kubernetes and Helm to MCP clients. It
is written in Go against the official `github.com/modelcontextprotocol/go-sdk`
(v1.8.0), which speaks the MCP revision `2026-07-28`, and runs over stdio or
Streamable HTTP at `/mcp`.

The catalog is complete and executable: 394 tools in eleven toolsets, 87 of them
registered by default. Every tool is generated from the Portainer 2.45 and Docker
Engine 1.57 specifications and executed by one shared handler, so enabling a toolset
no longer stops startup.

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

The tool surface is declarative. `api/toolsets.yaml` and `api/default-tools.yaml` feed
`tools/cataloggen`, which writes `internal/catalog/generated.yaml`; that file and the
curated `internal/catalog/catalog.yaml` are embedded in the binary. The curated part
holds descriptions and dependencies; the generated part holds every tool. A tool that
is not in the generated catalog cannot be called.

Eleven toolsets and 394 tools are declared today; 87 register by default, and
`--exhaustive` registers every operation of the enabled sets. The catalog is validated
at startup: duplicate names, a tool without a toolset, a missing `readOnlyHint`, a
write tool without `destructiveHint` or `idempotentHint`, or a dependency on an
unknown toolset all fail before the server serves anything.
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
| Regenerate the tool catalog | `make catalog` |
| Format, vet and test | `make check` |
| Race detector | `go test -race ./...` |
| Live stdio run against a real instance | `bash scripts/smoke.sh config.local.yaml` |
| Documentation gate | `bash scripts/docs-check.sh` |

Package layout: `cmd/portainer-mcp` for the entry point, `internal/catalog` for the
embedded catalog and its validation, `internal/config` for the YAML configuration,
`internal/mcpserver` for registration and the shared handler, `internal/portainer` for
the HTTP executor. `api/` holds the OpenAPI documents and the curated generator
inputs, and `tools/cataloggen` is the generator. The details are in
[Development](development.md) and [Architecture](architecture.md).

## History

- [Current stage goal](CURRENT-GOAL.md) — the current stage plan, what it gates on and
  where it stops. Russian only.
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
