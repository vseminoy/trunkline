#!/usr/bin/env bash
# test-approval-gate.sh — таблица случаев для core/developer/scripts/approval-gate.sh
#
# Зачем. Гейт одобрений — одна из двух машинных проверок пайплайна, то есть ответ
# проекта на «enforcement is entirely social». Внутри — машина состояний над
# журналом событий: `start` открывает прогон, `approval` и `skipped` уменьшают долг,
# `done` закрывает. Такое проверяется таблицей, а не чтением: дефект, из-за которого
# `feature` без изменений в БД не мог закоммититься, прожил три ревью ровно потому,
# что скрипт ни разу не запускали против журнала.
#
# Каждый случай — свой временный git-репозиторий: журнал, staged-файлы, ожидаемый
# код возврата. Запуск: bash scripts/test-approval-gate.sh

set -uo pipefail

GATE="$(cd "$(dirname "$0")/.." && pwd)/core/developer/scripts/approval-gate.sh"
[ -f "$GATE" ] || { echo "не найден $GATE" >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { echo "нужен jq" >&2; exit 2; }

PASS=0; FAIL=0
FEATURE='[1,2,3,4,5,6,7,"7a",8,9,10,11,12,15,17,18,20]'

# run_case <имя> <ожидаемый код> <журнал> <staged-файлы через пробел> [metrics-флаг]
run_case() {
  local name="$1" want="$2" journal="$3" staged="$4" metrics="${5:-.pipeline/workdir/metrics/pipeline.jsonl}"
  local dir; dir=$(mktemp -d)
  (
    cd "$dir" || exit 2
    git init -q . && git config user.email t@t && git config user.name t
    printf '## Pipeline flags\n- profile: p\n- metrics: %s\n' "$metrics" > AGENTS.md
    mkdir -p .pipeline/workdir/metrics
    [ -n "$journal" ] && printf '%s\n' "$journal" > .pipeline/workdir/metrics/pipeline.jsonl
    for f in $staged; do mkdir -p "$(dirname "$f")"; echo x > "$f"; done
    git add -A >/dev/null 2>&1
    bash "$GATE" >/dev/null 2>&1
    echo $? > .rc
  )
  local got; got=$(cat "$dir/.rc" 2>/dev/null || echo "?")
  if [ "$got" = "$want" ]; then
    PASS=$((PASS+1)); printf '  ok    %s\n' "$name"
  else
    FAIL=$((FAIL+1)); printf '  FAIL  %s — ожидали exit %s, получили %s\n' "$name" "$want" "$got"
    printf '        вывод гейта:\n'; (cd "$dir" && bash "$GATE" 2>&1 | sed 's/^/          /' | head -6)
  fi
  rm -rf "$dir"
}

echo "approval-gate.sh — таблица случаев"

# --- Регрессия, ради которой патч и делался -----------------------------------
# feature без изменений в БД: шаг 6 неприменим и объявлен `skipped`, три остальных
# одобрения получены. До патча тут был exit 2 без честного выхода.
run_case "feature: шаг 6 skipped, 5/7/7a одобрены → пропустить" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":"5","event":"approval"}
{"slice":"007","step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","step":"7","event":"approval"}
{"slice":"007","step":"7a","event":"approval"}' \
"internal/plan/service.go"

# --- Патч не должен ослабить гейт ----------------------------------------------
run_case "feature: шаг 6 skipped, но одобрения 7 нет → блокировать" 2 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":"5","event":"approval"}
{"slice":"007","step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","step":"7a","event":"approval"}' \
"internal/plan/service.go"

run_case "feature: шаг 6 в плане, не skipped, не одобрен → блокировать" 2 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":"5","event":"approval"}
{"slice":"007","step":"7","event":"approval"}
{"slice":"007","step":"7a","event":"approval"}' \
"internal/plan/service.go"

run_case "skipped на не-гейтовом шаге (16) не влияет на долг" 2 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":16,"event":"skipped","reason":"not a hot path"}' \
"internal/plan/service.go"

# --- Маршруты с другим набором гейтов ------------------------------------------
run_case "bug: единственный гейт (7) одобрен → пропустить" 0 \
'{"slice":"008","step":0,"event":"start","task_type":"bug","steps_planned":[1,7,8,9,10,18,20]}
{"slice":"008","step":"7","event":"approval"}' \
"internal/plan/service.go"

run_case "bug: одобрения 7 нет → блокировать" 2 \
'{"slice":"008","step":0,"event":"start","task_type":"bug","steps_planned":[1,7,8,9,10,18,20]}' \
"internal/plan/service.go"

run_case "refactor: маршрут без одобрений → пропустить" 0 \
'{"slice":"009","step":0,"event":"start","task_type":"refactor","steps_planned":[1,3,4,8,9,10,11,12,18,20]}' \
"internal/plan/service.go"

# --- Границы, объявленные в ENFORCEMENT.md -------------------------------------
run_case "прогон закрыт (done) → ничего не должен" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":20,"event":"done","confirmations":14,"rollbacks":0,"duration_min":210}' \
"internal/plan/service.go"

# Маршрут audit ничего не пишет, поэтому 7a у него — отчёт, а не гейт: его нет в
# steps_planned, и шаг 20 закрывает прогон. Нарушь любое из двух — и следующий
# обычный коммит в проекте окажется заблокирован одобрением, которого никто не
# может дать. Случай сторожит именно эту форму.
run_case "audit: 7a не гейт, прогон закрыт → обычный коммит проходит" 0 \
'{"slice":"012","step":0,"event":"start","task_type":"audit","steps_planned":[1,9,10,20]}
{"slice":"012","step":"7a","event":"consistency","gaps":0,"mismatches":0,"trace_check":"pass"}
{"slice":"012","step":9,"event":"review","critical":0,"important":2,"nit":4}
{"slice":"012","step":20,"event":"done","confirmations":3,"rollbacks":0,"duration_min":25}' \
"internal/plan/service.go"

# --- Цикл по changeset'ам: одобрения принадлежат changeset'у, а не задаче -------
# Регрессия: пока долг считался на прогон, первый changeset платил за всех — его
# четыре строки открывали коммиты всем следующим. На декомпозированной задаче это
# большинство коммитов.
run_case "changeset 2 без собственных одобрений → блокировать" 2 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","changeset":1,"step":"5","event":"approval"}
{"slice":"007","changeset":1,"step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","changeset":1,"step":"7","event":"approval"}
{"slice":"007","changeset":1,"step":"7a","event":"approval"}
{"slice":"007","changeset":2,"step":"5","event":"approval"}' \
"internal/plan/service.go"

run_case "changeset 2 со своими одобрениями → пропустить" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","changeset":1,"step":"5","event":"approval"}
{"slice":"007","changeset":1,"step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","changeset":1,"step":"7","event":"approval"}
{"slice":"007","changeset":1,"step":"7a","event":"approval"}
{"slice":"007","changeset":2,"step":"5","event":"approval"}
{"slice":"007","changeset":2,"step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","changeset":2,"step":"7","event":"approval"}
{"slice":"007","changeset":2,"step":"7a","event":"approval"}' \
"internal/plan/service.go"

# `skipped` тоже принадлежит changeset'у: первый не трогал БД, второй трогает —
# и обязан получить одобрение шага 6, а не унаследовать чужой пропуск.
run_case "пропуск шага 6 в changeset 1 не закрывает шаг 6 в changeset 2" 2 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","changeset":1,"step":"5","event":"approval"}
{"slice":"007","changeset":1,"step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","changeset":1,"step":"7","event":"approval"}
{"slice":"007","changeset":1,"step":"7a","event":"approval"}
{"slice":"007","changeset":2,"step":"5","event":"approval"}
{"slice":"007","changeset":2,"step":"7","event":"approval"}
{"slice":"007","changeset":2,"step":"7a","event":"approval"}' \
"internal/plan/service.go"

# Маршруты без шага 4 и журналы, написанные до появления поля: отсутствие
# `changeset` читается как единственный changeset, поведение прежнее.
run_case "журнал без поля changeset → один changeset, как раньше" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}
{"slice":"007","step":"5","event":"approval"}
{"slice":"007","step":6,"event":"skipped","reason":"no database changes"}
{"slice":"007","step":"7","event":"approval"}
{"slice":"007","step":"7a","event":"approval"}' \
"internal/plan/service.go"

run_case "нет строки start → это не прогон пайплайна" 0 \
'{"slice":"007","step":9,"event":"review","critical":0}' \
"internal/plan/service.go"

run_case "в индексе только артефакты → блокировать нечего" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}' \
"internal/plan/rules.md api/openapi.yaml migrations/001.sql .pipeline/workdir/metrics/x.json"

run_case "журнала нет → гейту нечего читать" 0 "" "internal/plan/service.go"

run_case "metrics: none → гейт объявляет, что не действует" 0 \
'{"slice":"007","step":0,"event":"start","task_type":"feature","steps_planned":'"$FEATURE"'}' \
"internal/plan/service.go" "none"

echo
if [ "$FAIL" -eq 0 ]; then
  echo "$PASS случаев — PASSED"
else
  echo "$PASS ok, $FAIL FAILED"
fi
exit $(( FAIL > 0 ))
