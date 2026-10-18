#!/bin/bash
# Блокирует коммит, если mypy, ruff или unit-тесты не проходят.
#
# Скрипт вызывается в двух режимах, и различать их обязательно:
#
#   claude — PreToolUse-хук Claude Code. На stdin приходит JSON, коммит нужно
#            распознать по .tool_input.command; всё остальное пропускается.
#   git    — реальный .git/hooks/pre-commit. Никакого JSON нет, а сам факт
#            вызова уже означает коммит. Режим объявляет переменная
#            TRUNKLINE_GATE_MODE, которую выставляет установленный хук.
set -uo pipefail

if [ "${TRUNKLINE_GATE_MODE:-claude}" != "git" ]; then
  INPUT=$([ -t 0 ] && echo '{}' || cat)
  CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
  case "$CMD" in
    *"git commit"*) ;;
    *) exit 0 ;;
  esac
  cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
fi

# Гейт одобрений — стек-агностичный, живёт в ядре, а не здесь: что считается
# одобрением, определяет маршрут, а маршрут одинаков на любом стеке. Вызывается
# первым: он читает журнал и дифф, это дешевле проверок типов и тестов, и
# блокировать «код до одобрения спеки» имеет смысл до того, как разработчик
# ждал mypy.
for root in .trunkline .; do
  APPROVAL="$root/core/developer/scripts/approval-gate.sh"
  if [ -f "$APPROVAL" ]; then
    bash "$APPROVAL" || exit $?
    break
  fi
done

# Чужой стек пропускается молча.
[ -f pyproject.toml ] || exit 0

# 1. Тип-чек и линт — только по проиндексированным .py: полный `mypy src` на
# каждый коммит дороже, а diff уже прошёл предыдущие коммиты.
FILES=$(git diff --cached --name-only --diff-filter=ACM -- '*.py')
if [ -n "$FILES" ]; then
  if command -v ruff >/dev/null 2>&1; then
    if ! OUT=$(echo "$FILES" | xargs ruff check 2>&1); then
      printf 'Коммит заблокирован: ruff check не проходит.\n%s\n' "$OUT" >&2
      exit 2
    fi
  fi
  if command -v mypy >/dev/null 2>&1; then
    if ! OUT=$(echo "$FILES" | xargs mypy 2>&1); then
      printf 'Коммит заблокирован: mypy не проходит.\n%s\n' "$OUT" >&2
      exit 2
    fi
  fi
fi

# 2. Unit-тесты. Integration требуют живой БД (PostgreSQL+PostGIS) и OSRM,
# поэтому в предкоммитный гейт не входят — их запускает CI (`commands.test_integration`).
if command -v pytest >/dev/null 2>&1; then
  if ! OUT=$(pytest -m "not integration" -q 2>&1); then
    printf 'Коммит заблокирован: unit-тесты не проходят.\n%s\n' "$OUT" >&2
    exit 2
  fi
fi
exit 0
