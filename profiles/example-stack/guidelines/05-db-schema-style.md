# 05. Стиль схемы БД

**Скелет.** Роль `db_schema_style`. Читают шаги 3, 6, 9.

Описать здесь:

- **СУБД и часовой пояс хранения** — одной строкой в начале, это влияет на всё остальное.
- **Где живёт документация** по таблицам и полям. Комментарии в самой базе не
  рассинхронизируются с ней; отдельный документ рассинхронизируется всегда.
- **Обязательные формы полей** — автоинкремент, дата/время: строго, с перечислением
  запрещённых альтернатив. Формулировка «предпочтительно» здесь не работает.
- **Нейминг** — как называется таблица (хороший критерий: имя отвечает на вопрос, чем
  является одна запись), поля из внешних систем, ключи, индексы.
- **Доступ** — какие `grant` обязательны.
- **Индексы** — под что заводятся обязательно.
- **Содержание комментария** — «суть + зачем», с примером хорошего и плохого.

Заканчивать **одним полным примером таблицы**, в котором видны все обязательные формы
сразу: он экономит больше, чем мини-пример под каждым правилом.

## Чек-лист

- [ ] <по пункту на правило выше>

---

## Как это выглядит заполненным

Пример на демо-стеке профиля (PostgreSQL 16). Заменить на свои правила.

> **СУБД и время.** PostgreSQL 16, всё время хранится в UTC (`TIMESTAMPTZ`).
>
> **Документация.** Только `COMMENT ON` в самой базе. Отдельного документа по таблицам
> нет: он рассинхронизируется всегда, комментарий в базе — никогда.
>
> **Обязательные формы полей.** Первичный ключ — `id BIGINT GENERATED ALWAYS AS IDENTITY`;
> `SERIAL` и `BIGSERIAL` запрещены. Время создания и изменения — `created_at`, `updated_at`
> типа `TIMESTAMPTZ NOT NULL DEFAULT now()`; `TIMESTAMP` без зоны и `epoch`-числа запрещены.
>
> **Нейминг.** Таблица — существительное во множественном числе; критерий: имя отвечает
> на вопрос, чем является **одна запись** (`orders` → «заказ»). Поле из внешней системы
> несёт её префикс (`crm_client_id`). Внешний ключ — `<таблица_в_ед_числе>_id`.
> Индекс — `ix_<таблица>__<поля>`, уникальный — `ux_<таблица>__<поля>`.
>
> **Доступ.** На каждую новую таблицу — `GRANT SELECT, INSERT, UPDATE, DELETE ON <t> TO app_rw;`
> и `GRANT SELECT ON <t> TO app_ro;` в том же файле миграции.
>
> **Индексы.** Обязательны на каждый внешний ключ и на каждое поле, по которому
> в задаче есть фильтрация или сортировка.
>
> **Комментарий — «суть + зачем».**
> Хорошо: `'Момент подтверждения оплаты банком; NULL — оплата не подтверждена'`.
> Плохо: `'Дата оплаты'` — не говорит ни что значит NULL, ни чей это момент.

```sql
CREATE TABLE billing.orders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id   BIGINT      NOT NULL REFERENCES billing.clients (id),
    crm_deal_id TEXT,
    amount      NUMERIC(19, 4) NOT NULL,
    status      TEXT        NOT NULL CHECK (status IN ('new', 'paid', 'cancelled')),
    paid_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_orders__client_id ON billing.orders (client_id);
CREATE UNIQUE INDEX ux_orders__crm_deal_id ON billing.orders (crm_deal_id)
    WHERE crm_deal_id IS NOT NULL;

COMMENT ON TABLE  billing.orders         IS 'Заказ клиента: одна запись — один оформленный заказ';
COMMENT ON COLUMN billing.orders.crm_deal_id IS 'Идентификатор сделки в CRM; NULL — заказ заведён не из CRM';
COMMENT ON COLUMN billing.orders.paid_at IS 'Момент подтверждения оплаты банком; NULL — оплата не подтверждена';

GRANT SELECT, INSERT, UPDATE, DELETE ON billing.orders TO app_rw;
GRANT SELECT ON billing.orders TO app_ro;
```
