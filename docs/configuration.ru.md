> Русский перевод `docs/configuration.md`; английский текст — авторитетный.

[Индекс документации](index.ru.md) · [English](configuration.md)

# Конфигурация

Один YAML-файл на процесс, один блок на инстанс Portainer, один список наборов.
Файл читается один раз при старте и проверяется строго: неизвестный ключ,
отсутствующий инстанс или сломанная зависимость набора останавливают сервер до
того, как он начнёт обслуживать запросы.

Этот файл читает `portainer-mcp`. Второй бинарь, `arcane-mcp`, файла не читает
вообще: он говорит с одним менеджером, и его конфигурация — это его флаги (таблица
флагов — в [Бэкенде Arcane](arcane.ru.md)).

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

`config.example.yaml` в корне репозитория — та же форма, но с комментариями.

## Верхний уровень

| Ключ | Тип | Обязателен | Смысл |
|---|---|---|---|
| `server` | mapping | да | Транспорт и переключатели каталога |
| `instances` | list | да | Один или несколько серверов Portainer; минимум один |
| `toolsets` | list of strings | да | Имена наборов для регистрации; список не должен быть пустым |

Любой другой ключ на любом уровне отвергается (YAML-декодирование идёт с
`KnownFields`), поэтому опечатка вроде `toolset:` падает, а не игнорируется молча.

## `server`

| Ключ | Тип | По умолчанию | Смысл |
|---|---|---|---|
| `transport` | string | `stdio` | `stdio` или `http`. Пустое значение означает `stdio` |
| `listen` | string | — | Адрес для прослушивания, например `127.0.0.1:17717`. Обязателен при `transport: http`, при stdio игнорируется |
| `read_only` | bool | `false` | Регистрировать только тулы с `readOnlyHint: true` |
| `exhaustive` | bool | `false` | Регистрировать все операции включённых наборов вместо набора по умолчанию |

HTTP-транспорт открыт обычным HTTP и обслуживает Streamable HTTP по пути `/mcp`.
Ставьте TLS перед ним, если эндпоинт доступен извне хоста.

## `instances[]`

| Ключ | Тип | Обязателен | Смысл |
|---|---|---|---|
| `name` | string | да | Имя, которым инстанс вызывается через аргумент `instance`. Уникальное, непустое |
| `url` | string | да | Абсолютный `http`- или `https`-URL с хостом, например `https://portainer.example.com:9443` |
| `api_key` | string | да | API-ключ Portainer, уходит в заголовке `X-API-KEY`. Непустой после подстановки `${VAR}` |
| `tls_verify` | bool или string | `false` | Проверять сертификат сервера. `false` принимает самоподписанные сертификаты |
| `ca_file` | string | — | PEM-файл с CA-сертификатами в дополнение к системному хранилищу |

Первый инстанс — цель по умолчанию, когда тул вызван без `instance`. Неизвестное
имя в аргументе — ошибка со списком настроенных имён.

`tls_verify` по умолчанию `false`, то есть проверка сертификата выключена и
самоподписанные инстансы работают сразу. Ставьте `true` для инстанса с сертификатом
публичного CA и добавляйте `ca_file`, когда CA частный.

`ca_file` читается один раз при старте. Нечитаемый файл или файл без сертификатов
роняет старт, а не оставляет доверие пустым молча.

## `toolsets[]`

Имена наборов для регистрации в написании каталога: `core`, `docker`, `swarm`,
`k8s`, `helm`, `stacks`, `gitops`, `edge`, `admin`, `observability`, `raw`. Что
внутри каждого — в [toolsets.ru.md](toolsets.ru.md).

Список проверяется дважды: каждое имя должно быть в каталоге, и каждая зависимость
включённого набора должна быть включена рядом. `helm` без `k8s`, например,
отвергается на старте.

`toolsets` валидируется даже когда `--toolsets` заменяет его из командной строки.
Держите в файле корректный список.

## Подстановка `${VAR}`

Строковые поля принимают ссылку на переменную окружения вместо литерала:

```yaml
instances:
  - name: prod
    url: ${PORTAINER_URL}
    api_key: ${PORTAINER_API_KEY}
```

| Поле | Принимает `${VAR}` |
|---|---|
| `server.transport` | да |
| `instances[].url` | да |
| `instances[].api_key` | да |
| `instances[].ca_file` | да |
| `instances[].tls_verify` | да, в строковой форме |

Правила по коду:

- Ссылка должна занимать всё значение. `${A}-suffix` берётся буквально, без
  подстановки.
- Пробелы по краям обрезаются до проверки.
- Незаданная или пустая переменная — ошибка, а не пустая строка. Для `api_key`
  сообщение называет переменную: `instance "prod": api_key is empty; set
  PORTAINER_API_KEY`.
- Подставленное значение подставляется снова, если само является ссылкой; цепочка,
  которая продолжает указывать на другую ссылку, падает с `nests too deep`.
- `tls_verify` принимает и обычное YAML-булево. Строковая форма проходит ту же
  подстановку, затем `strconv.ParseBool`, поэтому работают `true`, `1`, `false` и
  `0`, а всё остальное падает с `expected a boolean, got "..."`.

## Флаги

| Флаг | По умолчанию | Действие |
|---|---|---|
| `--config` | `config.yaml` | Путь к YAML-файлу |
| `--toolsets` | — | Имена через запятую, пробелы вокруг элементов обрезаются. Заменяет `toolsets` из файла; неизвестное имя падает до чтения файла |
| `--read-only` | `false` | Складывается с `server.read_only`: read-only включается любым из них |
| `--exhaustive` | `false` | Складывается с `server.exhaustive`: любой из них регистрирует все операции включённых наборов вместо набора по умолчанию |
| `--version` | — | Напечатать `portainer-mcp <version> (<commit>, <date>)` и выйти до чтения каталога и конфигурации |

`--version` заполняется при сборке через `-X main.version`, `-X main.commit` и
`-X main.date`; локальная сборка без тега печатает `dev (none, unknown)`.

## Бинарь Arcane

У `arcane-mcp` нет ни файла конфигурации, ни `--config`. Тот же `config.Config` он
собирает в памяти из флагов (`config.Single`): URL становится единственным инстансом,
`--toolsets` — списком, и результат проходит ту же проверку, что YAML-файл, — поэтому
неизвестный набор, выключенная зависимость, относительный URL или пустой ключ падают
с теми же сообщениями, что перечислены ниже.

| Флаг | По умолчанию | Смысл |
|---|---|---|
| `--url` | — | Базовый URL менеджера; обязателен, абсолютный `http(s)` |
| `--api-key` | `${ARCANE_API_KEY}` | Уходит в заголовке `X-API-Key`; незаданная переменная падает с `instance "arcane": api_key is empty; set ARCANE_API_KEY` |
| `--name` | `arcane` | Имя инстанса в текстах ошибок |
| `--toolsets` | все наборы каталога Arcane | Имена через запятую; пустой список означает все наборы, а не ошибку |

`--url` и `--api-key` принимают ссылку `${VAR}` на всё значение так же, как строковые
поля YAML. Транспорт всегда stdio: ни `--listen`, ни HTTP-слушателя нет, поэтому у
блока `server` здесь нет аналога. Остальные флаги — `--read-only`, `--exhaustive`,
`--allow-exec` и `--version` — это переключатели каталога, они перечислены вместе с
наборами Arcane в [Бэкенде Arcane](arcane.ru.md).

## Ошибки старта

Сообщения, которые можно встретить до того, как сервер начнёт обслуживать запросы,
приведены так, как их печатает код:

| Сообщение | Причина |
|---|---|
| `read config: ...` | Файл не читается |
| `parse config <path>: ...` | Некорректный YAML или неизвестный ключ |
| `config: at least one instance is required` | `instances` пуст или отсутствует |
| `config: instance url "..." has no name` | Инстанс без `name` |
| `config: duplicate instance name "..."` | Два инстанса с одним именем |
| `config: instance "...": url must be absolute http(s), got "..."` | `url` относительный или с другой схемой |
| `no toolsets enabled` | `toolsets` пуст |
| `unknown toolset "..." (known: ...)` | Имени нет в каталоге; известные имена перечислены |
| `toolset "..." requires "...", which is not enabled` | Не включена зависимость |
| `instance "...": api_key is required` | `api_key` пуст |
| `config: server.listen is required for http transport` | `transport: http` без `listen` |
| `config: server.transport must be stdio or http, got "..."` | Другое имя транспорта |
| `no tools selected for toolsets ...` | Read-only-фильтр убрал все тулы выборки |

## Примеры

### Транспорт HTTP

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

Здесь `read_only: true` убирает все тулы без аннотации read-only. При
`toolsets: [core]` и наборе по умолчанию все семь зарегистрированных тулов `core`
помечены read-only, так что остаются все семь.

### Несколько инстансов

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

Вызов без `instance` идёт в `prod`; `{"instance": "lab"}` — во второй.
