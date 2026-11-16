# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - unreleased

### Added

- A second binary, `arcane-mcp`, serves [Arcane](https://getarcane.app) managers from
  the same repository: `--url` and an `X-API-Key` from `--api-key`, a catalog of 399
  tools in thirteen toolsets with 53 registered by default, and the same handler as
  the Portainer server.
- `tools/specgen` converts the Arcane OpenAPI 3.1 document to `api/arcane-3.1.yaml`
  deterministically (`make spec-arcane`), and `tools/cataloggen -profile arcane`
  builds `internal/catalog/arcane/generated.yaml`.
- `make build-arcane`, a second GoReleaser build and `scripts/smoke-arcane.sh`, which
  drives the Arcane server over stdio against a live manager.
- `container_exec` runs one shell line inside an Arcane container and returns what it
  printed. Arcane publishes no exec operation — the terminal is a WebSocket route — so the
  tool is registered beside the catalog and sits behind `--allow-exec` (or
  `ARCANE_ALLOW_EXEC=true`); `--read-only` never registers it. The command goes to the
  container's shell unquoted, capped at 4 KiB, the answer at 64 KiB, and the session is
  cut at `timeout_sec` (default 30, at most 300). Arcane reports no exit status, so the
  tool types a `printf '__mcp_exit:%d__'` marker line into the same shell after the command
  and returns `exit: N`; a command that ends the shell itself returns no status. Every call
  is audited on standard error.

## [0.2.0] - 2026-11-16

### Added

- Multipart operations (endpoint creation, file-based stack creation, TLS
  certificate upload and the other five) now work: the generator marks them
  `form: true`, and a `body` value shaped `{filename, content}` or
  `{filename, base64}` is sent as a file part.
- `container_exec` runs the command it creates and returns what it printed,
  demultiplexing stdout and stderr when the exec has no TTY.

### Fixed

- The `until` bound of a streaming operation accepts epoch seconds, not only a
  timestamp string, so the refusal the tool prints can actually be acted on.
- `k8s_raw` appends its path to `/api/endpoints/{environment}/kubernetes`, the
  route Portainer answers on.
- Raw tools send their `body` and pass `metadata.managedFields` through unchanged
  (116 tools strip it).
- `container_exec` expands the start path from the id it just created; a second
  call used to start the first exec again and answer `exec command … has already
  run`.

[0.2.0]: https://github.com/akrhin/portainer-mcp/releases/tag/v0.2.0

## [0.1.0] - 2026-11-16

### Added

- Go module `github.com/akrhin/portainer-mcp` on Go 1.27, depending on
  `github.com/modelcontextprotocol/go-sdk` v1.8.0 and `gopkg.in/yaml.v3`.
- Declarative tool catalog embedded into the binary as two documents:
  `internal/catalog/catalog.yaml` keeps what a person curates — eleven toolsets
  (`core`, `docker`, `swarm`, `k8s`, `helm`, `stacks`, `gitops`, `edge`, `admin`,
  `observability`, `raw`) with their descriptions and `depends_on` edges — and
  `internal/catalog/generated.yaml` keeps every tool, generated from the Portainer
  2.45 and Docker Engine 1.57 specifications: 394 tools, 87 of them in the default
  set.
- Streaming detection driven by the catalog: `streaming_params` and
  `streaming_paths` in `api/toolsets.yaml` mark the seven operations that do not
  answer on their own — four with a `follow` parameter, three whose path streams
  whatever the arguments are. A call that would stay open until the client times
  out is refused before the request, with the reason and the way to bound it.
- `strip_managed_fields` in the catalog: the tools whose path lies under the
  Kubernetes API (116 of them) drop `metadata.managedFields` at every depth, so a
  Kubernetes list no longer arrives several times larger than it is.
- An array answer cut at 1 MiB arrives as a page — the first 50 elements plus
  `_truncated`, `_total` and a `_hint` naming the tool to narrow the list with —
  instead of a broken prefix. Answers that are not arrays keep the marker.
- `scripts/smoke.sh` probes `system_events` only under `SMOKE_STREAM=1`, and then
  checks that it was refused rather than left hanging; without the variable it
  fails the run when a streaming tool is in the probe list.
- Catalog validation on startup: unique toolset and tool names, `readOnlyHint`
  required on every tool, `destructiveHint` required on write tools, `input` and
  `output` schemas constrained to `type: object`, dependencies must name known
  toolsets.
- Toolset selection check: an unknown toolset is a startup error that lists the
  known names, and a toolset whose dependency is not enabled is refused.
- YAML configuration for multiple Portainer instances: `name`, `url` (absolute
  `http`/`https`), `api_key`, `tls_verify`, `ca_file`, with `${VAR}` expansion for
  string fields and a boolean-or-`${VAR}` form for `tls_verify`.
- Server switches in configuration: `server.transport` (`stdio` by default,
  `http` requires `server.listen`), `server.read_only`, `server.exhaustive`.
- Entry point `cmd/portainer-mcp` with `--config`, `--toolsets`, `--read-only` and
  `--exhaustive`; stdio transport by default, Streamable HTTP at `/mcp` when
  `server.transport` is `http`.
- Catalog generator `tools/cataloggen` (`make catalog`): reads both specifications
  and the two curated files in `api/`, and writes `internal/catalog/generated.yaml`.
  The run is deterministic — the same inputs produce the same bytes — and
  `tools/cataloggen/cataloggen_test.go` fails when the committed file drifts.
- `api/toolsets.yaml`: tag-to-toolset map covering all 31 Portainer and 15 Docker
  tags, name rules for `<entity>_<verb>` tool names with an explicit rewrite and
  unnamed-operation list, parameter renames, and the three raw extras.
- `api/default-tools.yaml`: the 87 operations registered when `--exhaustive` is off.
- One universal executor, `(*portainer.Client).Do`, instead of a generated client:
  path parameters are escaped and substituted, query values are encoded (arrays as
  repeated keys or comma-joined), the body is passed through, non-2xx replies
  become the existing `APIError`, and an answer above 1 MiB is truncated with a
  marker in the text.
- Registration is data: tool input schema, annotations and HTTP operation all come
  from the catalog, so `--exhaustive` now registers every operation of the enabled
  toolsets instead of refusing to start, and `--read-only` filters at registration
  time as before. `internal/mcpserver/core.go` is gone; `core` is data too.
- `Makefile` with `build`, `check` and `catalog` targets.
- `scripts/smoke.sh` accepts `SMOKE_TOOLSETS` and calls one read tool from every
  enabled set on top of the curated `core` calls.
- Documentation skeleton: `docs/index.md` with a Russian mirror, an archived market
  research note in `docs/history/`, and `scripts/docs-check.sh` as the docs gate.
- `LICENSE` (MIT), `SECURITY.md` and `CONTRIBUTING.md`, each with a Russian mirror.

### Notes

- Docker operations go through the Portainer proxy
  (`/api/endpoints/{environment}/docker<path>`) with `environment` as a required
  argument; the server never opens a Docker socket itself. The docker-engine
  version prefix in the specification is deliberately not pinned.

[0.1.0]: https://github.com/akrhin/portainer-mcp/releases/tag/v0.1.0
