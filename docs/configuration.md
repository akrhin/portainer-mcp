[Documentation index](index.md) · [Русский](configuration.ru.md)

# Configuration

One YAML file for the process, one block per Portainer instance, one list of
toolsets. The file is read once at startup and is checked strictly: an unknown key,
a missing instance or a broken toolset dependency stops the server before it serves
anything.

`portainer-mcp` reads that file. The second binary, `arcane-mcp`, reads no file at
all: one manager is all it talks to, and its flags are its configuration
([Arcane backend](arcane.md) has the flag table).

```yaml
server:
  transport: stdio
  read_only: false
  exhaustive: false
instances:
  - name: prod
    url: https://portainer.example.com:9443
    api_key: ${PORTAINER_API_KEY}
    tls_verify: false
toolsets: [core]
```

`config.example.yaml` in the repository root is the same shape with comments.

## Top level

| Key | Type | Required | Meaning |
|---|---|---|---|
| `server` | mapping | yes | Transport and catalog switches |
| `instances` | list | yes | One or more Portainer servers; at least one |
| `toolsets` | list of strings | yes | Toolset names to register; must not be empty |

Any other key at any level is rejected (`yaml` decoding runs with `KnownFields`), so
a typo such as `toolset:` fails instead of being silently ignored.

## `server`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `transport` | string | `stdio` | `stdio` or `http`. An empty value means `stdio` |
| `listen` | string | — | Address to bind, for example `127.0.0.1:17717`. Required when `transport` is `http`, ignored with stdio |
| `read_only` | bool | `false` | Register only tools annotated `readOnlyHint: true` |
| `exhaustive` | bool | `false` | Register every operation of the enabled toolsets instead of the default set |

The HTTP transport is plain HTTP and serves Streamable HTTP at `/mcp`. Terminate TLS
in front of it when the endpoint is reachable from outside the host.

## `instances[]`

| Key | Type | Required | Meaning |
|---|---|---|---|
| `name` | string | yes | Identifier used by the `instance` argument of every tool. Unique, non-empty |
| `url` | string | yes | Absolute `http` or `https` URL with a host, for example `https://portainer.example.com:9443` |
| `api_key` | string | yes | Portainer API key, sent as the `X-API-KEY` header. Non-empty after `${VAR}` expansion |
| `tls_verify` | bool or string | `false` | Verify the server certificate. `false` accepts self-signed certificates |
| `ca_file` | string | — | PEM file with CA certificates to trust in addition to the system pool |

The first instance is the default target when a tool is called without `instance`.
An unknown name in the argument is an error that lists the configured names.

`tls_verify` defaults to `false`, which means certificate verification is off and
self-signed instances work out of the box. Set it to `true` for any instance with a
certificate from a public CA, and add `ca_file` when the CA is private.

`ca_file` is read once at startup. A file that cannot be read, or one that contains
no certificate, fails startup rather than silently trusting nothing.

## `toolsets[]`

Names of the toolsets to register, in catalog spelling: `core`, `docker`, `swarm`,
`k8s`, `helm`, `stacks`, `gitops`, `edge`, `admin`, `observability`, `raw`. What each
one contains is in [toolsets.md](toolsets.md).

The list is checked twice: every name must exist in the catalog, and every
dependency of an enabled toolset must be enabled next to it. `helm` without `k8s`,
for example, is refused at startup.

`toolsets` is validated even when `--toolsets` replaces it on the command line. Keep
a valid list in the file.

## `${VAR}` expansion

String fields accept a whole-value environment reference instead of a literal:

```yaml
instances:
  - name: prod
    url: ${PORTAINER_URL}
    api_key: ${PORTAINER_API_KEY}
```

| Field | Accepts `${VAR}` |
|---|---|
| `server.transport` | yes |
| `instances[].url` | yes |
| `instances[].api_key` | yes |
| `instances[].ca_file` | yes |
| `instances[].tls_verify` | yes, as a string form |

Rules, from the implementation:

- The reference must cover the whole value. `${A}-suffix` is taken literally, not
  expanded.
- Surrounding whitespace is trimmed before the check.
- An unset or empty variable is an error, never an empty string. For `api_key` the
  message names the variable to set: `instance "prod": api_key is empty; set
  PORTAINER_API_KEY`.
- The expanded value is expanded again if it is itself a reference; a chain that keeps
  pointing at another reference fails with `nests too deep`.
- `tls_verify` also accepts a plain YAML boolean. Its string form goes through the
  same expansion and then `strconv.ParseBool`, so `true`, `1`, `false` and `0` work,
  anything else fails with `expected a boolean, got "..."`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--config` | `config.yaml` | Path to the YAML file |
| `--toolsets` | — | Comma-separated names, whitespace around items is trimmed. Replaces `toolsets` from the file; an unknown name fails before the file is read |
| `--read-only` | `false` | Combined with `server.read_only`: either one enables read-only registration |
| `--exhaustive` | `false` | Combined with `server.exhaustive`: either one registers every operation of the enabled toolsets instead of the default set |
| `--version` | — | Print `portainer-mcp <version> (<commit>, <date>)` and exit before the catalog or config is read |

`--version` is filled at build time through `-X main.version`, `-X main.commit` and
`-X main.date`; an untagged local build prints `dev (none, unknown)`.

## The Arcane binary

`arcane-mcp` has no configuration file and no `--config`. It builds the same
`config.Config` in memory from flags (`config.Single`): the URL becomes the single
instance, `--toolsets` becomes the list, and the result goes through the same
validation as a YAML file — so an unknown toolset, a disabled dependency, a relative
URL or an empty key fails with the same messages listed below.

| Flag | Default | Meaning |
|---|---|---|
| `--url` | — | Base URL of the manager; required, and an absolute `http(s)` URL |
| `--api-key` | `${ARCANE_API_KEY}` | Sent as `X-API-Key`; an unset variable fails with `instance "arcane": api_key is empty; set ARCANE_API_KEY` |
| `--name` | `arcane` | Instance name used in error messages |
| `--toolsets` | every set of the Arcane catalog | Comma-separated names; an empty list means all of them, not an error |

`--url` and `--api-key` accept a whole-value `${VAR}` reference exactly as the YAML
string fields do. The transport is always stdio: there is no `--listen` and no HTTP
listener, so the `server` block has no equivalent here. The remaining flags —
`--read-only`, `--exhaustive`, `--allow-exec` and `--version` — are catalog switches,
listed with the Arcane toolsets in [Arcane backend](arcane.md).

## Startup errors

Messages you can meet before the server serves anything, quoted as the code produces
them:

| Message | Cause |
|---|---|
| `read config: ...` | The file cannot be read |
| `parse config <path>: ...` | Invalid YAML, or an unknown key |
| `config: at least one instance is required` | `instances` is empty or missing |
| `config: instance url "..." has no name` | An instance without `name` |
| `config: duplicate instance name "..."` | Two instances share a name |
| `config: instance "...": url must be absolute http(s), got "..."` | `url` is relative or has another scheme |
| `no toolsets enabled` | `toolsets` is empty |
| `unknown toolset "..." (known: ...)` | A name not in the catalog; the known names are listed |
| `toolset "..." requires "...", which is not enabled` | A dependency is missing |
| `instance "...": api_key is required` | `api_key` is empty |
| `config: server.listen is required for http transport` | `transport: http` without `listen` |
| `config: server.transport must be stdio or http, got "..."` | Another transport name |
| `no tools selected for toolsets ...` | Read-only filtering removed every tool of the selection |

## Examples

### HTTP transport

```yaml
server:
  transport: http
  listen: 127.0.0.1:17717
  read_only: true
instances:
  - name: prod
    url: ${PORTAINER_URL}
    api_key: ${PORTAINER_API_KEY}
    tls_verify: true
toolsets: [core]
```

`read_only: true` here removes every tool that is not annotated read-only. With
`toolsets: [core]` and the default set, all seven registered `core` tools are
read-only, so all seven stay.

### Several instances

```yaml
instances:
  - name: prod
    url: https://portainer.example.com:9443
    api_key: ${PORTAINER_PROD_KEY}
    tls_verify: true
  - name: lab
    url: https://portainer.lab.example.com:9443
    api_key: ${PORTAINER_LAB_KEY}
    tls_verify: false
    ca_file: /etc/portainer-mcp/lab-ca.pem
toolsets: [core]
```

A tool call without `instance` goes to `prod`; `{"instance": "lab"}` goes to the
second one.
