# SafeHeaders-Go 101 — from zero to a production-grade Go library

A complete, self-paced course — **26 lessons across 6 modules, plus a hands-on lab at the
end of every module and a capstone** — that takes someone who knows a little programming all
the way to building, securing, fuzzing, and shipping a real Go library. Every example is
drawn from the **actual code in this repository** (`safeheaders-go`): nine pure-Go ports of
popular C single-header libraries — JSON/XML/glTF parsers, a WAV decoder, a ZIP/DEFLATE codec,
an image loader, and a from-scratch **TrueType rasterizer**.

The labs aren't reading — you *write and run* code against the repo: build a bounds-checked
binary parser, make a worker pool deadlock and then fix it, add a decode-bomb guard, fuzz a
parser until it crashes and commit the seed. Each has guided steps, expected output, and a
full solution.

There are no toy examples. When we teach worker pools, we read the real `ParseBatch`. When we
teach a deadlock, it's an **actual bug this project shipped** — an under-sized results channel
that wedged the pool under cancellation — and the **real one-line fix** that a 10-agent audit
and `go test -race` caught. When we teach bounds-safety, it's the TrueType `glyf` decoder that
parses untrusted font files byte by byte. The security is real because the threats are real.

## Run it

The lessons are plain Markdown served with **[MkDocs](https://www.mkdocs.org/) +
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)** — light/dark themes,
search, and server-side syntax highlighting.

```bash
pip install -r 101/requirements.txt      # mkdocs-material
mkdocs serve -f 101/mkdocs.yml            # → http://localhost:8000
```

Use the sun/moon toggle in the header to switch **light/dark**. To build a static site for
hosting (GitHub Pages / Cloudflare Pages):

```bash
mkdocs build -f 101/mkdocs.yml           # → 101/site/
```

## The curriculum

| Module | Lessons | Focus |
|--------|---------|-------|
| — | Welcome | Orientation & the whole map |
| **1 · Go & the workspace** | 01–04 | Why pure-Go ports, modules/`go.work`, error wrapping, `io.Reader`/`Writer` |
| **2 · Parsing untrusted input** | 05–10 + Lab A | JSON tokenizer, stdlib wrappers, RIFF/WAV, and TrueType (tables → outlines → rasterizer) |
| **3 · Concurrency in Go** | 11–15 + Lab B | goroutines/channels, worker pools, `context` cancellation, a real deadlock, data races |
| **4 · Security & DoS resistance** | 16–20 + Lab C | threat model, OOM allocation, decode/decompression bombs, recursion limits, configurable caps |
| **5 · Testing, fuzzing & correctness** | 21–23 + Lab D | table tests/`-race`/coverage, `go test -fuzz`, round-trip & property tests |
| **6 · Tooling, CI & the audit story** | 24–26 + Capstone | `gofmt`/`vet`/golangci-lint, GitHub Actions/`govulncheck`/`gosec`, the 10-agent audit |

Work them **in order** — each builds on the last. Module 3 (concurrency) and Module 4
(security) are the heart: they teach the bug classes that actually shipped here and were
fixed.

## How it's structured

```
101/
├── mkdocs.yml              # site config: nav (6 modules), Material theme, code highlighting
├── requirements.txt        # mkdocs-material
└── lessons/                # docs_dir — the lessons (plain Markdown)
    ├── index.md            # Welcome (the homepage)
    ├── 01-…  …  26-….md    # 26 numbered lessons
    ├── lab-a-…  …  lab-d-….md
    ├── capstone-….md
    ├── glossary.md
    └── assets/extra.css    # restrained editorial overrides (hairlines, one accent, no shadows)
```

To add or edit a lesson: drop an `NN-title.md` in `lessons/`, add it to `nav:` in
`mkdocs.yml`, and `mkdocs serve` live-reloads.

## Design notes

- **Real code, always.** Every snippet cites its source file (and line where stable). Open
  the real file and read it next to the lesson.
- **Bugs are first-class.** Several lessons dissect a defect this repo actually had —
  deadlock, data race, OOM, decode bomb, billion-laughs — because the fix only makes sense
  once you've seen the break.
- **Three habits:** read the real file → run the "Try it" → explain it back.
