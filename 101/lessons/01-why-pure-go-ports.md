# 01 - Why pure-Go ports of C libraries

> **Objectives:** Understand why reimplementing C single-header libraries in pure Go
> buys you memory safety, zero-CGO deployment, and built-in concurrency — and what
> you give up. Survey all nine SafeHeaders-Go modules so the rest of the course has
> a map to navigate.
> Estimated time: 15 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Single-header C library** = "a complete tool packed into one envelope — handy to carry, but if the blade slips, it cuts your hand." A parser or codec living entirely in one `.h` file is convenient to drop into a project, but it runs inside C's memory model where a single bad index causes a buffer overflow.
- **Pure Go** = "the same tool re-forged in a material that blunts itself before it can cut you." The reimplementation is written entirely in Go using only the standard library — no `import "C"`, no platform compiler flags, no shared object to bundle — so `go build` produces a self-contained static binary on any platform.
- **Memory safety by default** = "a guardrail bolted to every floor of the building, not a warning sign at the bottom of the stairs." Go's runtime bounds-checks every slice access and manages heap lifetimes through garbage collection, so the out-of-bounds reads, writes, and use-after-free bugs that generate the most C-parser CVEs cannot exist in this code by construction.
- **No CGO** = "removing the bridge between two cities so you never have to worry about it collapsing mid-crossing." CGO bridges Go and C at runtime; dropping it means cross-compilation (`GOOS=linux GOARCH=arm64` from a Mac) works without a C toolchain, static linking stays truly static, and the C memory model is never reintroduced at the boundary.
- **Concurrency built in** = "the building was designed with multiple staircases — adding a new path is fitting a door, not demolishing a wall." Go's goroutines and channels are first-class language features, so a parallel parsing path like `ParseParallel` is a natural extension, not an afterthought bolted on with pthreads.

**Why it matters:** the libraries this project replaces are correct and fast, but
they are one malformed input away from a process crash in production. SafeHeaders-Go
gives you the same parsing capability with a safety net you do not have to build
yourself.

**See it — C library via CGO vs. pure-Go port: what changes and what stays.**

<svg viewBox="0 0 700 310" role="img" aria-labelledby="l01-t l01-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="l01-t">C single-header library via CGO versus pure-Go port</title>
  <desc id="l01-d">Two columns showing the CGO path on the left (Go service calls CGO bridge, CGO bridge calls C library, risks include segfault, no cross-compile, C memory model) and the pure-Go path on the right (Go service calls Go module directly, gains include memory safety, static binary, cross-compile anywhere), with a dividing line and labels in between.</desc>
  <defs>
    <marker id="l01-arr" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L0,7 L8,3.5 Z" fill="var(--md-default-fg-color--light)"/>
    </marker>
    <marker id="l01-arr-accent" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L0,7 L8,3.5 Z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
  </defs>

  <!-- Column headers -->
  <text x="175" y="26" text-anchor="middle" font-size="13" font-weight="600" fill="currentColor">CGO path</text>
  <text x="525" y="26" text-anchor="middle" font-size="13" font-weight="600" fill="var(--md-accent-fg-color,#00897b)">Pure-Go port</text>

  <!-- Divider -->
  <line x1="350" y1="35" x2="350" y2="295" stroke="var(--md-default-fg-color--lightest)" stroke-width="1" stroke-dasharray="4 3"/>

  <!-- LEFT SIDE: CGO path -->
  <!-- Go service box -->
  <rect x="60" y="44" width="140" height="40" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="130" y="69" text-anchor="middle" font-size="12" fill="currentColor">Go service</text>

  <!-- Arrow down to CGO bridge -->
  <line x1="130" y1="84" x2="130" y2="116" stroke="var(--md-default-fg-color--light)" stroke-width="1.4" marker-end="url(#l01-arr)"/>

  <!-- CGO bridge box -->
  <rect x="60" y="118" width="140" height="40" rx="6" fill="none" stroke="#e5484d" stroke-width="1.4"/>
  <text x="130" y="143" text-anchor="middle" font-size="12" fill="#e5484d">CGO bridge</text>

  <!-- Arrow down to C library -->
  <line x1="130" y1="158" x2="130" y2="190" stroke="var(--md-default-fg-color--light)" stroke-width="1.4" marker-end="url(#l01-arr)"/>

  <!-- C library box -->
  <rect x="60" y="192" width="140" height="40" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="130" y="217" text-anchor="middle" font-size="12" fill="currentColor">C library (.h)</text>

  <!-- Risk labels -->
  <text x="130" y="254" text-anchor="middle" font-size="10" fill="#e5484d">segfault kills process</text>
  <text x="130" y="269" text-anchor="middle" font-size="10" fill="#e5484d">no cross-compile</text>
  <text x="130" y="284" text-anchor="middle" font-size="10" fill="#e5484d">C memory model at boundary</text>

  <!-- RIGHT SIDE: Pure-Go path -->
  <!-- Go service box -->
  <rect x="500" y="44" width="140" height="40" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="570" y="69" text-anchor="middle" font-size="12" fill="currentColor">Go service</text>

  <!-- Arrow down to Go module (accent colour) -->
  <line x1="570" y1="84" x2="570" y2="153" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l01-arr-accent)"/>

  <!-- Go module box (accent border) -->
  <rect x="500" y="155" width="140" height="40" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="570" y="180" text-anchor="middle" font-size="12" fill="var(--md-accent-fg-color,#00897b)">Go module (pure)</text>

  <!-- Gain labels -->
  <text x="570" y="218" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">memory-safe by construction</text>
  <text x="570" y="233" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">static binary, go build anywhere</text>
  <text x="570" y="248" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">race detector &amp; fuzzer see all code</text>
</svg>

---

## The problem with C single-header libraries in Go services

Most Go HTTP services eventually need to parse something: JSON, XML, images, audio,
fonts, archives. The Go standard library covers the common cases, but specialized
formats often have only a C implementation available. The typical solution is CGO:

```go
// The CGO route — you don't want this in production
// #include "jsmn.h"
// import "C"
```

CGO works, but it comes with costs that compound in production:

- Cross-compilation breaks. You need the C toolchain for the target platform.
- Crash isolation is gone. A segfault in the C code kills the Go process.
- Race detectors and fuzz engines cannot see inside C memory.
- Docker images grow: you need a C runtime in the base image.
- Static binaries become pseudo-static: the C library may still dynamic-link.

SafeHeaders-Go's answer: rewrite the interesting parts in Go, accept that you will
not match hand-tuned SIMD throughput on every benchmark, and gain everything above
in return.

---

## The nine modules at a glance

The project is a `go.work` workspace — nine independent Go modules under one repo.
You can use any one of them without pulling in the others.

```bash
# From README.md — install only what you need
go get github.com/alikatgh/safeheaders-go/jsmn-go
go get github.com/alikatgh/safeheaders-go/stb-image-go
```

Here is what each module does and which C library it replaces:

| Module | Replaces | What it parses / produces |
|---|---|---|
| `jsmn-go` | jsmn (Zaitsev) | JSON tokenizer — fast, allocation-light |
| `cjson-go` | cJSON (Gamble) | JSON marshal / unmarshal with parallel arrays |
| `tinyxml2-go` | tinyxml2 (Thomason) | XML DOM — elements, attributes, traversal |
| `cgltf-go` | cgltf (Kuhlmann) | glTF 2.0 — 3-D model assets, parallel batch |
| `dr-wav-go` | dr_wav (Reid) | RIFF/PCM WAV audio — binary chunk parser |
| `stb-image-go` | stb_image (Barrett) | PNG / JPEG / GIF decode, batch concurrent |
| `stb-truetype-go` | stb_truetype (Barrett) | TrueType glyph rasteriser, LRU cache |
| `miniz-go` | miniz | ZIP / DEFLATE compress and extract |
| `linenoise-go` | linenoise (Sanfilippo) | CLI line editing with history |

All are marked **Stable** in `README.md`; eight are at `v0.5.0`, with `linenoise-go` at `v0.1.0`:

> Status: all 9 modules are production-ready. Every module is lint-clean,
> race-tested, fuzzed where it parses untrusted input, and above the 70%
> coverage gate.

---

## What "production-ready" actually required

The initial ports were straightforward Go translations. Making them production-ready
meant finding and fixing a class of bugs that only show up under adversarial inputs
or concurrent load. A 10-agent security audit (`docs/audits/2026-06-23-code-review-security-audit.md`)
turned up 25 issues (0 critical, 5 high); all 25 are fixed in the current codebase.

Three representative fixes show what the hardening involved:

**Memory exhaustion — dr-wav-go.** The original port read a size field from the file
header and called `make([]byte, size)`. A malformed WAV could declare `size = 2 GB`
and crash the process before reading a single sample. The fix, in [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md),
caps every allocation to `r.Len()` — the bytes actually present in the reader — so
the claimed size cannot exceed reality.

**Decompression bomb — miniz-go.** ZIP archives can be crafted so that a tiny
compressed file expands to gigabytes (the classic "zip bomb"). [`miniz-go/miniz.go`](src/miniz-go-miniz-go.md)
enforces `MaxDecompressedSize` (256 MiB by default) as an aggregate budget across
all entries in an archive, not just per-stream. A single stream limit is easy to
circumvent with many small entries; the aggregate budget is not.

**Billion-laughs / stack overflow — tinyxml2-go.** XML entity expansion or deeply
nested elements can make a recursive descent parser overflow the goroutine stack.
Go's `recover()` cannot catch a stack overflow — the process simply dies. The fix in
[`tinyxml2-go/tinyxml2.go`](src/tinyxml2-go-tinyxml2-go.md) is a hard ceiling at `maxNestingDepth = 10000` in
`parseElement`, checked before each recursive call, so the depth is bounded
before the stack is exhausted.

These are all documented in `SECURITY.md` under "Built-in DoS Protections":

```
| Module        | Protection                                          | Knob (default)                     |
|---------------|-----------------------------------------------------|-------------------------------------|
| jsmn-go       | Max input size + max token count                    | Config.MaxInputSize (100 MB),       |
|               |                                                     | MaxTokens (1,000,000)               |
| tinyxml2-go   | Max input size, node count, nesting depth           | ParseWithConfig / Config            |
| dr-wav-go     | Allocation capped to bytes present                  | always on                           |
| stb-image-go  | Decode-bomb guard (pixel cap before decode)         | MaxImagePixels (64 MP; 0 disables)  |
| miniz-go      | Decompression-bomb guard (aggregate cap)            | MaxDecompressedSize (256 MiB)       |
```

---

## The zero-dependency constraint

`SECURITY.md` states:

> SafeHeaders-Go has zero external dependencies (pure stdlib). This minimizes
> supply chain attack surface.

This is a deliberate design constraint. Every `go.mod` in the workspace lists only
the standard library. There is no `github.com/some-vendor/something` to audit, pin,
or worry about in a supply-chain scan. The tradeoff is that anything not in the
standard library must be written from scratch — which is exactly what
`stb-truetype-go` does: a full TrueType rasteriser including contour flattening,
scanline crossing, and span accumulation, implemented in [`sfnt.go`](src/stb-truetype-go-sfnt-go.md) without any
external font library.

---

## How the quality bar is maintained

The CI pipeline ([`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md)) runs on every commit:

```yaml
# Abbreviated from go-ci.yaml
jobs:
  test:      # go test ./... for every module
  lint:      # golangci-lint v2 (.golangci.yml, 50+ linters)
  security:  # gosec + govulncheck
  fuzz:      # go test -fuzz — runs weekly on a schedule
  build:     # matrix: linux / macOS / Windows
```

The 70% coverage gate is enforced in CI. The race detector is mandatory:

```bash
# README.md — Development Setup
go test -race ./...
```

Fuzzing found the two dr-wav crashes (OOM from a malformed size field). The
regression seeds are committed under `testdata/fuzz/` so those inputs are re-run on
every CI pass, not just during dedicated fuzz sessions.

---

!!! note "Try it"
    Clone the repo and run the full test suite across all nine modules:

    ```bash
    cd /path/to/safeheaders-go
    go test ./...
    ```

    Expected outcome: all tests pass with no output (Go's test runner is silent on
    success). Add `-v` to see individual test names. Then add the race detector:

    ```bash
    go test -race ./...
    ```

    Expected outcome: same passing result. If any test fails with `-race` but passes
    without it, that is a data race — a real concurrency bug. The linenoise-go history
    race (fixed in [`linenoise-go/linenoise.go`](src/linenoise-go-linenoise-go.md) with `sync.Mutex`) was caught exactly
    this way.

---

!!! warning "What you give up"
    Pure Go does not reach the throughput of SIMD-optimised C for single-threaded
    workloads. The README is explicit about this:

    > Because throughput depends heavily on CPU count, input shape, and allocator
    > behavior, this README intentionally does not quote fixed numbers — measure on
    > your target hardware.

    The parallel APIs (`ParseParallel`, `LoadBatchConcurrent`, `ParseBatch`) close
    much of the gap for large inputs. For small inputs the goroutine overhead is not
    worth paying: `jsmn-go` always parses serially below 4 KB.

---

!!! tip "Scope of this course"
    Each subsequent lesson dives into one concrete topic: a specific bug, a safety
    mechanism, a concurrency pattern, or a fuzz-testing workflow — always grounded in
    the actual source files. By the end you will be able to read the implementation,
    extend it, and apply the same hardening patterns to your own parsers.

    Lessons you will reach soon:
    - The deadlock bug and its fix in [`jsmn-go/parallel.go`](src/jsmn-go-parallel-go.md) and [`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md)
    - The data race in `linenoise-go/linenoise.go` and how `-race` catches it
    - Fuzz testing: how `go test -fuzz` found the dr-wav OOM

---

## Key takeaways

- Pure-Go ports eliminate an entire class of memory-safety bugs that the C versions
  can never be free of, at the cost of some single-threaded throughput.
- Dropping CGO makes cross-compilation trivial and keeps the binary fully static.
- Zero external dependencies means the supply-chain attack surface is the Go
  standard library — nothing more.
- "Production-ready" is not a marketing claim here: it required finding and fixing
  25 audit findings, adding fuzz regression seeds, enforcing a 70% coverage gate,
  and running the race detector on every CI build.
- The nine modules are independent. Use only what your project needs; the workspace
  structure (`go.work`) means they co-exist without forcing you to pull them all in.
