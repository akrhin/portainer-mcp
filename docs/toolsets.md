[Documentation index](index.md) · [Русский](toolsets.ru.md)

# Toolsets

The Portainer catalog carries 394 tools in eleven toolsets. Every tool is data: a name,
a description, MCP annotations, an input schema, an optional output schema and the HTTP
operation behind it. No handler restates any of that. The list itself lives in
`internal/catalog/generated.yaml`; this page is a view of it, and when the two
disagree the server registers what the generated file says.

The Arcane binary ships a catalog of its own — 445 tools in seventeen toolsets — built
by the same generator from a separate pair of files in `api/` and
`internal/catalog/arcane/`. It is described in [Arcane backend](arcane.md); every
statement below is about the Portainer catalog.

## Where the tools come from

| File | Role |
|---|---|
| `api/portainer-2.45.yaml`, `api/docker-engine-1.57.yaml` | The OpenAPI documents every tool is derived from |
| `api/toolsets.yaml` | Tag-to-toolset map, per-operation overrides, skipped paths, streaming marks, `managedFields` paths, name rewrites and the three raw extras |
| `api/default-tools.yaml` | The 87 tools registered when `--exhaustive` is off |
| `tools/cataloggen` (`make catalog`) | Reads the four files above and writes `internal/catalog/generated.yaml`; the same run builds the Arcane catalog from its own inputs (`-profile` builds one of them) |
| `internal/catalog/catalog.yaml` | Toolset names, descriptions and `depends_on` edges; no tool lists |
| `internal/catalog/generated.yaml` | Every tool of every set; embedded in the binary |

Both documents are embedded with `//go:embed` and merged at startup
(`internal/catalog/catalog.go`): the curated document supplies the set descriptions
and dependencies, the generated one supplies the tools. The merge refuses a toolset
that appears in only one of them, and refuses a tool list in `catalog.yaml`. The
Arcane pair under `internal/catalog/arcane/` is embedded and merged the same way, by
`LoadArcane` instead of `Load`.

## Summary

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

The dependency graph is `helm → k8s → core` and
`gitops → stacks → docker → core`; `edge`, `admin` and `observability` hang off
`core`. A set cannot be enabled without its dependencies, and `raw` is never pulled
in by anything.

`--exhaustive` is the difference between the two right-hand columns: without it the
server registers only tools marked `default: true`, which is the curated set in
`api/default-tools.yaml`. With it, every tool of the enabled sets is registered.
`--read-only` filters either selection down to tools annotated `readOnlyHint: true`
at registration time, so the client never sees a tool it may not call.

## How a tool runs

One handler serves every tool. `internal/mcpserver/tools.go` builds the request from
the catalog entry and the arguments — path parameters are escaped and substituted,
remaining input properties become query values (arrays as repeated keys, or one
comma-joined value where the catalog says so), and the body property is sent as JSON.
A body that the document declares as a plain object of scalars is not wrapped: its
fields arrive as arguments of their own, so a search takes `query` rather than
`body: {query}`. A body with a nested object, an array or free-form keys keeps the
wrapper.
`internal/portainer/exec.go` performs the call with `X-API-KEY`. A JSON answer comes
back both as text content and as `structuredContent`; 237 tools also declare an
output schema, which the client receives as `outputSchema`.

## `core` — 29 tools, 7 by default

Server identity and settings (`system_version`, `system_status`, `system_info`,
`whoami`, `motd`, `settings_get`, `settings_public`, and the writes
`settings_update`, `system_upgrade`), environments (`environment_list`,
`environment_inspect`, `endpoint_*` create/update/delete, associations, snapshots,
summary), and endpoint registries.

The default set is the seven identity and inventory tools:
`system_version`, `system_status`, `system_info`, `settings_get`, `whoami`,
`environment_list`, `environment_inspect`.

## `docker` — 79 tools, 13 by default

Containers (`container_list`, `container_inspect`, `container_logs`, create, start,
stop, restart, delete, exec, stats, top, archive, prune and the rest), images,
networks, volumes, plugins, distribution and daemon system calls. Docker operations go
through the Portainer proxy; `environment` is a required argument, renamed from the
specification's `environmentId` by `param_renames` in `api/toolsets.yaml`.

The default set covers the common container and image reads plus the lifecycle
writes: `container_list`, `container_inspect`, `container_logs`, `container_create`,
`container_start`, `container_stop`, `container_restart`, `container_delete`,
`container_exec`, `image_list`, `image_inspect`, `image_delete`, `network_list`.

## `swarm` — 30 tools, 6 by default

Services, nodes, tasks, secrets, configs and the swarm membership calls (`swarm_init`,
`swarm_join`, `swarm_leave`, unlock). The default set is `service_list`,
`service_inspect`, `service_create`, `service_delete`, `node_list`, `task_list`.

## `k8s` — 105 tools, 14 by default

Namespaces, pods, deployments, replica sets, jobs and cron jobs, config maps, secrets,
services, ingresses and ingress controllers, persistent volumes and claims, storage
classes, resource quotas, roles and role bindings, service accounts, nodes and
metrics, plus a manifest dry run and a dashboard listing.

The default set is `kubernetes_namespaces_list`, `kubernetes_pods_list`,
`kubernetes_pod_logs`, `kubernetes_pod_delete`, `kubernetes_pod_restart`,
`kubernetes_deployment_scale`, `kubernetes_deployment_rollback`,
`kubernetes_config_maps_list`, `kubernetes_secrets_list`,
`kubernetes_services_list`, `kubernetes_cluster_ingresses_list`,
`kubernetes_persistent_volume_claims_list`, `kubernetes_nodes_list`, `node_drain`.

## `helm` — 11 tools, 6 by default

Repositories (`helm_user_repositories_list`, create and delete), release listing and
inspection, history, chart search and show, and the lifecycle writes
`helm_install`, `helm_rollback`, `helm_delete`. The default set is
`helm_list`, `helm_inspect`, `helm_install`, `helm_rollback`, `helm_delete`,
`helm_user_repositories_list`.

## `stacks` — 36 tools, 9 by default

Portainer stacks for Docker standalone, Docker Swarm and Kubernetes, created from a
file, a git repository, a string or a URL; `stack_list`, `stack_inspect`,
`stack_file_inspect`, start, stop, update, migrate, git redeploy and delete; webhooks;
custom templates.

The default set is `stack_list`, `stack_inspect`, `stack_start`, `stack_stop`,
`stack_update`, `stack_delete`, `stack_create_docker_standalone_file`,
`stack_create_docker_standalone_repository`,
`stack_create_docker_standalone_string`.

## `gitops` — 14 tools, 5 by default

GitOps sources (list, inspect, create, update, delete, test), workflows (list,
inspect, summary) and a repository file preview. The default set is
`gitops_sources_list`, `gitops_source_inspect`, `gitops_sources_create_git`,
`gitops_sources_delete`, `gitops_sources_test`.

## `edge` — 28 tools, 8 by default

Edge groups, edge stacks, edge jobs and job tasks, and the edge status of an
environment. The default set is `edge_group_list`, `edge_stack_list`,
`edge_stack_inspect`, `edge_stack_file`, `edge_job_list`, `edge_job_tasks_list`,
`edge_job_task_logs_inspect`, `endpoint_edge_status_inspect`.

## `admin` — 55 tools, 12 by default

Users, teams and memberships, roles, registries, endpoint groups, resource controls,
tags, backup and restore, LDAP check, SSL settings, templates and the authentication
calls. The default set is `user_list`, `user_inspect`, `user_create`, `user_delete`,
`team_list`, `team_create`, `team_membership_create`, `role_list`,
`endpoint_group_list`, `registry_list`, `tag_list`, `resource_control_create`.

`backup` returns a server backup archive: treat the response as sensitive.

## `observability` — 4 tools, all by default

`status_inspect`, `system_events`, `kubernetes_events` and
`kubernetes_events_namespace_list`. The two event streams take a time window and
return what happened in it.

## `raw` — 3 tools, all by default

`portainer_raw`, `docker_raw`, `k8s_raw` — one arbitrary request per target, with the
method, path, query and JSON body chosen by the caller. The path is appended to
`/api`, `/api/endpoints/{environment}/docker` or
`/api/endpoints/{environment}/kubernetes`;
`docker_raw` and `k8s_raw` require `environment`. All three are annotated as writes,
because a raw request may change anything, and none is pulled in as a dependency.
Enabling `raw` is an explicit decision by the operator.

Multipart operations are marked `form: true` by the generator. Their `body` is sent
as `multipart/form-data`, and a value shaped `{filename, content}` or
`{filename, base64}` becomes a file part; every other key becomes a text field.

## Streaming operations

Seven tools do not answer on their own. Four stream when a parameter is true:
`container_logs`, `service_logs`, `task_logs` and `kubernetes_pod_logs` carry a
`follow` parameter. Three stream by path whatever the arguments are: `system_events`,
`container_attach` and `exec_start`.

A call that would stay open until the client times out is refused before any request
is sent, with a message naming the limit to change: pass `follow=false`, or bound the
window with `until`. The flags come from `streaming_params` and `streaming_paths` in
`api/toolsets.yaml`, and the refusal lives in one place, so it covers every tool the
catalog marks.

## Kubernetes metadata

Kubernetes answers repeat `metadata.managedFields` in every object, and it is most of
their volume. The 116 tools whose operation path contains `/kubernetes/` are marked
`strip_managed_fields: true` by the generator; before the answer reaches the model,
the field is removed at every depth. Tools outside that path, including `k8s_raw`,
keep the answer as it arrived.

## Large answers

A response body is cut at 1 MiB. If the cut body is a JSON array, the tool returns a
page instead of a broken prefix: the first 50 elements plus `_truncated`, `_total`
and a `_hint` naming the tool to narrow the list with. A cut body that is not an
array keeps the plain marker `…[truncated at N bytes: ask for a narrower filter or a
smaller range]`.

An answer that arrives whole but is larger than 24 KiB is paged the same way: a
top-level array becomes `{items, _truncated, _total, _hint}`, and a list under `data`
keeps its envelope with the counts added. The budget keeps one answer at roughly six
thousand tokens, and under the 32 KiB a gateway forwards whole without wrapping it.

## Selection rules

- An empty `toolsets` list is an error; at least one set must be named.
- A name that is not in the catalog is rejected with the list of known names.
- A dependency that is not enabled is rejected:
  `toolset "helm" requires "k8s", which is not enabled`.
- With `--read-only` or `server.read_only`, write tools are dropped at registration
  time. The client's `tools/list` never shows them.
- If the selection ends with zero tools, startup fails with
  `no tools selected for toolsets ...` rather than serving an empty server.

## Changing the surface

Do not edit `internal/catalog/generated.yaml`; it is generated. Add or move an
operation by editing `api/toolsets.yaml` (the tag map, an `operation_toolsets`
override, or an `extras` entry), choose whether it belongs to the default set in
`api/default-tools.yaml`, then run:

```sh
make catalog
go test ./tools/cataloggen
```

`TestBuildMatchesCommittedFile` fails when the committed file drifts from a fresh
run, so the diff is the review surface.

`tool_overrides` in the same file sets the `description` and the read/write hints of a
tool the generator cannot annotate correctly. It covers `container_exec`, the one tool
that needs two requests: it creates the exec, starts it, and returns what the command
printed instead of the bare exec id.
