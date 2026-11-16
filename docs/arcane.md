[Documentation index](index.md) · [Русский](arcane.ru.md)

# Arcane backend

The repository builds two MCP servers from one catalog generator: `portainer-mcp`
speaks to Portainer, `arcane-mcp` speaks to an [Arcane](https://getarcane.app) manager.
They share the handler, the configuration types and the embedded catalog machinery, and
they are enabled in an MCP client independently.

Arcane 3.0.1 answers JSON under `/api` and authenticates with an `X-API-Key` header. The
catalog covers 399 tools in thirteen toolsets; 53 are registered by default, and
`--exhaustive` registers every operation of the enabled sets.

## Where the Arcane tools come from

| File | Role |
|---|---|
| `api/arcane-3.1.yaml` | The converted OpenAPI document; written by `tools/specgen`, never edited by hand |
| `api/arcane-toolsets.yaml` | Tag-to-toolset map, overrides, skipped tags and paths, name rewrites and the `arcane_raw` extra |
| `api/arcane-default-tools.yaml` | The 53 tools registered without `--exhaustive` |
| `tools/cataloggen -profile arcane` | Reads the three files above and writes `internal/catalog/arcane/generated.yaml` |
| `internal/catalog/arcane/catalog.yaml` | Toolset names, descriptions and `depends_on` edges |
| `internal/catalog/arcane/generated.yaml` | Every tool of every set; embedded in the binary |

`catalog.LoadArcane()` merges the two documents the same way the Portainer catalog is
merged, so validation — duplicate names, missing annotations, unknown dependencies — is
identical. `make catalog` regenerates both catalogs in one run.

## Toolsets

| Toolset | Depends on | Tools | Default |
|---|---|---|---|
| `core` | — | 37 | 8 |
| `containers` | `core` | 15 | 8 |
| `images` | `core` | 46 | 6 |
| `volumes` | `core` | 26 | 3 |
| `networks` | `core` | 8 | 3 |
| `projects` | `core` | 46 | 6 |
| `backups` | — | 27 | 3 |
| `gitops` | `core` | 26 | 4 |
| `swarm` | `core` | 54 | 4 |
| `podman` | `core` | 26 | 2 |
| `security` | `core` | 42 | 4 |
| `admin` | `core` | 45 | 2 |
| `raw` | — | 1 | 0 |

Total: 399 tools, 53 in the default set. `backups` and `raw` stand alone; everything else
needs `core`, which carries the environment (agent) and system calls.

Arcane addresses every resource through an agent: paths read
`/api/agents/{id}/containers/{containerId}`, where `id` is the environment id from
`environments_list`. Container actions are one tool, `container_action`, whose `action` argument selects
start, stop, restart, kill, pause or unpause; projects work the same way through
`project_action`.

## Running commands in a container

Arcane's OpenAPI document has no exec operation. The terminal lives on a WebSocket route,
`GET /api/agents/{id}/ws/containers/{containerId}/terminal?shell=/bin/sh&cols=&rows=`, so
the catalog can not describe it and `container_exec` is registered by hand. It is off
unless the operator asks for it: `--allow-exec` or `ARCANE_ALLOW_EXEC=true`, and never in
`--read-only`. It adds one tool beyond the 399 of the catalog.

> **This tool runs an arbitrary command inside a container, with the same rights as the
> process already running there.** Nothing is quoted or matched against a list on this
> side: the line goes to the container's shell exactly as typed. Enable it only for an
> agent and a manager where that is what you want, and treat the client that holds the
> tool as holding a shell on every container of that manager.

What a call does: it opens one TTY session, writes `command` followed by `exit`, reads
until the server closes the connection, then closes it. There is no stdin stream — the
session ends with the command, and it is closed on every path, including a timeout and an
error.

| Argument | Default | Effect |
|---|---|---|
| `id` | — | Environment (agent) id, as `environments_list` returns it |
| `container` | — | Container id or name prefix |
| `command` | — | The shell line; empty is refused, 4 KiB is the limit |
| `timeout_sec` | `30` | `1`–`300`; on expiry the session is closed and the partial output returned |
| `instance` | — | Instance name, as on the other tools |

Two more limits are fixed: the answer is cut at 64 KiB with a `[truncated at 65536
bytes]` marker, and a frame larger than 1 MiB ends the session.

What the answer contains, and what it does not:

- **No exit code.** Arcane never sends one: its exec runs with a TTY, the socket closes
  when the shell exits, and the status is only reachable through `ExecInspect`, which the
  manager does not expose. The audit line therefore prints `exit=unavailable`, and a
  failing command is recognised by what it printed, not by a status.
- **TTY echo.** The session is interactive, so the output repeats the command and the
  prompt, keeps `\r\n` line endings and carries terminal escape sequences. The server
  writes back `stdout` and `stderr` merged.
- **Failures before the shell starts** arrive as a line of text and then an abnormal close;
  the answer then carries `[session closed <code>: <reason>]`.

Every call writes one line to the server's standard error:
`container_exec agent=… container=… exit=unavailable status=ok|timeout|closed <code>: <reason>|<error text>
duration=… command="…"`, the command cut to 200 characters.

## Run it

```sh
make build-arcane                       # ./arcane-mcp
ARCANE_API_KEY=... ./arcane-mcp --url http://arcane:3552 --toolsets core,containers
```

There is no configuration file. One process serves one manager, given by `--url`; the key
comes from `--api-key` and defaults to the `${ARCANE_API_KEY}` reference, so an unset
variable fails at startup instead of sending an empty header.

| Flag | Default | Effect |
|---|---|---|
| `--url` | — | Base URL of the manager, for example `http://arcane:3552`; required |
| `--api-key` | `${ARCANE_API_KEY}` | Value of the `X-API-Key` header |
| `--name` | `arcane` | Instance name carried in error messages |
| `--toolsets` | all | Comma-separated toolset names |
| `--read-only` | `false` | Register only tools annotated `readOnlyHint: true` |
| `--exhaustive` | `false` | Register every operation of the enabled toolsets |
| `--allow-exec` | `false` | Add `container_exec`, the one-shot container shell; `ARCANE_ALLOW_EXEC=true` sets it too, and `--read-only` wins over both |
| `--version` | — | Print `arcane-mcp <version> (<commit>, <date>)` and exit |

A client entry:

```json
{
  "mcpServers": {
    "arcane": {
      "command": "/usr/local/bin/arcane-mcp",
      "args": ["--url", "http://arcane:3552", "--toolsets", "core,containers,projects"],
      "env": { "ARCANE_API_KEY": "..." }
    }
  }
}
```

## Regenerating the catalog

The converted document is committed; the vendor JSON it comes from is not. To refresh it,
save the manager's OpenAPI 3.1 document as `arcane-openapi.json` in the repository root
and run:

```sh
make spec-arcane    # go run ./tools/specgen -in arcane-openapi.json -out api/arcane-3.1.yaml
make catalog        # go run ./tools/cataloggen, both profiles
```

`specgen` converts the 3.1 document deterministically: `$schema` keys are dropped, a
`type` array collapses to its single non-`null` member, numbers keep their integer form
and the `servers` list is replaced by `/api`. A second run over the same input produces
the same bytes.

## Limits on this pass

- Arcane 3.0.1 has no exec operation in its OpenAPI document: the terminal is a
  WebSocket route the catalog can not describe, so `container_exec` is hand-written and
  off by default (see above). Its protocol reports no exit code.
- The Stream (server-sent events), Uploads, Mobile Push, MFA, Passkeys and OIDC tags are
  skipped: they are browser flows or long-lived streams rather than request/response
  operations.
- `HEAD` operations and the two `/events/changes` and `/system/upgrade/progress` streams
  are dropped by `skip_methods` and `skip_paths`.
- The container images published by the release workflow still contain `portainer-mcp`
  only; the Arcane binary ships in the archives.
- The Arcane client reuses the Portainer HTTP executor with a different authentication
  header and error label. The two servers share one implementation, so a fix in one
  transport reaches both.
