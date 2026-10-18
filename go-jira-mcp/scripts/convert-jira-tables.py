#!/usr/bin/env python3
"""Convert {code:markdown} test tables to JIRA wiki markup."""
import re
import sys
from pathlib import Path


def convert_backticks(s: str) -> str:
    return re.sub(r"`([^`]+)`", r"{{\1}}", s)


def parse_row(line: str) -> list[str]:
    inner = line.strip()
    if inner.startswith("|"):
        inner = inner[1:]
    if inner.endswith("|"):
        inner = inner[:-1]
    return [c.strip() for c in inner.split("|")]


def is_separator(line: str) -> bool:
    s = line.strip()
    return bool(re.match(r"^\|[\s\-:|]+\|$", s))


def is_table_row(line: str) -> bool:
    s = line.strip()
    return s.startswith("|") and s.endswith("|") and not is_separator(s)


def convert_block(text: str) -> str:
    lines = text.split("\n")
    out: list[str] = []
    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        if stripped.startswith("## "):
            out.append("h3." + stripped[2:])
            i += 1
            continue
        if stripped.startswith("> "):
            out.append("bq. " + stripped[2:])
            i += 1
            continue
        if stripped == "":
            out.append("")
            i += 1
            continue
        if is_table_row(stripped):
            cells = parse_row(stripped)
            if i + 1 < len(lines) and is_separator(lines[i + 1]):
                out.append("|| " + " || ".join(convert_backticks(c) for c in cells) + " ||")
                i += 2
                continue
            out.append("| " + " | ".join(convert_backticks(c) for c in cells) + " |")
            i += 1
            continue

        out.append(line)
        i += 1
    return "\n".join(out).rstrip("\n")


def convert_file(path: Path) -> bool:
    text = path.read_text(encoding="utf-8")
    pattern = re.compile(r"\{code:markdown\}\n(.*?)\n\{code\}", re.DOTALL)
    if not pattern.search(text):
        return False

    def repl(m: re.Match) -> str:
        return convert_block(m.group(1))

    new_text = pattern.sub(repl, text)
    if new_text != text:
        path.write_text(new_text, encoding="utf-8")
        return True
    return False


def main() -> None:
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("tasks")
    changed = 0
    for path in sorted(root.rglob("*.md")):
        if convert_file(path):
            print(path)
            changed += 1
    print(f"converted {changed} files")


if __name__ == "__main__":
    main()
