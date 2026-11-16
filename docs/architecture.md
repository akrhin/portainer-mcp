[Documentation index](index.md) · [Русский](architecture.ru.md)

# Architecture

Four packages, one direction of dependency: `cmd` wires `config`, `catalog`,
`mcpserver` and `portainer`; nothing imports `cmd`.

| Path | Contents |
|---|---|
| `cmd/portainer-mcp/` | Entry point: flags, catalog load, config load, transport |
| `internal/catalog/` | Catalog types, YAML parsing, validation, embedded `catalog.yaml` |
| `internal/config/` | YAML configuration, `${VAR}` expansion, validation against the catalog |
| `internal/mcpserver/` | Registration from the catalog, tool handlers, instance resolution |
| `internal/portainer/` | HTTP client for the Portainer API |
| `internal/mcpserver/testdata/` | Golden file for the registered tool list |
| `scripts/` | `docs-check.sh` (documentation gate), `smoke.sh` (live stdio run) |

## The catalog is the interface

`internal/catalog/catalog.yaml` is embedded with `//go:embed` and parsed at startup.
It holds every tool's name, description, annotations, input schema and output schema.
`mcpserver` converts a catalog entry into an `mcp.Tool` and registers the handler named
by the entry; the handler carries no metadata of its own.

Consequences worth knowing:

- A tool absent from the catalog cannot be registered, and a handler without a
  catalog entry is never reached.
- Read-only mode is a registration filter, not a runtime check: `selectTools` drops
  every tool whose `readOnlyHint` is not true, so `tools/list` never advertises a
  dropped tool.
- The golden test compares the registered `tools/list` against
  `internal/mcpserver/testdata/tools_core.golden.json`, so a catalog edit that changes
  the surface fails the suite until the golden file is regenerated.

Validation runs on the embedded document at every start, so a malformed catalog fails
before any transport is opened: unsupported `version`, unnamed or duplicate toolset,
duplicate tool name, a tool without `readOnlyHint`, a write tool without
`destructiveHint`, an `input` or `output` with a `type` other than `object`, and a
dependency on a toolset that is not declared.

## Startup

`main.run` performs the steps in this order, and every failure ends the process with
`portainer-mcp: <error>` on stderr and exit code 1:

1. Parse `--config`, `--toolsets`, `--read-only`, `--exhaustive`.
2. Load and validate the embedded catalog.
3. Validate the `--toolsets` list against the catalog, before the config file is read.
4. Load the config file: decode with unknown fields rejected, expand `${VAR}`, then
   validate instances, toolsets and transport.
5. Build the server: resolve the effective toolset list and read-only flag, refuse
   `exhaustive`, check dependencies, and refuse a toolset with no handlers.
6. Construct one `portainer.Client` per configured instance.
7. Register the selected tools, then run the transport.

The effective settings are: `--toolsets` replaces `toolsets` when non-empty;
`--read-only` and `server.read_only` are combined with OR; `--exhaustive` and
`server.exhaustive` are combined with OR and either one aborts.

Registration failure is not silent. If the selection ends with zero tools — for
example every tool was filtered out — `New` returns `no tools selected for toolsets
...` instead of serving an empty server.

## Request flow

```text
MCP client
  → stdio or Streamable HTTP at /mcp
  → SDK dispatches CallTool for the registered name
  → adapt: decode arguments into map[string]any
  → handler: resolve the instance, call the Portainer client
  → portainer.Client.get: GET over HTTP with X-API-KEY
  → adapt: marshal the result into text content and structuredContent
```

Two details shape the error contract:

- `adapt` never returns a protocol error for an API failure. It converts the error to
  a result with `isError: true` and the error text as content, so a failing Portainer
  call looks like a tool-level failure to the model instead of a broken session.
- Decoding arguments is strict at the top level: arguments that are not a JSON object
  produce `arguments must be a JSON object: ...` as an `isError` result. Argument
  types are checked per handler — `environment_inspect` requires an integer `id` and
  reports `id is required` or `id must be an integer`.

Instance resolution happens in the handler layer. Every tool takes an optional
`instance` string; empty or absent means the first configured instance, and an
unknown name produces `unknown instance "x" (configured: ...)`.

## Portainer client

`internal/portainer` is a read client, not a generated SDK. It offers one method per
`core` endpoint, all thin wrappers over `get`:

| Method | Path |
|---|---|
| `SystemVersion` | `/api/system/version` |
| `SystemStatus` | `/api/system/status` |
| `SystemInfo` | `/api/system/info` |
| `Settings` | `/api/settings` |
| `CurrentUser` | `/api/users/me` |
| `Endpoints` | `/api/endpoints` |
| `Endpoint(id)` | `/api/endpoints/{id}` |

`New` builds the HTTP client: a 120-second timeout, `X-API-KEY` on every request,
`Accept: application/json`, and a TLS configuration where `InsecureSkipVerify` is the
negation of `tls_verify`. When `ca_file` is set, its PEM certificates are appended to
the system pool; an unreadable file or a file with no certificates is a construction
error.

Non-2xx responses become `*APIError` carrying the status and up to 4 KiB of the
response body. Its `Error` text distinguishes the three statuses an operator meets:
401 points at the API key, 403 says the key may not perform the operation, 404
reports the missing resource; everything else falls back to
`portainer: <status> <status text>: <body>`.

The `core` tools return `json.RawMessage` for the endpoints whose response shape is
Portainer's business, and a reduced struct for `environment_list`, where the numeric
type and status are translated into names.

## Transports

- **stdio** (default): `srv.Run(ctx, &mcp.StdioTransport{})`. Nothing is printed to
  stdout except protocol messages; startup errors go to stderr.
- **Streamable HTTP**: `mcp.NewStreamableHTTPHandler` returns the same server for every
  request, mounted at `/mcp` on a fresh `http.ServeMux`. The listener is plain HTTP —
  TLS is the front proxy's job. There is no authentication layer of its own: anyone
  who reaches the port can use the configured API keys.

The protocol revision is negotiated by the SDK (`github.com/modelcontextprotocol/go-sdk`
v1.8.0), which speaks `2026-07-28`; the repository does not pin a revision of its own.
The smoke script sends `2026-07-28` in its `initialize` request.

## What stage 0 does not contain

- No handlers for `docker`, `swarm`, `k8s`, `helm`, `stacks`, `gitops`, `edge`,
  `admin`, `observability` or `raw`; enabling one is a startup error.
- No generated client and no `--exhaustive` mode; that arrives with the OpenAPI-based
  client in stage 1.
- No retries, rate limiting, caching or request logging on the Portainer client.
- No per-session state: the HTTP handler serves one shared server value, and
  everything request-specific lives in the request context.
