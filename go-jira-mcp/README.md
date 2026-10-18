# go-jira-mcp

MCP-сервер на Go, который читает задачи из markdown-файлов и заводит их в JIRA
(Server / Data Center) внутри эпика, определяемого именем папки.

Полное ТЗ — [docs/TZ.md](./docs/TZ.md). Формат markdown-задачи —
[docs/task-format.md](./docs/task-format.md).

Транспорт — **stdio**: сервер запускается локально и подключается к Claude Code
(или другому MCP-клиенту).

---

## 1. Требования

- Go **1.26+**
- Доступ к JIRA-инстансу `https://jira.example.com` и Personal Access Token (PAT).

## 2. Сборка

```bash
make build
# или вручную:
go build -o bin/server ./cmd/server
```

Бинарник окажется в `bin/server`.

## 3. Конфигурация

Конфиг читается из переменных окружения. Локально удобно держать их в файле
`.env` в корне проекта — сервер подхватит его автоматически при старте
(переменные реального окружения имеют приоритет над `.env`).

| Переменная | Обяз. | Назначение | Пример / умолчание |
|---|---|---|---|
| `JIRA_URL` | да | base URL инстанса | `https://jira.example.com` |
| `JIRA_PERSONAL_ACCESS_TOKEN` | да | PAT для `Authorization: Bearer` | — |
| `JIRA_EPIC_FIELD` | да* | id поля «Epic Link» | `customfield_10008` |
| `TASKS_DIR` | нет | корень с эпик-папками | `./tasks` |
| `JIRA_LINK_TYPE` | нет | имя типа связи блокировки | `Blocks` |
| `JIRA_MAX_ATTACHMENT_BYTES` | нет | лимит размера вложения | `10485760` (10 МБ) |

\* `JIRA_EPIC_FIELD` определяется один раз: вызовите инструмент
`find_epic_field` (или `GET /rest/api/2/field`), запишите id в `.env`.

Пример `.env`:

```dotenv
JIRA_URL=https://jira.example.com
JIRA_PERSONAL_ACCESS_TOKEN=ваш_PAT
JIRA_EPIC_FIELD=customfield_10008
TASKS_DIR=./tasks
```

> Файл `.env` с токеном уже добавлен в `.gitignore` — секрет не попадёт в репозиторий.

## 4. Подготовка задач

Внутри `TASKS_DIR` создайте подпапки по эпикам (имя папки = ключ эпика), а в них
положите `.md`-файлы задач:

```
tasks/
└── PROJ-855/                     # эпик PROJ-855
    ├── C1-fix-excel-export.md
    ├── C1-fix-excel-export/    # папка задачи: вложения и картинки
    │   └── error.png
    ├── in_jira/                # сюда `pull_epic` скачивает задачи из JIRA
    │   ├── PROJ901-uskorit-vygruzku.md
    │   └── PROJ901-uskorit-vygruzku/   # вложения задачи, скачанные из JIRA
    │       └── error.png
    └── done/                   # сюда уезжают заведённые задачи
```

Формат файла (обязательный YAML front-matter + тело из трёх блоков) описан в
[docs/task-format.md](./docs/task-format.md).

## 5. Регистрация в Claude Code

Зарегистрируйте собранный бинарник как MCP-сервер. Переменные окружения
передаются флагами `-e`:

```bash
claude mcp add jira-tasks \
  -e JIRA_URL=https://jira.example.com \
  -e JIRA_PERSONAL_ACCESS_TOKEN=ваш_PAT \
  -e JIRA_EPIC_FIELD=customfield_10008 \
  -e TASKS_DIR=/path/to/your-project/go-jira-mcp/tasks \
  -- /path/to/your-project/go-jira-mcp/bin/server
```

Проверить, что сервер виден и запускается:

```bash
claude mcp list
claude mcp get jira-tasks
```

Удалить регистрацию: `claude mcp remove jira-tasks`.

> Если переменные уже лежат в `.env` рядом с бинарником/в рабочей директории,
> флаги `-e` можно опустить — сервер прочитает `.env` сам. Явные `-e` надёжнее,
> т.к. не зависят от рабочей директории, из которой клиент запускает процесс.

## 6. Использование из Claude Code

После регистрации инструменты доступны модели. В диалоге Claude Code можно
просто просить на естественном языке, например:

- «Проверь подключение к JIRA» → вызовет `check_connection`.
- «Покажи список markdown-задач по эпику PROJ-855» → `list_markdown_tasks` с `epic: "PROJ-855"`.
- «Сделай dry-run по эпику PROJ-855» → `dry_run` с `epic: "PROJ-855"` (покажет, что будет создано, без записи в JIRA).
- «Заведи все готовые задачи эпика PROJ-855» → `create_all_tasks` с `epic: "PROJ-855"`, `publish_unready: false`.
- «Заведи все задачи эпика PROJ-855, включая черновики» → `create_all_tasks` с `epic: "PROJ-855"`, `publish_unready: true`.
- «Заведи задачу из файла tasks/PROJ-855/C1-fix-excel-export.md» → `create_jira_task`.

`list_markdown_tasks`, `dry_run` и `create_all_tasks` работают **по одному эпику**:
параметр `epic` (имя папки эпика, напр. `PROJ-855`) обязателен. Эти инструменты
никогда не сканируют сразу всё дерево `TASKS_DIR` — нужно явно указать, с каким
эпиком работаем.

Рекомендуемый порядок первого запуска (см. TZ §10):

1. `check_connection` — токен и URL рабочие.
2. `find_epic_field` — один раз узнать id поля «Epic Link», записать в `.env`.
3. `dry_run` по нужному эпику — посмотреть, что будет создано (с проверкой вложений и резолва `links`).
4. `create_all_tasks` по тому же эпику — завести задачи пачкой.

### Доступные инструменты (tools)

| Tool | Что делает |
|---|---|
| `check_connection` | `GET /myself` — проверка URL и токена |
| `find_epic_field` | `GET /field` — найти id поля «Epic Link» |
| `list_markdown_tasks` | сканирует папку **одного эпика** (`epic`), отдаёт список задач (только чтение) |
| `dry_run` | показывает по **одному эпику** (`epic`), что было бы создано, без записи в JIRA |
| `create_jira_task` | создаёт/обновляет одну задачу из указанного файла |
| `create_all_tasks` | заводит все задачи **одного эпика** (`epic`) пачкой; параметр `publish_unready: bool` |

Поведение `ready`, `links`, вложений, переноса в `done/` и защиты от дублей —
строго по TZ §5 и §7.

## 7. Использование из терминала (CLI, без Claude)

Те же операции доступны без Claude/MCP — через CLI-бинарник `bin/cli` и цели
`make`. CLI вызывает тот же код (`internal/tools.Service`), что и MCP-инструменты,
поэтому поведение идентично. Конфиг читается из `.env` / переменных окружения так
же, как у сервера. Вывод — JSON.

Каждая цель `make` сама пересобирает `bin/cli` при необходимости.

| Команда `make` | Эквивалентный tool | Что делает |
|---|---|---|
| `make check_connection` | `check_connection` | `GET /myself` — проверка URL и токена |
| `make find_epic_field` | `find_epic_field` | `GET /field` — найти id поля «Epic Link» |
| `make list_markdown_tasks <эпик>` | `list_markdown_tasks` | сканирует папку одного эпика, список задач (только чтение) |
| `make dry_run <эпик>` | `dry_run` | показывает по одному эпику, что было бы создано, без записи в JIRA |
| `make create_jira_task <файл>` | `create_jira_task` | создаёт/обновляет одну задачу из файла |
| `make create_all_tasks <эпик>` | `create_all_tasks` | заводит все задачи одного эпика пачкой |
| `make pull_epic <папка-эпика>` | `pull_epic` | скачивает все задачи эпика в `<папка-эпика>/in_jira` |

Параметры:

- `PUBLISH=1` — добавляет `--publish-unready` (заводит и черновики `ready: false`).
  Применим к `dry_run`, `create_jira_task`, `create_all_tasks`.
- Эпик для `list_markdown_tasks`, `dry_run`, `create_all_tasks` передаётся
  позиционно (`make dry_run PROJ-855`) **или** через `EPIC=` (`make dry_run EPIC=PROJ-855`)
  и **обязателен**: эти команды работают только по одному эпику.
- Файл для `create_jira_task` передаётся позиционно **или** через `FILE=`.
- Папка эпика для `pull_epic` передаётся позиционно **или** через `DIR=`.

### Скачивание задач эпика (`pull_epic`)

`pull_epic` делает обратную операцию: **выгружает** все задачи эпика из JIRA в
локальные markdown-файлы. Как и все остальные команды, запускается **из корня
проекта** — заходить в папку эпика вручную не нужно. Эпик указывается путём к его
папке; make сам перейдёт в неё перед запуском CLI:

```bash
make pull_epic tasks/PROJ-855        # позиционно
make pull_epic DIR=tasks/PROJ-855    # или через DIR=
```

Что делает команда:

1. Берёт имя указанной папки как ключ эпика (напр. `PROJ-855`); если оно не похоже на
   ключ JIRA (`БУКВЫ-ЧИСЛО`) — ошибка.
2. Ищет в JIRA все задачи этого эпика (JQL по полю «Epic Link»).
3. На каждую задачу пишет `.md`-файл в подпапку **`in_jira/`** этой папки эпика:
   front-matter с полями `project`, `summary`, `type`, `labels`, `attachments` и
   уже известными `jira_key`, `jira_url`, `jira_created_at` (§5). Все задачи
   помечаются **`ready: true`**.
4. Если у задачи есть вложения — скачивает их в **папку задачи** рядом с `.md`
   (`in_jira/<имя-файла-без-расширения>/`, как при заведении новых задач, §3.3) и
   перечисляет в поле `attachments`.

Особенности:

- **Текст задачи скачивается из JIRA как есть** — описание кладётся в тело файла
  ровно так, как оно хранится в JIRA (wiki-разметка не переписывается). Это нужно,
  чтобы при последующем обновлении задачи (перенос файла в корень эпика и
  `create_jira_task`/`create_all_tasks`) текст в JIRA остался прежним.
- Имя файла — `<ключ-без-дефиса>-<слаг-темы>.md` (напр. `PROJ901-uskorit-vygruzku.md`):
  стабильно для каждой задачи, повторный запуск **перезаписывает** файлы на месте,
  а не плодит дубли. Префикс остаётся валидным (§2.1), так что файл можно перенести
  в корень папки эпика и доработать.
- Подпапка `in_jira/` при заведении задач (`create_all_tasks`) **не сканируется**
  (как и любая вложенная папка), а заданный `jira_key` защищает от повторного
  создания (§5) — скачанные мирроры не уедут обратно в JIRA случайно.

```bash
# проверки и просмотр
make check_connection
make find_epic_field
make list_markdown_tasks
make dry_run                 # только готовые
make dry_run PUBLISH=1       # включая черновики

# заведение задач
make create_jira_task tasks/PROJ-855/C1-fix-excel-export.md      # позиционно
make create_jira_task FILE=tasks/PROJ-855/C1-fix-excel-export.md # или через FILE=
make create_jira_task tasks/PROJ-855/B1-skeleton.md PUBLISH=1    # + черновик
make create_all_tasks                                          # все готовые пачкой
make create_all_tasks PUBLISH=1                                # все, включая черновики
```

Тот же бинарник можно звать напрямую (подкоманды совпадают с именами tools):

```bash
make cli                                  # собрать bin/cli
./bin/cli list_markdown_tasks
./bin/cli dry_run --publish-unready
./bin/cli create_jira_task --publish-unready tasks/PROJ-855/C1-fix-excel-export.md
./bin/cli                                 # без аргументов — справка по командам
```

> `pull_epic` эпик принимает **необязательным** аргументом: `./bin/cli pull_epic PROJ-855`
> (папка ищется под `TASKS_DIR`) или без аргумента — тогда эпиком считается текущая
> папка, как это делает цель `make pull_epic tasks/PROJ-855`. Как MCP-инструмент имя
> эпика фактически обязательно: у серверного процесса нет осмысленной рабочей папки.

> При позиционном вызове `make create_jira_task <файл>` make дополнительно
> печатает строку вида `«tasks/...» не требует обновления` — это безвредная
> косметика (путь распознаётся как имя цели); сама команда отрабатывает. Форма
> `FILE=...` этой строки не даёт.

## 8. Запуск напрямую (отладка)

Сервер общается по stdio и сам по себе интерактивно не используется, но можно
проверить, что он стартует без ошибок конфигурации:

```bash
set -a; . ./.env; set +a
./bin/server
# (ждёт MCP-клиента по stdin/stdout; Ctrl-C для выхода)
```

Логи сервер пишет в **stderr**, чтобы не мешать MCP-протоколу на stdout.

## 9. Структура проекта

```
go-jira-mcp/
├── cmd/server/main.go       # точка входа MCP-сервера (stdio)
├── cmd/cli/main.go          # CLI: те же операции без Claude/MCP
├── internal/
│   ├── config/              # чтение конфига из env/.env
│   ├── jira/                # клиент JIRA REST API v2
│   ├── markdown/            # парсер front-matter + тела, lifecycle файлов
│   └── tools/              # реализация MCP-инструментов
├── tasks/                   # эпик-папки с задачами (рантайм-данные)
├── docs/                    # ТЗ и формат задачи
├── .env                     # секреты (в .gitignore)
└── Makefile
```
