[![CI](https://github.com/akrhin/portainer-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/akrhin/portainer-mcp/actions/workflows/ci.yml) [![Security](https://github.com/akrhin/portainer-mcp/actions/workflows/security.yml/badge.svg)](https://github.com/akrhin/portainer-mcp/actions/workflows/security.yml) [![Release](https://img.shields.io/github/v/release/akrhin/portainer-mcp)](https://github.com/akrhin/portainer-mcp/releases/latest)

[Documentation](docs/index.md) · [Русский](README.ru.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

# portainer-mcp

An MCP server that exposes Portainer to MCP clients: Portainer's own API, Docker
Engine through Portainer environments, Kubernetes and Helm. The surface is generated
from the Portainer 2.45 and Docker Engine 1.57 OpenAPI documents — 394 tools across
eleven toolsets, 87 of them registered by default.

Written in Go against the official `github.com/modelcontextprotocol/go-sdk` v1.8.0.
The revision a client negotiates with the server follows the SDK, which speaks MCP
`2026-07-28`; the server also answers the legacy `2025-11-25` handshake. Transport
is stdio by default or Streamable HTTP at `/mcp`.

The tool surface is data. `api/toolsets.yaml` maps OpenAPI tags and paths to toolsets
and carries the name rewrites; `api/default-tools.yaml` lists the 87 tools registered
without `--exhaustive`. `make catalog` (`go run ./tools/cataloggen`) turns both, plus
the two specifications, into `internal/catalog/generated.yaml`, which is embedded in
the binary. `internal/catalog/catalog.yaml` keeps only toolset descriptions and
dependencies. A tool that is not in the generated catalog cannot be called.

**Status: the catalog is executable.** All 394 tools are defined as data and run
through one handler. Without flags, startup registers the 87-tool default set of the
enabled toolsets; `--exhaustive` registers every operation of those toolsets, and
`--read-only` filters either set down to tools annotated read-only.

## Requirements

- Go 1.27 or newer to build (`go.mod` pins `go 1.27.4`). The resulting binary is
  static and needs no runtime.
- A Portainer instance reachable over HTTP or HTTPS, and an API key whose
  permissions you are willing to give the tools. Self-signed certificates are
  accepted only when `tls_verify` is `false` for that instance.

## Build

```sh
make build      # ./portainer-mcp
make check      # gofmt, go vet, go test
make catalog    # regenerate internal/catalog/generated.yaml from api/
```

## Configure

Copy the example and edit the instance block:

```sh
cp config.example.yaml config.yaml
```

```yaml
server:
  transport: stdio
instances:
  - name: prod
    url: https://portainer.example.com:9443
    api_key: ${PORTAINER_API_KEY}
toolsets: [core]
```

`${VAR}` is resolved from the environment at startup; a reference to an unset
variable is an error, not an empty value. Unknown keys are rejected. The full key
reference, validation rules and error messages are in
[docs/configuration.md](docs/configuration.md).

## Run

```sh
PORTAINER_API_KEY=... ./portainer-mcp --config config.yaml
```

With `transport: stdio` the process speaks MCP on stdin and stdout and stays silent
until a message arrives. To register it in a client that launches stdio servers:

```json
{
  "mcpServers": {
    "portainer": {
      "command": "/usr/local/bin/portainer-mcp",
      "args": ["--config", "/etc/portainer-mcp/config.yaml"],
      "env": { "PORTAINER_API_KEY": "..." }
    }
  }
}
```

For HTTP, set `transport: http` and give `listen` an address. With the example's
`127.0.0.1:17717`, Streamable HTTP is served at `http://127.0.0.1:17717/mcp`.
`listen` has no default and is required for HTTP; the listener is plain HTTP, so
put TLS in front of it if the endpoint leaves the host.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--config` | `config.yaml` | Path to the YAML configuration |
| `--toolsets` | — | Comma-separated toolset names; replaces the configured list |
| `--read-only` | `false` | Register only tools annotated `readOnlyHint: true` |
| `--exhaustive` | `false` | Register every operation of the enabled toolsets instead of the default set |
| `--version` | — | Print `portainer-mcp <version> (<commit>, <date>)` and exit |

Flags override the file where they overlap. `--read-only` is combined with
`server.read_only` (either one enables it); `--toolsets` replaces `toolsets`;
`--exhaustive` is combined with `server.exhaustive`.

## Toolsets

Eleven toolsets, selected by name. A toolset cannot be enabled without the ones it
depends on. The counts are what the generated catalog holds and what the default set
registers:

| Toolset | Depends on | Tools | Default |
|---|---|---|---|
| `core` | — | 29 | 7 |
| `docker` | `core` | 79 | 13 |
| `swarm` | `docker` | 30 | 6 |
| `k8s` | `core` | 105 | 14 |
| `helm` | `k8s` | 11 | 6 |
| `stacks` | `docker` | 36 | 9 |
| `gitops` | `stacks` | 14 | 5 |
| `edge` | `core` | 28 | 8 |
| `admin` | `core` | 55 | 12 |
| `observability` | `core` | 4 | 4 |
| `raw` | — | 3 | 3 |

Total: 394 tools, 87 in the default set. Every tool accepts an optional `instance`
argument naming a configured instance, and defaults to the first one. The default set
is curated in `api/default-tools.yaml`; `--exhaustive` ignores it and registers every
tool of the enabled sets. The full list, per set and with the HTTP operation behind
each tool, is `internal/catalog/generated.yaml`; [docs/toolsets.md](docs/toolsets.md)
explains the layout and the special handling of streaming and Kubernetes metadata.

## Arcane backend

The same repository builds a second server, `arcane-mcp`, for an Arcane manager. It has
its own catalog — 399 tools in thirteen toolsets, 53 of them by default — generated by
the same tool from `api/arcane-3.1.yaml`.

```sh
make build-arcane
ARCANE_API_KEY=... ./arcane-mcp --url http://arcane:3552 --toolsets core,containers
```

`--url` is required and there is no configuration file. The key comes from `--api-key`
(default `${ARCANE_API_KEY}`) and travels in the `X-API-Key` header. Arcane 3.0.1 has no
exec or terminal endpoint, so there is no container shell; the Stream, Uploads, Mobile
Push, MFA, Passkeys and OIDC tags are skipped. [docs/arcane.md](docs/arcane.md) has the
toolsets, the flags and the regeneration steps.

## Documentation

- [Documentation index](docs/index.md) — the entry point and the page map.
- [Configuration reference](docs/configuration.md) — every key, default and error.
- [Toolsets](docs/toolsets.md) — the eleven sets, their sizes, dependencies and default tools.
- [Arcane backend](docs/arcane.md) — the second binary: its toolsets, flags and limits.
- [Architecture](docs/architecture.md) — packages, startup, request flow, transports.
- [Development](docs/development.md) — tests, golden files, smoke run, release flow.
- [Contributing](CONTRIBUTING.md) — build gate, commit style, adding a tool.
- [Security policy](SECURITY.md) — private reporting and what counts as a vulnerability.

## Docker and releases

Releases are cut from a `v*` tag. The workflow builds `linux`, `darwin` and `windows`
archives for `amd64` and `arm64` with checksums and SBOMs, and publishes a multi-arch
image to `ghcr.io/akrhin/portainer-mcp` (`:latest` and the version tag). The image is
distroless and runs as a non-root user. `v0.3.0` is out, so the quickest start is
the release archive or the image. Every archive carries both binaries, `portainer-mcp`
and `arcane-mcp`:

```sh
docker run -i --rm -e PORTAINER_API_KEY -v "$PWD/config.yaml:/config.yaml" \
  ghcr.io/akrhin/portainer-mcp:v0.3.0 --config /config.yaml
go install github.com/akrhin/portainer-mcp/cmd/portainer-mcp@v0.3.0
go install github.com/akrhin/portainer-mcp/cmd/arcane-mcp@v0.3.0
```

To build the image from the checkout instead, `docker build -t portainer-mcp .`.

## License

MIT — see [LICENSE](LICENSE).
