[Documentation index](index.md) · [Русский](development.ru.md)

# Development

Building, testing and shipping this repository. Contribution rules and the review
checklist are in [CONTRIBUTING.md](../CONTRIBUTING.md); this page is the mechanics.

## Requirements

- Go 1.27 or newer. `go.mod` pins `go 1.27.4`, and CI reads the version from that
  file rather than from a matrix entry.
- A Portainer instance only for the smoke run. Unit tests use `httptest` and need no
  network.
- `docker` only to build the image.
- `npx` only for the Markdown lint that CI runs.

## Targets

| Command | What it does |
|---|---|
| `make build` | `go build -o portainer-mcp ./cmd/portainer-mcp` |
| `make check` | `gofmt -l .`, then `go vet ./...`, then `go test ./...` |
| `go test ./...` | The suite alone |
| `go build ./...` | Compile every package without producing a binary |
| `go test -race ./...` | The suite under the race detector |
| `go test ./internal/catalog/` | One package |
| `bash scripts/docs-check.sh` | Documentation gate |

`make check` is the gate: it fails on an unformatted file, a vet finding or a failing
test. There is no default `make test` target; call `go test` directly.

## Tests

The suite is 24 tests in four packages. All of them are table-driven and none of them
touches the network.

| Package | What it covers |
|---|---|
| `internal/catalog` | The embedded catalog validates; parse errors for version, duplicate names, missing hints, non-object schemas; schema descriptions are strings; `CheckToolsets` accepts and rejects the right combinations |
| `internal/config` | `${VAR}` expansion for string fields and for `tls_verify`; unknown toolset; dependency not enabled; missing API key; no instances |
| `internal/mcpserver` | `tools/list` golden; read-only mode drops write tools; unknown toolset fails; unimplemented toolset fails; `--exhaustive` fails |
| `internal/portainer` | The `X-API-KEY` header is sent; a 401 becomes the documented `APIError` text; endpoint listing decodes |

The catalog and the registered tool list are held together by a golden test:
`internal/mcpserver/testdata/tools_core.golden.json` is the expected `tools/list`
response. Any catalog edit that changes the surface fails the suite. Regenerate it
deliberately and read the diff:

```sh
go test ./internal/mcpserver -run TestToolsListGolden -update
```

`go test -race ./...` is required before a pull request that touches concurrency, the
HTTP transport or shared state; CI runs it on every push and pull request.

## Smoke run

`scripts/smoke.sh` drives the built binary over stdio against a real Portainer and
calls every `core` tool. It is the check the unit suite cannot make: real credentials,
real responses, real error text.

```sh
PORTAINER_API_KEY=... SMOKE_ENV_ID=1 bash scripts/smoke.sh config.local.yaml
```

- The argument is the config path; it defaults to `config.local.yaml`.
- The config must select `core`. Read-only mode does not matter for this run: all
  seven `core` tools are annotated read-only, so they survive the filter either way.
- `$PORTAINER_API_KEY` is the variable the example config expands; any variable name
  works as long as the config references it.
- `$SMOKE_ENV_ID` is the environment id passed to `environment_inspect`, default `1`.
- The script builds into a temporary directory, sends `initialize` with protocol
  version `2026-07-28`, `notifications/initialized`, `tools/list` and the seven calls,
  and expects nine replies. It fails if any reply carries `isError: true`, printing
  the offending text.

It exits 0 with `OK: server served all core tools`; anything else is a failure to
read, not a warning.

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
| Language parity | Every `.md` has a `.ru.md` sibling and vice versa, except `AGENTS.md`, `CHANGELOG.md`, `.github/**` and `docs/history/**` |
| Anchors | Facts in `docs/anchors.txt` still match: the catalog version, module path, SDK version, Go directive, flag names, and three values quoted on the index page |

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
| `security.yml` | push to `main`, pull request, weekly | gosec v2.22.4 to SARIF, govulncheck v1.1.4, gitleaks over the full history, dependency review on pull requests, CodeQL `security-extended` |
| `release.yml` | tag `v*` | goreleaser v2, Docker buildx, `ghcr.io` login |

The docs jobs are conditional on the files existing, so a checkout without
`scripts/docs-check.sh` or `docs/**/*.md` does not fail CI.

## Release

A `v*` tag runs `.goreleaser.yaml` with `release --clean`. The artifacts are:

- Archives for `linux`, `darwin` and `windows` on `amd64` and `arm64`, `tar.gz` and
  `zip` for Windows, named `portainer-mcp_<version>_<os>_<arch>`.
- `checksums.txt`, SHA-256.
- SBOMs for every archive, generated with syft (the workflow installs it).
- A multi-arch container image on `ghcr.io/akrhin/portainer-mcp`, one image per
  architecture plus manifests for `:<version>` and `:latest`.

The release notes are generated from Conventional Commit subjects and grouped into
features, bug fixes, performance, refactoring and documentation; `chore`, `test`,
`ci`, `style`, `build` and merge commits are filtered out. This is why commit
prefixes matter beyond tidiness.

No release has been tagged yet, so neither binaries nor images exist. To build the
image locally:

```sh
docker build -t portainer-mcp .
```

The image is built from `golang:1.27-alpine` and runs on
`gcr.io/distroless/static:nonroot`. It has no `HEALTHCHECK`: over stdio the process
is silent until a message arrives, so a liveness probe would have nothing to read.

## Changing the tool surface

The procedure is in [CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-tool). What the gate
checks afterwards:

- The catalog still validates at startup, including the annotation rules.
- The golden file matches, or is regenerated in the same change.
- A read-only or exhaustive case exists for the new tool, because both switch what is
  registered.
- The CHANGELOG entry and the configuration or toolset documentation are updated in
  the same pull request.

The startup error for an unimplemented toolset comes from the `implemented` map in
`internal/mcpserver/server.go`. Adding the handlers and the name to that map removes
the error; no other switch needs flipping.
