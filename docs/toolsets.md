[Documentation index](index.md) · [Русский](toolsets.ru.md)

# Toolsets

The catalog declares 113 operations across eleven toolsets. Every operation has a
name, a description, annotations and optional input and output schemas, and nothing
else in the code restates them. This page is a view of
`internal/catalog/catalog.yaml`; when the two disagree, the catalog is what the
server registers.

## Status

Only `core` has handlers. The other ten sets are declared with all of their
operations and annotations, but selecting one stops the server:

```text
toolset "docker" not implemented yet (stage 0 implements: core)
```

That is deliberate: a tool that cannot answer is worse than an absent tool, because
a model will call it. The catalog entry doubles as the specification for the stage
that implements it.

## Summary

| Toolset | Depends on | Operations | Read-only | Write | Status |
|---|---|---|---|---|---|
| `core` | — | 7 | 7 | 0 | Implemented |
| `docker` | `core` | 24 | 11 | 13 | Declared for stage 1 |
| `swarm` | `docker` | 7 | 4 | 3 | Declared for stage 1 |
| `k8s` | `core` | 17 | 11 | 6 | Declared for stage 3 |
| `helm` | `k8s` | 8 | 4 | 4 | Declared for stage 3 |
| `stacks` | `docker` | 13 | 5 | 8 | Declared for stage 2 |
| `gitops` | `stacks` | 6 | 4 | 2 | Declared for stage 2 |
| `edge` | `core` | 8 | 8 | 0 | Declared for stage 4 |
| `admin` | `core` | 16 | 11 | 5 | Declared for stage 4 |
| `observability` | `core` | 4 | 4 | 0 | Declared for stage 4 |
| `raw` | — | 3 | 0 | 3 | Declared for stage 4 |

The dependency graph is `helm → k8s → core` and `gitops → stacks → docker → core`;
`edge`, `admin` and `observability` hang off `core`. A set cannot be enabled without
its dependencies, and `raw` is never pulled in by anything.

## `core` — implemented

Server identity and inventory, all read-only, all backed by Portainer's own API:

| Tool | Effect |
|---|---|
| `system_version` | `GET /api/system/version` — version, API version, edition |
| `system_status` | `GET /api/system/status` — liveness of the Portainer server |
| `system_info` | `GET /api/system/info` — host and platform counters |
| `settings_get` | `GET /api/settings` — server settings |
| `whoami` | `GET /api/users/me` — the user behind the API key |
| `environment_list` | `GET /api/endpoints`, reduced to id, name, type, status and URL |
| `environment_inspect` | `GET /api/endpoints/{id}` with snapshot history; takes an integer `id` |

`environment_list` translates Portainer's numeric environment type into `docker`,
`agent`, `azure`, `edge-agent`, `kubernetes-local`, `kubernetes`, `edge-kubernetes`,
or `unknown(<n>)`, and status into `up`, `down` or `unknown(<n>)`. Other `core` tools
return the Portainer response as it arrives.

## `docker`

Containers: `docker_container_list`, `docker_container_inspect`,
`docker_container_create`, `docker_container_start`, `docker_container_stop`,
`docker_container_restart`, `docker_container_remove`, `docker_container_logs`,
`docker_container_stats`, `docker_container_top`, `docker_container_exec`.

Images: `docker_image_list`, `docker_image_pull`, `docker_image_remove`.

Networks: `docker_network_list`, `docker_network_create`, `docker_network_connect`,
`docker_network_remove`.

Volumes: `docker_volume_list`, `docker_volume_create`, `docker_volume_remove`.

Daemon: `docker_system_df`, `docker_system_info`, `docker_system_events`.

Thirteen of the twenty-four change state. Creating, starting and pulling are the only
writes without `destructiveHint: true`; stopping, restarting, removing, exec-ing,
connecting a network and removing images, volumes and networks are marked
destructive.

## `swarm`

Services: `swarm_service_list`, `swarm_service_inspect`, `swarm_service_create`,
`swarm_service_scale`, `swarm_service_remove`. Cluster: `swarm_node_list`,
`swarm_task_list`. Creating, scaling and removing services are writes.

## `k8s`

Namespaces and workloads: `k8s_namespace_list`, `k8s_pod_list`, `k8s_pod_logs`,
`k8s_pod_delete`, `k8s_deployment_scale`, `k8s_deployment_restart`,
`k8s_deployment_rollback`.

Configuration and networking: `k8s_service_list`, `k8s_ingress_list`,
`k8s_configmap_list`, `k8s_secret_list`, `k8s_pvc_list`, `k8s_kubeconfig_get`.

Nodes and events: `k8s_node_drain`, `k8s_node_cordon`, `k8s_event_list`,
`k8s_custom_resource_list`.

Deleting a pod, scaling, restarting or rolling back a deployment, draining or
cordoning a node are the six writes. `k8s_secret_list` returns secret metadata only,
never values; `k8s_kubeconfig_get` returns a kubeconfig, which is credential
material — fetch it only with an API key whose permissions you trust.

## `helm`

`helm_repository_list`, `helm_chart_search`, `helm_release_list`,
`helm_release_values` are reads; `helm_release_install`, `helm_release_upgrade`,
`helm_release_rollback`, `helm_release_uninstall` are writes.

## `stacks`

Reads: `stack_list`, `stack_inspect`, `stack_file_get`, `stack_images`, and
`stack_webhook`, which returns the webhook URL of a stack.

Account-level writes: `stack_create_file`, `stack_create_git`, `stack_create_string`,
`stack_update`, `stack_start`, `stack_stop`, `stack_remove`, `stack_migrate`.

## `gitops`

`gitops_source_list`, `gitops_source_add`, `gitops_source_remove`,
`gitops_source_test`, `gitops_file_preview`, `gitops_workflow_list`. Adding or
removing a source is a write.

## `edge`

All eight are reads: `edge_group_list`, `edge_stack_list`, `edge_job_list`,
`edge_job_logs`, `edge_config_list`, `edge_key_list`, `edge_wait_for_command`,
`edge_update_schedule_list`. `edge_wait_for_command` waits for a command to reach an
edge environment, so it may block for the duration of that wait.

## `admin`

Users and teams: `admin_user_list`, `admin_user_inspect`, `admin_user_create`,
`admin_user_remove`, `admin_team_list`, `admin_team_create`, `admin_membership_add`,
`admin_role_list`.

Inventory and settings: `admin_endpoint_group_list`, `admin_resource_control_list`,
`admin_registry_list`, `admin_tag_list`, `admin_backup_get`, `admin_ldap_get`,
`admin_ssl_get`, `admin_settings_update`.

Creating and removing users, adding memberships and updating settings are the five
writes. `admin_backup_get` returns a server backup archive: treat the response as
sensitive.

## `observability`

Four reads: `observability_user_activity`, `observability_endpoint_activity`,
`observability_docker_events`, `observability_k8s_events`. The two event streams take
a time window and return what happened in it.

## `raw`

`portainer_raw`, `docker_raw`, `k8s_raw` — one arbitrary request per target, with the
method, path and body chosen by the caller. All three are writes by annotation
because a raw request may change anything, and none of them is pulled in as a
dependency. Enabling `raw` is an explicit decision by the operator.

## Selection rules

- An empty `toolsets` list is an error; at least one set must be named.
- A name that is not in the catalog is rejected with the list of known names.
- A dependency that is not enabled is rejected: `toolset "helm" requires "k8s",
  which is not enabled`.
- With `--read-only` or `server.read_only`, write tools are dropped at registration
  time. The client's `tools/list` never shows them; there is no runtime check inside
  a handler.
- Enabling a declared set with no handlers is an error, not an empty toolset.

The compile-time guard is a map of implemented toolsets in
`internal/mcpserver/server.go`; adding handlers for a set means adding its name
there, and the startup error disappears on its own.
