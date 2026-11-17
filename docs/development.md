[Documentation index](index.md) · [Русский](development.ru.md)

# Development

Building, testing and shipping this repository. Contribution rules and the review
checklist are in [CONTRIBUTING.md](../CONTRIBUTING.md); this page is the mechanics.

## Requirements

- Go 1.27 or newer. `go.mod` pins `go 1.27.4`, and CI reads the version from that
  file rather than from a matrix entry.
- A Portainer instance for the smoke run, and an Arcane manager for the Arcane one.
  Unit tests use `httptest` and need no network.
- `docker` only to build the image.
- `npx` only for the Markdown lint that CI runs.

## Targets

| Command | What it does |
|---|---|
| `make build` | `go build -o portainer-mcp ./cmd/portainer-mcp` |
| `make build-arcane` | `go build -o arcane-mcp ./cmd/arcane-mcp` |
| `make check` | `gofmt -l .`, then `go vet ./...`, then `go test ./...` |
| `make catalog` | `go run ./tools/cataloggen`, rewriting both `generated.yaml` files |
| `make spec-arcane` | `go run ./tools/specgen -in arcane-openapi.json -out api/arcane-3.1.yaml`; needs that document saved in the repository root |
| `go test ./...` | The suite alone |
| `go build ./...` | Compile every package without producing a binary |
| `go test -race ./...` | The suite under the race detector |
| `go test ./internal/catalog/` | One package |
| `bash scripts/docs-check.sh` | Documentation gate |

`make check` is the gate: it fails on an unformatted file, a vet finding or a failing
test. There is no default `make test` target; call `go test` directly.

## Tests

The suite is 69 test functions in six packages. All of them are table-driven and none
of them touches the network.

| Package | What it covers |
|---|---|
| `internal/arcane` | The client sends `X-API-Key`, and its errors carry the `arcane` label |
| `internal/catalog` | The merged catalog validates; parse errors for version, duplicate names, missing hints, non-object schemas; the merge refuses a set in only one document; schema descriptions are strings; `CheckToolsets` accepts and rejects the right combinations |
| `internal/config` | `${VAR}` expansion for string fields and for `tls_verify`; unknown toolset; dependency not enabled; missing API key; no instances |
| `internal/mcpserver` | `tools/list` golden; exhaustive registration exposes every tool of the sets; read-only drops write tools; a call through MCP reaches path, query and body, on both catalogs; streaming refusal; `managedFields` stripping; truncated-array paging; the answer budget; multipart bodies; flat body fields; `container_exec` opt-in and its websocket output, cut and timeout; unknown toolset fails |
| `internal/portainer` | The `X-API-KEY` header is sent; a 401 becomes the documented `APIError` text; endpoint listing decodes; `Do` sends method, path, query and body; truncation, non-JSON answers, path expansion, query encoding and field stripping |
| `tools/cataloggen` | Both committed `generated.yaml` files match a fresh run byte for byte; one tool per operation plus the raw extras; the Arcane counts, name forms, bounded streams and binary bodies; response flags; tool naming and camelCase splitting |

The catalog and the registered tool list are held together by a golden test:
`internal/mcpserver/testdata/tools_core_docker.golden.json` is the expected
`tools/list` for the default `core` and `docker` sets — 20 tools. Any catalog change
that alters the surface fails the suite. Regenerate it deliberately and read the
diff:

```sh
go test ./internal/mcpserver -run TestToolsListGolden -update
```

The generator has its own reproducibility gate. `TestBuildMatchesCommittedFile` runs
`build()` and compares the bytes with the committed file; a catalog edit that is not
followed by `make catalog` fails it. The full check is still `make check`.

`go test -race ./...` is required before a pull request that touches concurrency, the
HTTP transport or shared state; CI runs it on every push and pull request.

## Smoke run

`scripts/smoke.sh` drives the built binary over stdio against a real Portainer. It
calls the curated `core` tools and one read probe from every set named in
`SMOKE_TOOLSETS`. This is the check the unit suite cannot make: real credentials, real
responses, real error text.

```sh
PORTAINER_API_KEY=... SMOKE_ENV_ID=1 SMOKE_TOOLSETS=core,docker \
  bash scripts/smoke.sh config.local.yaml
```

- The argument is the config path; it defaults to `config.local.yaml`.
- `SMOKE_TOOLSETS` defaults to `core` and must be a comma-separated list the config
  also enables. Each extra set adds one read probe: `container_list`, `node_list`,
  `kubernetes_namespaces_list`, `helm_list`, `stack_list`, `gitops_sources_list`,
  `edge_group_list`, `user_list` or `status_inspect`.
- `$PORTAINER_API_KEY` is the variable the example config expands; any variable name
  works as long as the config references it.
- `$SMOKE_ENV_ID` is the environment id passed to `environment_inspect` and to the
  probes, default `1`.
- `SMOKE_STREAM=1` adds `system_events` and expects the call to be refused as
  unbounded, not to hang. Without it the script fails if a streaming tool is in the
  probe list.
- The script builds into a temporary directory, sends `initialize` with protocol
  version `2026-07-28`, `notifications/initialized`, `tools/list` and the calls, and
  checks the reply count against the ids it sent. A `core` failure or a 4xx from a
  probe fails the run; a 5xx from a probe is reported as a warning.

It exits 0 with `OK: core tools answered and every enabled set answered one read
probe`; anything else is a failure to read, not a warning.

`scripts/smoke-arcane.sh` is the same shape for the second binary, and it needs no
config file:

```sh
ARCANE_URL=http://arcane:3552 ARCANE_API_KEY=... bash scripts/smoke-arcane.sh
```

- `ARCANE_URL` and `ARCANE_API_KEY` are both required; the script exits 2 without them.
- `SMOKE_TOOLSETS` defaults to `core,containers,projects,images,volumes,networks`, the
  sets the fixed probe list touches.
- `SMOKE_AGENT_ID` defaults to `0` and is passed to every probe as `id`.
- It builds `arcane-mcp` into a temporary directory, sends `initialize` with protocol
  version `2026-07-28`, `notifications/initialized`, `tools/list` and eight read calls:
  `environments_list`, `environment_inspect`, `containers_list`,
  `container_status_count`, `projects_list`, `image_update_summary`, `volumes_list`,
  `networks_list`. It fails on any reply that carries `isError: true`, and exits 0
  with `OK: every probe answered`.
- The probe list is fixed, so a narrowed `SMOKE_TOOLSETS` must still cover every set it
  reads from: `volumes_list` lives in `volumes` and `networks_list` in `networks`.

## Documentation gate

```sh
bash scripts/docs-check.sh
```

Five checks run over every Markdown file outside `.git`, `.tmp` and `vendor`:

| Check | What it enforces |
|---|---|
| Links | Every relative `*.md` link resolves to an existing file |
| Forbidden paths | No content from `docs/forbidden.txt`: private address ranges, personal home directories, sandbox paths, internal service names. `docs/history/` is exempt |
| Orphans | Every page under `docs/` is linked from another page (`docs/index.md` and archives are exempt) |
| Language parity | Every `.md` has a `.ru.md` sibling and vice versa, except `AGENTS.md`, `CHANGELOG.md`, `docs/CURRENT-GOAL.md`, `.github/**` and `docs/history/**` |
| Anchors | Facts in `docs/anchors.txt` still match: the catalog version, module path, SDK version, Go directive, flag names, and the values quoted on the index page |

Anchors are one line each, `mode|regex|path`, where the mode is `must` or `mustnot`.
They exist so a value quoted in prose fails the gate when the code moves, instead of
drifting silently.

Markdown lint runs separately, with its rules in `.markdownlint-cli2.yaml`:

```sh
npx --yes markdownlint-cli2@0.24.1
```

The config disables line-length, inline HTML, bare-URL and first-line-heading rules,
and excludes `docs/history/**` from the glob.

## Continuous integration

| Workflow | Trigger | Jobs |
|---|---|---|
| `ci.yml` | push to `main`, pull request | `gofmt -l .`, `go vet`, `go test -race`, `go build`; `golangci-lint` v2.15.0 with `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused, misspell, revive) |
| `docs.yml` | push to `main`, pull request | `scripts/docs-check.sh`, `markdownlint-cli2@0.24.1` |
| `security.yml` | push to `main`, pull request, weekly | gosec v2.30.0 to SARIF, govulncheck v1.9.0, gitleaks over the full history, dependency review on pull requests, CodeQL `security-extended` |
| `release.yml` | tag `v*` | goreleaser v2, Docker buildx, `ghcr.io` login |

The docs jobs are conditional on the files existing, so a checkout without
`scripts/docs-check.sh` or `docs/**/*.md` does not fail CI.

## Release

A `v*` tag runs `.goreleaser.yaml` with `release --clean`. The artifacts are:

- Archives for `linux`, `darwin` and `windows` on `amd64` and `arm64`, `tar.gz` and
  `zip` for Windows, named `portainer-mcp_<version>_<os>_<arch>`. Each archive holds
  both binaries, `portainer-mcp` and `arcane-mcp`.
- `checksums.txt`, SHA-256.
- SBOMs for every archive, generated with syft (the workflow installs it).
- A multi-arch container image on `ghcr.io/akrhin/portainer-mcp`, one image per
  architecture plus manifests for `:<version>` and `:latest`. The image runs the
  Portainer server; there is no image for `arcane-mcp` yet.

The release build fills `--version` through
`-X main.version={{ .Version }} -X main.commit={{ .Commit }} -X main.date={{ .Date }}`
in `.goreleaser.yaml`; a local `go build` prints `dev (none, unknown)`.

The release notes are generated from Conventional Commit subjects and grouped into
features, bug fixes, performance, refactoring and documentation; `chore`, `test`,
`ci`, `style`, `build` and merge commits are filtered out. This is why commit
prefixes matter beyond tidiness.

`v0.4.2` is the current tag, so the release archives and the
`ghcr.io/akrhin/portainer-mcp` image exist; build a local image with
`docker build -t portainer-mcp .`.

The image is built from `golang:1.27-alpine` and runs on
`gcr.io/distroless/static:nonroot`. It has no `HEALTHCHECK`: over stdio the process
is silent until a message arrives, so a liveness probe would have nothing to read.

## Changing the tool surface

The surface is data. Do not edit `generated.yaml` — either of them — and do not add a
handler: add or move the operation in `api/toolsets.yaml` (the tag map, an
`operation_toolsets` override, or an `extras` entry), choose whether it joins the
default set in `api/default-tools.yaml`, then run:

```sh
make catalog
make catalog && git diff --exit-code internal/catalog/generated.yaml
```

For the Arcane catalog the same applies to `api/arcane-toolsets.yaml` and
`api/arcane-default-tools.yaml`; to rebuild its converted specification from a fresh
Arcane OpenAPI document, save it as `arcane-openapi.json` and run `make spec-arcane`
first. `go run ./tools/cataloggen -profile arcane` builds that catalog alone.

The second command is the determinism check: a second run must change nothing. Then
run the gates:

```sh
make check
go test ./tools/cataloggen
```

What the gates check afterwards:

- The merged catalog validates at startup, including the annotation rules.
- `generated.yaml` matches a fresh run and the golden `tools/list` matches, or both
  were regenerated in the same change.
- A read-only or exhaustive case exists for the new tool, because both switches what
  is registered.
- The CHANGELOG entry and the configuration or toolset documentation are updated in
  the same pull request.

There is no "implemented" map to update: a tool in the catalog is registered and
executed by the shared handler, and a tool outside it cannot be called.
