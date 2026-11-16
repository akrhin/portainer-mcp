# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

No release has been cut yet. Everything below is on `main` and not yet tagged.

## [Unreleased]

### Added

- Go module `github.com/akrhin/portainer-mcp` on Go 1.27, depending on
  `github.com/modelcontextprotocol/go-sdk` v1.8.0 and `gopkg.in/yaml.v3`.
- Declarative tool catalog in `internal/catalog/catalog.yaml`, embedded into the
  binary: eleven toolsets (`core`, `docker`, `swarm`, `k8s`, `helm`, `stacks`,
  `gitops`, `edge`, `admin`, `observability`, `raw`) with explicit `depends_on`
  edges, and 113 operations declared across them.
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
- Registration from the catalog: enabled toolsets are checked with their
  dependencies, read-only mode skips every tool that is not annotated
  `readOnlyHint: true`, and a tool with no handler fails startup instead of being
  registered dead.
- Seven read-only `core` tools implemented: `system_version`, `system_status`,
  `system_info`, `settings_get`, `whoami`, `environment_list`,
  `environment_inspect`.
- `Makefile` with `build` and `check` targets.
- Documentation skeleton: `docs/index.md` with a Russian mirror, an archived market
  research note in `docs/history/`, and `scripts/docs-check.sh` as the docs gate.
- `LICENSE` (MIT), `SECURITY.md` and `CONTRIBUTING.md`, each with a Russian mirror.

### Notes

- Stage 0 implements the `core` handlers only. Every other toolset is declared but
  selecting it fails at startup until its stage lands.
