#!/usr/bin/env bash
# test-token-cost.sh — таблица случаев для `trunkline token-cost`
# (internal/cli/tokencost.go)
#
# Зачем. Отчёт о стоимости — единственный артефакт пайплайна, который врёт
# правдоподобно. Гейт, посчитавший пробелы неверно, падает или не падает, и это
# видно; счёт, завышенный втрое, выглядит ровно как счёт. Поэтому здесь
# проверяются не коды выхода, а числа.
#
# Главный из этих рисков — дедупликация. Claude Code пишет один ответ API
# несколькими строками транскрипта (по строке на content-блок), и каждая несёт
# один и тот же usage. Наивная сумма завышает в 2-9 раз на обычном прогоне.
#
# Запуск: bash scripts/test-token-cost.sh

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINDIR=$(mktemp -d)
( cd "$ROOT" && go build -o "$BINDIR/trunkline" ./cmd/trunkline ) || { echo "сборка trunkline не удалась" >&2; exit 2; }
TC="$BINDIR/trunkline token-cost"

PASS=0; FAIL=0
WORK=$(mktemp -d); trap 'rm -rf "$WORK" "$BINDIR"' EXIT
mkdir -p "$WORK/tr" "$WORK/proj/.pipeline/workdir/metrics"

# call <файл> <ts> <модель> <sidechain 0|1> <msg-id> <повторов> <in> <cw5m> <cw1h> <cr> <out>
# Повторы — это одна и та же реплика, размноженная по content-блокам: ровно то,
# что делает хост. cw5m/cw1h = -1 означает старый транскрипт без разбивки TTL.
call() {
  python3 - "$@" <<'PY'
import json, sys
p, ts, model, side, mid, rep, i, w5, w1, cr, out = sys.argv[1:12]
w5, w1 = int(w5), int(w1)
usage = {
    "input_tokens": int(i), "output_tokens": int(out),
    "cache_creation_input_tokens": (w5 + w1) if w5 >= 0 else -w5 - w1,
    "cache_read_input_tokens": int(cr),
}
if w5 >= 0:
    usage["cache_creation"] = {"ephemeral_5m_input_tokens": w5, "ephemeral_1h_input_tokens": w1}
else:  # старый формат: только общая сумма, без разбивки по TTL
    usage["cache_creation_input_tokens"] = -w5
with open(p, "a") as f:
    for _ in range(int(rep)):
        f.write(json.dumps({
            "type": "assistant", "timestamp": ts, "isSidechain": side == "1",
            "cwd": "/x", "sessionId": p.split("/")[-1][:-6], "requestId": "req_" + mid,
            "message": {"id": mid, "model": model, "usage": usage},
        }) + "\n")
PY
}

# case <имя> <ожидаемый код> <регулярка по выводу|-> -- <аргументы…>
case_() {
  local name="$1" want="$2" want_re="$3"; shift 3; [ "$1" = "--" ] && shift
  local out rc ok=1
  out=$(cd "$WORK/proj" && $TC --transcripts "$WORK/tr" "$@" 2>&1); rc=$?
  [ "$rc" = "$want" ] || ok=0
  if [ "$want_re" != "-" ] && ! printf '%s\n' "$out" | grep -Eq "$want_re"; then ok=0; fi
  if [ "$ok" = 1 ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"
  else
    FAIL=$((FAIL+1))
    printf '  FAIL  %s — ждали exit %s и /%s/, получили exit %s\n' "$name" "$want" "$want_re" "$rc"
    printf '%s\n' "$out" | sed 's/^/          /'
  fi
}

echo "trunkline token-cost — таблица случаев"
echo

# --- Дедупликация: ради неё всё и написано ------------------------------------
# Один вызов, размноженный по трём блокам. 1 000 000 output у Opus 5 = $25.00.
# Без дедупа было бы 3 000 000 и $75.00 — и ничто в отчёте не выглядело бы не так.
call "$WORK/tr/dedup.jsonl" 2026-08-01T10:00:00Z claude-opus-5 0 msg1 3 0 0 0 0 1000000
case_ "три строки одного message.id — один вызов" 0 '1 000 000.*25\.00.*\$25\.00' \
  -- --session dedup
case_ "и один вызов в шапке, не три" 0 '1 API call\b' -- --session dedup

# Служебная запись хоста о 429: нулевой usage, модель <synthetic>. Это не вызов.
call "$WORK/tr/dedup.jsonl" 2026-08-01T10:05:00Z '<synthetic>' 0 msgerr 1 999 999 999 999 999
case_ "<synthetic> не вызов и не деньги" 0 '1 API call\b' -- --session dedup

# --- Типы токенов и их множители ----------------------------------------------
# 400k×1.25×$5 = $2.50, 600k×2×$5 = $6.00, 1M cache read ×0.1×$5 = $0.50, итого $9.00.
call "$WORK/tr/kinds.jsonl" 2026-08-01T10:00:00Z claude-opus-5 0 k1 2 0 400000 600000 1000000 0
case_ "5m и 1h тарифицируются раздельно" 0 'cache write 5m.*\$2\.50' -- --session kinds
case_ "1h — это 2x, а не 1.25x"        0 'cache write 1h.*\$6\.00' -- --session kinds
case_ "чтение кэша — 0.1x"             0 'cache read.*\$0\.50'     -- --session kinds
case_ "итог по типам сходится"         0 'TOTAL.*\$9\.00'          -- --session kinds

# Старый транскрипт без разбивки TTL: неизвестное относим к дешёвому множителю,
# чтобы отчёт не завышал. 1M×1.25×$5 = $6.25.
call "$WORK/tr/legacy.jsonl" 2026-08-01T10:00:00Z claude-opus-5 0 l1 1 0 -1000000 0 0 0
case_ "транскрипт без TTL-разбивки — по нижнему множителю" 0 'cache write 5m.*1 000 000.*\$6\.25' \
  -- --session legacy

# --- Цены ---------------------------------------------------------------------
# Датированный id должен найти семейство: 1M output у Haiku 4.5 = $5.00.
call "$WORK/tr/haiku.jsonl" 2026-08-01T10:00:00Z claude-haiku-4-5-20251001 0 h1 1 0 0 0 0 1000000
case_ "датированный id матчится по префиксу семейства" 0 'TOTAL.*\$5\.00' -- --session haiku

# Модель, которой в таблице нет. Ноль в такой строке читается как «бесплатно».
call "$WORK/tr/unknown.jsonl" 2026-08-01T10:00:00Z model-from-the-future 0 u1 1 0 0 0 0 1000000
case_ "неизвестная модель — токены есть, цены нет" 0 'no price for model-from-the-future' \
  -- --session unknown
case_ "и в итог она не приходит нулём"  0 'TOTAL.*—' -- --session unknown

cat > "$WORK/prices.json" <<'EOF'
{"as_of":"2026-08-31","models":{"claude-opus-5":{"in":2.0,"out":10.0}},
 "multipliers":{"cache_write_5m":1.25,"cache_write_1h":2.0,"cache_read":0.1}}
EOF
case_ "--prices замещает таблицу целиком" 0 'TOTAL.*\$10\.00' \
  -- --session dedup --prices "$WORK/prices.json"
case_ "и дата таблицы едет вместе с ней" 0 'Prices as of 2026-08-31' \
  -- --session dedup --prices "$WORK/prices.json"

# --- Устаревание таблицы ------------------------------------------------------
# Неизвестная модель видна сама по себе: она приходит без цены. Изменившаяся
# цена на известной модели не видна ничем, кроме этого предупреждения, — поэтому
# оно и есть. Даты считаются от сегодня, иначе тест протухнет вместе с таблицей.
mkprices() { printf '{"as_of":"%s","models":{"claude-opus-5":{"in":5.0,"out":25.0}}}\n' "$1" > "$2"; }
# reldate N — N months ago as YYYY-MM-DD, on both GNU date (Linux) and BSD
# date (macOS), whose -v takes the offset instead of -d.
reldate() {
  if date -d "-$1 months" +%Y-%m-%d >/dev/null 2>&1; then
    date -d "-$1 months" +%Y-%m-%d
  else
    date -v-"$1"m +%Y-%m-%d
  fi
}
mkprices "$(reldate 4)" "$WORK/old.json"
mkprices "$(reldate 1)" "$WORK/fresh.json"
printf '{"as_of":"позавчера","models":{"claude-opus-5":{"in":5.0,"out":25.0}}}\n' > "$WORK/nodate.json"

case_ "таблице больше трёх месяцев — отчёт об этом говорит" 0 'These prices are 4 months old' \
  -- --session dedup --prices "$WORK/old.json"
case_ "и это предупреждение, а не отказ считать" 0 'TOTAL.*\$25\.00' \
  -- --session dedup --prices "$WORK/old.json"
if (cd "$WORK/proj" && $TC --transcripts "$WORK/tr" --session dedup \
      --prices "$WORK/fresh.json" 2>&1 | grep -q 'These prices are'); then
  FAIL=$((FAIL+1)); printf '  FAIL  свежая таблица молчит — предупреждение всё же напечатано\n'
else
  PASS=$((PASS+1)); printf '  ok    свежая таблица не печатает предупреждения\n'
fi
case_ "as_of не дата — не падаем и не пугаем" 0 'Prices as of позавчера' \
  -- --session dedup --prices "$WORK/nodate.json"
case_ "json несёт возраст таблицы машинно" 0 '"prices_stale": true' \
  -- --session dedup --prices "$WORK/old.json" --format json

# Таблица по умолчанию — файл рядом со скриптом, а не константа в коде.
case_ "битая таблица цен — отказ с именем файла, а не тихий ноль" 2 'cannot read --prices' \
  -- --session dedup --prices "$WORK/nosuch.json"
printf '{"as_of":"2026-01-01","models":{}}\n' > "$WORK/empty.json"
case_ "пустой models — тоже отказ" 2 'cannot read --prices' \
  -- --session dedup --prices "$WORK/empty.json"

# --- Привязка к слайсу --------------------------------------------------------
cat > "$WORK/proj/.pipeline/workdir/metrics/pipeline.jsonl" <<'EOF'
{"ts":"2026-08-05T10:00:00+00:00","slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":[1]}
{"ts":"2026-08-05T12:00:00+00:00","slice":"007","step":20,"event":"done","confirmations":4,"rollbacks":0,"duration_min":120}
{"ts":"2026-08-06T10:00:00+00:00","slice":"008","step":0,"event":"start","session_id":"bound","task_type":"fix","steps_planned":[1]}
{"ts":"2026-08-06T12:00:00+00:00","slice":"008","step":20,"event":"done","session_id":"bound","confirmations":2,"rollbacks":0,"duration_min":120}
{"ts":"2026-08-07T10:00:00+00:00","slice":"009","step":0,"event":"start","task_type":"fix","steps_planned":[1]}
EOF

call "$WORK/tr/win.jsonl" 2026-08-05T11:00:00Z claude-opus-5 0 w1 1 0 0 0 0 1000000  # внутри 007
call "$WORK/tr/win.jsonl" 2026-08-05T20:00:00Z claude-opus-5 0 w2 1 0 0 0 0 4000000  # после done
case_ "окно слайса отсекает работу после done" 0 'TOTAL.*\$25\.00' -- --slice 007
case_ "и честно говорит, что привязка по часам" 0 'wall-clock' -- --slice 007

# Тот же интервал, но в журнале есть session_id: считается только своя сессия.
call "$WORK/tr/bound.jsonl" 2026-08-06T11:00:00Z claude-opus-5 0 b1 1 0 0 0 0 1000000
call "$WORK/tr/other.jsonl" 2026-08-06T11:00:00Z claude-opus-5 0 o1 1 0 0 0 0 8000000
case_ "session_id из журнала отсекает соседнюю сессию" 0 'TOTAL.*\$25\.00' -- --slice 008
case_ "и отчёт называет привязку точной" 0 'exact' -- --slice 008

# Незакрытый прогон: окно открыто вправо, и это сказано, а не скрыто.
call "$WORK/tr/win.jsonl" 2026-08-07T11:00:00Z claude-opus-5 0 w3 1 0 0 0 0 1000000
case_ "слайс без done — окно открыто, и это названо" 0 'unfinished' -- --slice 009

# --- Субагенты ----------------------------------------------------------------
call "$WORK/tr/sub.jsonl" 2026-08-01T10:00:00Z claude-opus-5 0 s1 1 0 0 0 0 1000000
call "$WORK/tr/sub.jsonl" 2026-08-01T10:01:00Z claude-opus-5 1 s2 1 0 0 0 0 3000000
case_ "субагенты входят в итог"        0 'TOTAL.*\$100\.00'       -- --session sub
case_ "и выделены отдельной строкой"   0 'of which subagents.*75%' -- --session sub

# --- Отказы -------------------------------------------------------------------
case_ "нет такого слайса в журнале" 2 'has no events' -- --slice 999
case_ "пустая выборка — не отчёт на ноль" 2 'no API calls in scope' \
  -- --since 2030-01-01T00:00:00Z
case_ "нет каталога транскриптов" 2 'not measurable here' -- --all --transcripts "$WORK/nope"
case_ "--since не по ISO-8601" 2 'ISO-8601' -- --since вчера
case_ "селектор обязателен" 2 'exactly one of --slice' --

echo
if [ "$FAIL" -eq 0 ]; then echo "$PASS случаев — PASSED"; else echo "$PASS ok, $FAIL FAILED"; fi

# Свежесть самой поставляемой таблицы — не случай таблицы: тест, падающий по
# календарю, красит в красный код, который не менялся. Но и молчать об этом
# некому, поэтому — заметка после итога.
SHIPPED="$(cd "$(dirname "$0")/.." && pwd)/core/developer/scripts/prices.json"
python3 - "$SHIPPED" <<'PY'
import json, sys
from datetime import datetime, timezone
try:
    as_of = json.load(open(sys.argv[1]))["as_of"]
    age = (datetime.now(timezone.utc) - datetime.strptime(as_of, "%Y-%m-%d").replace(tzinfo=timezone.utc)).days
except Exception:
    sys.exit(0)
if age > 92:
    print(f"\nЗАМЕТКА: prices.json взят {as_of}, {round(age/30.44)} мес. назад — "
          f"инструмент это скажет в каждом отчёте. Сверьте anthropic.com/pricing.")
PY

exit $(( FAIL > 0 ))
