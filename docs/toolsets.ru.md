> Русский перевод `docs/toolsets.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](toolsets.md)

# Наборы тулов

Каталог объявляет 113 операций в одиннадцати наборах. У каждой операции есть имя,
описание, аннотации и необязательные схемы входа и выхода, и больше нигде в коде
они не повторяются. Эта страница — представление
`internal/catalog/catalog.yaml`; если они расходятся, сервер регистрирует то, что
лежит в каталоге.

## Статус

Обработчики есть только у `core`. Остальные десять наборов объявлены со всеми
операциями и аннотациями, но выбор такого набора останавливает сервер:

```text
toolset "docker" not implemented yet (stage 0 implements: core)
```

Это осознанно: тул, который не может ответить, хуже отсутствующего, потому что
модель его вызовет. Запись в каталоге заодно служит спецификацией для этапа, который
её реализует.

## Сводка

| Набор | Зависит от | Операций | Read-only | Write | Статус |
|---|---|---|---|---|---|
| `core` | — | 7 | 7 | 0 | Реализован |
| `docker` | `core` | 24 | 11 | 13 | Объявлен на этап 1 |
| `swarm` | `docker` | 7 | 4 | 3 | Объявлен на этап 1 |
| `k8s` | `core` | 17 | 11 | 6 | Объявлен на этап 3 |
| `helm` | `k8s` | 8 | 4 | 4 | Объявлен на этап 3 |
| `stacks` | `docker` | 13 | 5 | 8 | Объявлен на этап 2 |
| `gitops` | `stacks` | 6 | 4 | 2 | Объявлен на этап 2 |
| `edge` | `core` | 8 | 8 | 0 | Объявлен на этап 4 |
| `admin` | `core` | 16 | 11 | 5 | Объявлен на этап 4 |
| `observability` | `core` | 4 | 4 | 0 | Объявлен на этап 4 |
| `raw` | — | 3 | 0 | 3 | Объявлен на этап 4 |

Граф зависимостей: `helm → k8s → core` и `gitops → stacks → docker → core`;
`edge`, `admin` и `observability` висят на `core`. Набор не включить без его
зависимостей, а `raw` не подтягивается ничем.

## `core` — реализован

Идентичность сервера и инвентаризация, всё read-only, всё поверх собственного API
Portainer:

| Тул | Что делает |
|---|---|
| `system_version` | `GET /api/system/version` — версия, версия API, редакция |
| `system_status` | `GET /api/system/status` — живость сервера Portainer |
| `system_info` | `GET /api/system/info` — счётчики хоста и платформы |
| `settings_get` | `GET /api/settings` — настройки сервера |
| `whoami` | `GET /api/users/me` — пользователь за API-ключом |
| `environment_list` | `GET /api/endpoints`, сведённый к id, имени, типу, статусу и URL |
| `environment_inspect` | `GET /api/endpoints/{id}` с историей снимков; принимает целочисленный `id` |

`environment_list` переводит числовой тип окружения Portainer в `docker`, `agent`,
`azure`, `edge-agent`, `kubernetes-local`, `kubernetes`, `edge-kubernetes` или
`unknown(<n>)`, а статус — в `up`, `down` или `unknown(<n>)`. Остальные тулы `core`
возвращают ответ Portainer как он пришёл.

## `docker`

Контейнеры: `docker_container_list`, `docker_container_inspect`,
`docker_container_create`, `docker_container_start`, `docker_container_stop`,
`docker_container_restart`, `docker_container_remove`, `docker_container_logs`,
`docker_container_stats`, `docker_container_top`, `docker_container_exec`.

Образы: `docker_image_list`, `docker_image_pull`, `docker_image_remove`.

Сети: `docker_network_list`, `docker_network_create`, `docker_network_connect`,
`docker_network_remove`.

Тома: `docker_volume_list`, `docker_volume_create`, `docker_volume_remove`.

Демон: `docker_system_df`, `docker_system_info`, `docker_system_events`.

Тринадцать из двадцати четырёх меняют состояние. Создание, запуск и скачивание —
единственные write-тулы без `destructiveHint: true`; остановка, перезапуск,
удаление, exec, подключение сети и удаление образов, томов и сетей помечены как
разрушительные.

## `swarm`

Сервисы: `swarm_service_list`, `swarm_service_inspect`, `swarm_service_create`,
`swarm_service_scale`, `swarm_service_remove`. Кластер: `swarm_node_list`,
`swarm_task_list`. Создание, масштабирование и удаление сервисов — записи.

## `k8s`

Неймспейсы и рабочие нагрузки: `k8s_namespace_list`, `k8s_pod_list`, `k8s_pod_logs`,
`k8s_pod_delete`, `k8s_deployment_scale`, `k8s_deployment_restart`,
`k8s_deployment_rollback`.

Конфигурация и сеть: `k8s_service_list`, `k8s_ingress_list`, `k8s_configmap_list`,
`k8s_secret_list`, `k8s_pvc_list`, `k8s_kubeconfig_get`.

Ноды и события: `k8s_node_drain`, `k8s_node_cordon`, `k8s_event_list`,
`k8s_custom_resource_list`.

Удаление пода, масштабирование, перезапуск и откат деплоймента, drain и cordon
ноды — это шесть записей. `k8s_secret_list` возвращает только метаданные секретов,
никогда значения; `k8s_kubeconfig_get` возвращает kubeconfig, то есть учётные
данные — тяните его только ключом, чьим правам доверяете.

## `helm`

`helm_repository_list`, `helm_chart_search`, `helm_release_list`,
`helm_release_values` — чтения; `helm_release_install`, `helm_release_upgrade`,
`helm_release_rollback`, `helm_release_uninstall` — записи.

## `stacks`

Чтения: `stack_list`, `stack_inspect`, `stack_file_get`, `stack_images` и
`stack_webhook`, который отдаёт URL вебхука стека.

Записи уровня аккаунта: `stack_create_file`, `stack_create_git`,
`stack_create_string`, `stack_update`, `stack_start`, `stack_stop`, `stack_remove`,
`stack_migrate`.

## `gitops`

`gitops_source_list`, `gitops_source_add`, `gitops_source_remove`,
`gitops_source_test`, `gitops_file_preview`, `gitops_workflow_list`. Добавление и
удаление источника — записи.

## `edge`

Все восемь — чтения: `edge_group_list`, `edge_stack_list`, `edge_job_list`,
`edge_job_logs`, `edge_config_list`, `edge_key_list`, `edge_wait_for_command`,
`edge_update_schedule_list`. `edge_wait_for_command` ждёт команду для edge-агента,
так что может висеть всё время ожидания.

## `admin`

Пользователи и команды: `admin_user_list`, `admin_user_inspect`, `admin_user_create`,
`admin_user_remove`, `admin_team_list`, `admin_team_create`, `admin_membership_add`,
`admin_role_list`.

Инвентаризация и настройки: `admin_endpoint_group_list`,
`admin_resource_control_list`, `admin_registry_list`, `admin_tag_list`,
`admin_backup_get`, `admin_ldap_get`, `admin_ssl_get`, `admin_settings_update`.

Создание и удаление пользователей, добавление в команды и обновление настроек — это
пять записей. `admin_backup_get` возвращает архив резервной копии сервера: считайте
ответ чувствительным.

## `observability`

Четыре чтения: `observability_user_activity`, `observability_endpoint_activity`,
`observability_docker_events`, `observability_k8s_events`. Два потока событий
принимают временное окно и возвращают то, что в нём произошло.

## `raw`

`portainer_raw`, `docker_raw`, `k8s_raw` — один произвольный запрос на каждую цель,
с методом, путём и телом по выбору вызывающего. Все три помечены как записи, потому
что сырой запрос может изменить что угодно, и ни один не подтягивается как
зависимость. Включение `raw` — явное решение оператора.

## Правила выбора

- Пустой список `toolsets` — ошибка; хотя бы один набор должен быть назван.
- Имя, которого нет в каталоге, отвергается со списком известных имён.
- Не включённая зависимость отвергается: `toolset "helm" requires "k8s", which is
  not enabled`.
- При `--read-only` или `server.read_only` write-тулы отбрасываются на этапе
  регистрации. `tools/list` у клиента их не показывает; проверки внутри обработчика
  нет.
- Включение объявленного набора без обработчиков — ошибка, а не пустой набор.

Защита на этапе компиляции — карта реализованных наборов в
`internal/mcpserver/server.go`; чтобы набор заработал, к обработчикам добавляют его
имя, и ошибка старта исчезает сама.
