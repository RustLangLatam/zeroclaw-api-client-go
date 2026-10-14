#!/usr/bin/env python3
"""The endpoint map of docs/book/src/reference/endpoints.md, built from the
`(METHOD /path)` that each exported method's doc comment names.

    python3 scripts/endpoint-map.py           # print the table
    python3 scripts/endpoint-map.py --write   # replace the table in the page
    python3 scripts/endpoint-map.py --check   # fail if the page is stale (CI)
"""
import glob
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
PAGE = ROOT / "docs/book/src/reference/endpoints.md"
HEADER = "| Method | Path | File | Function |"
METHOD = re.compile(r"((?:^//[^\n]*\n)+)func \((?:\w+) \*?(\w+)\) ([A-Z]\w*)\(", re.M)
ROUTE = re.compile(r"\b(GET|POST|PUT|PATCH|DELETE|OPTIONS) (/[^\s)?,;]*)")


def table():
    rows = set()
    for file in sorted(glob.glob(str(ROOT / "*.go"))):
        if file.endswith("_test.go"):
            continue
        source = pathlib.Path(file).read_text(encoding="utf-8")
        for match in METHOD.finditer(source):
            doc = " ".join(line[2:].strip() for line in match.group(1).splitlines())
            for method, path in ROUTE.findall(doc):
                rows.add((path, method, pathlib.Path(file).name, f"{match.group(2)}.{match.group(3)}"))
    lines = [HEADER, "|---|---|---|---|"]
    lines += [f"| `{m}` | `{p}` | `{f}` | `{fn}` |" for p, m, f, fn in sorted(rows)]
    return "\n".join(lines) + "\n"


def page_with(new_table):
    text = PAGE.read_text(encoding="utf-8")
    return text[: text.index(HEADER)] + new_table


if __name__ == "__main__":
    generated = table()
    if "--write" in sys.argv:
        PAGE.write_text(page_with(generated), encoding="utf-8")
    elif "--check" in sys.argv:
        if PAGE.read_text(encoding="utf-8") != page_with(generated):
            sys.exit("docs/book/src/reference/endpoints.md is stale: run scripts/endpoint-map.py --write")
    else:
        sys.stdout.write(generated)
