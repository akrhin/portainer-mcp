> Русский перевод `README.md`; английский текст — авторитетный.

[Документация](docs/index.ru.md) · [English](README.md) · [Участие](CONTRIBUTING.ru.md) · [Безопасность](SECURITY.ru.md) · [Изменения](CHANGELOG.md)

# portainer-mcp

Этот репозиторий собирает два MCP-сервера на одном генераторе каталога, оба на Go
поверх официального `github.com/modelcontextprotocol/go-sdk` v1.8.0.

| Бинарь | С чем говорит | Каталог | По умолчанию |
|---|---|---|---|
| `portainer-mcp` | Portainer: его собственный API, Docker Engine через окружения Portainer, Kubernetes и Helm | 394 тула в одиннадцати наборах | 87 |
| `arcane-mcp` | Менеджер [Arcane](https://getarcane.app): контейнеры, проекты, образы, тома, сети, безопасность и администрирование | 445 тулов в семнадцати наборах | 53 |

`portainer-mcp` говорит на MCP через stdio по умолчанию или Streamable HTTP по пути
`/mcp`; `arcane-mcp` работает только через stdio. Ревизию MCP при рукопожатии
определяет SDK — он говорит на `2026-07-28`; сервер отвечает и на устаревшее
рукопожатие `2025-11-25`.

Поверхность тулов — данные. `api/toolsets.yaml` разносит теги и пути OpenAPI по
наборам и задаёт переименования; `api/default-tools.yaml` перечисляет 87 тулов,
которые регистрируются без `--exhaustive`. `make catalog` (`go run
./tools/cataloggen`) собирает из обоих файлов и двух спецификаций
`internal/catalog/generated.yaml`, который вшивается в бинарь. Профиль Arcane читает
`api/arcane-toolsets.yaml`, `api/arcane-default-tools.yaml` и `api/arcane-3.1.yaml`
тем же порядком. `internal/catalog/catalog.yaml` хранит только описания наборов и
зависимости. Вызвать тул, которого нет в сгенерированном каталоге, нельзя.

**Статус: оба каталога исполняемы.** Каждый тул — 394 у Portainer, 445 у Arcane —
описан данными и идёт через один общий обработчик. Без флагов старт регистрирует
набор по умолчанию каждого сервера, 87 тулов у Portainer и 53 у Arcane;
`--exhaustive` регистрирует все операции этих наборов, а `--read-only` фильтрует
любой из наборов до тулов с аннотацией read-only.

## Требования

- Go 1.27 или новее для сборки (`go.mod` фиксирует `go 1.27.4`). Полученный бинарь
  статический и во время работы ничего не требует.
- Инстанс Portainer, доступный по HTTP или HTTPS, и API-ключ, права которого вы
  готовы отдать тулам. Самоподписанный сертификат принимается, только если для
  инстанса указано `tls_verify: false`.
- Для `arcane-mcp` — менеджер Arcane и API-ключ к нему; базовый URL менеджера
  задаётся обязательным флагом, а не ключом конфигурации.

## Сборка

```sh
make build        # ./portainer-mcp
make build-arcane # ./arcane-mcp
make check        # gofmt, go vet, go test
make catalog      # пересобрать оба сгенерированных каталога из api/
```

## Настройка

Скопируйте пример и поправьте блок инстанса:

```sh
cp config.example.yaml config.yaml
```

```yaml
server:
  transport: stdio
instances:
  - name: prod
    url: https://portainer.example.com:9443
    api_key: ${PORTAINER_API_KEY}
toolsets: [core]
```

`${VAR}` разрешается из окружения при старте; ссылка на незаданную переменную —
ошибка, а не пустое значение. Неизвестные ключи отвергаются. Полный справочник
ключей, правил валидации и сообщений об ошибках — в
[docs/configuration.ru.md](docs/configuration.ru.md).

## Запуск

```sh
PORTAINER_API_KEY=... ./portainer-mcp --config config.yaml
```

При `transport: stdio` процесс говорит на MCP через stdin и stdout и молчит, пока не
придёт сообщение. Чтобы подключить его в клиенте, который сам запускает stdio-серверы:

```json
{
  "mcpServers": {
    "portainer": {
      "command": "/usr/local/bin/portainer-mcp",
      "args": ["--config", "/etc/portainer-mcp/config.yaml"],
      "env": { "PORTAINER_API_KEY": "..." }
    }
  }
}
```

Для HTTP поставьте `transport: http` и задайте адрес в `listen`. С адресом из
примера (`127.0.0.1:17717`) Streamable HTTP обслуживается по
`http://127.0.0.1:17717/mcp`. У `listen` нет значения по умолчанию, и он обязателен
для HTTP; слушатель открыт обычным HTTP, поэтому, если эндпоинт выходит за пределы
хоста, поставьте перед ним TLS.

## Флаги

| Флаг | По умолчанию | Действие |
|---|---|---|
| `--config` | `config.yaml` | Путь к YAML-конфигурации |
| `--toolsets` | — | Имена наборов через запятую; заменяет список из конфигурации |
| `--read-only` | `false` | Регистрировать только тулы с `readOnlyHint: true` |
| `--exhaustive` | `false` | Регистрировать все операции включённых наборов, а не набор по умолчанию |
| `--version` | — | Напечатать `portainer-mcp <version> (<commit>, <date>)` и выйти |

Флаги перекрывают файл там, где совпадают. `--read-only` складывается с
`server.read_only` (достаточно любого), `--toolsets` заменяет `toolsets`,
`--exhaustive` складывается с `server.exhaustive`. У `arcane-mcp` файла конфигурации
нет, а набор флагов свой: таблица — в разделе [«Бэкенд Arcane»](#бэкенд-arcane).

## Наборы тулов

Одиннадцать наборов, выбираются по имени. Набор не включить без тех, от которых он
зависит. Числа ниже — что лежит в сгенерированном каталоге и что регистрирует набор
по умолчанию:

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

Всего 394 тула, 87 в наборе по умолчанию. Каждый тул принимает необязательный
аргумент `instance` с именем настроенного инстанса и по умолчанию берёт первый.
Набор по умолчанию задан в `api/default-tools.yaml`; `--exhaustive` его игнорирует
и регистрирует все тулы включённых наборов. Полный список по наборам и HTTP-операция
за каждым тулом — `internal/catalog/generated.yaml`;
[docs/toolsets.ru.md](docs/toolsets.ru.md) объясняет раскладку и особую обработку
стриминговых тулов и метаданных Kubernetes.

## Бэкенд Arcane

Этот же репозиторий собирает второй сервер, `arcane-mcp`, для менеджера Arcane. У него
свой каталог — 445 тулов в семнадцати наборах, 53 из них по умолчанию — который строит
тот же генератор из `api/arcane-3.1.yaml`.

```sh
make build-arcane
ARCANE_API_KEY=... ./arcane-mcp --url http://arcane:3552 --toolsets core,containers
```

`--url` обязателен, файла конфигурации нет. Ключ приходит из `--api-key` (по умолчанию
`${ARCANE_API_KEY}`) и уходит в заголовке `X-API-Key`. В OpenAPI-документе Arcane
операции exec нет: терминал живёт на WebSocket-маршруте, поэтому разовый шелл в
контейнере — написанный руками `container_exec`. Он регистрируется только при
`--allow-exec` (или `ARCANE_ALLOW_EXEC=true`) и никогда в режиме read-only. Наборы,
флаги и перегенерация — в [docs/arcane.ru.md](docs/arcane.ru.md).

## Документация

- [Индекс документации](docs/index.ru.md) — точка входа и карта страниц.
- [Справочник конфигурации](docs/configuration.ru.md) — все ключи, значения по
  умолчанию и ошибки.
- [Наборы тулов](docs/toolsets.ru.md) — одиннадцать наборов, их размеры, зависимости
  и тулы по умолчанию.
- [Бэкенд Arcane](docs/arcane.ru.md) — второй бинарь: его наборы, флаги и ограничения.
- [Архитектура](docs/architecture.ru.md) — пакеты, старт, поток запроса, транспорты.
- [Разработка](docs/development.ru.md) — тесты, golden-файлы, smoke-прогон, релиз.
- [Участие](CONTRIBUTING.ru.md) — ворота сборки, стиль коммитов, добавление тула.
- [Политика безопасности](SECURITY.ru.md) — приватное сообщение об уязвимости и что
  считается уязвимостью.

## Docker и релизы

Релизы выходят по тегу `v*`. Workflow собирает архивы для `linux`, `darwin` и
`windows` на `amd64` и `arm64` с контрольными суммами и SBOM, а также публикует два
multi-arch образа: `ghcr.io/akrhin/portainer-mcp` и `ghcr.io/akrhin/arcane-mcp`
(тег `:latest` и тег версии). Оба distroless и работают не от root. Релиз `v0.4.3`
вышел, поэтому быстрее всего взять архив релиза или образ: каждый архив несёт оба
бинаря, а образ — тот, чьё имя носит.

```sh
docker run -i --rm -e PORTAINER_API_KEY -v "$PWD/config.yaml:/config.yaml" \
  ghcr.io/akrhin/portainer-mcp:v0.4.3 --config /config.yaml
docker run -i --rm -e ARCANE_API_KEY \
  ghcr.io/akrhin/arcane-mcp:v0.4.3 --url https://arcane.example:3552
go install github.com/akrhin/portainer-mcp/cmd/portainer-mcp@v0.4.3
go install github.com/akrhin/portainer-mcp/cmd/arcane-mcp@v0.4.3
```

Собрать образ из чекаута: `docker build -t portainer-mcp .` и
`docker build -f Dockerfile.arcane -t arcane-mcp .`. Оба сервера говорят по MCP через
stdio, поэтому контейнеру нужен `-i`.

## Лицензия

MIT — см. [LICENSE](LICENSE).
