> Русский перевод `docs/arcane.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](arcane.md)

# Бэкенд Arcane

Из этого репозитория собираются два MCP-сервера на одном генераторе каталога:
`portainer-mcp` говорит с Portainer, `arcane-mcp` — с менеджером
[Arcane](https://getarcane.app). Они делят обработчик, типы конфигурации и встроенный
каталог, а в MCP-клиенте включаются независимо.

Arcane 3.0.1 отвечает JSON под `/api` и авторизует по заголовку `X-API-Key`. Каталог
содержит 399 тулов в тринадцати наборах; 53 регистрируются по умолчанию, а
`--exhaustive` регистрирует все операции включённых наборов.

## Откуда берутся тулы Arcane

| Файл | Роль |
|---|---|
| `api/arcane-3.1.yaml` | Сконвертированный документ OpenAPI; его пишет `tools/specgen`, руками не правится |
| `api/arcane-toolsets.yaml` | Карта тегов в наборы, переопределения, пропущенные теги и пути, переименования и extra `arcane_raw` |
| `api/arcane-default-tools.yaml` | 53 тула, регистрируемых без `--exhaustive` |
| `tools/cataloggen -profile arcane` | Читает три файла выше и пишет `internal/catalog/arcane/generated.yaml` |
| `internal/catalog/arcane/catalog.yaml` | Имена наборов, описания и рёбра `depends_on` |
| `internal/catalog/arcane/generated.yaml` | Все тулы всех наборов; встроен в бинарь |

`catalog.LoadArcane()` сливает два документа так же, как каталог Portainer, поэтому
проверка — дубли имён, отсутствующие аннотации, неизвестные зависимости — одна и та же.
`make catalog` перестраивает оба каталога за один прогон.

## Наборы

| Набор | Зависит от | Тулов | По умолчанию |
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

Всего 399 тулов, 53 в наборе по умолчанию. `backups` и `raw` стоят отдельно, остальным
нужен `core` — в нём вызовы окружений (агентов) и системы.

Arcane адресует ресурсы через агента: пути выглядят как
`/api/agents/{id}/containers/{containerId}`, где `id` — идентификатор окружения из
`environments_list`. Действия над контейнером — один тул `container_action`, аргумент
`action` выбирает start, stop, restart, kill, pause или unpause; проекты работают так же
через `project_action`.

## Запуск

```sh
make build-arcane                       # ./arcane-mcp
ARCANE_API_KEY=... ./arcane-mcp --url http://arcane:3552 --toolsets core,containers
```

Файла конфигурации нет. Один процесс обслуживает один менеджер, заданный `--url`; ключ
приходит из `--api-key` и по умолчанию равен ссылке `${ARCANE_API_KEY}`, поэтому
незаданная переменная падает на старте, а не отправляет пустой заголовок.

| Флаг | По умолчанию | Действие |
|---|---|---|
| `--url` | — | Базовый URL менеджера, например `http://arcane:3552`; обязателен |
| `--api-key` | `${ARCANE_API_KEY}` | Значение заголовка `X-API-Key` |
| `--name` | `arcane` | Имя инстанса в текстах ошибок |
| `--toolsets` | все | Имена наборов через запятую |
| `--read-only` | `false` | Регистрировать только тулы с `readOnlyHint: true` |
| `--exhaustive` | `false` | Регистрировать все операции включённых наборов |
| `--version` | — | Напечатать `arcane-mcp <version> (<commit>, <date>)` и выйти |

Запись в клиенте:

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

## Перегенерация каталога

Сконвертированный документ лежит в репозитории; вендорный JSON, из которого он получен,
— нет. Чтобы обновить, сохраните OpenAPI 3.1 документ менеджера как
`arcane-openapi.json` в корне репозитория и выполните:

```sh
make spec-arcane    # go run ./tools/specgen -in arcane-openapi.json -out api/arcane-3.1.yaml
make catalog        # go run ./tools/cataloggen, оба профиля
```

`specgen` конвертирует документ 3.1 детерминированно: ключи `$schema` выбрасываются,
массив `type` сворачивается в единственный не-`null` элемент, числа сохраняют целую
форму, а список `servers` заменяется на `/api`. Повторный прогон по тому же входу даёт
те же байты.

## Ограничения этого захода

- В Arcane 3.0.1 нет эндпоинтов exec, attach и terminal, поэтому консоли контейнера нет.
  Ближайшее — `container_processes_list`, список процессов внутри контейнера.
- Теги Stream (server-sent events), Uploads, Mobile Push, MFA, Passkeys и OIDC
  пропущены: это браузерные сценарии или долгоживущие потоки, а не операции
  запрос/ответ.
- Операции `HEAD` и два потока `/events/changes` и `/system/upgrade/progress`
  отброшены через `skip_methods` и `skip_paths`.
- Образы контейнеров из релизного workflow по-прежнему содержат только
  `portainer-mcp`; бинарь Arcane едет в архивах.
- Клиент Arcane переиспользует HTTP-исполнитель Portainer с другим заголовком
  авторизации и другим префиксом ошибок. Транспорт у серверов общий, поэтому
  исправление в одном доходит до обоих.
