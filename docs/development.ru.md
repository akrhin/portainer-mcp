> Русский перевод `docs/development.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](development.md)

# Разработка

Сборка, тесты и выпуск в этом репозитории. Правила участия и чек-лист ревью — в
[CONTRIBUTING.ru.md](../CONTRIBUTING.ru.md); эта страница — механика.

## Требования

- Go 1.27 или новее. `go.mod` фиксирует `go 1.27.4`, и CI берёт версию из этого
  файла, а не из матрицы.
- Инстанс Portainer — только для smoke-прогона. Юнит-тесты используют `httptest` и
  сети не требуют.
- `docker` — только чтобы собрать образ.
- `npx` — только для линта Markdown, который гоняет CI.

## Цели

| Команда | Что делает |
|---|---|
| `make build` | `go build -o portainer-mcp ./cmd/portainer-mcp` |
| `make check` | `gofmt -l .`, затем `go vet ./...`, затем `go test ./...` |
| `make catalog` | `go run ./tools/cataloggen`, переписывает `internal/catalog/generated.yaml` |
| `go test ./...` | Только набор тестов |
| `go build ./...` | Компиляция всех пакетов без бинаря |
| `go test -race ./...` | Набор под детектором гонок |
| `go test ./internal/catalog/` | Один пакет |
| `bash scripts/docs-check.sh` | Ворота документации |

`make check` — это ворота: он падает на неотформатированном файле, замечании vet или
упавшем тесте. Цели `make test` по умолчанию нет; вызывайте `go test` напрямую.

## Тесты

В наборе 38 тестов в пяти пакетах. Все они табличные, и ни один не ходит в сеть.

| Пакет | Что проверяет |
|---|---|
| `internal/catalog` | Слитый каталог проходит валидацию; ошибки разбора на версии, дублях имён, отсутствующих подсказках, схемах не-объектах; слияние отвергает набор только в одном документе; описания схем — строки; `CheckToolsets` принимает и отвергает нужные комбинации |
| `internal/config` | Подстановка `${VAR}` в строковые поля и в `tls_verify`; неизвестный набор; не включённая зависимость; отсутствующий API-ключ; отсутствие инстансов |
| `internal/mcpserver` | Golden-файл `tools/list`; exhaustive-регистрация отдаёт каждый тул наборов; read-only убирает write-тулы; вызов через MCP доходит до пути, query и тела; отказ на стриминге; вырезание `managedFields`; постраничная выдача обрезанного массива; неизвестный набор падает |
| `internal/portainer` | Заголовок `X-API-KEY` отправляется; 401 превращается в задокументированный текст `APIError`; список эндпоинтов декодируется; `Do` отправляет метод, путь, query и тело; обрезание, не-JSON ответы, разворачивание пути, кодирование query и вырезание поля |
| `tools/cataloggen` | Закоммиченный `generated.yaml` совпадает со свежим прогоном байт в байт; один тул на операцию плюс сырые extra-тулы; флаги ответа; именование тулов и разбиение camelCase |

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
| `security.yml` | push в `main`, pull request, раз в неделю | gosec v2.22.4 в SARIF, govulncheck v1.1.4, gitleaks по всей истории, dependency review на pull request, CodeQL `security-extended` |
| `release.yml` | тег `v*` | goreleaser v2, Docker buildx, вход в `ghcr.io` |

Задачи документации условны по наличию файлов, поэтому checkout без
`scripts/docs-check.sh` или без `docs/**/*.md` не валит CI.

## Релиз

Тег `v*` запускает `.goreleaser.yaml` с `release --clean`. Артефакты:

- Архивы для `linux`, `darwin` и `windows` на `amd64` и `arm64`, `tar.gz` и `zip`
  для Windows, с именем `portainer-mcp_<version>_<os>_<arch>`.
- `checksums.txt`, SHA-256.
- SBOM для каждого архива, собранный syft (workflow его ставит).
- Multi-arch образ в `ghcr.io/akrhin/portainer-mcp`: по образу на архитектуру плюс
  манифесты `:<version>` и `:latest`.

Релизная сборка заполняет `--version` через
`-X main.version={{ .Version }} -X main.commit={{ .Commit }} -X main.date={{ .Date }}`
в `.goreleaser.yaml`; локальный `go build` печатает `dev (none, unknown)`.

Заметки к релизу собираются из заголовков Conventional Commits и группируются на
features, исправления, производительность, рефакторинг и документацию; `chore`,
`test`, `ci`, `style`, `build` и merge-коммиты отфильтрованы. Поэтому префиксы
коммитов важны не только для аккуратности.

Тег `v0.1.0` выпущен, поэтому архивы релиза и образ `ghcr.io/akrhin/portainer-mcp`
существуют; локально образ собирается командой `docker build -t portainer-mcp .`.

Образ собирается из `golang:1.27-alpine` и работает на
`gcr.io/distroless/static:nonroot`. `HEALTHCHECK` у него нет: по stdio процесс молчит,
пока не придёт сообщение, и проверке живости нечего читать.

## Изменение поверхности тулов

Поверхность — данные. Не правьте `internal/catalog/generated.yaml` и не добавляйте
обработчик: добавьте или перенесите операцию в `api/toolsets.yaml` (карта тегов,
переопределение в `operation_toolsets` или запись в `extras`), выберите
принадлежность к набору по умолчанию в `api/default-tools.yaml`, затем выполните:

```sh
make catalog
make catalog && git diff --exit-code internal/catalog/generated.yaml
```

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
