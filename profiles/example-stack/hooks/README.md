# Хуки профиля — ШАБЛОН

`pre-commit-gate.sh` — машинный гейт перед коммитом (слот `hooks.pre_commit_gate`).
Установка описана в `core/developer/ENFORCEMENT.md`.

Скрипт вызывает команды из секции `commands:` профиля. Слот `null` → соответствующая
проверка пропускается, а не подменяется выдуманной командой.

## Как это выглядит заполненным

Заполненный `run_gate` для демо-стека профиля (FastAPI + SQLAlchemy + Alembic):

```bash
run_gate() {
    step "build (mypy)" mypy app
    step "lint (ruff)"  ruff check .
    step "unit-тесты"   pytest -m "not integration" -q
}
```

Интеграционные тесты в pre-commit не гоняются: им нужны контейнеры, и коммит перестаёт
быть операцией на секунды. Их место — CI (`commands.test_integration`).
