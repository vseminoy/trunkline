#!/usr/bin/env bash
# test-review-check.sh — таблица случаев для `trunkline review-check`
# (internal/cli/reviewcheck.go)
#
# Зачем. Скрипт восстанавливает C1 (субагентов) там, где хост их не умеет: шаги
# 9+10 идут двумя вызовами API, каждый с чистого контекста. У него есть режим
# отказа, которого нет ни у одного другого скрипта пайплайна, — HTTP 200 с
# мусором. Ответ «Конечно, сейчас проверю этот код!» или отчёт, обрезанный по
# лимиту токенов, выглядят как работа и работой не являются. Это структурный
# близнец молчаливой матрицы 7a, и если его не ловить, болезнь, вылеченная
# гейтами снаружи, воспроизводится внутри верификатора. Половина таблицы — про это.
#
# Вторая половина — про то, чего скрипт делать не должен: выносить вердикт,
# возвращать код 1, печатать ключ, выдавать частичное ревью за целое.
#
# Сеть не нужна: все вызовы идут в мок на 127.0.0.1, который поднимает сам тест.
#
# Запуск: bash scripts/test-review-check.sh

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RCSRC="$ROOT/internal/cli/reviewcheck.go"
[ -f "$RCSRC" ] || { echo "не найден $RCSRC" >&2; exit 2; }
[ -f "$ROOT/agent-skills/agents/code-reviewer.md" ] || {
  echo "пуст сабмодуль agent-skills — нужен git submodule update --init" >&2; exit 2; }
BINDIR=$(mktemp -d)
( cd "$ROOT" && go build -o "$BINDIR/trunkline" ./cmd/trunkline ) || { echo "сборка trunkline не удалась" >&2; exit 2; }
RC="$BINDIR/trunkline review-check"

PASS=0; FAIL=0
WORK=$(mktemp -d); trap 'rm -rf "$WORK" "$BINDIR"; kill $(jobs -p) 2>/dev/null' EXIT

KEY="secret-must-never-be-printed"
printf 'diff --git a/x.go b/x.go\n+func New() {}\n' > "$WORK/c.diff"
printf '| ID | Rule | Origin | Changeset |\n| R-007-1 | amount > 0 | task item 1 | 1 |\n' \
  > "$WORK/rules.md"
printf '# Cross-check\n\nR-007-1: contract OK, tests OK\n' > "$WORK/consistency.md"
printf '# Go guideline\n\nОшибки не глотать.\n' > "$WORK/guideline.md"

# --- Мок OpenAI-совместимого эндпоинта ----------------------------------------
# Режим задаётся переменной MODE и меняет ТОЛЬКО второй вызов: так каждый случай
# проверяет заодно, что валидный первый отчёт не выбрасывается вместе с отказом.
cat > "$WORK/mock.py" <<'PYEOF'
import json, os, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
MODE = os.environ.get("MODE", "ok")
N = [0]; RENAMED = [False]
LONG = "## Summary\n\n" + ("Finding about the changeset. " * 40)

class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        N[0] += 1
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        print(json.dumps({"n": N[0], "path": self.path,
                          "auth": self.headers.get("Authorization"),
                          "fields": sorted(k for k in body if k != "messages"),
                          "system": len(body["messages"][0]["content"]),
                          "user": body["messages"][1]["content"]}), file=sys.stderr)
        sys.stderr.flush()
        if MODE == "renamed" and not RENAMED[0] and "max_tokens" in body:
            RENAMED[0] = True
            return self.send(400, {"error": "use max_completion_tokens instead"})
        if MODE == "429" and N[0] >= 2:
            return self.send(429, {"error": "rate limited"})
        if MODE == "noreason" and N[0] >= 2:
            return self.send(200, {"choices": [{"message": {"content": "",
                "reasoning_content": "думаю" * 500}, "finish_reason": "length"}]})
        if MODE == "garbagejson" and N[0] >= 2:
            raw = b"<html>gateway</html>"
            self.send_response(200); self.send_header("Content-Length", str(len(raw)))
            self.end_headers(); return self.wfile.write(raw)
        finish, text = "stop", LONG
        if MODE == "length" and N[0] >= 2: finish = "length"
        if MODE == "short" and N[0] >= 2: text = "Sure, let me review this code!"
        self.send(200, {"choices": [{"message": {"content": text},
                                     "finish_reason": finish}],
                        "usage": {"prompt_tokens": 19000, "completion_tokens": 800,
                                  "total_tokens": 19800}})
    def send(self, code, payload):
        raw = json.dumps(payload).encode()
        self.send_response(code); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw))); self.end_headers()
        self.wfile.write(raw)

HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
PYEOF

PORT=$(( 18000 + RANDOM % 900 ))
SRV=""
start_mock() {
  stop_mock
  PORT=$(( PORT + 1 ))
  : > "$WORK/srv.log"
  MODE="$1" python3 "$WORK/mock.py" "$PORT" 2> "$WORK/srv.log" &
  SRV=$!
  for _ in $(seq 1 50); do
    python3 - "$PORT" <<'EOF' && return 0
import socket, sys
s = socket.socket(); s.settimeout(0.2)
sys.exit(0 if s.connect_ex(("127.0.0.1", int(sys.argv[1]))) == 0 else 1)
EOF
    sleep 0.1
  done
  echo "мок не поднялся на $PORT" >&2; exit 2
}
stop_mock() { [ -n "$SRV" ] && kill "$SRV" 2>/dev/null; SRV=""; }

# run_case <имя> <ожидаемый код> <аргументы…> — вывод остаётся в $WORK/out
OUT="$WORK/out"
run_case() {
  local name="$1" want="$2"; shift 2
  local rc
  TRUNKLINE_REVIEW_API_KEY="$KEY" $RC "$@" > "$OUT" 2>&1; rc=$?
  if [ "$rc" = "$want" ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"; return 0
  fi
  FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали exit %s, получили %s\n' "$name" "$want" "$rc"
  sed 's/^/          /' "$OUT" | tail -5
  return 1
}

# assert_out <да|нет> <regexp> <за что> — про последний прогон
assert_out() {
  local want="$1" re="$2" name="$3"
  if grep -qE "$re" "$OUT"; then found=да; else found=нет; fi
  if [ "$found" = "$want" ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"
  else
    FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали «%s» для /%s/\n' "$name" "$want" "$re"
  fi
}

API=(--base-url "PLACEHOLDER" --model mock-1)
full() {
  echo --diff "$WORK/c.diff" --rules "$WORK/rules.md" \
       --consistency "$WORK/consistency.md" --guideline "backend=$WORK/guideline.md" \
       --changeset 1 --retries 0
}

echo "trunkline review-check — таблица случаев"

# --- Запуск невозможен: это ошибка вызова, а не пустое ревью -------------------
run_case "пустой дифф — ревью ни о чём читается как ревью о чём-то" 2 \
  --diff /dev/null --base-url "http://127.0.0.1:1/v1" --model m

run_case "нет персон по --skills-root — пересказывать их нечем" 2 \
  $(full) --skills-root "$WORK/nope" --base-url "http://127.0.0.1:1/v1" --model m

run_case "--guideline без ROLE=PATH" 2 $(full) --guideline nope \
  --base-url "http://127.0.0.1:1/v1" --model m

run_case "нет --base-url: куда слать код — решает оператор, не дефолт" 2 \
  $(full) --model m

run_case "нет --model" 2 $(full) --base-url "http://127.0.0.1:1/v1"

rc=0
env -u TRUNKLINE_REVIEW_API_KEY $RC $(full) \
  --base-url "http://127.0.0.1:1/v1" --model m > "$OUT" 2>&1 || rc=$?
if [ "$rc" = 2 ]; then PASS=$((PASS+1)); printf '  ok    %s\n' "без ключа в окружении — exit 2"
else FAIL=$((FAIL+1)); printf '  FAIL  без ключа — ожидали 2, получили %s\n' "$rc"; fi

# --- Сухой прогон: проверить сборку промпта в закрытом контуре, без ключа ------
run_case "--dry-run собирает промпты и никуда не ходит" 0 $(full) --dry-run \
  --base-url "http://127.0.0.1:1/v1" --model m
assert_out да "Critic agent \(step 9\).*chars" "сухой прогон печатает размер обеих персон"
assert_out да "nothing reviewed" "сухой прогон не выдаёт себя за ревью"

# --- Счастливый путь ----------------------------------------------------------
start_mock ok
run_case "оба отчёта получены → 0" 0 $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
assert_out да "COMPLETE" "итоговая строка"
assert_out да "No verdict" "скрипт вслух отказывается от вердикта"
assert_out нет "Critical: [0-9]" "severity не считается"
assert_out нет "$KEY" "ключ не попадает в отчёт, который уедет в .pipeline/workdir/review/"
assert_out да "finish_reason .stop." "провенанс: модель, эндпоинт, расход, finish_reason"

# Промпт — это сами файлы персон, а не пересказ: 37 КБ и 40 КБ дословно.
sys1=$(python3 -c "import json,sys;print(json.loads(open(sys.argv[1]).readline())['system'])" "$WORK/srv.log")
if [ "$sys1" -gt 30000 ]; then
  PASS=$((PASS+1)); printf '  ok    %s\n' "персона уходит дословно ($sys1 симв.), а не пересказом"
else
  FAIL=$((FAIL+1)); printf '  FAIL  промпт критика всего %s символов — похоже на пересказ\n' "$sys1"
fi
# Реестр правил и сверка 7a попадают в задачу содержимым, а не путём: у HTTP-вызова
# нет файловой системы проекта, в отличие от субагента.
if grep -q "R-007-1" "$WORK/srv.log" && grep -q "Cross-check" "$WORK/srv.log"; then
  PASS=$((PASS+1)); printf '  ok    %s\n' "rules.md и consistency.md уходят содержимым, а не путём"
else
  FAIL=$((FAIL+1)); printf '  FAIL  реестр или сверка 7a не дошли до модели\n'
fi

start_mock ok
run_case "--only critic — один вызов" 0 $(full) --only critic \
  --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
calls=$(grep -c '"n":' "$WORK/srv.log")
if [ "$calls" = 1 ]; then PASS=$((PASS+1)); printf '  ok    %s\n' "--only делает ровно один вызов"
else FAIL=$((FAIL+1)); printf '  FAIL  --only сделал %s вызовов\n' "$calls"; fi

# --- HTTP 200 с мусором: режим отказа, которого нет у других скриптов ----------
start_mock length
run_case "обрезка по лимиту токенов (finish_reason: length) → 2" 2 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
assert_out да "NOT A REPORT" "обрезанный отчёт помечен, а не принят"
assert_out да "INCOMPLETE" "итог — незавершённый шаг"
assert_out да "Finding about the changeset" "полученный текст всё равно напечатан"
assert_out да "Critic agent" "валидный отчёт критика не выброшен вместе с отказом"
assert_out да "review_verify: unavailable" "агенту сказано, что писать в журнал"

start_mock short
run_case "«Сейчас проверю!» короче порога → 2" 2 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
assert_out да "below the 400" "названа причина, а не просто отказ"

start_mock noreason
run_case "весь бюджет ушёл в reasoning, ответа нет → 2" 2 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
assert_out да "reasoning_content" "названа настоящая причина пустого ответа"

start_mock garbagejson
run_case "HTTP 200 не-JSON (страница шлюза) → 2" 2 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1

# --- Сетевые отказы -----------------------------------------------------------
start_mock 429
run_case "rate limit на второй персоне → частичный отказ, 2" 2 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
assert_out да "Missing: Security agent" "названа недостающая персона"

run_case "эндпоинт недоступен → 2, а не тишина" 2 \
  $(full) --base-url "http://127.0.0.1:1/v1" --model mock-1

# --- Диалект шлюза ------------------------------------------------------------
start_mock renamed
run_case "шлюз требует max_completion_tokens — переименовать, а не падать" 0 \
  $(full) --base-url "http://127.0.0.1:$PORT/v1" --model mock-1
n_old=$(grep -c 'max_tokens"' "$WORK/srv.log")
if [ "$n_old" = 1 ]; then
  PASS=$((PASS+1)); printf '  ok    %s\n' "переименование запомнено: вторая персона не платит за то же 400"
else
  FAIL=$((FAIL+1)); printf '  FAIL  max_tokens ушёл %s раз — переименование не запомнилось\n' "$n_old"
fi
stop_mock

# --- Кода 1 нет намеренно ------------------------------------------------------
# У trace-check.py 1 значит «проверка отработала и провалилась». Здесь такого
# исхода нет: скрипт не читает то, что напечатал, и вердикта не выносит. Дыра на
# месте единицы — сообщение, и она должна быть названа в docstring.
if grep -q "not used, on purpose" "$RCSRC"; then
  PASS=$((PASS+1)); printf '  ok    %s\n' "отсутствие кода 1 объявлено в docstring"
else
  FAIL=$((FAIL+1)); printf '  FAIL  docstring не объясняет, почему кода 1 нет\n'
fi
if grep -nE "exitCode\(1\)|return 1$" "$RCSRC" > /dev/null; then
  FAIL=$((FAIL+1)); printf '  FAIL  в скрипте появился выход с кодом 1\n'
else
  PASS=$((PASS+1)); printf '  ok    %s\n' "кода 1 нет и в коде"
fi

echo
if [ "$FAIL" -eq 0 ]; then echo "$PASS случаев — PASSED"; else echo "$PASS ok, $FAIL FAILED"; fi
exit $(( FAIL > 0 ))
