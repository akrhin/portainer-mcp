[Documentation index](index.md) · [Русский](architecture.ru.md)

# Architecture

Six Go packages and one direction of dependency: `cmd` wires `catalog`, `config`,
`mcpserver` and `portainer`; `tools/cataloggen` is a build-time generator that imports
the catalog types; nothing imports `cmd`. The API specifications in `api/` are inputs
to the generator, not compiled code.

| Path | Contents |
|---|---|
| `cmd/portainer-mcp/` | Entry point: flags, catalog load, config load, transport |
| `api/` | The two OpenAPI documents plus the curated `toolsets.yaml` and `default-tools.yaml` |
| `tools/cataloggen/` | Generator behind `make catalog`; writes `internal/catalog/generated.yaml` |
| `internal/catalog/` | Catalog types, embedding and merge of the two documents, validation |
| `internal/config/` | YAML configuration, `${VAR}` expansion, validation against the catalog |
| `internal/mcpserver/` | Registration from the catalog, the one tool handler, instance resolution |
| `internal/portainer/` | HTTP executor, response helpers, the instance client |
| `internal/mcpserver/testdata/` | Golden files for the registered `tools/list` |
| `scripts/` | `docs-check.sh` (documentation gate), `smoke.sh` (live stdio run) |

## The catalog is the interface

Two embedded documents make up the catalog. `internal/catalog/catalog.yaml` is curated
by hand and holds only the eleven toolset names, descriptions and `depends_on` edges.
`internal/catalog/generated.yaml` holds every tool — name, description, annotations,
input schema, optional output schema and HTTP operation. `catalog.go` embeds both with
`//go:embed` and `Merge` (`internal/catalog/catalog.go:98`) attaches the generated
tools to the curated sets. The merge refuses a set that appears in only one document,
a version mismatch, and a tool list inside `catalog.yaml`.

The generated document is not written by hand. `make catalog`
(`go run ./tools/cataloggen`) reads the Portainer 2.45 and Docker Engine 1.57
specifications in `api/`, the tag-to-toolset mapping in `api/toolsets.yaml` and the
default list in `api/default-tools.yaml`, then writes the file. The output is
deterministic, and `TestBuildMatchesCommittedFile` fails when the committed file
differs from a fresh run.

Consequences worth knowing:

- A tool absent from the generated catalog cannot be registered.
- Read-only mode is a registration filter, not a runtime check: `selectTools`
  (`internal/mcpserver/server.go:58`) drops every tool whose `readOnlyHint` is not
  true, and without `--exhaustive` it also drops every tool not marked `default`.
- The golden test compares `tools/list` for `core` plus `docker` with
  `internal/mcpserver/testdata/tools_core_docker.golden.json` (20 default tools), so a
  catalog change that alters the surface fails the suite until the file is regenerated.

Validation runs on the merged catalog at every start, before any transport opens:
unsupported `version`, unnamed or duplicate toolset, duplicate tool name, a tool
without `readOnlyHint`, a write tool without `destructiveHint` or `idempotentHint`,
an `input` whose type is not `object`, an `output` whose type is neither `object` nor
`array`, a streaming parameter that is not an input property, an operation path that
does not start with `/`, and a dependency on a toolset that is not declared.

## Startup

`main.run` performs the steps in this order, and every failure ends the process with
`portainer-mcp: <error>` on stderr and exit code 1:

1. Parse `--config`, `--toolsets`, `--read-only`, `--exhaustive` and `--version`.
   `--version` prints `portainer-mcp <version> (<commit>, <date>)` and returns before
   the catalog or config is read.
2. Load and validate the merged catalog.
3. Validate the `--toolsets` list against the catalog, before the config file is read.
4. Load the config file: decode with unknown fields rejected, expand `${VAR}`, then
   validate instances, toolsets and transport.
5. Build the server: resolve the effective toolset list, read-only and exhaustive
   flags, check dependencies, construct one client per instance, then register the
   selected tools.
6. Run the transport.

The effective settings are: `--toolsets` replaces `toolsets` when non-empty;
`--read-only` and `server.read_only` are combined with OR; `--exhaustive` and
`server.exhaustive` are combined with OR.

Registration failure is not silent. If the selection ends with zero tools — for
example every tool was filtered out — `New` returns `no tools selected for toolsets
...` instead of serving an empty server.

## Request flow

```text
MCP client
  → stdio or Streamable HTTP at /mcp
  → SDK dispatches CallTool for the registered name
  → adapt: decode arguments into map[string]any
  → handler: resolve the instance, build the request from the catalog entry
  → portainer.Client.Do: HTTP with X-API-KEY
  → apiResult: strip managedFields, shape the answer, set structuredContent
```

The handler (`internal/mcpserver/tools.go:20`) turns one catalog entry into a request:
path parameters are escaped and substituted, every remaining input property becomes a
query value (arrays as repeated keys, or one comma-joined value where the catalog
declares it), and the body property is marshalled as JSON. Raw tools take the method
and path from the arguments and append the path to their base, refusing a path that
does not start with `/` or contains `..`.

Two details shape the error contract:

- `adapt` never returns a protocol error for an API failure. It converts the error to
  a result with `isError: true` and the error text as content, so a failing Portainer
  call looks like a tool-level failure to the model instead of a broken session.
- Argument decoding is strict at the top level: arguments that are not a JSON object
  produce `arguments must be a JSON object: ...` as an `isError` result. A missing
  path placeholder produces `missing path parameter: ...`; a missing required body
  produces `<name> is required`.

Instance resolution happens in the handler layer. Every tool takes an optional
`instance` string; empty or absent means the first configured instance, and an
unknown name produces `unknown instance "x" (configured: ...)`.

## Response shaping

`internal/portainer/exec.go` and `apiResult` decide what reaches the model:

- **Streaming refusal.** Four tools carry `follow` (`container_logs`,
  `service_logs`, `task_logs`, `kubernetes_pod_logs`) and three stream by path
  (`system_events`, `container_attach`, `exec_start`). A call that would stay open is
  refused before anything is sent, with a message that names the bound to set:
  `follow=false` or `until`.
- **Kubernetes metadata.** The 116 tools whose operation path contains `/kubernetes/`
  carry `strip_managed_fields: true`; `metadata.managedFields` is removed at every
  depth before the answer is serialized.
- **Size cap.** A response body is cut at 1 MiB (`MaxResponse` in
  `internal/portainer/exec.go:17`). A cut JSON array becomes a page: the first 50
  elements plus `_truncated`, `_total` and `_hint`. A cut non-array keeps the marker
  `…[truncated at N bytes: ...]`.
- **Structured content.** A complete JSON answer is returned both as text and as
  `structuredContent`; 237 tools also declare an output schema, which becomes the
  tool's `outputSchema`.

## Portainer client

`internal/portainer` is an HTTP executor, not a generated SDK. Every tool goes through
`(*Client).Do` (`internal/portainer/exec.go:31`), which sends the method, path, query
and body with `X-API-KEY` and `Accept: application/json`, and reads at most
`MaxResponse + 1` bytes.

`New` builds the HTTP client: a 120-second timeout, and a TLS configuration where
`InsecureSkipVerify` is the negation of `tls_verify`. When `ca_file` is set, its PEM
certificates are appended to the system pool; an unreadable file or a file with no
certificates is a construction error.

Non-2xx responses become `*APIError` carrying the status and up to 4 KiB of the
response body. Its `Error` text distinguishes the three statuses an operator meets:
401 points at the API key, 403 says the key may not perform the operation, 404
reports the missing resource; everything else falls back to
`portainer: <status> <status text>: <body>`.

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

## What the server does not do

- No retries, rate limiting, caching or request logging in front of Portainer.
- No websocket carry: `/containers/{id}/attach/ws` is excluded by `skip_paths` in
  `api/toolsets.yaml`, because a request/response tool call cannot hold that stream.
- No per-session state: the HTTP handler serves one shared server value, and
  everything request-specific lives in the request context.
