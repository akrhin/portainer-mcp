> Русский перевод `docs/development.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](development.md)

# Разработка

Сборка, тесты и выпуск в этом репозитории. Правила участия и чек-лист ревью — в
[CONTRIBUTING.ru.md](../CONTRIBUTING.ru.md); эта страница — механика.

## Требования

- Go 1.27 или новее. `go.mod` фиксирует `go 1.27.4`, и CI берёт версию из этого
  файла, а не из матрицы.
- Инстанс Portainer — для smoke-прогона, менеджер Arcane — для прогона Arcane.
  Юнит-тесты используют `httptest` и сети не требуют.
- `docker` — только чтобы собрать образ.
- `npx` — только для линта Markdown, который гоняет CI.

## Цели

| Команда | Что делает |
|---|---|
| `make build` | `go build -o portainer-mcp ./cmd/portainer-mcp` |
| `make build-arcane` | `go build -o arcane-mcp ./cmd/arcane-mcp` |
| `make check` | `gofmt -l .`, затем `go vet ./...`, затем `go test ./...` |
| `make catalog` | `go run ./tools/cataloggen`, переписывает оба `generated.yaml` |
| `make spec-arcane` | `go run ./tools/specgen -in arcane-openapi.json -out api/arcane-3.1.yaml`; документ нужно положить в корень репозитория |
| `go test ./...` | Только набор тестов |
| `go build ./...` | Компиляция всех пакетов без бинаря |
| `go test -race ./...` | Набор под детектором гонок |
| `go test ./internal/catalog/` | Один пакет |
| `bash scripts/docs-check.sh` | Ворота документации |

`make check` — это ворота: он падает на неотформатированном файле, замечании vet или
упавшем тесте. Цели `make test` по умолчанию нет; вызывайте `go test` напрямую.

## Тесты

В наборе 69 тестовых функций в шести пакетах. Все они табличные, и ни один не ходит в сеть.

| Пакет | Что проверяет |
|---|---|
| `internal/arcane` | Клиент отправляет `X-API-Key`, а его ошибки несут метку `arcane` |
| `internal/catalog` | Слитый каталог проходит валидацию; ошибки разбора на версии, дублях имён, отсутствующих подсказках, схемах не-объектах; слияние отвергает набор только в одном документе; описания схем — строки; `CheckToolsets` принимает и отвергает нужные комбинации |
| `internal/config` | Подстановка `${VAR}` в строковые поля и в `tls_verify`; неизвестный набор; не включённая зависимость; отсутствующий API-ключ; отсутствие инстансов |
| `internal/mcpserver` | Golden-файл `tools/list`; exhaustive-регистрация отдаёт каждый тул наборов; read-only убирает write-тулы; вызов через MCP доходит до пути, query и тела — на обоих каталогах; отказ на стриминге; вырезание `managedFields`; постраничная выдача обрезанного массива; бюджет ответа; тела multipart; плоские поля тела; опт-ин `container_exec` и его вывод по websocket, обрезание и таймаут; неизвестный набор падает |
| `internal/portainer` | Заголовок `X-API-KEY` отправляется; 401 превращается в задокументированный текст `APIError`; список эндпоинтов декодируется; `Do` отправляет метод, путь, query и тело; обрезание, не-JSON ответы, разворачивание пути, кодирование query и вырезание поля |
| `tools/cataloggen` | Оба закоммиченных `generated.yaml` совпадают со свежим прогоном байт в байт; один тул на операцию плюс сырые extra-тулы; счётчики, формы имён, ограниченные стримы и бинарные тела Arcane; флаги ответа; именование тулов и разбиение camelCase |

Каталог и список зарегистрированных тулов держит вместе golden-тест:
`internal/mcpserver/testdata/tools_core_docker.golden.json` — ожидаемый ответ
`tools/list` для наборов `core` и `docker` по умолчанию, 20 тулов. Любая правка
каталога, меняющая поверхность, валит набор. Перегенерируйте файл осознанно и
прочтите дифф:

```sh
go test ./internal/mcpserver -run TestToolsListGolden -update
```

У генератора свой тест воспроизводимости. `TestBuildMatchesCommittedFile` запускает
`build()` и сравнивает байты с закоммиченным файлом; правка каталога без
`make catalog` валит его. Полная проверка всё равно `make check`.

`go test -race ./...` обязателен перед pull request, который трогает конкурентность,
HTTP-транспорт или общее состояние; CI гоняет его на каждом push и pull request.

## Smoke-прогон

`scripts/smoke.sh` прогоняет собранный бинарь по stdio против настоящего Portainer.
Он вызывает курируемые тулы `core` и один read-пробник из каждого набора,
названного в `SMOKE_TOOLSETS`. Это проверка, которую юнит-набор сделать не может:
настоящие учётные данные, настоящие ответы, настоящий текст ошибок.

```sh
PORTAINER_API_KEY=... SMOKE_ENV_ID=1 SMOKE_TOOLSETS=core,docker \
  bash scripts/smoke.sh config.local.yaml
```

- Аргумент — путь к конфигурации; по умолчанию `config.local.yaml`.
- `SMOKE_TOOLSETS` по умолчанию `core` и должен быть списком через запятую, который
  конфигурация тоже включает. Каждый дополнительный набор добавляет один
  read-пробник: `container_list`, `node_list`, `kubernetes_namespaces_list`,
  `helm_list`, `stack_list`, `gitops_sources_list`, `edge_group_list`, `user_list`
  или `status_inspect`.
- `$PORTAINER_API_KEY` — переменная, которую разворачивает пример конфигурации; имя
  может быть любым, пока конфигурация на него ссылается.
- `$SMOKE_ENV_ID` — id окружения для `environment_inspect` и пробников, по умолчанию
  `1`.
- `SMOKE_STREAM=1` добавляет `system_events` и ждёт отказа как на неограниченном
  стриме, а не зависания. Без него скрипт падает, если стриминговый тул попал в
  список пробников.
- Скрипт собирает бинарь во временный каталог, отправляет `initialize` с версией
  протокола `2026-07-28`, `notifications/initialized`, `tools/list` и вызовы, и
  сверяет число ответов с отправленными id. Падение `core` или 4xx от пробника
  валит прогон; 5xx от пробника печатается предупреждением.

Успех — код 0 и строка
`OK: core tools answered and every enabled set answered one read probe`; всё
остальное — отказ, который нужно читать, а не предупреждение.

`scripts/smoke-arcane.sh` устроен так же для второго бинаря и файла конфигурации не
требует:

```sh
ARCANE_URL=http://arcane:3552 ARCANE_API_KEY=... bash scripts/smoke-arcane.sh
```

- `ARCANE_URL` и `ARCANE_API_KEY` обязательны; без них скрипт выходит с кодом 2.
- `SMOKE_TOOLSETS` по умолчанию `core,containers,projects,images,volumes,networks` —
  это наборы, которых касается фиксированный список пробников.
- `SMOKE_AGENT_ID` по умолчанию `0` и передаётся каждому пробнику как `id`.
- Скрипт собирает `arcane-mcp` во временный каталог, отправляет `initialize` с версией
  протокола `2026-07-28`, `notifications/initialized`, `tools/list` и восемь
  read-вызовов: `environments_list`, `environment_inspect`, `containers_list`,
  `container_status_count`, `projects_list`, `image_update_summary`, `volumes_list`,
  `networks_list`. Любой ответ с `isError: true` валит прогон; успех — код 0 и строка
  `OK: every probe answered`.
- Список пробников фиксирован, поэтому сужённый `SMOKE_TOOLSETS` обязан покрывать все
  наборы, из которых он читает: `volumes_list` живёт в `volumes`, а `networks_list` —
  в `networks`.

## Ворота документации

```sh
bash scripts/docs-check.sh
```

По всем Markdown-файлам вне `.git`, `.tmp` и `vendor` проходят пять проверок:

| Проверка | Что требует |
|---|---|
| Ссылки | Каждая относительная ссылка на `*.md` ведёт на существующий файл |
| Запрещённые пути | Ничего из `docs/forbidden.txt`: приватные диапазоны адресов, личные домашние каталоги, пути песочниц, имена внутренних сервисов. `docs/history/` освобождён |
| Сироты | На каждую страницу `docs/` есть ссылка с другой страницы (`docs/index.md` и архив освобождены) |
| Языковой паритет | У каждого `.md` есть сосед `.ru.md` и наоборот, кроме `AGENTS.md`, `CHANGELOG.md`, `docs/CURRENT-GOAL.md`, `.github/**` и `docs/history/**` |
| Якоря | Факты из `docs/anchors.txt` всё ещё совпадают: версия каталога, путь модуля, версия SDK, директива Go, имена флагов и значения, названные на индексной странице |

Якорь — одна строка `mode|regex|path`, где mode — `must` или `mustnot`. Они нужны,
чтобы значение, названное в прозе, валило ворота при изменении кода, а не уплывало
молча.

Линт Markdown идёт отдельно, его правила — в `.markdownlint-cli2.yaml`:

```sh
npx --yes markdownlint-cli2@0.24.1
```

Конфигурация отключает длину строки, инлайновый HTML, голые URL и правило первого
заголовка, а `docs/history/**` исключает из glob.

## Непрерывная интеграция

| Workflow | Триггер | Задачи |
|---|---|---|
| `ci.yml` | push в `main`, pull request | `gofmt -l .`, `go vet`, `go test -race`, `go build`; `golangci-lint` v2.15.0 по `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused, misspell, revive) |
| `docs.yml` | push в `main`, pull request | `scripts/docs-check.sh`, `markdownlint-cli2@0.24.1` |
| `security.yml` | push в `main`, pull request, раз в неделю | gosec v2.30.0 в SARIF, govulncheck v1.9.0, gitleaks по всей истории, dependency review на pull request, CodeQL `security-extended` |
| `release.yml` | тег `v*` | goreleaser v2, Docker buildx, вход в `ghcr.io` |

Задачи документации условны по наличию файлов, поэтому checkout без
`scripts/docs-check.sh` или без `docs/**/*.md` не валит CI.

## Релиз

Тег `v*` запускает `.goreleaser.yaml` с `release --clean`. Артефакты:

- Архивы для `linux`, `darwin` и `windows` на `amd64` и `arm64`, `tar.gz` и `zip`
  для Windows, с именем `portainer-mcp_<version>_<os>_<arch>`. В каждом архиве оба
  бинаря, `portainer-mcp` и `arcane-mcp`.
- `checksums.txt`, SHA-256.
- SBOM для каждого архива, собранный syft (workflow его ставит).
- Multi-arch образ в `ghcr.io/akrhin/portainer-mcp`: по образу на архитектуру плюс
  манифесты для `:<version>` и `:latest`; образ запускает сервер Portainer, отдельного
  образа для `arcane-mcp` пока нет.

Релизная сборка заполняет `--version` через
`-X main.version={{ .Version }} -X main.commit={{ .Commit }} -X main.date={{ .Date }}`
в `.goreleaser.yaml`; локальный `go build` печатает `dev (none, unknown)`.

Заметки к релизу собираются из заголовков Conventional Commits и группируются на
features, исправления, производительность, рефакторинг и документацию; `chore`,
`test`, `ci`, `style`, `build` и merge-коммиты отфильтрованы. Поэтому префиксы
коммитов важны не только для аккуратности.

Тег `v0.4.1` — текущий, поэтому архивы релиза и образ `ghcr.io/akrhin/portainer-mcp`
существуют; локально образ собирается командой `docker build -t portainer-mcp .`.

Образ собирается из `golang:1.27-alpine` и работает на
`gcr.io/distroless/static:nonroot`. `HEALTHCHECK` у него нет: по stdio процесс молчит,
пока не придёт сообщение, и проверке живости нечего читать.

## Изменение поверхности тулов

Поверхность — данные. Не правьте `generated.yaml` — ни тот, ни другой — и не
добавляйте обработчик: добавьте или перенесите операцию в `api/toolsets.yaml` (карта
тегов, переопределение в `operation_toolsets` или запись в `extras`), выберите
принадлежность к набору по умолчанию в `api/default-tools.yaml`, затем выполните:

```sh
make catalog
make catalog && git diff --exit-code internal/catalog/generated.yaml
```

Для каталога Arcane то же самое относится к `api/arcane-toolsets.yaml` и
`api/arcane-default-tools.yaml`; чтобы пересобрать его конвертированную спецификацию
из свежего OpenAPI-документа Arcane, положите её как `arcane-openapi.json` и сначала
выполните `make spec-arcane`. `go run ./tools/cataloggen -profile arcane` собирает
только этот каталог.

Вторая команда — проверка детерминизма: второй прогон не должен ничего менять.
Затем прогоните ворота:

```sh
make check
go test ./tools/cataloggen
```

Что проверят ворота после правки:

- Слитый каталог валидируется на старте, включая правила аннотаций.
- `generated.yaml` совпадает со свежим прогоном, а golden-файл `tools/list` — с
  поверхностью, или оба перегенерированы в той же правке.
- Для нового тула есть случай read-only или exhaustive, потому что оба переключают
  регистрацию.
- Запись в CHANGELOG и правка документации конфигурации или наборов — в том же pull
  request.

Карты «реализованных» наборов больше нет: тул из каталога регистрируется и
исполняется общим обработчиком, а тул вне каталога вызвать нельзя.
