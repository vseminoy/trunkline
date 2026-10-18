# 09. Структурированное логирование

Роль `logging` (не спрашивается ядром — используется надстройками шагов профиля, шаги 8,
9 и 16).

Логирование — структурированное, по модели `log/slog` из Go: у каждой записи есть
**стабильное имя события** и **поля ключ–значение**; значения в текст сообщения не
интерполируются. Запись — это событие с данными, а не строка для человека: по ней ищут,
фильтруют и считают.

## Библиотека: structlog поверх stdlib `logging`

`structlog` — единственный API логирования в коде сервиса. Он настраивается **поверх**
stdlib `logging` (`structlog.stdlib.ProcessorFormatter`), поэтому записи сторонних
библиотек (uvicorn, alembic, httpx, psycopg), которые пишут через stdlib, проходят через
тот же конвейер процессоров и выходят в том же формате. Один процесс — один формат вывода.

Почему не голый stdlib с `extra={...}`: стандартный форматтер поля из `extra` не выводит
вовсе, а контекст (`request_id`, `run_id`) пришлось бы пробрасывать вручную в каждый
вызов. Почему не свой JSON-форматтер: нет bound-логгеров, и контекст запроса снова
становится ручной работой.

| `slog` (Go) | `structlog` |
|---|---|
| `slog.Info("msg", "k", v)` | `log.info("event_name", k=v)` |
| `logger.With("k", v)` | `log = log.bind(k=v)` |
| контекст через `ctx` + handler | `structlog.contextvars.bind_contextvars(k=v)` |
| `slog.NewJSONHandler` / `NewTextHandler` | `JSONRenderer()` / `ConsoleRenderer()` |
| `slog.SetDefault` | `configure_logging(settings)` — один раз при старте |

## Настройка

Одна функция `configure_logging(settings)` в `src/logging.py`, вызывается один раз в
app factory до создания приложения. Параметры — из `pydantic-settings`:

- `LOG_LEVEL` — `DEBUG` | `INFO` | `WARNING` | `ERROR`, по умолчанию `INFO`;
- `LOG_FORMAT` — `json` (контейнер, стенд) | `console` (локальная разработка, цветной
  человекочитаемый вывод), по умолчанию `json`.

Вывод — только `stdout` (контейнер собирает поток сам; файлов логов сервис не пишет).

Цепочка процессоров (общая для structlog и для stdlib-записей через `foreign_pre_chain`):

```python
shared = [
    structlog.contextvars.merge_contextvars,   # request_id, run_id из контекста
    structlog.stdlib.add_log_level,
    structlog.stdlib.add_logger_name,
    structlog.processors.TimeStamper(fmt="iso", utc=True),  # см. «Время» ниже
    structlog.processors.StackInfoRenderer(),
    structlog.processors.format_exc_info,       # только для json; console рисует сам
]
renderer = JSONRenderer() if settings.log_format == "json" else ConsoleRenderer()
```

**Время.** По умолчанию профиль пишет метку времени в UTC ISO-8601. Если `AGENTS.md`
проекта задаёт другую политику времени (например, наивное локальное время), метка
логов следует ей (`TimeStamper(fmt="iso", utc=False)`) — формат времени в логах не
должен расходиться с временем в БД и API.

**Access-лог uvicorn выключен** (`--no-access-log` или `log_config`): HTTP-запрос
логирует middleware приложения — одной записью со всеми полями. Иначе каждый запрос
пишется дважды, и в одной из двух записей нет `request_id`.

## Правила записи

- **Имя события** — `snake_case`, вида `<объект>_<что_произошло>`: `osrm_request_finished`,
  `plan_build_failed`, `ticket_status_changed`. Имя стабильно: по нему строятся поиск и
  алерты, переименование — это изменение наблюдаемости, а не косметика.
- **Данные — только полями.** `log.info("osrm_request_finished", endpoint="table",
  status=200, duration_ms=412)`, а не `log.info(f"OSRM ответил {status} за {ms} мс")`.
- **Единицы — в имени поля:** `duration_ms`, `distance_m`, `size_bytes`.
- **Логгер модуля** — `log = structlog.get_logger(__name__)` на уровне модуля. Это не
  нарушает правило DI из `06-code-style.md`: логгер не держит состояния, которое нужно
  подменять в тесте, — конфигурация глобальна по своей природе, как и в `slog.SetDefault`.
- **Контекст запроса.** HTTP-middleware в начале запроса вызывает `clear_contextvars()` и
  `bind_contextvars(request_id=...)` (из заголовка `X-Request-ID` или новый UUID; тот же
  id возвращается в ответном заголовке), в конце пишет `http_request_finished` с `method`,
  `path` (шаблон маршрута, а не URL с параметрами), `status`, `duration_ms`.
  Доменный идентификатор долгой операции (например, `run_id` построения плана)
  добавляется через `bind_contextvars` в начале операции — все записи внутри неё несут
  его без явной передачи.
- **Контекст и потоки.** `contextvars` не переходят в поток автоматически при
  `run_in_executor`; `anyio.to_thread.run_sync` и `asyncio.to_thread` контекст копируют.
  CPU-bound код (солвер) запускать через них, чтобы записи из потока сохранили
  `request_id`/`run_id`.
- **Исключения** — `log.exception("event_name", ...)` внутри `except`: трассировка
  попадает в запись структурно. `error=str(e)` вместо этого — теряет стек.
- **Уровни.** `debug` — детали для разработки (по записи на элемент цикла); `info` —
  границы операций (начало/конец, итоговые счётчики); `warning` — деградация, после
  которой работа продолжается (fallback, повтор), и отказ по вине клиента (`4xx`);
  `error` — операция не выполнена по вине сервера или зависимости (`5xx`).
  Запись на каждую итерацию цикла — только `debug`.

Что и где логируется при ошибках (бизнес-ошибка в маршруте, ошибка БД/внешнего
сервиса в месте возникновения) — `10-errors.md`.

## Что не попадает в логи никогда

Токены, пароли, строки подключения с паролем, заголовки `Authorization`/`Cookie`, тела
запросов и ответов целиком, персональные данные (ФИО, телефоны, адреса клиентов) — только
их идентификаторы. Флаг `secrets_in_code` на логи не распространяется.

## Тесты

Логирование проверяется по событию и полям, а не по тексту:
`structlog.testing.capture_logs()` возвращает список словарей записей —
`assert {"event": "plan_build_finished", "assigned": 12} <= entries[-1].items()`.
Отдельный тест на `configure_logging` проверяет, что `json`-режим даёт одну валидную
JSON-строку на запись, а stdlib-запись стороннего логгера выходит в том же формате.

## Чек-лист

- [ ] Нет `print` и нет f-строк/`%`-интерполяции данных в сообщении лога.
- [ ] Имя события — стабильный `snake_case`, данные — полями, единицы — в имени поля.
- [ ] `configure_logging` вызывается один раз; формат и уровень — из настроек.
- [ ] Access-лог uvicorn выключен, HTTP-запрос логирует middleware с `request_id`.
- [ ] Долгая операция привязывает свой идентификатор через `bind_contextvars`.
- [ ] В записях нет секретов и персональных данных.
