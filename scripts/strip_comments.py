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

Generated sources (any `generated/` or `generated-rq/` directory, or a `*.gen.ts`
file) are skipped by default: codegen regenerates them with their comments, so
stripping them is undone and shows as churn. Pass --include-generated to override.

After writing, the changed files are re-linted with ESLint (the "vet" step),
grouped by the package that owns them. A stripped `eslint-disable-next-line`
would otherwise surface only in CI. Pass --no-vet to skip it.

Usage:
    python3 scripts/strip_comments.py [path ...] [--dry-run] [--check] [--quiet]
        [--include-generated] [--no-vet]

Default path is apps/VyzoriX_web/src/lib/local-store.

Exit codes: 0 clean or written; 1 --check found comments or vet failed; 2 bad usage.
"""

import argparse
import re
import subprocess
import sys
from pathlib import Path

DEFAULT_PATH = "apps/VyzoriX_web/src/lib/local-store"
EXTENSIONS = {".ts", ".tsx", ".mts", ".cts", ".js", ".jsx"}

# Machine-generated sources are regenerated WITH their comments (orval, TanStack
# Router). Stripping them would be undone by the next codegen run and would show
# as churn, so they are skipped unless --include-generated is passed.
GENERATED_DIR_NAMES = {"generated", "generated-rq"}
GENERATED_FILE_SUFFIXES = (".gen.ts", ".gen.tsx")

# ESLint flat-config filenames, used to find the package a file belongs to so
# the vet step runs with that package's config and tsconfig.
ESLINT_CONFIGS = ("eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts")


def is_generated(path: Path) -> bool:
    if any(part in GENERATED_DIR_NAMES for part in path.parts):
        return True
    return path.name.endswith(GENERATED_FILE_SUFFIXES)

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


def collect(paths: list[str], include_generated: bool = False) -> list[Path]:
    files: list[Path] = []
    for raw in paths:
        path = Path(raw)
        if path.is_file():
            if path.suffix in EXTENSIONS and (include_generated or not is_generated(path)):
                files.append(path)
            continue
        for candidate in sorted(path.rglob("*")):
            if any(part.startswith(".") for part in candidate.parts):
                continue
            if candidate.is_file() and candidate.suffix in EXTENSIONS:
                if not include_generated and is_generated(candidate):
                    continue
                files.append(candidate)
    return files


def find_package_root(path: Path) -> Path | None:
    """Walk up from `path` to the nearest directory holding an ESLint config.

    ESLint v9 resolves its flat config relative to the working directory, so the
    vet step must run with cwd set to the package that owns the file.
    """
    for directory in [path.parent, *path.parents]:
        if any((directory / name).exists() for name in ESLINT_CONFIGS):
            return directory
    return None


def vet(files: list[Path], quiet: bool) -> int:
    """Run ESLint over the given files, grouped by owning package.

    Comments carry the `eslint-disable-next-line` directives a linter relies on;
    stripping them can surface a rule the directive was suppressing. Re-linting
    right after the strip catches that here rather than in CI. Returns the number
    of files ESLint reported on (0 when clean or ESLint is unavailable).
    """
    eslint = Path("node_modules/.bin/eslint")
    if not eslint.exists():
        if not quiet:
            print("eslint not found; skipping vet", file=sys.stderr)
        return 0

    by_package: dict[Path, list[Path]] = {}
    for file in files:
        root = find_package_root(file)
        if root is None:
            continue
        by_package.setdefault(root, []).append(file)

    offending = 0
    for root, members in by_package.items():
        rel = [str(m.relative_to(root)) for m in members]
        result = subprocess.run(
            [str((root / eslint).resolve()) if (root / eslint).exists() else str(eslint.resolve()), *rel],
            cwd=root,
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            offending += len(members)
            if not quiet:
                print(result.stdout.strip())
                print(result.stderr.strip(), file=sys.stderr)
    return offending


def safe_to_write(original: str, cleaned: str) -> str | None:
    """Return a reason not to write `cleaned`, or None when the result looks sane.

    The scanner understands strings and regexes but NOT JSX text, so a literal such as
    `<code>/*</code>` reads as an unterminated block comment and swallows the rest of the
    file. That is silent truncation, and it destroyed a real file once. A comment strip can
    only ever remove comments, so a result that loses a large share of the file, or most of
    its lines, is a truncation rather than a strip — refuse it and say so.
    """
    lost = len(original) - len(cleaned)
    if lost <= 0:
        return None

    # Comments are a minority of any real file. Losing more than 60% of the bytes means
    # something other than comments went missing.
    if lost > len(original) * 0.6:
        return f"would drop {lost} of {len(original)} bytes — looks like truncation, not a strip"

    original_lines = original.count("\n")
    cleaned_lines = cleaned.count("\n")
    if original_lines and cleaned_lines < original_lines * 0.4:
        return (
            f"would drop {original_lines - cleaned_lines} of {original_lines} lines — "
            "likely an unterminated comment that swallowed code"
        )

    return None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("paths", nargs="*", default=[DEFAULT_PATH])
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--quiet", action="store_true")
    parser.add_argument(
        "--include-generated",
        action="store_true",
        help="also strip generated files (off by default: codegen restores them)",
    )
    parser.add_argument(
        "--no-vet",
        action="store_true",
        help="skip the post-strip ESLint pass",
    )
    args = parser.parse_args()

    targets = collect(args.paths or [DEFAULT_PATH], include_generated=args.include_generated)
    if not targets:
        print("no files matched", file=sys.stderr)
        return 2

    changed = 0
    saved = 0
    touched: list[Path] = []
    refused = 0

    for path in targets:
        original = path.read_text(encoding="utf-8")
        cleaned = tidy(strip_comments(original))
        if cleaned == original:
            continue

        reason = safe_to_write(original, cleaned)
        if reason is not None:
            refused += 1
            print(f"refused: {path}: {reason}", file=sys.stderr)
            continue

        changed += 1
        saved += len(original) - len(cleaned)
        touched.append(path)
        if not args.quiet:
            print(f"  {path}: -{len(original) - len(cleaned)} bytes")

        if not args.dry_run and not args.check:
            # Write via a temp file and rename, so an interrupted run can never leave a
            # half-written source in place of a working one.
            tmp = path.with_suffix(path.suffix + ".stripped")
            tmp.write_text(cleaned, encoding="utf-8")
            tmp.replace(path)

    verb = "would change" if (args.dry_run or args.check) else "changed"
    print(f"\n{verb} {changed} of {len(targets)} files ({saved} bytes)")

    if refused:
        print(f"refused {refused} file(s) whose result looked truncated", file=sys.stderr)
        return 1

    if args.check and changed:
        return 1

    # Vet the files actually written, so a comment that carried a directive is
    # caught here. A dry run writes nothing, so there is nothing to vet.
    if touched and not args.dry_run and not args.no_vet:
        offending = vet(touched, args.quiet)
        if offending:
            print(f"\nvet: eslint reported on {offending} file(s)", file=sys.stderr)
            return 1
        if not args.quiet:
            print("vet: clean")

    return 0


if __name__ == "__main__":
    sys.exit(main())
