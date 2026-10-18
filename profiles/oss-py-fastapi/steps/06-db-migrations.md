# Надстройка профиля oss-py-fastapi — шаг 6 (Миграции БД)

Читать сразу после `core/developer/steps/06-db-migrations.md`.

**Гайдлайны:** `guidelines/01-migrations.md`, `03-data-types.md`,
`05-db-schema-style.md`, `07-aiosql.md`.

Уточнения:

- Ревизия создаётся `alembic revision -m "<slug>"`, `upgrade()`/`downgrade()` —
  чистый SQL через `op.execute(...)`, без деклативных ORM-моделей.
- `GIST`-индекс на боевой таблице — `CREATE INDEX CONCURRENTLY` с
  `# transactional_ddl = False` в файле ревизии.
- Порядок артефактов одной задачи: ревизия Alembic → `queries/*.sql`, если запросы
  ссылаются на новую колонку/таблицу → маппинг в репозитории (`03-data-types.md`).
- Совместимость с работающим кодом обязательна: `NOT NULL` без значения по
  умолчанию и переименование колонки одним шагом — запрещены (`01-migrations.md`).

**Выход:** новая ревизия `alembic/versions/`, согласованные с ней `queries/*.sql`
и репозиторий.
