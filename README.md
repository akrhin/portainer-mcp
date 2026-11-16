[Documentation](docs/index.md) · [Русский](README.ru.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

# portainer-mcp

An MCP server that exposes Portainer to MCP clients: Portainer itself today, with
Docker, Kubernetes and Helm declared in the catalog for the stages that follow.

Written in Go against the official `github.com/modelcontextprotocol/go-sdk` v1.8.0.
The revision a client negotiates with the server follows the SDK, which speaks MCP
`2026-07-28`; the server also answers the legacy `2025-11-25` handshake. Transport
is stdio by default or Streamable HTTP at `/mcp`.

The tool surface is data: `internal/catalog/catalog.yaml` is embedded in the binary
and drives registration, read-only filtering and startup validation. A tool that is
not in the catalog cannot be called.

**Status: stage 0.** The catalog declares eleven toolsets and 113 operations; only
`core` has handlers. Enabling any other toolset stops the server with
`not implemented yet`, so a client never sees a tool that cannot answer.

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
| `--exhaustive` | `false` | One tool per API operation; refused until stage 1 |

Flags override the file where they overlap. `--read-only` is combined with
`server.read_only` (either one enables it); `--toolsets` replaces `toolsets`.

## Toolsets

Eleven toolsets, selected by name. A toolset cannot be enabled without the ones it
depends on. Only `core` is implemented:

`system_version`, `system_status`, `system_info`, `settings_get`, `whoami`,
`environment_list`, `environment_inspect`.

Declared but not implemented: `docker`, `swarm`, `k8s`, `helm`, `stacks`, `gitops`,
`edge`, `admin`, `observability`, `raw`. Enabling one is a startup error, which is
deliberate: a client must never see a tool that cannot answer.

Every tool accepts an optional `instance` argument naming a configured instance, and
defaults to the first one. What each toolset contains, its dependencies and the tool
names behind them are in [docs/toolsets.md](docs/toolsets.md).

## Documentation

- [Documentation index](docs/index.md) — the entry point and the page map.
- [Configuration reference](docs/configuration.md) — every key, default and error.
- [Toolsets](docs/toolsets.md) — the eleven sets, dependencies and declared tools.
- [Architecture](docs/architecture.md) — packages, startup, request flow, transports.
- [Development](docs/development.md) — tests, golden files, smoke run, release flow.
- [Contributing](CONTRIBUTING.md) — build gate, commit style, adding a tool.
- [Security policy](SECURITY.md) — private reporting and what counts as a vulnerability.

## Docker and releases

Releases are cut from a `v*` tag. The workflow builds `linux`, `darwin` and `windows`
archives for `amd64` and `arm64` with checksums and SBOMs, and publishes a multi-arch
image to `ghcr.io/akrhin/portainer-mcp` (`:latest` and the version tag). The image is
distroless and runs as a non-root user. No release has been tagged yet; until one is,
build the image yourself:

```sh
docker build -t portainer-mcp .
```

## License

MIT — see [LICENSE](LICENSE).
