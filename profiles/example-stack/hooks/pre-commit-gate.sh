#!/usr/bin/env bash
# Машинный гейт перед коммитом. Установка — core/developer/ENFORCEMENT.md.
#
# ШАБЛОН: заменить команды в run_gate на слоты `commands:` своего профиля.
# Слот `null` в профиле → соответствующая строка удаляется, а не подменяется
# выдуманной командой: пропущенная проверка честнее зелёной несуществующей.
set -uo pipefail

failed=0

step() {  # step "<имя>" <команда...>
    local name="$1"; shift
    echo "── $name"
    if ! "$@"; then
        echo "   ✗ $name" >&2
        failed=1
    fi
}

run_gate() {
    # Пример заполнения для демо-стека профиля (FastAPI + SQLAlchemy + Alembic):
    #
    # step "build (mypy)"  mypy app
    # step "lint (ruff)"   ruff check .
    # step "unit-тесты"    pytest -m "not integration" -q
    #
    # Интеграционные тесты в pre-commit не гоняются: им нужны контейнеры,
    # и коммит перестанет быть операцией на секунды. Их место — CI.

    echo "pre-commit gate: шаблон не заполнен — проверки не выполнялись" >&2
    failed=1
}

run_gate

if [ "$failed" -ne 0 ]; then
    echo "pre-commit gate: НЕ ПРОЙДЕН — коммит остановлен" >&2
    exit 1
fi

echo "pre-commit gate: пройден"
