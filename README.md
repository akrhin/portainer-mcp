[![CI](https://github.com/akrhin/portainer-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/akrhin/portainer-mcp/actions/workflows/ci.yml) [![Security](https://github.com/akrhin/portainer-mcp/actions/workflows/security.yml/badge.svg)](https://github.com/akrhin/portainer-mcp/actions/workflows/security.yml) [![Release](https://img.shields.io/github/v/release/akrhin/portainer-mcp)](https://github.com/akrhin/portainer-mcp/releases/latest)

[Documentation](docs/index.md) · [Русский](README.ru.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

# portainer-mcp

Two MCP servers are built here from one catalog generator, both in Go against the
official `github.com/modelcontextprotocol/go-sdk` v1.8.0.

| Binary | Speaks to | Catalog | Default set |
|---|---|---|---|
| `portainer-mcp` | Portainer: its own API, Docker Engine through Portainer environments, Kubernetes and Helm | 394 tools in eleven toolsets | 87 |
| `arcane-mcp` | An [Arcane](https://getarcane.app) manager: containers, projects, images, volumes, networks, security and administration | 445 tools in seventeen toolsets | 53 |

`portainer-mcp` speaks MCP over stdio by default or Streamable HTTP at `/mcp`;
`arcane-mcp` is stdio only. The revision a client negotiates follows the SDK, which
speaks MCP `2026-07-28`; the server also answers the legacy `2025-11-25` handshake.

The tool surface is data. `api/toolsets.yaml` maps OpenAPI tags and paths to toolsets
and carries the name rewrites; `api/default-tools.yaml` lists the 87 tools registered
without `--exhaustive`. `make catalog` (`go run ./tools/cataloggen`) turns both, plus
the two specifications, into `internal/catalog/generated.yaml`, which is embedded in
the binary. The Arcane profile reads `api/arcane-toolsets.yaml`,
`api/arcane-default-tools.yaml` and `api/arcane-3.1.yaml` the same way.
`internal/catalog/catalog.yaml` keeps only toolset descriptions and dependencies. A
tool that is not in the generated catalog cannot be called.

**Status: both catalogs are executable.** Every tool — 394 for Portainer, 445 for
Arcane — is defined as data and run through one shared handler. Without flags, startup
registers the default set of the enabled toolsets, 87 and 53 respectively;
`--exhaustive` registers every operation of those toolsets, and `--read-only` filters
either set down to tools annotated read-only.

## Requirements

- Go 1.27 or newer to build (`go.mod` pins `go 1.27.4`). The resulting binary is
  static and needs no runtime.
- A Portainer instance reachable over HTTP or HTTPS, and an API key whose
  permissions you are willing to give the tools. Self-signed certificates are
  accepted only when `tls_verify` is `false` for that instance.
- For `arcane-mcp`, an Arcane manager and an API key for it; the manager's base URL
  is a required flag, not a configuration key.

## Build

```sh
make build        # ./portainer-mcp
make build-arcane # ./arcane-mcp
make check        # gofmt, go vet, go test
make catalog      # regenerate both generated catalogs from api/
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
`--exhaustive` is combined with `server.exhaustive`. `arcane-mcp` has no configuration
file and its own flag set; the table is under [Arcane backend](#arcane-backend).

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
its own catalog — 445 tools in seventeen toolsets, 53 of them by default — generated by
the same tool from `api/arcane-3.1.yaml`.

```sh
make build-arcane
ARCANE_API_KEY=... ./arcane-mcp --url http://arcane:3552 --toolsets core,containers
```

`--url` is required and there is no configuration file. The key comes from `--api-key`
(default `${ARCANE_API_KEY}`) and travels in the `X-API-Key` header. Arcane's OpenAPI
document has no exec operation — its terminal is a WebSocket route — so the one-shot
container shell is a hand-written `container_exec`, registered only under
`--allow-exec` (or `ARCANE_ALLOW_EXEC=true`) and never in read-only mode.
[docs/arcane.md](docs/arcane.md) has the toolsets, the flags and the regeneration
steps.

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
distroless and runs as a non-root user. `v0.4.1` is out, so the quickest start is
the release archive or the image. Every archive carries both binaries, `portainer-mcp`
and `arcane-mcp`; the image carries `portainer-mcp` only.

```sh
docker run -i --rm -e PORTAINER_API_KEY -v "$PWD/config.yaml:/config.yaml" \
  ghcr.io/akrhin/portainer-mcp:v0.4.1 --config /config.yaml
go install github.com/akrhin/portainer-mcp/cmd/portainer-mcp@v0.4.1
go install github.com/akrhin/portainer-mcp/cmd/arcane-mcp@v0.4.1
```

To build the image from the checkout instead, `docker build -t portainer-mcp .`.

## License

MIT — see [LICENSE](LICENSE).
