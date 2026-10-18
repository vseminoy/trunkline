#!/usr/bin/env python3
"""
lint-l10n.py — гейт расхождения локализаций ядра.

Зачем: `language` во флагах позволяет проекту читать ядро на своём языке
(`core/l10n/<lang>/`, файл замещает английский аналог). Две копии одного текста
разъезжаются всегда — вопрос только в том, заметит это гейт или пользователь
на прогоне. Здесь замечает гейт.

Механика: `core/l10n/<lang>/l10n.lock` хранит sha256 английского исходника
на момент, когда перевод был объявлен актуальным. Английский файл поменялся,
а перевод — нет → расхождение.

Что проверяется:

  1. Сирота       — файл в локализации, которому нет английского исходника.
  2. Расхождение  — английский исходник изменился после перевода.
  3. Непокрытое   — английский файл без перевода (WARN: работает фолбэком).
  4. Целостность  — в lock нет записей о файлах, которых больше нет.
  5. Протокол     — маркеры, которые машина и шаг 7a читают буквально
                    (`[STOP]`, `[CONDITIONAL]`, `ADDED`/`MODIFIED`/`REMOVED`,
                    идентификаторы правил `R-007-2`), не потеряны, не размножены
                    и не переведены.
                    Хеш этого не ловит: он меряет дрейф исходника, а не целость
                    перевода.
  6. Front-matter — `name:`/`description:` скилла совпадают с исходником: по ним
                    хост-модель выбирает скилл, а проверяют их линтер скиллов и
                    routing-evals — оба только на английской версии.

Запуск:  python3 scripts/lint-l10n.py
Принять текущее состояние как переведённое:
         python3 scripts/lint-l10n.py --update [<lang>]

Коды выхода: 0 — чисто, 1 — есть расхождения или сироты.
"""

import hashlib
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORE = os.path.join(ROOT, "core")
L10N = os.path.join(CORE, "l10n")
LOCK = "l10n.lock"


def sha(path):
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def english_files():
    """Английское ядро: всё под core/, кроме самих локализаций."""
    out = {}
    for base, dirs, names in os.walk(CORE):
        if os.path.abspath(base).startswith(os.path.abspath(L10N)):
            continue
        dirs[:] = [d for d in dirs if os.path.join(base, d) != L10N]
        for n in names:
            if n.endswith(".md"):
                p = os.path.join(base, n)
                out[os.path.relpath(p, CORE).replace(os.sep, "/")] = p
    return out


def localized_files(lang):
    root = os.path.join(L10N, lang)
    out = {}
    for base, _, names in os.walk(root):
        for n in names:
            if n.endswith(".md"):
                p = os.path.join(base, n)
                out[os.path.relpath(p, root).replace(os.sep, "/")] = p
    return out


def languages():
    if not os.path.isdir(L10N):
        return []
    return sorted(d for d in os.listdir(L10N)
                  if os.path.isdir(os.path.join(L10N, d)))


def load_lock(lang):
    p = os.path.join(L10N, lang, LOCK)
    if not os.path.exists(p):
        return {}
    with open(p, encoding="utf-8") as f:
        return json.load(f).get("sources", {})


def save_lock(lang, sources):
    p = os.path.join(L10N, lang, LOCK)
    payload = {
        "_comment": "sha256 английского исходника на момент перевода. "
                    "Обновлять только вместе с самим переводом: "
                    "python3 scripts/lint-l10n.py --update " + lang,
        "language": lang,
        "sources": dict(sorted(sources.items())),
    }
    with open(p, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)
        f.write("\n")


# Маркеры протокола. Написание одно на все языки — английское: маркер читают
# машина и шаг 7a, а не человек, и второе написание означает второй парсер.
# Локализованная форма стоила ровно того, ради чего гейт и заведён: `[УСЛОВНЫЙ]`,
# просклонённый в `[УСЛОВНЫЕ]`, читается нормально и не парсится ничем.
# Скобочные маркеры считаются по открывающей части: хвост («— описание ДО кода»)
# — проза и переводится свободно.
PROTOCOL = [
    r"\[STOP",
    r"\[CONDITIONAL",
    r"\bADDED\b",
    r"\bMODIFIED\b",
    r"\bREMOVED\b",
    # Заголовки шаблона базы знаний. Стали протоколом, когда появился
    # core/developer/scripts/kb-sync.py: он ищет таблицу правил по заголовку, а
    # окно дрейфа — по дате `Updated:`. Переведённый заголовок не даёт ошибки —
    # он даёт ЛОЖНОЕ «чисто»: таблица не найдена, `0 rule(s)`, exit 0. Это хуже
    # отказа, потому что шаг 21 идёт искать смысл в записи, механику которой
    # никто не проверил. Ровно тот же класс, что `[УСЛОВНЫЕ]` вместо
    # `[CONDITIONAL]`, и ловится тем же способом.
    r"## Business rules in force",
    r"## Invariants and pitfalls",
]
# Полями шапки (`Updated:`, `Slice:`) список не расширяется: их носит и состояние
# анализа (`core/analytic/skills/analysis-state/SKILL.md`, поле `Updated:`), где
# они законная проза и переводятся. Список применяется ко всем файлам сразу и
# различить эти два случая не может, так что тут он ловит заголовки — они
# встречаются только в шаблоне базы знаний, — а про поля шапки сказано в самом
# `core/skills/project-knowledge/SKILL.md`.

# Идентификаторы живут внутри прозы, поэтому их число законно расходится
# («каждое правило» против «у каждого правила свой»). Проверяется присутствие:
# было в исходнике — обязано быть и в переводе.
#
# Ловится конкретная форма `R-007-2`, а не шаблон `R-<run>-<n>`: шаблон — проза,
# его угловые скобки переводятся (`R-<прогон>-<n>`), а конкретный идентификатор
# в примере, таблице или строке кода обязан доехать буквально.
PROTOCOL_PRESENCE = [r"\bR-\d+-\d+\b"]


FRONTMATTER = re.compile(r"\A---\n.*?\n---\n", re.S)


def count(pattern, text):
    return len(re.findall(pattern, text))


def frontmatter_drift(rel, en_path, loc_path):
    """Front-matter скилла — маршрутизация, а не проза: он обязан совпадать.

    По `description:` хост-модель выбирает скилл, и проверяют его линтер скиллов
    и routing-evals — оба на английском исходнике. Переведённое описание едет
    в проект непроверенным, поэтому front-matter копируется как есть.
    """
    en_text = open(en_path, encoding="utf-8").read()
    loc_text = open(loc_path, encoding="utf-8").read()
    en_fm, loc_fm = FRONTMATTER.match(en_text), FRONTMATTER.match(loc_text)
    if not en_fm:
        return []
    if not loc_fm:
        return [f"{rel}: в исходнике есть front-matter, в переводе нет"]
    if en_fm.group(0) != loc_fm.group(0):
        return [f"{rel}: front-matter разошёлся с исходником — `name:`/`description:` "
                f"копируются как есть, по ним идёт маршрутизация скилла "
                f"(см. core/l10n/README.md)"]
    return []


def protocol_drift(rel, en_path, loc_path):
    """Расхождение по маркерам протокола между исходником и переводом."""
    out = []
    en_text = open(en_path, encoding="utf-8").read()
    loc_text = open(loc_path, encoding="utf-8").read()

    for pat in PROTOCOL:
        want = count(pat, en_text)
        got = count(pat, loc_text)
        if want != got:
            out.append(f"{rel}: маркер `{pat}` — {want} в исходнике, {got} в переводе; "
                       f"переводится проза, не протокол (см. core/l10n/README.md)")
    for pat in PROTOCOL_PRESENCE:
        if count(pat, en_text) and not count(pat, loc_text):
            out.append(f"{rel}: идентификатор `{pat}` есть в исходнике и потерян в переводе")
    return out


def check(lang, en):
    errors, warnings = [], []
    loc = localized_files(lang)
    lock = load_lock(lang)

    for rel in sorted(loc):
        if rel not in en:
            errors.append(f"{lang}/{rel}: перевод без английского исходника — "
                          f"файл переименован или удалён в ядре")
            continue
        recorded = lock.get(rel)
        current = sha(en[rel])
        if recorded is None:
            errors.append(f"{lang}/{rel}: нет записи в {LOCK} — "
                          f"неизвестно, с какой версии переводили")
        elif recorded != current:
            errors.append(f"{lang}/{rel}: английский исходник изменился после перевода "
                          f"→ перевести заново и `--update {lang}`")
        errors += protocol_drift(f"{lang}/{rel}", en[rel], loc[rel])
        errors += frontmatter_drift(f"{lang}/{rel}", en[rel], loc[rel])

    for rel in sorted(lock):
        if rel not in loc:
            errors.append(f"{lang}/{LOCK}: запись про {rel}, а перевода нет")

    # Непереведённое — не бэклог, а объём локализации (core/l10n/README.md,
    # «Что локализуется»). Строкой на файл он читается как список задач и
    # приучает пролистывать вывод гейта, поэтому сводится в одну строку:
    # счётчик выше (`n/m файлов переведено`) говорит то же самое.
    missing = sorted(set(en) - set(loc))
    if missing:
        warnings.append(f"{lang}: {len(missing)} файлов без перевода — "
                        f"прогон берёт английский (фолбэк, так и задумано)")

    return errors, warnings, len(loc), len(en)


def update(lang, en):
    loc = localized_files(lang)
    sources = {}
    for rel in loc:
        if rel in en:
            sources[rel] = sha(en[rel])
    save_lock(lang, sources)
    print(f"{lang}: зафиксировано {len(sources)} переводов в {LOCK}")


def main():
    args = sys.argv[1:]
    en = english_files()
    langs = languages()

    if not langs:
        print("Локализаций нет — ядро только на английском.")
        return 0

    if args and args[0] == "--update":
        targets = args[1:] or langs
        for lang in targets:
            if lang not in langs:
                sys.exit(f"нет локализации {lang}; есть: {', '.join(langs)}")
            update(lang, en)
        return 0

    errors, warnings = [], []
    print(f"Английское ядро: {len(en)} файлов. Локализации: {', '.join(langs)}\n")
    for lang in langs:
        e, w, n_loc, n_en = check(lang, en)
        errors += e
        warnings += w
        print(f"  {lang}: {n_loc}/{n_en} файлов переведено")

    print()
    for e in errors:
        print(f"  ERROR   {e}")
    for w in warnings:
        print(f"  WARN    {w}")

    print(f"\n{len(errors)} error(s), {len(warnings)} warning(s) — "
          f"{'FAILED' if errors else 'PASSED'}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
