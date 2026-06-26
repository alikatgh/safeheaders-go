#!/usr/bin/env python3
"""Generate the "Source code" browser for SafeHeaders-Go 101.

For every real source file in the repo we emit a tiny stub page under
`101/lessons/src/`. The stub uses pymdownx.snippets (`--8<--`) to pull the
LIVE file content into a syntax-highlighted MkDocs page at build time — so the
course site lets you click a file citation and read the actual source in the
browser, with zero content duplication in git (the stubs are ~6 lines each).

Run from the repo root:   python3 101/gen_source_pages.py
Then build (also from repo root):  mkdocs build -f 101/mkdocs.yml --strict

It also prints the `nav:` block to paste under mkdocs.yml's "Source code"
section, and rewrites src/index.md. Re-run whenever the file list changes.
"""
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_DIR = os.path.join(REPO, "101", "lessons", "src")
GH = "https://github.com/alikatgh/safeheaders-go/blob/main"

# (Section title, [repo-relative paths]) — order = nav order.
SECTIONS = [
    ("Library modules", [
        "jsmn-go/jsmn.go", "jsmn-go/config.go", "jsmn-go/parallel.go",
        "cjson-go/cjson.go",
        "cgltf-go/cgltf.go",
        "tinyxml2-go/tinyxml2.go", "tinyxml2-go/config.go",
        "dr-wav-go/dr_wav.go",
        "stb-image-go/stb_image.go",
        "stb-truetype-go/sfnt.go", "stb-truetype-go/stb_truetype.go",
        "miniz-go/miniz.go",
        "linenoise-go/linenoise.go",
    ]),
    ("Examples", [
        "examples/production-usage/main.go",
        "examples/json-parser/main.go",
        "examples/jsmn-demo/main.go",
        "examples/linenoise-repl/main.go",
    ]),
    ("Build, CI & tooling", [
        ".github/workflows/go-ci.yaml",
        "Dockerfile",
        "Makefile",
    ]),
]


def slug(path):
    return re.sub(r"[^a-z0-9]+", "-", path.lower()).strip("-")


def lang(path):
    base = os.path.basename(path)
    if path.endswith(".go"):
        return "go"
    if path.endswith((".yaml", ".yml")):
        return "yaml"
    if base == "Dockerfile":
        return "dockerfile"
    if base == "Makefile":
        return "make"
    return "text"


def main():
    os.makedirs(SRC_DIR, exist_ok=True)
    missing = []
    nav_lines = ['  - "Source code":', "      - Browse the source: src/index.md"]
    index = [
        "# Source code — read the real files\n",
        "Every file below is the **live source from this repository**, pulled in at "
        "build time and syntax-highlighted. This is the same code the lessons cite — "
        "click any file to read it in the browser.\n",
    ]

    for title, paths in SECTIONS:
        nav_lines.append(f'      - "{title}":')
        index.append(f"\n## {title}\n")
        for rel in paths:
            abspath = os.path.join(REPO, rel)
            if not os.path.exists(abspath):
                missing.append(rel)
                continue
            n = sum(1 for _ in open(abspath, encoding="utf-8", errors="replace"))
            s = slug(rel)
            page = (
                "---\nhide:\n  - toc\n---\n\n"
                f"# `{rel}`\n\n"
                f"[:material-github: View on GitHub]({GH}/{rel}) "
                f"· {n} lines · live source, included at build time\n\n"
                f"```{lang(rel)}\n--8<-- \"{rel}\"\n```\n"
            )
            with open(os.path.join(SRC_DIR, f"{s}.md"), "w", encoding="utf-8") as fh:
                fh.write(page)
            nav_lines.append(f'          - "{rel}": src/{s}.md')
            # index.md lives inside src/, so its links are relative to src/.
            index.append(f"- [`{rel}`]({s}.md) — {n} lines")

    with open(os.path.join(SRC_DIR, "index.md"), "w", encoding="utf-8") as fh:
        fh.write("\n".join(index) + "\n")

    if missing:
        print("WARNING: skipped missing files:", ", ".join(missing), file=sys.stderr)
    print("\n".join(nav_lines))


if __name__ == "__main__":
    main()
