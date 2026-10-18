#!/usr/bin/env bash
# test-kb-sync.sh — таблица случаев для `trunkline kb-sync`
# (internal/cli/kbsync.go)
#
# Зачем. Это машинная половина шага 21 — единственного шага, который читает базу
# знаний обратно. У базы один отказ, который стоит дорого: не пропуск записи
# (пропуск виден — шаг 1 пересобирает AS-IS и говорит об этом), а *враньё*.
# Неверную запись шаг 1 берёт за исходный AS-IS, и прогон, поверивший ей, в код
# уже не смотрит. Скрипт решает механическую часть: якорь разрешается, файл
# существует, идентификатор кому-то принадлежит. Смысл — за шагом 21.4.
#
# Отдельная забота теста — чтобы «чисто» и «нечего сверять» не слипались:
# отсутствие записи о модуле это exit 0 с явным NOTE (первое описание), а не
# зелёная сверка. Слипание этих двух исходов и есть способ получить базу, про
# которую все думают, что она проверена.
#
# Запуск: bash scripts/test-kb-sync.sh

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINDIR=$(mktemp -d)
( cd "$ROOT" && go build -o "$BINDIR/trunkline" ./cmd/trunkline ) || { echo "сборка trunkline не удалась" >&2; exit 2; }
KB="$BINDIR/trunkline kb-sync"

PASS=0; FAIL=0
WORK=$(mktemp -d); trap 'rm -rf "$WORK" "$BINDIR"' EXIT
R="$WORK/proj"

# sedi — in-place sed that works on both GNU sed (Linux) and BSD sed (macOS),
# whose -i takes the backup suffix as a mandatory, separate argument.
sedi() {
  if sed --version >/dev/null 2>&1; then sed -i "$@"; else sed -i '' "$@"; fi
}

# --- проект: модуль src/payments, база знаний, реестр правил ------------------
# proj — имя каталога репозитория, оно же умолчание флага project: база знаний
# держит по каталогу на проект, INDEX.md лежит в её корне и покрывает все.
mkdir -p "$R/src/payments" "$R/.pipeline/knowledge/proj/modules" \
         "$R/.pipeline/knowledge/proj/slices" "$R/.pipeline/workdir/rules"

cat > "$R/src/payments/service.go" <<'EOF'
package payments

// R-004-2: сумма платежа больше нуля
func CreatePayment(amount int) error { return nil }
EOF
cat > "$R/src/payments/handler.go" <<'EOF'
package payments

func Charge() {}
EOF
cat > "$R/.pipeline/workdir/rules/260813142530.md" <<'EOF'
| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-004-2 | сумма > 0 | development | |
EOF
touch "$R/.pipeline/knowledge/proj/slices/260813-payments-create.md"
cat > "$R/.pipeline/knowledge/INDEX.md" <<'EOF'
# Code knowledge index

| Project | Module | File | Last slice | Date |
|---|---|---|---|---|
| proj | src/payments | proj/modules/payments.md | 260813-payments-create | 2026-08-13 |
| other | src/payments | other/modules/payments.md | 260901-x | 2026-09-01 |
EOF

ENTRY="$R/.pipeline/knowledge/proj/modules/payments.md"

# entry <строки таблицы правил…> — запись, у которой всё остальное корректно
entry() {
  {
    echo "# Module: src/payments"
    echo
    echo "Updated: 2099-01-01 | Slice: 260813-payments-create"
    echo
    echo "## Layers and key types"
    echo '- service: `src/payments/service.go` — `CreatePayment`'
    echo '- handler: `src/payments/handler.go` — `Charge`'
    echo
    echo "## Business rules in force"
    echo "| ID | Statement | Where it is enforced | Slice |"
    echo "|---|---|---|---|"
    for row in "$@"; do echo "$row"; done
  } > "$ENTRY"
}

CLEAN='| R-004-2 | сумма > 0 | `src/payments/service.go:CreatePayment` | 260813-payments-create |'

# run_case <имя> <ожидаемый код> <ожидаемая метка в выводе или -> <аргументы…>
run_case() {
  local name="$1" want="$2" tag="$3"; shift 3
  local out rc
  out=$($KB --repo "$R" --module src/payments --ext .go "$@" 2>&1); rc=$?
  local bad=""
  [ "$rc" = "$want" ] || bad="ожидали exit $want, получили $rc"
  if [ -z "$bad" ] && [ "$tag" != "-" ] && ! printf '%s' "$out" | grep -q "$tag"; then
    bad="в выводе нет $tag"
  fi
  if [ -z "$bad" ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"
  else
    FAIL=$((FAIL+1)); printf '  FAIL  %s — %s\n' "$name" "$bad"
    printf '%s\n' "$out" | sed 's/^/          /' | head -8
  fi
}

echo "trunkline kb-sync — таблица случаев"

# --- зелёный: запись сходится с кодом ----------------------------------------
# Важен не сам факт прохода, а то, что чистый прогон отличим от «сверять нечего».
entry "$CLEAN"
run_case "якоря разрешаются, идентификаторы известны — чисто" 0 "ANCHORS CLEAN"

# --- check 1: якорь ----------------------------------------------------------
entry '| R-004-2 | сумма > 0 | `src/payments/service.go:ValidateName` | 260813-payments-create |'
run_case "символ исчез из файла — STALE" 1 "STALE"

entry '| R-004-2 | сумма > 0 | `src/payments/gone.go:CreatePayment` | 260813-payments-create |'
run_case "файл якоря удалён — DEAD" 1 "DEAD"

entry '| R-004-2 | сумма > 0 | `src/payments/service.go:42` | 260813-payments-create |'
run_case "якорь на номер строки — NOTE, а не находка" 0 "line number"

entry '| R-004-2 | сумма > 0 | check в схеме | 260813-payments-create |'
run_case "проза вместо якоря — NOTE: разрешать нечего" 0 "prose"

# Проза в ячейке якоря не должна становиться находкой. Оба случая — из живой базы:
# `strconv.ParseInt` разбирается как имя + «расширение», а любая фраза с двоеточием
# (`format: uuid`) раньше проходила по ветке-фолбэку. Проверка, выдумывающая себе
# отказы, — это проверка, которую перестают читать.
entry '| R-004-2 | ключ абонента | `strconv.ParseInt` в `Upsert` | 260813-pay |'
run_case "квалифицированный идентификатор в ячейке — проза, не DEAD" 0 "prose"

entry '| R-004-2 | device_id — UUID | `format: uuid` + `device_id uuid` | 260813-pay |'
run_case "фраза с двоеточием в ячейке — проза, не DEAD" 0 "prose"

# --- check 2: идентификаторы -------------------------------------------------
entry "$CLEAN" '| R-999-1 | взялось ниоткуда | `src/payments/service.go:CreatePayment` | 260813-payments-create |'
run_case "правило записи нет ни в одном реестре — UNDECLARED" 1 "UNDECLARED"

# --- check 3: файлы ----------------------------------------------------------
entry "$CLEAN"
sedi 's|^- handler.*|- legacy: `src/payments/legacy.go` — `ChargeV1`|' "$ENTRY"
run_case "запись называет удалённый файл — GONE" 1 "GONE"

entry "$CLEAN"
sedi '/handler.go/d' "$ENTRY"
run_case "исходник модуля не описан нигде — UNDESCRIBED" 1 "UNDESCRIBED"

# --- check 4: гигиена и INDEX ------------------------------------------------
entry "$CLEAN"
python3 - "$ENTRY" <<'EOF'
import sys
p = sys.argv[1]
open(p, "a").write("\n".join("- строка %d" % i for i in range(200)))
EOF
run_case "запись перевалила за 150 строк — SIZE" 1 "SIZE"

entry "$CLEAN"
sedi 's/Slice: 260813-payments-create/Slice: 991231-nope/' "$ENTRY"
run_case "запись ссылается на несуществующий слайс — INDEX" 1 "INDEX"

# --- дрейф: коммиты после Updated --------------------------------------------
# Прогноз, а не вердикт: коммит мог не задеть ничего из описанного. Поэтому NOTE
# и exit 0 — иначе любой прогон в живом репозитории «красный» по построению.
entry "$CLEAN"
sedi 's/Updated: 2099-01-01/Updated: 2000-01-01/' "$ENTRY"
( cd "$R" && git init -q . && git config user.email t@t && git config user.name t \
  && git add -A && git commit -qm "правки в src/payments" ) >/dev/null 2>&1
run_case "коммиты после Updated — NOTE, прогон остаётся зелёным" 0 "commit(s) touched"

# Общая база: один и тот же модуль у двух проектов. Строка INDEX.md выбирается
# по паре (Project, Module) — иначе прогон получил бы чужую запись и все находки
# после этого были бы про чужой код.
entry "$CLEAN"
mkdir -p "$R/.pipeline/knowledge/other/modules"
cat > "$R/.pipeline/knowledge/other/modules/payments.md" <<'EOF'
# Module: src/payments

Updated: 2099-01-01 | Slice: 260901-x

## Layers and key types
- `src/nowhere/gone.go` — `Vanished`

## Business rules in force
| ID | Statement | Where | Origin |
|---|---|---|---|
| R-999-9 | чужое правило | `src/nowhere/gone.go` | development |
EOF
out=$($KB --repo "$R" --module src/payments 2>&1); rc=$?
if [ "$rc" = 0 ] && printf '%s' "$out" | grep -q "knowledge/proj/modules/payments.md"; then
  PASS=$((PASS+1)); printf '  ok    %s
' "общая база: берётся запись своего проекта, не одноимённая чужая"
else
  FAIL=$((FAIL+1)); printf '  FAIL  %s — exit %s
%s
' "общая база: своя запись" "$rc" "$(printf '%s' "$out" | sed 's/^/          /')"
fi

out=$($KB --repo "$R" --module src/payments --project other 2>&1); rc=$?
if printf '%s' "$out" | grep -q "knowledge/other/modules/payments.md"; then
  PASS=$((PASS+1)); printf '  ok    %s
' "--project переключает на каталог другого проекта в той же базе"
else
  FAIL=$((FAIL+1)); printf '  FAIL  %s — exit %s
%s
' "--project переключает каталог" "$rc" "$(printf '%s' "$out" | sed 's/^/          /')"
fi


# --- граничные случаи --------------------------------------------------------
entry "$CLEAN"
sedi 's/Updated: 2000-01-01/Updated: 2099-01-01/' "$ENTRY"
mv "$ENTRY" "$WORK/stash.md"
sedi '/src.payments/d' "$R/.pipeline/knowledge/INDEX.md"
run_case "записи о модуле нет — первое описание, а не чистая сверка" 0 "FIRST description"
mv "$WORK/stash.md" "$ENTRY"

out=$($KB --repo "$R" --module src/nope 2>&1); rc=$?
if [ "$rc" = 2 ]; then PASS=$((PASS+1)); printf '  ok    %s\n' "модуль не каталог — exit 2, а не пустая сверка"
else FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали exit 2, получили %s\n' "модуль не каталог" "$rc"; fi

out=$($KB --repo "$R" --module src/payments --knowledge .pipeline/nope 2>&1); rc=$?
if [ "$rc" = 2 ]; then PASS=$((PASS+1)); printf '  ok    %s\n' "базы нет — exit 2: bootstrap делает шаг 20"
else FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали exit 2, получили %s\n' "базы нет" "$rc"; fi

echo
if [ "$FAIL" -eq 0 ]; then echo "$PASS случаев — PASSED"; else echo "$PASS ok, $FAIL FAILED"; fi
exit $(( FAIL > 0 ))
