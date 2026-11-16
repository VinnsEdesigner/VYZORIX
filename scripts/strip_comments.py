#!/usr/bin/env python3
"""Strip comments from TypeScript sources without touching strings or regexes.

A naive stripper truncates any string containing "//", so
'https://api.example.com' becomes 'https:'. That destroys code silently.

This is a real scanner. It tracks:

  - line comments   // ...
  - block comments  /* ... */
  - single and double quoted strings, with backslash escapes
  - template literals, including ${...} interpolation with arbitrary nesting
  - regex literals, told apart from division by the preceding token
  - directive comments, which are load-bearing and are kept

Directives kept: eslint-disable*, eslint-enable, @ts-expect-error, @ts-ignore,
@ts-nocheck, @ts-check, prettier-ignore, biome-ignore, @__PURE__, vite-ignore.

Usage:
    python3 scripts/strip_comments.py [path ...] [--dry-run] [--check] [--quiet]

Default path is apps/VyzoriX_web/src/lib/local-store.

Exit codes: 0 clean or written; 1 --check found comments; 2 bad usage.
"""

import argparse
import re
import sys
from pathlib import Path

DEFAULT_PATH = "apps/VyzoriX_web/src/lib/local-store"
EXTENSIONS = {".ts", ".tsx", ".mts", ".cts", ".js", ".jsx"}

DIRECTIVE = re.compile(
    r"^\s*(eslint-(disable|enable)|@ts-(expect-error|ignore|nocheck|check)|"
    r"prettier-ignore|biome-ignore|@__PURE__|vite-ignore)"
)

CHARS_ALLOWING_REGEX = set("(,=:[!&|?{};+-*%~^<>")
KEYWORDS_ALLOWING_REGEX = {
    "return", "typeof", "instanceof", "in", "of", "new", "delete", "void",
    "case", "do", "else", "yield", "await", "throw",
}


def _regex_allowed(prev_char: str, prev_word: str) -> bool:
    if prev_char == "":
        return True
    if prev_char in CHARS_ALLOWING_REGEX:
        return True
    if prev_word in KEYWORDS_ALLOWING_REGEX:
        return True
    return False


def strip_comments(source: str) -> str:
    out: list[str] = []
    i = 0
    n = len(source)

    modes: list[str] = ["code"]
    brace_depth: list[int] = []

    prev_char = ""
    prev_word = ""
    word = ""

    while i < n:
        ch = source[i]
        nxt = source[i + 1] if i + 1 < n else ""

        if modes[-1] == "template":
            if ch == "\\" and nxt:
                out.append(source[i:i + 2])
                i += 2
                continue
            if ch == "`":
                out.append(ch)
                modes.pop()
                prev_char = "`"
                i += 1
                continue
            if ch == "$" and nxt == "{":
                out.append("${")
                modes.append("code")
                brace_depth.append(1)
                prev_char = "{"
                i += 2
                continue
            out.append(ch)
            i += 1
            continue

        if ch == "/" and nxt == "/":
            end = source.find("\n", i)
            line = source[i:end if end != -1 else n]
            body = line[2:]
            if DIRECTIVE.match(body):
                out.append(line)
            i = n if end == -1 else end
            continue

        if ch == "/" and nxt == "*":
            end = source.find("*/", i + 2)
            block = source[i:end + 2 if end != -1 else n]
            if DIRECTIVE.match(block[2:].lstrip("*").lstrip()):
                out.append(block)
            i = n if end == -1 else end + 2
            continue

        if ch in ("'", '"'):
            quote = ch
            out.append(ch)
            i += 1
            while i < n:
                c = source[i]
                if c == "\\" and i + 1 < n:
                    out.append(source[i:i + 2])
                    i += 2
                    continue
                out.append(c)
                i += 1
                if c == quote:
                    break
            prev_char = quote
            prev_word = ""
            continue

        if ch == "`":
            out.append(ch)
            modes.append("template")
            prev_char = "`"
            prev_word = ""
            i += 1
            continue

        if ch == "/" and _regex_allowed(prev_char, prev_word):
            out.append(ch)
            i += 1
            in_class = False
            closed = False
            while i < n:
                c = source[i]
                if c == "\\" and i + 1 < n:
                    out.append(source[i:i + 2])
                    i += 2
                    continue
                if c == "\n":
                    break
                if c == "[":
                    in_class = True
                elif c == "]":
                    in_class = False
                elif c == "/" and not in_class:
                    out.append(c)
                    i += 1
                    closed = True
                    break
                out.append(c)
                i += 1
            if closed:
                while i < n and source[i].isalpha():
                    out.append(source[i])
                    i += 1
            prev_char = "/"
            prev_word = ""
            continue

        if ch == "{" and brace_depth:
            brace_depth[-1] += 1
        elif ch == "}" and brace_depth:
            brace_depth[-1] -= 1
            if brace_depth[-1] == 0:
                modes.pop()
                brace_depth.pop()
                out.append(ch)
                prev_char = "}"
                prev_word = ""
                i += 1
                continue

        out.append(ch)
        if ch.isalnum() or ch == "_" or ch == "$":
            word += ch
        else:
            if word:
                prev_word = word
                word = ""
            if not ch.isspace():
                prev_char = ch
        i += 1

    return "".join(out)


def tidy(source: str) -> str:
    lines = [line.rstrip() for line in source.split("\n")]
    result: list[str] = []
    blank = 0
    for line in lines:
        if line == "":
            blank += 1
            if blank > 1:
                continue
        else:
            blank = 0
        result.append(line)
    while result and result[0] == "":
        result.pop(0)
    while result and result[-1] == "":
        result.pop()
    return "\n".join(result) + "\n"


def collect(paths: list[str]) -> list[Path]:
    files: list[Path] = []
    for raw in paths:
        path = Path(raw)
        if path.is_file():
            if path.suffix in EXTENSIONS:
                files.append(path)
            continue
        for candidate in sorted(path.rglob("*")):
            if any(part.startswith(".") for part in candidate.parts):
                continue
            if candidate.is_file() and candidate.suffix in EXTENSIONS:
                files.append(candidate)
    return files


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("paths", nargs="*", default=[DEFAULT_PATH])
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--quiet", action="store_true")
    args = parser.parse_args()

    targets = collect(args.paths or [DEFAULT_PATH])
    if not targets:
        print("no files matched", file=sys.stderr)
        return 2

    changed = 0
    saved = 0

    for path in targets:
        original = path.read_text(encoding="utf-8")
        cleaned = tidy(strip_comments(original))
        if cleaned == original:
            continue

        changed += 1
        saved += len(original) - len(cleaned)
        if not args.quiet:
            print(f"  {path}: -{len(original) - len(cleaned)} bytes")

        if not args.dry_run and not args.check:
            path.write_text(cleaned, encoding="utf-8")

    verb = "would change" if (args.dry_run or args.check) else "changed"
    print(f"\n{verb} {changed} of {len(targets)} files ({saved} bytes)")

    if args.check and changed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
