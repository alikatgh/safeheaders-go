# SafeHeaders-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/alikatgh/safeheaders-go/jsmn-go)](https://goreportcard.com/report/github.com/alikatgh/safeheaders-go/jsmn-go)
[![Tests](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml/badge.svg)](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml)
[![Coverage](https://codecov.io/gh/alikatgh/safeheaders-go/branch/main/graph/badge.svg)](https://codecov.io/gh/alikatgh/safeheaders-go)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![GoDoc](https://pkg.go.dev/badge/github.com/alikatgh/safeheaders-go)](https://pkg.go.dev/github.com/alikatgh/safeheaders-go)

**This is the project I built to learn Go** — by porting nine popular single-header
C libraries (JSON/XML/glTF parsers, a WAV decoder, a ZIP/DEFLATE codec, an image
loader, a from-scratch TrueType rasterizer, a line editor) to pure Go, then testing,
fuzzing, race-detecting, and security-auditing them until they held up.

Everything I learned along the way is distilled into **[SafeHeaders-Go 101](./101)** —
a complete, self-paced course built from this repo's real code and real bugs.

## The 101 course

**26 lessons across 6 modules, plus a hands-on lab per module and a capstone.** It takes
someone who knows a little programming through building, securing, fuzzing, and shipping
a real Go library.

There are no toy examples — every snippet cites an actual file in this repo. When the
course teaches worker pools, you read the real `ParseBatch`. When it teaches deadlocks,
it's an actual bug this project shipped (an under-sized results channel that wedged the
pool under cancellation) and the real one-line fix. When it teaches bounds-safety, it's
the TrueType `glyf` decoder parsing untrusted font files byte by byte.

| Module | Lessons | Focus |
|--------|---------|-------|
| — | Welcome | Orientation & the whole map |
| **1 · Go & the workspace** | 01–04 | Why pure-Go ports, modules/`go.work`, error wrapping, `io.Reader`/`Writer` |
| **2 · Parsing untrusted input** | 05–10 + Lab A | JSON tokenizer, stdlib wrappers, RIFF/WAV, and TrueType (tables → outlines → rasterizer) |
| **3 · Concurrency in Go** | 11–15 + Lab B | goroutines/channels, worker pools, `context` cancellation, a real deadlock, data races |
| **4 · Security & DoS resistance** | 16–20 + Lab C | threat model, OOM allocation, decode/decompression bombs, recursion limits, configurable caps |
| **5 · Testing, fuzzing & correctness** | 21–23 + Lab D | table tests/`-race`/coverage, `go test -fuzz`, round-trip & property tests |
| **6 · Tooling, CI & the audit story** | 24–26 + Capstone | `gofmt`/`vet`/golangci-lint, GitHub Actions/`govulncheck`/`gosec`, the 10-agent audit |

The labs aren't reading — you write and run code against the repo: build a bounds-checked
binary parser, make a worker pool deadlock and fix it, add a decode-bomb guard, fuzz a
parser until it crashes and commit the seed.

### Run the course locally

The lessons are plain Markdown served with [MkDocs](https://www.mkdocs.org/) +
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/):

```bash
pip install -r 101/requirements.txt       # mkdocs-material
mkdocs serve -f 101/mkdocs.yml            # → http://localhost:8000
```

See [101/README.md](./101/README.md) for the full curriculum and structure.

## The libraries

Nine pure-Go, zero-CGO ports of well-known single-header C libraries. Each module is
lint-clean, race-tested, fuzzed where it parses untrusted input, and above the 70%
coverage gate enforced in CI. Coverage figures are measured `go test -cover` totals.

| Module | Version | Coverage | Description |
|--------|---------|----------|-------------|
| [cgltf-go](./cgltf-go) | v0.5.0 | 93% | glTF 3D model loading with parallel assets |
| [tinyxml2-go](./tinyxml2-go) | v0.5.0 | 89% | XML DOM parsing with element traversal |
| [stb-image-go](./stb-image-go) | v0.5.0 | 89% | Image loading with batch decoding (PNG, JPEG, GIF) |
| [jsmn-go](./jsmn-go) | v0.5.0 | 88% | Fast JSON tokenizer with parallel parsing |
| [cjson-go](./cjson-go) | v0.5.0 | 83% | JSON marshaling/unmarshaling with parallel processing |
| [dr-wav-go](./dr-wav-go) | v0.5.0 | 82% | WAV audio (RIFF/PCM) parsing with concurrent decoding |
| [stb-truetype-go](./stb-truetype-go) | v0.5.0 | 81% | TrueType glyph rasterization (glyf outlines, anti-aliased) + LRU cache |
| [miniz-go](./miniz-go) | v0.5.0 | 79% | ZIP compression with concurrent chunking |
| [linenoise-go](./linenoise-go) | v0.1.0 | 77% | Minimal line editing library for CLI apps |

The theme across all of them: **memory safety** (bounds-checked, no `unsafe`),
**concurrency** (worker pools with `context` cancellation), and **DoS resistance**
(configurable size/token/recursion limits, decode- and decompression-bomb guards on
by default).

## Quick start

```bash
go get github.com/alikatgh/safeheaders-go/jsmn-go
```

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
    json := []byte(`{"name": "SafeHeaders-Go", "version": "0.5.0", "stable": true}`)

    // Serial parsing (small inputs)
    p := jsmngo.NewParser(100)
    count, err := p.Parse(json)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed %d tokens\n", count)

    // Parallel parsing (large inputs)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    tokens, err := jsmngo.ParseParallel(ctx, json)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed %d tokens in parallel\n", len(tokens))
}
```

More in [`examples/`](./examples): a production-style HTTP service, a JSON parser with
validation, a linenoise REPL, and others — see [examples/README.md](./examples/README.md).

## Performance notes

Each parser/codec module ships benchmarks — run them on your own hardware:

```bash
make bench                       # jsmn-go, stb-image-go, stb-truetype-go
cd jsmn-go && go test -bench=. -benchmem -run='^$' ./...
```

Where the concurrency helps (and where it doesn't):

- `jsmn-go`'s `ParseParallel` splits input at **top-level delimiters** and tokenizes
  chunks concurrently. It pays off on large streams of many top-level values; a single
  large object has no split points and transparently falls back to serial. Inputs below
  4 KB always parse serially — the goroutine overhead isn't worth it.
- `stb-image-go`, `cgltf-go`, `dr-wav-go`, `miniz-go` parallelize across **independent
  items** (a batch of images / assets / files), so speedup scales with item count, not
  the size of any single input.

This README intentionally quotes no fixed numbers — throughput depends on CPU count,
input shape, and allocator behavior. Measure on your target hardware.

## What building this taught me

The interesting parts of this repo are the scars, and the course is built around them:

- **A real deadlock** — a worker pool with an under-sized results channel that wedged
  under cancellation, caught by a multi-agent audit and `go test -race`
  ([lesson 14](./101/lessons/14-the-deadlock-bug.md)).
- **Decode and decompression bombs** — tiny inputs that expand to gigabytes, and the
  guards now on by default ([lesson 18](./101/lessons/18-decode-and-decompression-bombs.md)).
- **Billion-laughs / recursion blowups** in the XML parser, and configurable depth
  limits ([lesson 19](./101/lessons/19-recursion-and-billion-laughs.md)).
- **Fuzzing that actually found crashes** — jsmn, tinyxml2, dr-wav, and miniz are
  fuzzed in CI, with crash seeds committed ([lesson 22](./101/lessons/22-fuzzing.md)).
- **The audit story** — what a 10-agent code audit found and what it cost
  ([lesson 26](./101/lessons/26-the-audit-story.md)).

CI runs the full ladder on every commit: tests with `-race`, a 70% coverage gate,
golangci-lint, `gosec` and `govulncheck`, scheduled fuzzing, and multi-OS builds
(Linux, macOS, Windows).

## Documentation

- 📚 [**101/**](./101) — the full course (start here)
- 📖 [**CONTRIBUTING.md**](./CONTRIBUTING.md) — coding standards, architecture, porting guidelines
- 🔒 [**SECURITY.md**](./SECURITY.md) — threat model, limits, vulnerability reporting
- 🐛 [**ISSUES.md**](./ISSUES.md) — known issues and improvement tracker
- 📝 [**CHANGELOG.md**](./CHANGELOG.md) — version history
- 📦 Module READMEs — each module directory has installation, API reference, benchmarks, and known limitations

## Contributing

Contributions are welcome — bug reports, tests, docs, performance work, or a whole new
port. If you want to port another single-header C library, some candidates:

- [ ] [stb_vorbis.h](https://github.com/nothings/stb/blob/master/stb_vorbis.c) — Ogg Vorbis decoder
- [ ] [tinyobjloader.h](https://github.com/tinyobjloader/tinyobjloader) — OBJ 3D model loader
- [ ] [stb_perlin.h](https://github.com/nothings/stb/blob/master/stb_perlin.h) — Perlin noise
- [ ] [utf8.h](https://github.com/sheredom/utf8.h) — UTF-8 utilities

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines and the
[capstone lesson](./101/lessons/capstone-world-class-module.md) for what a finished
module looks like.

```bash
git clone https://github.com/alikatgh/safeheaders-go.git
cd safeheaders-go
go test -race ./...
golangci-lint run --config .golangci.yml
```

## Security

Please **do not** open a public issue for security vulnerabilities — email
[safeheaders@aulenor.com](mailto:safeheaders@aulenor.com) or use
[GitHub Security Advisories](https://github.com/alikatgh/safeheaders-go/security/advisories)
instead. See [SECURITY.md](./SECURITY.md) for the threat model, configurable limits,
and DoS-prevention guidance.

## License

MIT — see [LICENSE](./LICENSE). Original C libraries retain their respective licenses
(MIT, BSD, Public Domain, zlib).

SafeHeaders-Go reimplements these excellent C libraries — thank you to their authors:

- [jsmn](https://github.com/zserge/jsmn) by Serge Zaitsev (MIT)
- [stb](https://github.com/nothings/stb) by Sean Barrett (Public Domain / MIT)
- [cJSON](https://github.com/DaveGamble/cJSON) by Dave Gamble (MIT)
- [tinyxml2](https://github.com/leethomason/tinyxml2) by Lee Thomason (zlib)
- [cgltf](https://github.com/jkuhlmann/cgltf) by Johannes Kuhlmann (MIT)
- [dr_wav](https://github.com/mackron/dr_libs) by David Reid (Public Domain)
- [linenoise](https://github.com/antirez/linenoise) by Salvatore Sanfilippo (BSD)
