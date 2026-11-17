> Русский перевод `docs/toolsets.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](toolsets.md)

# Наборы тулов

Каталог Portainer несёт 394 тула в одиннадцати наборах. Каждый тул — данные: имя,
описание, аннотации MCP, схема входа, необязательная схема выхода и HTTP-операция за
ним. Ни один обработчик этого не повторяет. Сам список лежит в
`internal/catalog/generated.yaml`; эта страница — его представление, и при
расхождении сервер регистрирует то, что сказано в сгенерированном файле.

У бинаря Arcane свой каталог — 445 тулов в семнадцати наборах, — который собирает тот
же генератор из отдельной пары файлов в `api/` и `internal/catalog/arcane/`. Он описан
в [Бэкенде Arcane](arcane.ru.md); всё сказанное ниже относится к каталогу Portainer.

## Откуда берутся тулы

| Файл | Роль |
|---|---|
| `api/portainer-2.45.yaml`, `api/docker-engine-1.57.yaml` | OpenAPI-документы, из которых выводится каждый тул |
| `api/toolsets.yaml` | Карта «тег → набор», переопределения по операциям, пропущенные пути, стриминговые пометки, пути `managedFields`, переименования и три сырых extra-тула |
| `api/default-tools.yaml` | 87 тулов, которые регистрируются без `--exhaustive` |
| `tools/cataloggen` (`make catalog`) | Читает четыре файла выше и пишет `internal/catalog/generated.yaml`; тот же прогон собирает каталог Arcane из его собственных входов (`-profile` собирает один из них) |
| `internal/catalog/catalog.yaml` | Имена наборов, описания и рёбра `depends_on`; списков тулов нет |
| `internal/catalog/generated.yaml` | Каждый тул каждого набора; вшивается в бинарь |

Оба документа вшиваются через `//go:embed` и сливаются при старте
(`internal/catalog/catalog.go`): курируемый документ даёт описания наборов и
зависимости, сгенерированный — тулы. Слияние отвергает набор, который есть только в
одном из них, и отвергает список тулов в `catalog.yaml`. Пара Arcane в
`internal/catalog/arcane/` вшивается и сливается так же, только через `LoadArcane`
вместо `Load`.

## Сводка

| Набор | Зависит от | Тулов | По умолчанию |
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

Граф зависимостей: `helm → k8s → core` и `gitops → stacks → docker → core`;
`edge`, `admin` и `observability` висят на `core`. Набор не включить без его
зависимостей, а `raw` не подтягивается ничем.

`--exhaustive` — это разница между двумя правыми столбцами: без него сервер
регистрирует только тулы с `default: true`, то есть курируемый набор из
`api/default-tools.yaml`. С ним регистрируется каждый тул включённых наборов.
`--read-only` фильтрует любую выборку до тулов с `readOnlyHint: true` на этапе
регистрации, поэтому клиент не видит тул, который ему нельзя вызывать.

## Как исполняется тул

Все тулы обслуживает один обработчик. `internal/mcpserver/tools.go` собирает запрос
из записи каталога и аргументов: параметры пути экранируются и подставляются,
остальные свойства входа становятся параметрами запроса (массивы — повторёнными
ключами или одним значением через запятую, где так сказано в каталоге), а свойство
тела уходит JSON-ом. Тело, которое документ объявляет плоским объектом из скаляров,
не оборачивается: его поля приходят отдельными аргументами, поэтому поиск принимает
`query`, а не `body: {query}`. Тело с вложенным объектом, массивом или свободными
ключами сохраняет обёртку. `internal/portainer/exec.go` выполняет вызов с `X-API-KEY`.
JSON-ответ приходит и текстом, и в `structuredContent`; у 237 тулов объявлена ещё и
схема выхода, которую клиент получает как `outputSchema`.

## `core` — 29 тулов, 7 по умолчанию

Идентичность и настройки сервера (`system_version`, `system_status`, `system_info`,
`whoami`, `motd`, `settings_get`, `settings_public` и записи `settings_update`,
`system_upgrade`), окружения (`environment_list`, `environment_inspect`,
создание, обновление и удаление `endpoint_*`, ассоциации, снимки, сводка) и реестры
эндпоинтов.

Набор по умолчанию — семь тулов идентичности и инвентаризации: `system_version`,
`system_status`, `system_info`, `settings_get`, `whoami`, `environment_list`,
`environment_inspect`.

## `docker` — 79 тулов, 13 по умолчанию

Контейнеры (`container_list`, `container_inspect`, `container_logs`, создание,
запуск, остановка, перезапуск, удаление, exec, stats, top, архивы, prune и
остальное), образы, сети, тома, плагины, distribution и системные вызовы демона.
Операции Docker идут через прокси Portainer; `environment` — обязательный аргумент,
переименованный из `environmentId` спецификации через `param_renames` в
`api/toolsets.yaml`.

Набор по умолчанию покрывает обычные чтения контейнеров и образов плюс записи
жизненного цикла: `container_list`, `container_inspect`, `container_logs`,
`container_create`, `container_start`, `container_stop`, `container_restart`,
`container_delete`, `container_exec`, `image_list`, `image_inspect`, `image_delete`,
`network_list`.

## `swarm` — 30 тулов, 6 по умолчанию

Сервисы, ноды, задачи, секреты, конфиги и вызовы членства в кластере (`swarm_init`,
`swarm_join`, `swarm_leave`, unlock). Набор по умолчанию: `service_list`,
`service_inspect`, `service_create`, `service_delete`, `node_list`, `task_list`.

## `k8s` — 105 тулов, 14 по умолчанию

Неймспейсы, поды, деплойменты, replica sets, jobs и cron jobs, config maps, секреты,
сервисы, ingress и ingress-контроллеры, persistent volumes и claims, storage classes,
resource quotas, роли и role bindings, service accounts, ноды и метрики, а также
dry-run манифестов и список дашбордов.

Набор по умолчанию: `kubernetes_namespaces_list`, `kubernetes_pods_list`,
`kubernetes_pod_logs`, `kubernetes_pod_delete`, `kubernetes_pod_restart`,
`kubernetes_deployment_scale`, `kubernetes_deployment_rollback`,
`kubernetes_config_maps_list`, `kubernetes_secrets_list`,
`kubernetes_services_list`, `kubernetes_cluster_ingresses_list`,
`kubernetes_persistent_volume_claims_list`, `kubernetes_nodes_list`, `node_drain`.

## `helm` — 11 тулов, 6 по умолчанию

Репозитории (`helm_user_repositories_list`, создание и удаление), список и осмотр
релизов, история, поиск и показ чартов и записи жизненного цикла `helm_install`,
`helm_rollback`, `helm_delete`. Набор по умолчанию: `helm_list`, `helm_inspect`,
`helm_install`, `helm_rollback`, `helm_delete`, `helm_user_repositories_list`.

## `stacks` — 36 тулов, 9 по умолчанию

Стеки Portainer для Docker standalone, Docker Swarm и Kubernetes, созданные из
файла, git-репозитория, строки или URL; `stack_list`, `stack_inspect`,
`stack_file_inspect`, запуск, остановка, обновление, миграция, git-редеплой и
удаление; вебхуки; пользовательские шаблоны.

Набор по умолчанию: `stack_list`, `stack_inspect`, `stack_start`, `stack_stop`,
`stack_update`, `stack_delete`, `stack_create_docker_standalone_file`,
`stack_create_docker_standalone_repository`,
`stack_create_docker_standalone_string`.

## `gitops` — 14 тулов, 5 по умолчанию

Источники GitOps (список, осмотр, создание, обновление, удаление, проверка),
workflows (список, осмотр, сводка) и предпросмотр файла репозитория. Набор по
умолчанию: `gitops_sources_list`, `gitops_source_inspect`,
`gitops_sources_create_git`, `gitops_sources_delete`, `gitops_sources_test`.

## `edge` — 28 тулов, 8 по умолчанию

Edge-группы, edge-стеки, edge-задачи и задачи заданий, а также edge-статус
окружения. Набор по умолчанию: `edge_group_list`, `edge_stack_list`,
`edge_stack_inspect`, `edge_stack_file`, `edge_job_list`, `edge_job_tasks_list`,
`edge_job_task_logs_inspect`, `endpoint_edge_status_inspect`.

## `admin` — 55 тулов, 12 по умолчанию

Пользователи, команды и членство, роли, реестры, группы эндпоинтов, resource
controls, теги, резервные копии и восстановление, проверка LDAP, настройки SSL,
шаблоны и вызовы аутентификации. Набор по умолчанию: `user_list`, `user_inspect`,
`user_create`, `user_delete`, `team_list`, `team_create`, `team_membership_create`,
`role_list`, `endpoint_group_list`, `registry_list`, `tag_list`,
`resource_control_create`.

`backup` возвращает архив резервной копии сервера: считайте ответ чувствительным.

## `observability` — 4 тула, все по умолчанию

`status_inspect`, `system_events`, `kubernetes_events` и
`kubernetes_events_namespace_list`. Два потока событий принимают временное окно и
возвращают то, что в нём произошло.

## `raw` — 3 тула, все по умолчанию

`portainer_raw`, `docker_raw`, `k8s_raw` — один произвольный запрос на каждую цель,
с методом, путём, query и JSON-телом по выбору вызывающего. Путь дописывается к
`/api`, `/api/endpoints/{environment}/docker` или
`/api/endpoints/{environment}/kubernetes`; `docker_raw` и `k8s_raw` требуют
`environment`.
Все три помечены как записи, потому что сырой запрос может изменить что угодно, и ни
один не подтягивается как зависимость. Включение `raw` — явное решение оператора.

Мультипарт-операции генератор помечает `form: true`. Их `body` уходит как
`multipart/form-data`, а значение вида `{filename, content}` или `{filename, base64}`
становится файловой частью; остальные ключи — текстовые поля.

## Стриминговые операции

Семь тулов не отвечают сами по себе. Четыре стримят, когда параметр истинен:
`container_logs`, `service_logs`, `task_logs` и `kubernetes_pod_logs` несут параметр
`follow`. Три стримят по пути независимо от аргументов: `system_events`,
`container_attach` и `exec_start`.

Вызов, который висел бы до таймаута клиента, отвергается до отправки запроса, а
сообщение называет предел, который надо поменять: передайте `follow=false` или
ограничьте окно через `until`. Пометки живут в `streaming_params` и
`streaming_paths` в `api/toolsets.yaml`, а отказ — в одном месте, поэтому покрывает
каждый тул, помеченный каталогом.

## Метаданные Kubernetes

Ответы Kubernetes повторяют `metadata.managedFields` в каждом объекте, и это
большая часть их объёма. 116 тулов, чей путь операции содержит `/kubernetes/`,
помечены генератором `strip_managed_fields: true`; до того как ответ дойдёт до
модели, поле вырезается на любой глубине. Тулы вне этого пути, включая `k8s_raw`,
сохраняют ответ как он пришёл.

## Большие ответы

Тело ответа больше 24 КиБ отдаётся страницей, даже если пришло целиком: массив
верхнего уровня становится `{items, _truncated, _total, _hint}`, а список в поле
`data` сохраняет конверт и получает счётчики. Бюджет держит один ответ в пределах
примерно шести тысяч токенов и ниже 32 КиБ, которые шлюз пропускает целиком.

Тело ответа обрезается на 1 МиБ. Если обрезанное тело — JSON-массив, тул отдаёт
страницу вместо сломанного префикса: первые 50 элементов плюс `_truncated`, `_total`
и `_hint`, называющий тул, которым список можно сузить. Обрезанное тело не-массива
сохраняет простой маркер `…[truncated at N bytes: ask for a narrower filter or a
smaller range]`.

## Правила выбора

- Пустой список `toolsets` — ошибка; хотя бы один набор должен быть назван.
- Имя, которого нет в каталоге, отвергается со списком известных имён.
- Не включённая зависимость отвергается:
  `toolset "helm" requires "k8s", which is not enabled`.
- При `--read-only` или `server.read_only` write-тулы отбрасываются на этапе
  регистрации. `tools/list` у клиента их не показывает.
- Если выборка осталась без тулов, старт падает с
  `no tools selected for toolsets ...`, а не отдаёт пустой сервер.

## Изменение поверхности

Не правьте `internal/catalog/generated.yaml`: он сгенерирован. Добавить или
перенести операцию — правка `api/toolsets.yaml` (карта тегов, переопределение в
`operation_toolsets` или запись в `extras`), выбор принадлежности к набору по
умолчанию — правка `api/default-tools.yaml`, затем:

```sh
make catalog
go test ./tools/cataloggen
```

`TestBuildMatchesCommittedFile` падает, когда закоммиченный файл расходится со свежим
прогоном, так что дифф и есть поверхность ревью.

`tool_overrides` в том же файле задаёт `description` и подсказки чтения/записи для
тула, который генератор размечает неверно. Сейчас он покрывает `container_exec` —
единственный тул из двух запросов: создаёт exec, запускает его и возвращает то, что
напечатала команда, а не голый id.
