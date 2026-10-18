# 05. Стиль схемы БД

Роль `db_schema_style`. Читают шаги 3, 6, 9.

**СУБД и время.** PostgreSQL 16 + PostGIS — единственная целевая СУБД (см.
`stack.migrations` профиля). Всё время хранится в UTC.

**Документация.** `COMMENT ON TABLE`/`COMMENT ON COLUMN` — источник правды.
Отдельного документа по таблицам вне `.sql`-файлов нет: он рассинхронизируется
с базой, а комментарий рядом с DDL — синхронизируется той же ревизией.

**Обязательные формы полей.**
- Первичный ключ: `BIGINT GENERATED ALWAYS AS IDENTITY`. `SERIAL`/`BIGSERIAL`
  запрещены — они не переносятся между окружениями так же предсказуемо, как `IDENTITY`.
- Время создания/изменения — `created_at`, `updated_at`: `TIMESTAMPTZ NOT NULL DEFAULT
  now()`. `TIMESTAMP` без зоны и числа-эпохи запрещены.
- Геометрия: `geometry(Point, 4326)` (PostGIS, SRID 4326 = WGS84).

**Нейминг.** Таблица — существительное во множественном числе; критерий: имя отвечает
на вопрос, чем является **одна запись** (`requests` → «заявка», `brigades` → «бригада»).
Внешний ключ — `<таблица_в_ед_числе>_id`. Индекс — `ix_<таблица>__<поля>`, уникальный —
`ux_<таблица>__<поля>`.

**Индексы.** Обязательны на каждый внешний ключ и на каждое поле, по которому в задаче
есть фильтрация или сортировка (например, `status` заявки, временнóе окно), и `GIST`-
индекс на геометрической колонке при её наличии (`CONCURRENTLY` на боевой таблице,
см. `01-migrations.md`).

**Доступ.** На каждую новую таблицу — `GRANT SELECT, INSERT, UPDATE, DELETE ON <t> TO
app_rw;` и `GRANT SELECT ON <t> TO app_ro;` в той же ревизии.

**Комментарий — «суть + зачем».**
Хорошо: `'Момент выезда бригады на заявку; NULL — бригада ещё не выехала'`.
Плохо: `'Время выезда'` — не говорит, что значит `NULL`, ни к какому событию привязано.

```sql
-- Заявка на выезд: одна запись — одна заявка от диспетчерского центра.
CREATE TABLE requests (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    priority            TEXT        NOT NULL CHECK (priority IN ('normal', 'accident')),
    -- Требуемый навык бригады; сверяется с requests.required_skill в сервисе, не в БД.
    required_skill      TEXT        NOT NULL,
    geom                geometry(Point, 4326) NOT NULL,
    time_window_start   TIMESTAMPTZ NOT NULL,
    time_window_end     TIMESTAMPTZ NOT NULL,
    status              TEXT        NOT NULL CHECK (status IN ('pending', 'assigned', 'done')),
    assigned_engineer_id BIGINT     REFERENCES engineers (id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_requests__status ON requests (status);
CREATE INDEX ix_requests__assigned_engineer_id ON requests (assigned_engineer_id);
CREATE INDEX ix_requests__geom ON requests USING GIST (geom);

COMMENT ON TABLE  requests IS 'Заявка на выезд: одна запись — одна заявка от диспетчерского центра';
COMMENT ON COLUMN requests.assigned_engineer_id IS 'Назначенная бригада; NULL — заявка не назначена ни одной бригаде';

GRANT SELECT, INSERT, UPDATE, DELETE ON requests TO app_rw;
GRANT SELECT ON requests TO app_ro;
```

## Чек-лист

- [ ] Первичный ключ и временные метки — в обязательной форме.
- [ ] Индексы есть на каждом внешнем ключе и на полях фильтрации/сортировки.
- [ ] Таблица и колонки документированы `COMMENT ON`.
- [ ] Нейминг таблиц, ключей и индексов соответствует конвенции.
