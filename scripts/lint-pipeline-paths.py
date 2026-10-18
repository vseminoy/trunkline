#!/usr/bin/env python3
"""
lint-pipeline-paths.py — гейт «пути ядра резолвятся от корня пайплайна».

Зачем: ядро называет свои файлы путями вида `agent-skills/skills/.../SKILL.md`.
Раскладка репозитория и раскладка установленного проекта самоподобны — `core/`,
`profiles/` и `agent-skills/` лежат сиблингами и там, и там:

    репозиторий:      core/  profiles/  publishers/  agent-skills/
    проект:           .trunkline/core/  .trunkline/profiles/  …

Поэтому путь, который резолвится от корня репозитория, резолвится и от
`.trunkline/`. Обратное тоже верно: путь, ведущий за пределы корня пайплайна,
не резолвится НИ в одной установке — и отказывает молча, потому что агент просто
не найдёт файл и поедет дальше без скилла, без персоны, без чеклиста.

Этот линтер и есть основание для строки «payload is self-contained» в
`core/ORCHESTRATION.md`. Без него та строка — заявление, а не проверка.

Что проверяется:

  1. Каждая ссылка ядра резолвится от корня пайплайна.
  2. Каждая ссылка ведёт в то, что реально вендорится в проект (см. assets.go):
     `core/` без `core/l10n/`, `profiles/`, и подмножество `agent-skills/`.
     Ссылка на файл вне payload — дыра: в репозитории она есть, в проекте её нет.
  3. Ядро не называет корень пайплайна «репозиторием промптов»: у установленного
     проекта такого корня нет.
  4. Голое имя файла (`FOO.md` без каталога) либо разрешается как сосед по
     каталогу, либо входит в список артефактов прогона. Иначе это мёртвая
     ссылка: проверки 1–3 её не видят, потому что ищут префикс каталога, — так
     `PROMT-ANALYTIC.md` (старое имя `core/analytic/PIPELINE.md`) прожил в трёх
     местах, включая правило разрешения конфликтов, при зелёном гейте.
  5. Ядро не ссылается на номер строки в файле сабмодуля (`...md:170`).
     `agent-skills/` обновляется командой `git submodule update --remote`, и
     любой такой якорь после первого же апстримного коммита указывает не туда —
     молча, потому что строка по номеру всегда существует. Ссылаться на
     заголовок раздела.

Запуск: python3 scripts/lint-pipeline-paths.py
Коды выхода: 0 — чисто, 1 — есть ошибки.
"""

import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORE = os.path.join(ROOT, "core")

# Верхнеуровневые деревья, на которые ядру можно ссылаться: это ровно то, что
# кладётся рядом с ядром в обеих раскладках.
ANCHORED = ("core/", "profiles/", "publishers/", "agent-skills/")

# --- Что попадает в проект (держать в согласии с assets.go) -------------------
#
# `core/l10n/` — материал сборки, а не payload: локализация накладывается на
# английское дерево при установке, и в проекте каталога `l10n/` нет. Ссылаться на
# него как на путь, доступный в прогоне, нельзя.

NOT_VENDORED = ("core/l10n/",)

# Из сабмодуля вендорится подмножество — см. директивы go:embed в assets.go.
SKILLS_VENDORED = ("agent-skills/agents/", "agent-skills/hooks/",
                   "agent-skills/references/", "agent-skills/skills/",
                   "agent-skills/LICENSE")

# --- Формулировки, которые описывают несуществующий якорь ---------------------

# Сопоставляются по всему тексту файла, а не построчно: формулировка легко
# переносится на другую строку («корня репозитория\nпромптов»), и построчный
# поиск такую пропускает — что однажды уже случилось.

ANCHOR_FIX = "корень пайплайна (каталог с core/, profiles/, agent-skills/)"
BAD_ANCHOR = {
    r"prompt\s+repository": ANCHOR_FIX,
    r"репозитори\w*\s+(?:с\s+)?промпт\w*": ANCHOR_FIX,
}

# Ссылка в обратных кавычках. Плейсхолдеры (`<lang>`, `NN-*`, `...`) — не пути.
REF_RE = re.compile(r"`((?:core|profiles|agent-skills)/[^`]+)`")
PLACEHOLDER = ("<", "*", "...")

# --- Голые имена файлов -------------------------------------------------------
#
# Ссылка вида `FOO.md` без каталога проверками 1–3 не ловится: они ищут префикс.
# Такое имя допустимо ровно в двух случаях, всё остальное — мёртвая ссылка.

BARE_RE = re.compile(r"`([A-Za-z_][A-Za-z0-9_.-]*\.md)`")

# Артефакты, которые пайплайн создаёт В РАЗРАБАТЫВАЕМОМ ПРОЕКТЕ. У них нет и не
# может быть пути от корня пайплайна: они рождаются рядом с модулем, в каталоге
# задач или в корне проекта, и называются по имени именно поэтому.
RUNTIME_ARTIFACTS = {
    "AGENTS.md",             # файл проекта: блок `## Pipeline flags`
    "rules.md",              # шаг 2, реестр правил рядом с модулем
    "tests_readme.md",       # шаг 7, рядом с модулем
    "consistency.md",        # шаг 7a, рядом с модулем
    "review.md",             # шаг 9+10, корень проекта
    "sequence_diagrams.md",  # шаг 5, рядом с модулем
    "_analysis-state.md",    # состояние анализа, в каталоге задач
    "INDEX.md",              # внутри .pipeline/knowledge/
}

errors, warnings = [], []


def err(where, msg):
    errors.append(f"{where}: {msg}")


def warn(where, msg):
    warnings.append(f"{where}: {msg}")


def core_files():
    for base, _, names in os.walk(CORE):
        for n in sorted(names):
            if n.endswith(".md"):
                p = os.path.join(base, n)
                yield os.path.relpath(p, ROOT), p


def normalize(ref):
    """Отбрасывает якорь строки (`file.md:170`) и хвостовую пунктуацию."""
    ref = re.sub(r":\d+$", "", ref)
    return ref.rstrip("/.,;)")


def vendored(ref):
    if ref.startswith(NOT_VENDORED):
        return False, ("`core/l10n/` — материал сборки, в проект не кладётся: "
                       "локализация накладывается на английское дерево при установке")
    if ref.startswith("agent-skills/") and not ref.startswith(SKILLS_VENDORED):
        return False, ("из сабмодуля вендорится только agents/, hooks/, references/, "
                       "skills/ и LICENSE — см. assets.go")
    return True, ""


def check_refs():
    seen = 0
    for rel, path in core_files():
        for i, line in enumerate(open(path, encoding="utf-8"), 1):
            for raw in REF_RE.findall(line):
                if any(ph in raw for ph in PLACEHOLDER):
                    continue
                ref = normalize(raw)
                if not ref.startswith(ANCHORED):
                    continue
                seen += 1
                where = f"{rel}:{i}"
                if not os.path.exists(os.path.join(ROOT, ref)):
                    err(where, f"`{ref}` не резолвится от корня пайплайна")
                    continue
                ok, why = vendored(ref)
                if not ok:
                    err(where, f"`{ref}` вне payload — {why}")
    return seen


def check_bare_names():
    """Голое `FOO.md` — либо сосед по каталогу, либо артефакт прогона."""
    seen = 0
    for rel, path in core_files():
        here = os.path.dirname(path)
        for i, line in enumerate(open(path, encoding="utf-8"), 1):
            for name in BARE_RE.findall(line):
                if name in RUNTIME_ARTIFACTS:
                    continue
                seen += 1
                if os.path.exists(os.path.join(here, name)):
                    continue
                err(f"{rel}:{i}",
                    f"`{name}` не резолвится ни как сосед по каталогу, ни как "
                    f"артефакт прогона — назовите путь от корня пайплайна "
                    f"(`core/...`, `profiles/...`, `agent-skills/...`)")
    return seen


SUBMODULE_ANCHOR_RE = re.compile(r"`([^`]*?\.md):(\d+(?:-\d+)?)`")


def check_submodule_line_anchors():
    """Якорь на номер строки в сабмодуле гниёт при `--remote` и не падает."""
    skills = os.path.join(ROOT, "agent-skills")
    for rel, path in core_files():
        for i, line in enumerate(open(path, encoding="utf-8"), 1):
            for ref, lineno in SUBMODULE_ANCHOR_RE.findall(line):
                name = os.path.basename(ref)
                inside = ref.startswith("agent-skills/") or any(
                    name in files for _, _, files in os.walk(skills))
                if not inside:
                    continue
                err(f"{rel}:{i}",
                    f"`{ref}:{lineno}` — якорь на номер строки в сабмодуле; "
                    f"`git submodule update --remote` сдвинет его молча. "
                    f"Сослаться на заголовок раздела")


def check_anchor_wording():
    for rel, path in core_files():
        text = open(path, encoding="utf-8").read()
        for pattern, fix in BAD_ANCHOR.items():
            for m in re.finditer(pattern, text, re.IGNORECASE):
                line = text.count("\n", 0, m.start()) + 1
                phrase = " ".join(m.group(0).split())
                err(f"{rel}:{line}",
                    f"якорь путей назван «{phrase}» — у установленного "
                    f"проекта такого корня нет; правильный якорь: {fix}")


def main():
    n = check_refs()
    bare = check_bare_names()
    check_submodule_line_anchors()
    check_anchor_wording()

    print(f"Ядро: проверено {n} ссылок на файлы пайплайна "
          f"и {bare} голых имён\n")
    for w in warnings:
        print(f"  WARN    {w}")
    for e in errors:
        print(f"  ERROR   {e}")
    if warnings or errors:
        print()
    verdict = "FAILED" if errors else "PASSED"
    print(f"{len(errors)} error(s), {len(warnings)} warning(s) — {verdict}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
