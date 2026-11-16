Status: historical research note (RU-only by project policy), 2026-11-16

# Portainer MCP: состояние рынка и проект своего сервера

Дата: 2026-11-16. Метод: первоисточники (GitHub API, PyPI, OpenAPI-спеки Portainer 2.45 и Docker Engine 1.57, live probe против self-hosted Portainer EE 2.45.3), плюс чтение кода обоих серверов.

## 0. Приговор

| Вопрос | Ответ |
|---|---|
| Официальный сервер жив? | Формально да, фактически **объявлен неподдерживаемым** 2026-10-08 и заморожен на Portainer 2.45.3; вендор продаёт замену (Portainer Command, платно). |
| Он покрывает всё? | Нет: 239 тулов из 475, Docker — только сырым прокси, Kubernetes — без exec/port-forward, 97 операций API недостижимы ни одним профилем, Edge/Admin выключены по умолчанию. |
| Актуальная спека MCP? | Нет: официальный живёт на ревизии `2025-11-25` (handshake), текущая — `2026-07-28` (без сессий, `server/discover`, `resultType`). Плюс нет `outputSchema`/`structuredContent`, `destructiveHint`, resources/prompts, пагинации. |
| Альтернативы? | Ни одна не покрывает k8s + edge + gitops. Единственная Rust-попытка — 12 тулов, compose-стеки. |
| Свой сервер нужен? | Да, но **не «переписать 475 тулов»**, а «сгенерировать полное покрытие + тонкий набор глагольных тулов + наборы-переключатели». Go, не Rust. |

## 1. Официальный Portainer MCP: что с ним стало

Репозиторий: https://github.com/portainer/portainer-mcp — MIT, 242★, 48 форков, создан 2025-03-09, 88 коммитов, 24 релиза.

- **Последний коммит:** 2026-10-23 («Release 2.45.3»).
- **Коммит-эпитафия:** 2026-10-08 (#114) *«deprecate in favour of Portainer Command»*. В `README.md:5-9`: *«no longer the supported way»*, *«no SLA, no further feature development, no roadmap commitment»*, *«may be archived in the future»*.
- **Замена:** Portainer Command (BE, free 3 node) — read-only сессии с TTL, изменения только как git-PR с апрувом. Открытого MCP у вендора больше не будет.
- **Заморозка:** спека EE 2.45.3, при том что Portainer 3.x показан 2026-10-20; ветка `develop` в `portainer/portainer` заархивирована (`archive/develop-2026-10`), 3.x в публичном репо отсутствует.
- **Открыто:** 7 issues, 2 PR; PR #82 «budget the tool catalog» висит с 2026-07-27.

Генеалогия важна, потому что в разных обзорах фигурируют разные языки:

| Период | Реализация |
|---|---|
| до v0.7.0 (2026-02-26) | Go 1.24.2, `mark3labs/mcp-go v0.32.0`, `portainer/client-api-go/v2 v2.31.2`, testcontainers |
| с 2.41.0 (2026-05-19) | Python ≥3.11, **FastMCP (PrefectHQ) ≥3.4.2**, httpx, jmespath, pyyaml |

Текущий сервер **не пишет тулы руками**: `FastMCP.from_openapi(...)` на вшитой `data/portainer-patched.yaml` (`src/portainer_mcp/server.py:413`), руками сделаны только `docker_proxy`, `kubernetes_proxy` (`proxy.py:198-296`) и `get_guidance`.

Docker Hub `portainer/portainer-mcp`: 108 597 pulls. PyPI `mcp-portainer` 2.45.3: 4 635 загрузок/мес.

## 2. Live probe: что реально отдаёт существующий инстанс

- Официальный сервер, запущенный как `uvx --from mcp-portainer mcp-portainer` с `PORTAINER_PROFILES=BASE,DOCKER` и `PORTAINER_TLS_VERIFY=0` (самоподписанный сертификат), отдал **105 тулов** — это практический размер «профиля» против 475 в `ALL`.
- Проверенный инстанс: **Portainer EE 2.45.3 LTS, build 45**, Go 1.26.10, Docker 28.5.2, Compose v2.40.3, Helm v4.1.4, kubectl v1.35.1.
- Одно окружение (`docker`, unix socket) → 23 контейнера, 202 тома, 23 образа, 15 стеков; Docker Engine **29.9.2, API 1.56**.
- Проверено вызовом: `EndpointList` работает; `docker_proxy GET /version` → `ApiVersion 1.56`; `docker_proxy GET /networks` → список сетей. То есть Docker API доступен целиком через прокси.

## 3. Аудит официального сервера: двенадцать дефектов

1. **Снят с поддержки вендором** (README:5-9, #114). Вся автоматизация на нём — на мёртвой ветке.
2. **Нет плана на Portainer 3.x**; спека заморожена на 2.45.3 (24 релиза, все — 2.x).
3. **Docker не типизирован**: в Portainer-спеке 3 операции тега `docker` (`/gpus`, `/dashboard`, `/images`); start/stop/logs/exec/stats/pull/delete/сети/тома — только через `docker_proxy`, то есть без схемы, аннотаций и structured output.
4. **`docker_proxy`/`kubernetes_proxy` сломаны для конформных клиентов** — issue #112 (2026-09-17): *«outputSchema defined but no structured output returned»*. Единственный обход непокрытого API не работает по спеке.
5. **97 операций (20,6 %) недостижимы ни одним профилем** — 15 сиротских тегов (`docs/profiles.md:41-57`): addons 20, observability 18, omni 14, cloud_credentials 11, custom_templates 10, webhooks 6, useractivity 4, support 3, templates 2, kaas 1. Только через `PORTAINER_TAGS_EXTRA`/`ALL`.
6. **Профили не защищают от прокси**: `docker_proxy`/`kubernetes_proxy` регистрируются безусловно (`server.py:429-432`), поэтому профиль `BASE` даёт полный write к Docker и K8s API. Спасает только `PORTAINER_NO_PROXY=1`.
7. **Нет exec/attach/port-forward**: `/websocket/*` вырезаны из спеки (`spec/patch_spec.py:1-8`), прокси — обычный httpx без стриминга (`proxy.py:164-170`). Логи — только снимком, `follow` нет.
8. **Нет `outputSchema`/`structuredContent`** (`validate_output=False`, `server.py:419`), аннотации — только `readOnlyHint` (`server.py:159-165`), `destructiveHint`/`idempotentHint`/`title` пустые. **Ревизия протокола — `2025-11-25`** (константа в python-SDK), то есть на одну назад от текущей `2026-07-28`: сервер всё ещё на handshake-протоколе, без `server/discover` и `resultType`.
9. **Нет resources, prompts, elicitation** (проверено `list_tools`/`list_resources`/`list_prompts` — 0/0/0).
10. **Нет мультиинстансности**: один `PORTAINER_URL` на процесс (`server.py:320`).
11. **Каталог не масштабируется**: 239 тулов с длинным `select` в каждом описании, PR #82 «budget the tool catalog»; при `ALL` — 475 тулов в одном `tools/list`. Имена — сырые operationId со скачущим регистром (`StackList`, `systemInfo`, `ServiceImageStatus`, `Inspect_an_EnvironmentEndpoint_group`).
12. **Мелочи, которые ломают доверие**: k8s-стеки попали в профиль DOCKER (общий тег `stacks`: `StackCreateKubernetesString`, `StackDeleteKubernetesByName`); guidance-турникет возвращает гайд вместо результата после 1800 с простоя (`server.py:451-465`); обрезка ответа 50 000 символов (`shaping.py:40-45`); вшита EE-спека, на CE EE-операции дают 404 (#110, #111).

Тесты/CI: 251 тест в 18 файлах, `pytest` в CI, но **интеграционных против живого Portainer нет** (testcontainers остались в Go-эпохе).

## 4. Альтернативы (gh search, 2026-11-16)

| Репозиторий | Язык | ★ | Last push | Тулов | k8s | Edge | GitOps | Лицензия |
|---|---|---|---|---|---|---|---|---|
| portainer/portainer-mcp | Python | 242 | 2026-10-23 | 475 (105 при профилях BASE+DOCKER) | 122 оп. | 54 | 24 | MIT |
| berry-13/portainer-mcp | TS | 0 | 2026-08-04 | 103 | — | — | — | **нет** |
| Wixely/PortainerMCPSharp | C# | 0 | 2026-11-09 | 79 | — | — | — | MIT |
| jmrplens/portainer-mcp-enhanced | Go | 8 | 2026-08-01 | 98 | частично | — | — | MIT |
| ginkida/portainer-mcp | Python | 2 | 2026-09-23 | 55 | — | — | — | MIT |
| jbkunama1/hAI.PortainerMCP | Python | 0 | 2026-11-13 | 16 | — | — | — | MIT |
| Guthman/portainer-mcp | Rust | 0 | 2026-11-09 | 12–13 | — | — | — | MIT |
| strnad/portainer-mcp | Go | 0 | 2026-02-17 | форк Go-эпохи | — | — | — | — |

Что ценного есть у соседей:

- **jmrplens/portainer-mcp-enhanced** (Go, 98 тулов) — лучший существующий Go-каркас: декларативный `tools.yaml` (280 ключей `name:`, полные аннотации `readOnlyHint/destructiveHint/idempotentHint/title`), `--read-only` на этапе регистрации, `internal/k8sutil/stripper.go` вырезает `managedFields` из k8s-ответов. Минусы: `mark3labs/mcp-go v0.32.0` (до официального SDK), нет наборов-переключателей, 98 тулов плоским списком, k8s/edge/gitops неполные.
- Мультиинстанс есть только у `jbkunama1` (алиасы + ключ на алиас) и `berry-13` (без лицензии).
- Prompts/resources есть только у Rust-попытки, но она умеет лишь compose-стеки.

**Вывод:** рынок пуст в точке «полное покрытие + переключаемые наборы + актуальная спека». Это и есть ниша своего сервера.

## 5. Актуальная спека MCP: ревизия `2026-07-28`

Проверено на modelcontextprotocol.io/specification/versioning и в реестре deprecated.

Хронология ревизий: `2024-11-05` → `2025-03-26` → `2025-06-18` → `2025-11-25` → **`2026-07-28` (current)**.

Ломающие изменения против `2025-06-18`/`2025-11-25` — их важно знать до первой строки кода:

1. **Сессий больше нет.** `initialize`/`initialized`, `Mcp-Session-Id` убраны; версия и capabilities идут в `_meta` каждого запроса (`io.modelcontextprotocol/protocolVersion`), на HTTP — ещё и в заголовке `MCP-Protocol-Version`. Сервер принимает/отклоняет каждый запрос независимо (`UnsupportedProtocolVersionError` со списком поддерживаемых).
2. **`server/discover` — обязательный RPC**: версии, capabilities, identity одним запросом.
3. **`GET`-стрима нет:** подписки — `subscriptions/listen` (долгий POST-ответ), `resources/subscribe|unsubscribe` переехали туда же.
4. **`ping` и `logging/setLevel` удалены**; уровень логов — per-request; на stdio логи идут в `stderr`.
5. **Tasks вынесены из ядра** в расширение `io.modelcontextprotocol/tasks` (поллинг `tasks/get`).
6. **MRTR**: серверные запросы к клиенту (в т.ч. elicitation) — через `InputRequiredResult` + `inputRequests`/`inputResponses`, а не отдельным каналом.
7. **`resultType` обязателен** в любом результате; возобновляемость SSE (`Last-Event-ID`) удалена.
8. Мелочи, которые ломают «почти совместимые» серверы: заголовки `Mcp-Method`/`Mcp-Name` на POST с ответом `400 HeaderMismatch` при расхождении с телом; кэш-контракт `ttlMs` + `cacheScope` на пяти методах; коды `-32020..-32022`; OTel-контекст в `_meta`; `x-mcp-header` для проброса аргумента в HTTP-заголовок.
9. **Deprecated** (окно ≥12 мес., снятие не раньше ревизии ≥ 2027-07-28): Roots, Sampling, Logging, DCR, HTTP+SSE.

Транспорты: **stdio** и **Streamable HTTP** (один POST-эндпоинт, ответ JSON или SSE на запрос, проверка `Origin`, без сессий и резюма). WebSocket в спеке нет — websocket только upstream, к Portainer.

Авторизация MCP (OAuth 2.1 resource server, RFC 9728, `401` + `WWW-Authenticate: Bearer resource_metadata=…`) к Portainer не относится: Portainer принимает `X-API-KEY` (в `openapi.yaml` — `apiKey`) либо `Authorization: Bearer` из `POST /api/auth`.

Чек-лист для своего сервера:

| Обязано | Можно (делаем) | Не делаем |
|---|---|---|
| `server/discover`, `tools/list` с курсорами и `ttlMs`/`cacheScope`, `tools/call` с `isError` для ошибок исполнения, `resultType`, `MCP-Protocol-Version` + `Mcp-Method`/`Mcp-Name`, коды `-32020..-32022` | `outputSchema` + `structuredContent` на всех list/inspect; аннотации `title`/`readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`; `resource_link` для полных логов; `progress` для долгих операций; `x-mcp-header`; elicitation через MRTR (подтверждение destructive, если клиент умеет) | roots, sampling, logging, DCR, HTTP+SSE (deprecated); tasks-расширение как обязательное (см. ниже); собственный OAuth к Portainer |

Про tasks: расширение стабильно, полная поддержка есть только в `rmcp` (Go-SDK — нет, issue #626), а **матрица клиентов пуста** — ни Claude, ни Copilot, ни Cursor его не объявляют. Поэтому долгие операции (pull образа, деплой стека, install helm) делаем синхронными с `progress`-нотификациями и щедрым таймаутом, а не на tasks.

`x-mcp-header` закрывает частную задачу красиво: в HTTP-режиме `X-Portainer-Api-Key` объявляется аргументом тула, привязанным к заголовку, — вместо самодельного парсинга заголовков в middleware.

## 6. Поверхность API: цифры, из которых вытекает архитектура

| Источник | Путей | Операций | Тегов |
|---|---|---|---|
| Portainer 2.45 (`api/docs/openapi.yaml`, ветка `release/2.45`) | 224 | 288 | 31 |
| Docker Engine API 1.57 (`moby/moby api/swagger.yaml`) | 98 | 108 | 15 |
| Агрегированная спека официального MCP (с патчами) | 366 | 472 | 44 |

Portainer 2.45 по тегам (операций): kubernetes 107, stacks 22, endpoints 19, gitops 14, users 14, edge_jobs 11, helm 11, edge_stacks 10, custom_templates 9, endpoint_groups 7, registries 7, edge_groups 5, system 5, team_memberships 5, teams 5, webhooks 5, websocket 4, auth 3, docker 3, edge_agent 3, resource_controls 3, settings 3, tags 3, backup 2, ssl 2, templates 2, ldap 1, motd 1, roles 1, status 1, upload 1.

Docker Engine 1.57: Container 25, Image 16, Plugin 11, System 7, Network 7, Swarm 7, Volume 6, Service 6, Secret 5, Config 5, Exec 4, Node 4, Task 3, Distribution 1, Session 1.

Три следствия:

1. **Portainer API почти ничего не знает про Docker.** Docker идёт через catch-all `/api/endpoints/{id}/docker/{...}` → Docker Engine API, а Kubernetes — через `/api/kubernetes/{id}/{...}` → k8s API. Значит «все возможности» = три источника, и в спецификации Portainer их покрытие отсутствует по определению.
2. **Полный типизированный Docker-слой — это 108 операций**, а не 3. Писать их руками в MCP бессмысленно: нужен курируемый слой + прокси как escape hatch.
3. **Стриминг живёт на websocket**: `/websocket/exec` (docker exec), `/websocket/attach`, `/websocket/pod`, `/websocket/kubernetes-shell`. Любой нормальный `logs --follow`, exec и shell требуют websocket-клиента, а не httpx.

Серверная сторона берёт на себя рендер compose (v2.40.3) и исполнение Helm (v4.1.4) — клиенту достаточно отправить payload. Это сильно сокращает то, что надо реализовывать: никакого docker/compose/helm внутри самого бинаря.

## 7. Архитектура своего сервера

Пять слоёв, каждый тонкий:

```
                 MCP (stdio | Streamable HTTP)
   ┌───────────────────────────────────────────────┐
   │ 5. Каталог как данные (catalog.yaml, embedded)│  имя, набор, схема,
   │    → регистрация в SDK с аннотациями          │  аннотации, destructive
   ├───────────────────────────────────────────────┤
   │ 4. Регистрация: фильтр по наборам, read-only, │
   │    пагинация tools/list, prompts/resources    │
   ├───────────────────────────────────────────────┤
   │ 3. Курируемые домены (руками):                │
   │    docker · k8s · exec/logs (websocket) ·     │
   │    stacks · helm · edge                       │
   ├───────────────────────────────────────────────┤
   │ 2. Сгенерированный клиент Portainer OpenAPI   │
   │    (288 операций) + generic request layer     │
   ├───────────────────────────────────────────────┤
   │ 1. Транспорт/аутентификация: API key, TLS,    │
   │    мультиинстанс, кэш окружений, ретраи       │
   └───────────────────────────────────────────────┘
```

Решения и почему:

- **Каталог — данные, не код.** Один `catalog.yaml` (наборы, имена, описания «когда использовать», аннотации, `destructive`). Из него генерируется и `tools/list`, и валидация на старте (дубли имён, тулы без набора, write-тулы без `destructiveHint`). Так каталог нельзя «забыть» обновить при правке обработчика — есть тест паритета.
- **Генерация из OpenAPI — для длинного хвоста.** `oapi-codegen` даёт типизированный клиент Portainer (288 операций). Курируемый слой только там, где OpenAPI слаб: Docker (нет в спеке), k8s (нет в спеке), websocket.
- **Escape hatch вместо 475 тулов.** Три «сырых» тула (`portainer_raw`, `docker_raw`, `k8s_raw`) с whitelist методов: GET всегда, write — только вне read-only, пути — с обязательным префиксом. Секреты не фильтруем, но помечаем `destructiveHint: true` и `openWorldHint: true`. Это даёт 100 % покрытие без раздувания каталога.
- **Стриминг — websocket.** `exec`, `logs --follow`, k8s shell: отдельный пакет, вывод чанками в MCP progress-нотификации, финальный ответ — структурированный (`exit_code`, `stdout_tail`), полный лог — `resource_link` на временный файл.
- **Мультиинстанс с первого дня.** Конфиг — список `instances: [{name, url, api_key, tls, ca}]`; параметр `instance` есть у каждого тула и по умолчанию = первый. Это дешевле сделать сразу, чем потом менять схему всех тулов.
- **Аутентификация:** stdio — ключ из env; HTTP — passthrough `X-Portainer-Api-Key` каждого клиента с валидацией `GET /users/me` и TTL-кэшем 60 с (как у официального, это правильная схема), плюс гейт `MCP_AUTH_TOKEN` на сам сервер. OAuth к Portainer не нужен: API key — штатный механизм.
- **noise control:** проекции по умолчанию (как `select` у официального, только типизированные), вырезание `managedFields`/`last-applied-configuration` для k8s, `ComposeVersion`-мусор из docker-ответов, лимит на размер ответа с явной ошибкой «сузь проекцию» вместо молчаливой обрезки на 50 000 символов.
- **Кэш:** окружения/настройки/снапшоты — TTL 30–60 с; инвалидация по write-тулам своего же набора.
- **Протокол `2026-07-28` даёт бесплатно две вещи:** нет сессий → в HTTP-режиме сервер stateless, пасстхру ключа на запрос ложится естественно; и контракт кэша `tools/list` (`ttlMs: 3600000`, `cacheScope: "global"`) — каталог статичен, клиенту незачем его перезапрашивать. Обратная сторона: `resultType` обязателен в каждом результате, а `Mcp-Method`/`Mcp-Name` в заголовках обязаны совпадать с телом, иначе `400 HeaderMismatch` — это тесты, а не «поправим позже».
- **Кэш окружений — на ключ, не глобальный.** В HTTP-режиме у каждого клиента свой API key, и общий кэш списка окружений утёк бы между арендаторами.

## 8. Наборы тулов

Наборы — не теги OpenAPI (это и есть главная ошибка официального: сетка тегов даёт сирот и протечки), а **осмысленные домены с явными зависимостями**. Объединение — 87 тулов вместо 475, каждый набор 4–14 тулов.

| Набор | Тулов | Зависит от | Что внутри | Источник |
|---|---:|---|---|---|
| `core` | 7 | — | системы/статус/версия/лицензия, настройки (чтение), `whoami`, окружения (list/inspect/snapshots) | Portainer API |
| `docker` | 13 | core | контейнеры: list/inspect/create/start/stop/restart/remove/logs/stats/top/exec; образы: list/pull/remove; сети: list/create/connect/remove; тома: list/create/remove; `df`/`info`/`events` | Docker Engine API через прокси |
| `swarm` | 6 | docker | services (list/inspect/create/scale/remove), nodes, tasks | Docker Engine API |
| `k8s` | 14 | core | namespaces, pods (list/logs/delete), deployments (scale/restart/rollback), services, ingresses, configmaps, secrets (метаданные), PVC, nodes (drain/cordon), events, CRD/кастомные ресурсы, kubeconfig | Portainer k8s API + прокси |
| `helm` | 6 | k8s | репозитории, поиск чартов, релизы (list/install/upgrade/rollback/uninstall), values | Portainer helm API |
| `stacks` | 9 | docker | list/inspect/create (file/git/string)/update/start/stop/remove/migrate/file/webhook/images | Portainer stacks API |
| `gitops` | 5 | stacks | sources (list/add/remove/test), file preview, workflows | Portainer gitops API |
| `edge` | 8 | core | группы, edge-стеки, edge-jobs (+логи), configs, ключи, wait-for-command, update schedules | Portainer edge API |
| `admin` | 12 | core | users, teams, memberships, roles, endpoint groups, resource controls, registries, tags, backup, LDAP, SSL, settings (запись) | Portainer API |
| `observability` | 4 | core | user activity, endpoint activity, docker events, k8s events | Portainer + прокси |
| `raw` | 3 | — | `portainer_raw`, `docker_raw`, `k8s_raw` (GET всегда; write вне read-only) | все три |

Типовые конфигурации:

```yaml
# homelab: докер + стеки, без k8s и админки
toolsets: [core, docker, stacks, gitops]

# кластер: всё, кроме edge/admin
toolsets: [core, docker, swarm, k8s, helm, stacks, gitops]

# аудит: только чтение
read_only: true
toolsets: [core, docker, k8s, helm, stacks, edge, admin, observability]
```

Правила:
- `read_only: true` **не регистрирует** write-тулы (а не отбивает их в рантайме) — клиент не видит того, что нельзя.
- `--toolsets a,b` (CLI) перекрывает конфиг; неизвестный набор — ошибка старта, а не молчаливое игнорирование.
- Зависимости проверяются на старте: `helm` без `k8s` не поднимается (иначе тулы ведут в 404).
- `raw` не включается зависимостями никогда — только явно.

### 8.1 Выбранная упаковка: C — гибрид (решение 2026-11-16)

По умолчанию — 87 глагольных тулов из таблицы выше. Флаг `--exhaustive` дополнительно регистрирует по одному тулу на операцию из сгенерированного клиента, только для включённых наборов (⇒ до ~580 при `ALL`): типизированная схема на каждую ручку, сырые имена `operationId` в snake_case, те же аннотации. Так «полное покрытие» не зависит от того, угадал ли автор нужный глагол.

- `--exhaustive` без `--toolsets` = то же, что `ALL` у официального, но с наборами и read-only, которых там нет.
- В exhaustive-режиме глагольные тулы остаются: они удобнее для модели, генератор — страховка.
- Тест паритета: сумма операций во включённых наборах = число тулов в exhaustive-каталоге; расхождение — ошибка старта, а не тихий пропуск.

## 9. Go или Rust

Факты по SDK (проверено 2026-11-16):

| | `modelcontextprotocol/go-sdk` | `rmcp` (Rust) |
|---|---|---|
| Версия / дата | v1.8.0, 2026-09-14 | 3.10.0, 2026-11-12 |
| Звёзды | 5 349 | 4 117 |
| `2026-07-28` + dual-era | да | да |
| `server/discover`, `subscriptions/listen` | да | да |
| structured output | дженерики `AddTool[In,Out]` | `schemars` + `Json<T>` |
| MRTR / `resultType` | да | да (+ `request-state`) |
| tasks-расширение | **нет** (#626, SEP-1686) | да, полностью |
| бинарь / старт (замер) | 7,8 МБ / 9–15 мс | 3,9 МБ / 5–7 мс |
| первая сборка | ~15 с | ~2,5 мин |

| Критерий | Go | Rust |
|---|---|---|
| Готовый клиент Portainer | `portainer/client-api-go/v2`, есть у обоих форков | нет |
| Docker Engine | официальные типы `moby/moby/api/types` | `bollard` (полный, своя модель) |
| k8s | прокси JSON, типы не нужны | `kube` — тяжёлый, для прокси избыточен |
| Генерация из OpenAPI | `oapi-codegen` v2.8.2 (8 667★), проверен на 288 операциях Portainer | `progenitor` 0.17.0-alpha.1 |
| Стримы (logs follow, exec, ws-attach) | горутины + websocket, ~20 строк | не сложнее, но диагностика в rmcp хуже (`WorkerQuitReason::Fatal` вместо причины) |
| Размер/старт бинаря | 7,8 МБ, ~11 мс | 3,9 МБ, ~6 мс |

**Вывод: Go.** Разница 4 МБ и 5 мс не стоит потери готового Portainer-клиента, официального MCP-SDK и двух форков с рабочим кодом (98 тулов, k8s-стриппер). Rust выигрывает только в tasks-расширении, а его не поддерживает ни один клиент — покупать нечего. Rust имел бы смысл при жёстком лимите памяти образа или отсутствии Go-тулчейна; в задаче нет ни того, ни другого.

Стратегия: **новый репозиторий с заимствованием** — `tools.yaml`-подход и k8s-стриппер взять у `jmrplens`, остальное писать заново (у него нет наборов, k8s/edge неполные, SDK устаревший — `mark3labs/mcp-go v0.32.0`).

## 10. План и ворота

| Этап | Что | Ворота |
|---|---|---|
| 0 | Каркас: конфиг, мультиинстанс, SDK, `tools/list`, golden-снимок каталога | `make check` = build + vet + `tools/list` совпадает с каталогом |
| 1 | `core` + `docker` (прокси + 13 курируемых тулов) | интеграционный прогон против живого 2.45.3: list/inspect/logs/exec на реальном контейнере, `read_only` не отдаёт write-тулы |
| 2 | `stacks` + `gitops` + `swarm` | деплой тестового стека из git, откат, удаление; стек не остаётся после теста |
| 3 | `k8s` + `helm` | список подов, логи, scale деплоймента, `helm list`; на кластере-пустышке |
| 4 | `edge` + `admin` + `observability` + `raw` | CRUD пользователя/тега в тестовой команде, бэкап в файл, `raw` отбивает DELETE в read-only |
| 5 | Упаковка: Dockerfile, `server.json` для реестра MCP, доки, CI | `mcp-server` стартует из контейнера, публикация в реестр |

Оценка (AI-скорость, один человек с ревью): этапы 0–2 — 2–3 дня работы агента плюс 1–2 часа ревью в день; этапы 3–4 — 2–3 дня, потому что нужен живой кластер и тестовая BE-команда (внешняя зависимость, ждёт человека); упаковка — полдня. Итого **~1 неделя календарно**, доминирует не код, а доступы (кластер, BE-лицензия, тестовая команда) и приёмка на реальном сценарии.

## 11. Не-цели и риски

Не делаем:
- 475 тулов и «профили = теги OpenAPI» — это ровно та ошибка, из-за которой официальный неудобен.
- Свою реализацию docker/compose/helm внутрь бинаря — сервер уже умеет.
- OAuth к Portainer: API key штатный.
- Собственный веб-UI, мультитенантность, биллинг.

Риски:
- **Portainer 3.x** может сменить API и закрыть публичную спеку — тогда генерация из OpenAPI упирается в отсутствие спеки; лечится тем, что курируемый слой и прокси от версии почти не зависят.
- **websocket-пути недокументированы** (в спеке 4 пути, без схем сообщений) — придётся разбирать по коду Portainer и проверять на живом инстансе; это самый дорогой участок.
- **BE/CE расхождение**: часть операций (RBAC-политики, лицензия, LDAP) есть только в BE; набор `admin` обязан определять редакцию в рантайме (`/system/version` → `ServerEdition`) и прятать недоступное.
- **Тестовая среда**: приёмка требует живого кластера и второго Portainer; без них этапы 3–4 проверяются только на контракте схем.

## Приложение: источники

- https://github.com/portainer/portainer-mcp (README:5-9, `src/portainer_mcp/server.py:413`, `profiles.py:20-53`, `proxy.py:164-296`, `docs/profiles.md:41-57`)
- https://github.com/portainer/portainer — `api/docs/openapi.yaml` @ `release/2.45`; releases 2.45.3 (2026-10-21)
- https://github.com/moby/moby — `api/swagger.yaml` (Docker Engine API 1.57)
- https://pypi.org/pypi/mcp-portainer/json (2.45.3, 2026-10-23)
- https://github.com/jmrplens/portainer-mcp-enhanced (`tools.yaml`, `internal/k8sutil/stripper.go`)
- https://modelcontextprotocol.io/specification/versioning (current `2026-07-28`) и `/specification/2026-07-28/deprecated`
- https://github.com/modelcontextprotocol/go-sdk (v1.8.0, 2026-09-14), https://github.com/modelcontextprotocol/rust-sdk (rmcp 3.10.0, 2026-11-12)
- Live probe: self-hosted Portainer EE 2.45.3; проверены `systemVersion`, `EndpointList`, `docker_proxy /version`, `/networks`
