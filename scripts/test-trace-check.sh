#!/usr/bin/env bash
# test-trace-check.sh — таблица случаев для `trunkline trace-check`
# (internal/cli/tracecheck.go)
#
# Зачем. Это вторая из двух машинных проверок пайплайна и единственная, которая
# закрывает его *тихое* вырождение: правило, не дошедшее до артефакта, ничем
# больше не видно. Команда решает проверки 1-2 шага 7a, а с введением цикла по
# changeset'ам получила ещё и фильтрацию реестра — реестр пишется один раз на
# задачу, а сверка идёт на каждый changeset. Без фильтра правила будущих
# changeset'ов приходят как пробелы, и первый changeset не проходит 7a никогда.
#
# Запуск: bash scripts/test-trace-check.sh

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$(mktemp -d)/trunkline"
( cd "$ROOT" && go build -o "$BIN" ./cmd/trunkline ) || { echo "сборка trunkline не удалась" >&2; exit 2; }
TC="$BIN trace-check"

PASS=0; FAIL=0
WORK=$(mktemp -d); trap 'rm -rf "$WORK" "$(dirname "$BIN")"' EXIT

# Карточки покрытия: contract и tests несут R-007-1 и R-007-2, больше ничего.
mkdir -p "$WORK/m"
cat > "$WORK/m/cov-contract.md" <<'EOF'
| ID | Где в спеке |
| R-007-1 | createPlan.request.amount (minimum: 1) |
| R-007-2 | createPlan.request.currency (enum) |
EOF
cat > "$WORK/m/cov-tests.md" <<'EOF'
| ID | Тесты |
| R-007-1 | TestZeroAmount |
| R-007-2 | TestBadCurrency |
EOF

# Реестр всей задачи: шесть правил, разнесённых по трём changeset'ам.
cat > "$WORK/m/rules.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-2 | currency in RUB, USD | analysis | 1 |
| R-007-3 | name <= 100 chars | analysis | 2 |
| R-007-4 | day in 1..28 | analysis | 2 |
| R-007-8 | idempotency key | step 2 | 3 |
| R-007-9 | superseded by R-007-8 [WITHDRAWN] | step 2 | — |
EOF

# run_case <имя> <ожидаемый код> <аргументы…>
run_case() {
  local name="$1" want="$2"; shift 2
  local out rc
  out=$($TC "$@" 2>&1); rc=$?
  if [ "$rc" = "$want" ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"
  else
    FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали exit %s, получили %s\n' "$name" "$want" "$rc"
    printf '%s\n' "$out" | sed 's/^/          /' | tail -5
  fi
}

C=(--column "contract=$WORK/m/cov-contract.md" --column "tests=$WORK/m/cov-tests.md")

printf 'a: {d: "R-007-1"}\nb: {d: "R-007-2"}\nc: {d: "R-007-3"}\n' > "$WORK/m/c6art.yaml"
printf '| T1 | x | y | R-007-1 |\n| T2 | x | y | R-007-2 |\n| T3 | x | y | R-007-3 |\n' > "$WORK/m/c6art.md"

# Реестр из двух правил, оба покрыты contract и tests.
cat > "$WORK/m/orphan2.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-2 | currency in RUB, USD | analysis | 1 |
EOF
# Реестр из одного правила: R-007-2 в артефактах станет орфаном.
cat > "$WORK/m/onlyr1.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
EOF

echo "trunkline trace-check — таблица случаев"

# --- Регрессия цикла: без фильтра первый changeset не проходит никогда --------
run_case "changeset 1 из 3: только свои правила → PASSED" 0 \
  --rules "$WORK/m/rules.md" "${C[@]}" --changeset 1

run_case "без --changeset тот же реестр даёт пробелы (маршрут без шага 4)" 1 \
  --rules "$WORK/m/rules.md" "${C[@]}"

run_case "changeset 2: его правил в артефактах нет → пробелы" 1 \
  --rules "$WORK/m/rules.md" "${C[@]}" --changeset 2

# --- Колонка Changeset как контракт между шагами 4 и 7a -----------------------
printf '| ID | Rule | Origin |\n|--|--|--|\n| R-007-1 | x | analysis |\n| R-007-2 | y | analysis |\n' \
  > "$WORK/m/nocol.md"
run_case "шаг 4 не заполнил колонку → это ошибка запуска, не пропуск" 2 \
  --rules "$WORK/m/nocol.md" "${C[@]}" --changeset 1

cat > "$WORK/m/unassigned.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-2 | currency | analysis | 1 |
| R-007-3 | name length | analysis |  |
EOF
run_case "правило, не попавшее ни в один changeset → FAILED" 1 \
  --rules "$WORK/m/unassigned.md" "${C[@]}" --changeset 1

# --- Поведение, которое фильтр не должен был сломать --------------------------
run_case "WITHDRAWN не ждут нигде и он не пробел" 0 \
  --rules "$WORK/m/rules.md" "${C[@]}" --changeset 1

cat > "$WORK/m/gap.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-9 | never written anywhere | analysis | 1 |
EOF
run_case "настоящий пробел внутри changeset'а ловится" 1 \
  --rules "$WORK/m/gap.md" "${C[@]}" --changeset 1

# Отдельный реестр: R-007-1 и R-007-2 покрыты артефактами (иначе они пришли бы орфанами),
# а R-007-9 объявлен неприменимым в обеих живых колонках.
cat > "$WORK/m/na.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-2 | currency in RUB, USD | analysis | 1 |
| R-007-9 | audit log retention 90 days | analysis | 1 |
EOF
run_case "объявленный --na закрывает честную неприменимость" 0 \
  --rules "$WORK/m/na.md" "${C[@]}" --changeset 1 --na R-007-9=contract --na R-007-9=tests

run_case "та же неприменимость без --na — это пробел" 1 \
  --rules "$WORK/m/na.md" "${C[@]}" --changeset 1

cat > "$WORK/m/orphan.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
EOF
# --- Полные и частичные колонки ----------------------------------------------
# Схема несёт только R-007-1: по шагу 6 идентификатор получает лишь то ограничение,
# которое правило и выражает. Как обычная колонка она даёт ложный пробел на R-007-2.
mkdir -p "$WORK/m/mig"
printf 'create table t (amount numeric); -- R-007-1: amount above zero\n' > "$WORK/m/mig/001.sql"

run_case "схема как обычная колонка → ложный пробел на правиле без ограничения" 1 \
  --rules "$WORK/m/orphan2.md" "${C[@]}" --column "schema=$WORK/m/mig" --changeset 1

run_case "та же схема как --partial → отсутствие не пробел" 0 \
  --rules "$WORK/m/orphan2.md" "${C[@]}" --partial "schema=$WORK/m/mig" --changeset 1

run_case "орфан в частичной колонке ловится по-прежнему" 1 \
  --rules "$WORK/m/onlyr1.md" --column "tests=$WORK/m/cov-tests.md" \
  --partial "contract=$WORK/m/cov-contract.md" --changeset 1

run_case "только --partial — полноту проверять нечем" 2 \
  --rules "$WORK/m/orphan2.md" --partial "contract=$WORK/m/cov-contract.md" \
  --partial "tests=$WORK/m/cov-tests.md" --changeset 1

# --- Накопление внутри одной задачи ------------------------------------------
# Артефакты общие для всех changeset'ов задачи: к моменту 7a второго в контракте
# уже лежат правила первого. Орфан — это «нет в реестре ВООБЩЕ», а не «нет в
# срезе», иначе тупик просто переезжает с первого changeset'а на второй.
cat > "$WORK/m/two_cs.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | analysis | 1 |
| R-007-2 | currency in RUB, USD | analysis | 2 |
EOF
run_case "changeset 2: правила changeset 1 в артефактах — не орфаны" 0 \
  --rules "$WORK/m/two_cs.md" "${C[@]}" --changeset 2

# --- Пустой реестр ------------------------------------------------------------
# Фича, поведение которой целиком следует из гайдлайнов профиля, своих правил
# не имеет. Шаг 2 прав, написав пустой реестр; падать тут нечему.
printf '| ID | Rule | Origin | Changeset |\n|--|--|--|--|\n' > "$WORK/m/empty.md"
printf '{"ok": true}\n' > "$WORK/m/noids.json"
printf '| Test | Scenario | Expected | Rule |\n| TestHealth | GET | 200 | — |\n' > "$WORK/m/noids.md"
run_case "пустой реестр, кодов в артефактах нет → PASSED" 0 \
  --rules "$WORK/m/empty.md" --column "contract=$WORK/m/noids.json" \
  --column "tests=$WORK/m/noids.md" --changeset 1

# Но пустой реестр не должен прятать неверный --rules: коды в артефактах
# становятся орфанами и валят прогон.
run_case "пустой реестр, а в артефактах коды → орфаны, FAILED" 1 \
  --rules "$WORK/m/empty.md" "${C[@]}" --changeset 1

# --- Проверка 6: ничего не потеряно на передаче из задачи в реестр ------------
# Реестр самосогласован и без потерянного требования — пропажа видна только при
# сверке с текстом задачи, поэтому шаг 2 его и записывает.
cat > "$WORK/m/c6.md" <<'EOF'
## Requirements from the task text

1. КОГДА создан абонент, сервис ДОЛЖЕН вернуть 201.
2. ЕСЛИ номер занят, ТО сервис ДОЛЖЕН вернуть 409.

## Rules

| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | создание возвращает 201 | task item 1 | 1 |
| R-007-2 | дубль номера 409 | task item 2 | 1 |
EOF
run_case "все требования задачи стали правилами → PASSED" 0 \
  --rules "$WORK/m/c6.md" "${C[@]}" --changeset 1

cat > "$WORK/m/c6lost.md" <<'EOF'
## Requirements from the task text

1. КОГДА создан абонент, сервис ДОЛЖЕН вернуть 201.
2. ЕСЛИ номер занят, ТО сервис ДОЛЖЕН вернуть 409.
3. Сервис ДОЛЖЕН логировать отказы провайдера.

## Rules

| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | создание возвращает 201 | task item 1 | 1 |
| R-007-2 | дубль номера 409 | task item 2 | 1 |
EOF
run_case "требование задачи не стало правилом → LOST" 1 \
  --rules "$WORK/m/c6lost.md" "${C[@]}" --changeset 1

cat > "$WORK/m/c6dev.md" <<'EOF'
## Requirements from the task text

1. КОГДА создан абонент, сервис ДОЛЖЕН вернуть 201.
2. ЕСЛИ номер занят, ТО сервис ДОЛЖЕН вернуть 409.

## Rules

| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | создание возвращает 201 | task item 1 | 1 |
| R-007-2 | дубль номера 409 | task item 2 | 1 |
| R-007-3 | правило разработчика | development | 1 |
EOF
run_case "правило разработчика не требует пункта в тексте" 0 \
  --rules "$WORK/m/c6dev.md" --column "contract=$WORK/m/c6art.yaml" \
  --column "tests=$WORK/m/c6art.md" --changeset 1

cat > "$WORK/m/c6bad.md" <<'EOF'
## Requirements from the task text

1. КОГДА создан абонент, сервис ДОЛЖЕН вернуть 201.

## Rules

| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | создание возвращает 201 | task item 1 | 1 |
| R-007-2 | взято из воздуха | task item 7 | 1 |
EOF
run_case "происхождение ссылается на несуществующий пункт → ORIGIN" 1 \
  --rules "$WORK/m/c6bad.md" "${C[@]}" --changeset 1

run_case "R-007-2 в артефактах, но не в реестре → orphan" 1 \
  --rules "$WORK/m/orphan.md" "${C[@]}" --changeset 1

# --- Вторая задача по тому же модулю -----------------------------------------
# Прогону могут передать карточки покрытия прошлого прогона — на этом построен
# маршрут `audit`. Без учёта номера прогона их строки приходят орфанами, и 7a
# зелёный только на первой задаче по модулю. Реестр лежит в .pipeline/workdir/rules/NNN.md
# — номер прогона берётся из имени файла, поэтому работает и на пустом реестре.
mkdir -p "$WORK/p/rules"
cat > "$WORK/m/carry.yaml" <<'EOF'
| R-006-1 | createPlan.request.amount (minimum: 1) |
| R-006-2 | createPlan.request.currency (enum) |
| R-008-1 | createPlan.request.plan (maxLength: 100) |
EOF
cat > "$WORK/m/carry.md" <<'EOF'
| Test | Scenario | Expected | Rule |
| TestZeroAmount  | amount=0 | 400 | R-006-1 |
| TestBadCurrency | EUR      | 400 | R-006-2 |
| TestLongName    | 101 ch   | 400 | R-008-1 |
EOF
CARRY=(--column "contract=$WORK/m/carry.yaml" --column "tests=$WORK/m/carry.md")
cat > "$WORK/p/rules/008.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-008-1 | plan name <= 100 chars | task item 1 | 1 |
EOF
run_case "правила прошлых прогонов в переданных карточках — не орфаны" 0 \
  --rules "$WORK/p/rules/008.md" "${CARRY[@]}" --changeset 1

# Пустой реестр прогона 008: R-006-* остаются прошлым прогоном, а не орфанами.
printf '| ID | Rule | Origin | Changeset |\n|----|------|--------|-----------|\n' \
  > "$WORK/p/rules/009.md"
printf 'a: {d: "R-006-1"}\n' > "$WORK/m/onlyold.yaml"
printf '| T | x | y | R-006-1 |\n' > "$WORK/m/onlyold.md"
run_case "пустой реестр + только прошлые прогоны в карточках — PASSED" 0 \
  --rules "$WORK/p/rules/009.md" --column "contract=$WORK/m/onlyold.yaml" \
  --column "tests=$WORK/m/onlyold.md"

# Идентификатор *будущего* прогона орфаном остаётся: так виден --rules,
# показывающий на устаревший реестр.
printf 'a: {d: "R-010-1"}\n' > "$WORK/m/future.yaml"
printf '| T | x | y | R-010-1 |\n' > "$WORK/m/future.md"
run_case "идентификатор более позднего прогона — всё ещё орфан" 1 \
  --rules "$WORK/p/rules/009.md" --column "contract=$WORK/m/future.yaml" \
  --column "tests=$WORK/m/future.md"

# --- Владение файлом: одна колонка, один файл ---------------------------------
# Карточки покрытия лежат в одном каталоге, и каталог, отданный колонке, обходит
# их все. Без владения файлом правило, доехавшее до описания тестов и не дальше,
# отчитывается как доехавшее до схемы — ровно тот тихий отказ, ради которого
# прогон и заводится.
mkdir -p "$WORK/c/internal/plan" "$WORK/c/mig"
cat > "$WORK/c/rules.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | amount > 0 | task item 1 | 1 |
| R-007-2 | currency supported | task item 2 | 1 |
EOF
printf '| T1 | x | y | R-007-1 |\n| T2 | x | y | R-007-2 |\n' > "$WORK/c/internal/plan/cov-tests.md"
printf 'a: {d: "R-007-1"}\nb: {d: "R-007-2"}\n' > "$WORK/c/internal/plan/api.yaml"

# Две колонки накрывают файл одинаково — вопрос без ответа, а не жребий.
printf 'alter table t add check (amount > 0); -- R-007-1\n' > "$WORK/c/mig/001.sql"
run_case "один каталог в двух колонках — ошибка, а не выбор наугад" 2 \
  --rules "$WORK/c/rules.md" --column "contract=$WORK/c/internal/plan,$WORK/c/mig" \
  --partial "schema=$WORK/c/mig" --changeset 1

# Колонка, у которой не осталось своих файлов, отчиталась бы сплошными пробелами.
run_case "колонка без собственных файлов — exit 2, а не пробелы" 2 \
  --rules "$WORK/c/rules.md" \
  --column "contract=$WORK/c/internal/plan/cov-tests.md" \
  --column "tests=$WORK/c/internal/plan/cov-tests.md" --changeset 1

run_case "одна колонка — сверять не с чем" 2 \
  --rules "$WORK/m/rules.md" --column "tests=$WORK/m/cov-tests.md" --changeset 1

run_case "нет rules.md — это дефект шага 2, а не пропуск" 2 \
  --rules "$WORK/m/nope.md" "${C[@]}" --changeset 1

echo
if [ "$FAIL" -eq 0 ]; then echo "$PASS случаев — PASSED"; else echo "$PASS ok, $FAIL FAILED"; fi
exit $(( FAIL > 0 ))
