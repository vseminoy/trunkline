#!/usr/bin/env python3
"""
lint-core-profile-boundary.py — гейт границы «ядро ↔ профиль».

Зачем: расслоение core/profiles держится на дисциплине, а дисциплина без гейта
сползает обратно. Один `go build ./...`, дописанный в шаг «просто чтобы работало»,
и ядро снова знает про Go. Этот линтер ловит такое до коммита.

Что проверяется:

  1. Лексика стека в `core/`      — ядро не упоминает языки, инструменты и команды.
  2. Слоты существуют             — всё, на что ссылаются шаги (`commands.*`,
                                     `guidelines.roles.*`, `stack.*`, `hooks.*`),
                                     объявлено в каждом профиле.
  3. Целостность профиля          — обязательные ключи манифеста, существование
                                     файлов гайдлайнов, надстроек, хуков.
  4. Надстройки привязаны к шагам — каждый ключ `step_overlays` соответствует
                                     реальному файлу шага в ядре.
  5. Осиротевшие файлы            — надстройка на диске, не объявленная в манифесте.

Запуск: python3 scripts/lint-core-profile-boundary.py
Коды выхода: 0 — чисто, 1 — есть ошибки.

Требует PyYAML. Профили пишут третьи лица, поэтому парсить YAML руками нельзя.
"""

import os
import re
import sys

try:
    import yaml
except ImportError:
    sys.exit("нужен PyYAML: pip install pyyaml")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORE = os.path.join(ROOT, "core")
PROFILES = os.path.join(ROOT, "profiles")

# --- 1. Лексика, которой не место в ядре ------------------------------------
#
# Ключ — регулярка, значение — что предложить вместо неё.
# Имена профилей (ttk-go-echo-sqlc) сюда НЕ входят: имя профиля — не лексика стека,
# ядру можно называть профиль в примере флага.

STACK_LEXICON = {
    r"\boapi-codegen\b": "stack.contract.codegen",
    r"\bsqlc\b": "stack.db_access",
    r"\bgoose\b": "stack.migrations",
    r"\bEcho\b": "stack.http",
    r"\bgolangci\b": "required_project_files",
    r"\bgo build\b": "commands.build",
    r"\bgo test\b": "commands.test",
    r"\bmake lint\b": "commands.lint",
    r"\bmake test-unit\b": "commands.test_unit",
    r"\bgo\.mod\b": "профиль знает свои файлы сам",
    r"\bgo tool pprof\b": "надстройка профиля",
    r"timestamp\(0\)": "guidelines.roles.data_types",
    r"\bpgx\b": "надстройка профиля",
    r"\bslog\b": "надстройка профиля",
    r"\bgodoc\b": "надстройка профиля",
    r"\bGodoc\b": "надстройка профиля",
    r"\*_test\.go\b": "layers[].test_file",
    r"\bqueries/\*\.sql\b": "layers[].artifacts",
    r"\bOpenAPI\b": "stack.contract.kind",
    r"\bGo-специфика\b": "надстройка профиля",
    # Имена организаций в ядре — та же ошибка, что имена инструментов: ядро
    # универсально, вендор живёт в имени профиля и в его гайдлайнах.
    r"(?i)\bttk\b": "имя организации — только в профиле",
    r"\bттк\b": "имя организации — только в профиле",
    r"\.ttk/": "каталог состояния — `.pipeline/`",
    # Единственный обязательный для любого профиля файл — AGENTS.md (в нём флаги).
    # Остальные объявляет профиль в `required_project_files`.
    #
    # AGENTS.md сюда НЕ входит: это кросс-хостовая конвенция, которую ядро вправе
    # называть, — в отличие от файлов правил конкретного инструмента. CLAUDE.md
    # входит: пайплайн не привязан к одной LLM, и ядру незачем знать имя файла,
    # который подхватывает только один хост.
    r"\bCLAUDE\.md\b": "блок `## Pipeline flags` живёт в `AGENTS.md`",
    r"\.golangci\b": "объявить в `required_project_files` профиля",
    # Инструмент заведения задач — такая же стек-специфика, как компилятор:
    # ядро знает только слоты секции `tasks` профиля.
    r"\bgo-jira-mcp\b": "tasks.publisher",
    r"\btask-format\.md\b": "tasks.format_spec",
    r"\bcreate_all_tasks\b": "tasks.publisher.operations",
    r"\bcreate_jira_task\b": "tasks.publisher.operations",
    r"\bpull_epic\b": "tasks.publisher.operations",
    r"\bTASKS_DIR\b": "tasks.dir",
}

# Файлы ядра, которые линтер не смотрит: справочники по формату журнала и т.п.,
# где встречаются примеры строк из реальных прогонов.
CORE_SKIP = set()

# --- 2. Слоты, на которые ядро имеет право ссылаться -------------------------

SLOT_RE = re.compile(
    r"`(commands|stack|hooks|guidelines|tasks)\.([a-z_]+)(?:\.([a-z_]+))?`"
)
# Ядро переведено на английский (Ф2.3), поэтому роли ищутся по-английски, в двух
# формах сразу: «the roles `x` (…), `y` (…)» и «the `x` role». Первая — список,
# который в шагах переносится по строкам, поэтому разделитель между токенами
# допускает перенос; вторая — упоминание одной роли постфиксом.
ROLE_RE = re.compile(r"\broles?\s+((?:`[a-z_]+`(?:[^`]{0,60})?){1,8})")
ROLE_POSTFIX_RE = re.compile(r"`([a-z_]+)`\s+role\b")
ROLE_TOKEN_RE = re.compile(r"`([a-z_]+)`")

REQUIRED_KEYS = ["name", "version", "language", "stack", "commands",
                 "required_project_files", "layers", "guidelines"]

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
                rel = os.path.relpath(p, ROOT)
                if rel not in CORE_SKIP:
                    yield rel, p


def check_lexicon(profile_names):
    # Имя профиля — не лексика стека: ядру можно назвать `ttk-go-echo-sqlc`
    # в примере флага. Вырезаем имена до сопоставления, иначе `ttk-go-echo-sqlc`
    # ловится как «sqlc».
    def strip_names(line):
        for n in profile_names:
            line = line.replace(n, "<профиль>")
        return line

    for rel, path in core_files():
        for i, raw in enumerate(open(path, encoding="utf-8"), 1):
            line = strip_names(raw)
            for pattern, slot in STACK_LEXICON.items():
                m = re.search(pattern, line)
                if m:
                    hint = (slot if slot[0].isupper() or " " in slot.split(".")[0]
                            else f"взять из `{slot}` или унести в надстройку профиля")
                    err(f"{rel}:{i}", f"лексика стека «{m.group(0)}» в ядре → {hint}")


def collect_slot_refs():
    """Слоты и роли, на которые ссылаются файлы ядра."""
    slots, roles = set(), set()
    for rel, path in core_files():
        text = open(path, encoding="utf-8").read()
        for group, key, sub in SLOT_RE.findall(text):
            slots.add((group, key, sub or None))
        for run in ROLE_RE.findall(text):
            roles.update(ROLE_TOKEN_RE.findall(run))
        roles.update(ROLE_POSTFIX_RE.findall(text))
    return slots, roles


def dig(manifest, group, key, sub):
    node = manifest.get(group)
    if not isinstance(node, dict) or key not in node:
        return None
    if sub is None:
        return node[key]
    inner = node[key]
    if not isinstance(inner, dict) or sub not in inner:
        return None
    return inner[sub]


def check_tools(profile_names):
    """Инструменты не знают, чей профиль их объявил.

    `go-jira-mcp/` — издатель общего назначения: читает `.md` и заводит задачи в
    трекере. Какие в этих `.md` соглашения — вопрос профиля, объявившего издатель
    слотом `tasks.publisher`. Имя конкретного профиля в инструменте означает, что
    второй профиль с тем же издателем либо получит чужие правила, либо заведёт
    вторую копию спеки. Плейсхолдеры (`profiles/<имя>/`) — не нарушение.

    Каталог с рабочими данными (`tasks/`) исключён: это задачи, а не код инструмента.
    """
    roots = [os.path.join(ROOT, d) for d in ("go-jira-mcp", "publishers")]
    for tools in [d for d in roots if os.path.isdir(d)]:
        for base, dirs, names in os.walk(tools):
            dirs[:] = [d for d in dirs if d not in ("tasks", "bin", ".git")]
            for n in sorted(names):
                if not n.endswith((".md", ".go", ".json", ".yaml", ".yml")):
                    continue
                path = os.path.join(base, n)
                rel = os.path.relpath(path, ROOT)
                for i, line in enumerate(open(path, encoding="utf-8", errors="replace"), 1):
                    for pname in profile_names:
                        if pname in line:
                            err(f"{rel}:{i}",
                                f"инструмент называет профиль `{pname}` — он не должен знать, "
                                f"чей профиль его объявил; ссылаться на слот "
                                f"(`tasks.format_spec`) или на `profiles/<имя>/`")


def check_profile(name, slots, roles):
    pdir = os.path.join(PROFILES, name)
    mpath = os.path.join(pdir, "profile.yaml")
    rel = os.path.relpath(mpath, ROOT)

    if not os.path.exists(mpath):
        err(f"profiles/{name}", "нет profile.yaml")
        return
    try:
        m = yaml.safe_load(open(mpath, encoding="utf-8"))
    except yaml.YAMLError as e:
        err(rel, f"невалидный YAML: {e}")
        return

    for k in REQUIRED_KEYS:
        if k not in m:
            err(rel, f"нет обязательного ключа `{k}`")

    # 2. Каждый слот из ядра объявлен. Явный null — осознанное «нет», не ошибка.
    for group, key, sub in sorted(slots, key=lambda s: (s[0], s[1], s[2] or "")):
        if group == "guidelines" and key == "roles":
            continue  # роли проверяются отдельно
        # Явный `null` и отсутствие ключа — разные вещи, и путать их нельзя:
        # `tasks: null` — объявленное «трекера нет», ядро объявит шаги публикации
        # «не применимо»; ключа нет вовсе — автор просто забыл слот.
        if group in m and m[group] is None:
            continue
        node = m.get(group)
        if not isinstance(node, dict):
            err(rel, f"ядро ссылается на `{group}.*`, но секции `{group}` нет")
            continue
        if key not in node:
            err(rel, f"ядро ссылается на `{group}.{key}`, слота нет в манифесте")
            continue
        if sub and isinstance(node[key], dict) and sub not in node[key]:
            err(rel, f"ядро ссылается на `{group}.{key}.{sub}`, подслота нет")

    # Пути внутри профиля резолвятся ОТ КАТАЛОГА ПРОФИЛЯ. Проверять их от корня
    # репозитория нельзя: путь вида `profiles/<чужой>/guidelines/...` в скопированном
    # профиле тогда существует, линтер молчит, а копия читает гайдлайны оригинала.
    def in_profile(slot, val):
        if not isinstance(val, str) or not val:
            return None
        if os.path.isabs(val):
            err(rel, f"{slot} → {val}: абсолютный путь; пути даны от каталога профиля")
            return None
        if val.startswith("profiles/"):
            err(rel, f"{slot} → {val}: путь привязан к корню репозитория; "
                     f"убрать префикс `profiles/{name}/` — пути даны от каталога профиля")
            return None
        if val == ".." or val.startswith("../") or "/../" in val or val.endswith("/.."):
            err(rel, f"{slot} → {val}: путь выходит за каталог профиля")
            return None
        full = os.path.join(pdir, val)
        if not os.path.exists(full):
            err(rel, f"{slot} → {val} не существует в профиле")
            return None
        return full

    # Роли гайдлайнов: объявлены и файлы существуют.
    g = m.get("guidelines") or {}
    groot = g.get("root", "")
    declared = (g.get("roles") or {})
    for role in sorted(roles):
        if role not in declared:
            err(rel, f"ядро ссылается на роль гайдлайна `{role}`, её нет в guidelines.roles")
    in_profile("guidelines.root", groot)
    for role, fname in declared.items():
        in_profile(f"guidelines.roles.{role}", os.path.join(groot, fname))
    in_profile("guidelines.index", g.get("index"))

    # 3. Пути из манифеста существуют.
    #
    # Слота `references.*` здесь нет намеренно: эталоны артефактов живут в ядре
    # (`core/developer/references/`) — форма артефакта одинакова в любом стеке,
    # а стек-специфику в неё подставляют `layers` и надстройка шага. Профиль их
    # не объявляет, и ядру нечего резолвить.
    # Секция `tasks` — два разных якоря, и путать их нельзя:
    #   * документы профиля (`format_spec`, `publisher.setup`) едут в проект вместе
    #     с профилем → путь от каталога профиля, файл обязан существовать;
    #   * данные проекта (`dir`, `analysis_dir`) появляются в разрабатываемом проекте
    #     → путь от корня проекта, проверять существование здесь бессмысленно.
    # Раньше проверялись одинаково, от корня репозитория: `format_spec`, ведущий
    # в go-jira-mcp/, проходил гейт и отсутствовал в установленном проекте.
    # Секция `tasks` целиком null — осознанное «трекера нет», не ошибка.
    def in_project(slot, val):
        if not isinstance(val, str) or not val:
            return
        if os.path.isabs(val):
            err(rel, f"{slot} → {val}: абсолютный путь; путь даётся от корня "
                     f"разрабатываемого проекта")
        elif val.startswith("../") or val == "..":
            err(rel, f"{slot} → {val}: путь выходит за пределы проекта")
        elif val.split("/")[0] in ("core", "profiles", "agent-skills"):
            err(rel, f"{slot} → {val}: это путь в пайплайн, а слот описывает данные "
                     f"проекта (задачи коммитятся рядом с кодом)")

    tasks = m.get("tasks") or {}
    in_profile("tasks.format_spec", tasks.get("format_spec"))
    in_profile("tasks.estimation_calibration", tasks.get("estimation_calibration"))
    in_project("tasks.dir", tasks.get("dir"))
    in_project("tasks.analysis_dir", tasks.get("analysis_dir"))
    pub = tasks.get("publisher") or {}
    if isinstance(pub, dict):
        in_profile("tasks.publisher.setup", pub.get("setup"))
        # Контракт издателя — третий якорь: от корня пайплайна. Инструмент не
        # принадлежит профилю, издатель у профилей может быть общий, и копия
        # контракта в каждом профиле разъедется с кодом, который его исполняет.
        contract = pub.get("contract")
        if isinstance(contract, str) and contract:
            if not contract.startswith("publishers/"):
                err(rel, f"tasks.publisher.contract → {contract}: контракт издателя "
                         f"лежит в `publishers/<издатель>/`, он принадлежит инструменту, "
                         f"а не профилю")
            elif not os.path.exists(os.path.join(ROOT, contract)):
                err(rel, f"tasks.publisher.contract → {contract} не существует")

    for slot, val in (m.get("hooks") or {}).items():
        if isinstance(val, str) and val.endswith(".sh"):
            in_profile(f"hooks.{slot}", val)

    # 4. Надстройки привязаны к реальным шагам ядра.
    steps_dir = os.path.join(CORE, "developer", "steps")
    step_ids = {}
    for fn in os.listdir(steps_dir):
        mm = re.match(r"^(\d+[a-z]?(?:-\d+)?)-", fn)
        if mm:
            step_ids[mm.group(1)] = fn
    overlays = m.get("step_overlays") or {}
    seen = set()
    for sid, opath in overlays.items():
        sid = str(sid)
        if sid not in step_ids:
            err(rel, f"step_overlays[{sid}] — шага с таким номером в ядре нет")
        full = in_profile(f"step_overlays[{sid}]", opath)
        if full:
            seen.add(os.path.abspath(full))

    # 5. Надстройки на диске, не объявленные в манифесте.
    odir = os.path.join(pdir, "steps")
    if os.path.isdir(odir):
        for fn in sorted(os.listdir(odir)):
            if not fn.endswith(".md"):
                continue
            full = os.path.abspath(os.path.join(odir, fn))
            if full not in seen:
                warn(os.path.relpath(full, ROOT),
                     "надстройка не объявлена в step_overlays — читаться не будет")

    return m


def main():
    if not os.path.isdir(PROFILES):
        sys.exit("нет папки profiles/")

    names = sorted(n for n in os.listdir(PROFILES)
                   if os.path.isdir(os.path.join(PROFILES, n)))
    if not names:
        err("profiles/", "ни одного профиля")

    check_lexicon(names)
    slots, roles = collect_slot_refs()

    print(f"Ядро: {len(list(core_files()))} файлов, "
          f"{len(slots)} слотов и {len(roles)} ролей в ссылках")
    print(f"Профили: {', '.join(names)}\n")

    for n in names:
        check_profile(n, slots, roles)

    check_tools(names)

    for e in errors:
        print(f"  ERROR   {e}")
    for w in warnings:
        print(f"  WARN    {w}")

    print(f"\n{len(errors)} error(s), {len(warnings)} warning(s) — "
          f"{'FAILED' if errors else 'PASSED'}")
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
