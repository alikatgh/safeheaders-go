# Glossary

A reference for terms used throughout the SafeHeaders-Go 101 course. Definitions
are grouped by theme. If a term appears in a lesson, the lesson link is included.

---

## Go concurrency

**goroutine**
A lightweight thread managed by the Go runtime, not the OS. You start one with
`go f()`. The runtime multiplexes thousands of goroutines onto a small pool of OS
threads. Goroutines are cheap to create (a few kilobytes of stack) but can
deadlock if they wait on a channel or mutex that never unblocks.
See [Lesson 11](11-goroutines-channels-select.md).

**channel**
A typed pipe for passing values between goroutines. A send (`ch <- v`) blocks
until a receiver is ready; a receive (`v := <-ch`) blocks until a sender arrives.
Buffered channels (`make(chan T, N)`) allow up to N sends before blocking.
The size of the buffer matters: sizing it too small is a common cause of deadlocks.
See [Lesson 11](11-goroutines-channels-select.md) and [Lesson 14](14-the-deadlock-bug.md).

**select**
A Go statement that waits on multiple channel operations at once and proceeds with
whichever is ready first. If several are ready simultaneously, one is chosen at
random. A `default` branch makes it non-blocking. Used extensively in worker pools
to multiplex work and cancellation signals.

**worker pool**
A pattern where a fixed number of goroutines (workers) consume jobs from a shared
channel. Bounds the total number of goroutines so one big batch cannot exhaust
memory. Used in `jsmn-go` (`chunkWorker`), `stb-image-go` (`LoadBatchConcurrent`),
`cjson-go` (`UnmarshalArrayParallel`), and others.
See [Lesson 12](12-worker-pools.md).

**fan-out**
Dispatching one piece of work to many goroutines simultaneously, then collecting
results. Useful when items are independent (e.g. parsing ten separate image files).
Wasteful when items share the same expensive context, because every goroutine
re-reads it from scratch.

**deadlock**
A situation where every goroutine in a group is blocked waiting for another member
of the same group to proceed — so none ever does. The Go runtime detects total
deadlocks (all goroutines blocked) and panics with `all goroutines are asleep`.
Partial deadlocks (one goroutine stuck, rest running) hang silently.
See [Lesson 14](14-the-deadlock-bug.md).

**data race**
Two goroutines reading and writing the same memory location concurrently, with no
synchronization. The result is undefined: you may get stale data, torn values, or
a crash. Detected at runtime by the Go race detector (`go test -race`).
See [Lesson 15](15-data-races-and-mutexes.md).

**mutex (`sync.Mutex`)**
A mutual-exclusion lock. Only one goroutine can hold a mutex at a time; others
block on `Lock()` until it is released with `Unlock()`. Used in `linenoise-go`
to protect the shared history slice from concurrent `AddHistory`/`LoadHistory`
calls.
See [Lesson 15](15-data-races-and-mutexes.md).

**context (`context.Context`)**
A value that carries a deadline, cancellation signal, and optional key-value bag
through a call chain. Passing a context to a long-running function lets the caller
cancel it early (e.g. on HTTP request teardown or timeout). Worker pools in this
repo check `ctx.Done()` to abort in-flight jobs.
See [Lesson 13](13-context-cancellation.md).

---

## Error handling

**sentinel error**
A package-level `var` of type `error` whose identity, not just its message, is
meaningful. Callers check it with `errors.Is(err, ErrFoo)`. In `jsmn-go` the
sentinels are `ErrInputTooLarge`, `ErrTooManyTokens`, and `ErrEmptyInput`; callers
can branch on each without string-matching.
See [Lesson 03](03-errors-and-wrapping.md).

**error wrapping (`%w`)**
Using `fmt.Errorf("context: %w", err)` to attach a new message to an existing
error while preserving the original for `errors.Is`/`errors.As` inspection. The
wrapped chain can be unwrapped programmatically, so callers at any layer can still
detect a sentinel.
See [Lesson 03](03-errors-and-wrapping.md).

---

## Security and robustness

**decode bomb**
A small input that expands into a very large in-memory structure after decoding.
Classic example: a tiny image file that claims dimensions of 100,000 × 100,000
pixels, causing the decoder to allocate 40 GB for the pixel buffer. `stb-image-go`
guards against this by calling `image.DecodeConfig` to read dimensions before
allocating, then comparing against `MaxImagePixels`.
See [Lesson 18](18-decode-and-decompression-bombs.md).

**decompression bomb**
The compression-flavored decode bomb: a small zip or deflate stream that expands
to gigabytes of output (e.g. a file of repeated zeros compresses to near nothing).
`miniz-go` guards with `MaxDecompressedSize` — an aggregate budget across all
entries in a zip archive, not just per-stream.
See [Lesson 18](18-decode-and-decompression-bombs.md).

**billion-laughs attack**
An XML (or similar) structure where each entity references several others
exponentially: entity A contains B B B, B contains C C C, … After ten levels the
expansion is billions of nodes. `tinyxml2-go` defends with a hard nesting depth
ceiling (`maxNestingDepth = 10000`) enforced in `parseElement`.
`stb-truetype-go` faces the same shape in composite glyphs (a glyph that references
other glyphs recursively); `glyphBudget` caps the total number of components and
contour points.
See [Lesson 19](19-recursion-and-billion-laughs.md).

**OOM (out-of-memory)**
Allocating more memory than the host has available, causing the process to be
killed by the OS or the Go runtime to panic. In `dr-wav-go` an early version
allocated a `[]byte` whose size came directly from an untrusted header field;
fuzzing found inputs that set that field to 4 GB. The fix: cap the allocation
to the number of bytes actually present (`r.Len()`).
See [Lesson 17](17-unbounded-allocation-oom.md).

**fuzzing (`go test -fuzz`)**
An automated testing technique that generates mutated inputs and feeds them to a
function, hunting for panics, crashes, or unexpected errors. Go's built-in fuzzer
(available since Go 1.18) records any crash-inducing input as a seed file under
`testdata/fuzz/`. This repo runs fuzz tests on `jsmn-go`, `tinyxml2-go`,
`dr-wav-go`, and `miniz-go` weekly in CI.

**round-trip test**
A test that encodes data, then decodes the result, and checks that the final value
equals the original. It catches subtle codec bugs that do not cause a crash but
silently corrupt data. In `miniz-go`, a round-trip test caught that
`CreateArchiveConcurrent` was double-compressing: it stored already-deflated bytes
inside a Deflate entry, making the archive unreadable by standard tools.

**configurable limits**
Security-relevant ceilings (max input bytes, max tokens, max nesting depth, max
decompressed bytes) exposed as fields on a `Config` struct so callers can tighten
or loosen them for their use case. All SafeHeaders-Go modules ship a
`DefaultConfig` (production-safe), a `StrictConfig` (tighter), and an
`UnlimitedConfig` (for trusted offline use).
See [Lesson 20](20-configurable-limits.md).

---

## Parsing concepts

**tokenizer**
A function that reads raw bytes and splits them into a flat list of tokens — typed
spans with start/end offsets — without building a tree or resolving values. `jsmn`
is a tokenizer: it finds where each JSON string, number, object, or array begins
and ends, but leaves the value bytes in place for the caller to interpret.
See [Lesson 05](05-tokenizing-json-jsmn.md).

**DOM (Document Object Model)**
A tree representation of a structured document (XML, HTML) held entirely in
memory. `tinyxml2-go` builds a DOM: after `Parse` returns you can call
`FindDeep`/`FindAllDeep` to walk the element tree. Contrast with streaming/event
parsers that emit nodes one at a time and never hold the whole tree.

**streaming parser**
A parser that reads input incrementally and emits events or values one at a time,
keeping only a small window of data in memory at once. `cjson-go` exposes
`UnmarshalStream` and `MarshalStream` for this purpose. Contrast with DOM parsers
that load everything first.

**RIFF (Resource Interchange File Format)**
A generic container format used by WAV audio files. The file is divided into
labeled chunks (`RIFF`, `fmt `, `data`, …), each with a 4-byte tag and a 4-byte
little-endian size. `dr-wav-go` parses RIFF/PCM by hand in `dr_wav.go`, reading
chunk headers and dispatching to handlers like `readDataChunk`.
See [Lesson 07](07-binary-parsing-riff-wav.md).

**PCM (Pulse-Code Modulation)**
The raw, uncompressed representation of audio as a sequence of integer samples.
WAV files store PCM data in their `data` chunk. `dr-wav-go` exposes the samples
as a `[]byte` slice after parsing.

**glTF (GL Transmission Format)**
A JSON-based 3D asset format used in games and web graphics. `cgltf-go` parses
glTF 2.0 via `encoding/json` and then validates cross-references (scene→node,
node→mesh, etc.) in `ValidateGLTF`.

---

## TrueType / font internals

**SFNT**
The binary container format shared by TrueType and OpenType fonts. A font file
begins with an offset table listing named tables; `stb-truetype-go` reads it in
`parseSFNT` (in `sfnt.go`).
See [Lesson 08](08-truetype-1-sfnt-tables.md).

**glyf table**
The TrueType table that stores the actual outline geometry for every glyph as
a sequence of contour points. `stb-truetype-go` reads it in `glyphContours`
(in `sfnt.go`).
See [Lesson 09](09-truetype-2-glyph-outlines.md).

**cmap table**
The TrueType "character map" table that translates Unicode code points to glyph
IDs. `stb-truetype-go` supports cmap formats 0, 4, 6, and 12 (the most common
encodings), parsed in `cmapFormat0`, `cmapFormat4`, `cmapFormat6`, and
`cmapFormat12` in `sfnt.go`.
See [Lesson 08](08-truetype-1-sfnt-tables.md).

**Bézier curve (quadratic)**
A smooth curve defined by a start point, one off-curve control point, and an end
point. TrueType outlines are made of quadratic Bézier segments. `stb-truetype-go`
flattens them into line segments in `flattenQuad` (in `sfnt.go`) before
rasterizing.
See [Lesson 09](09-truetype-2-glyph-outlines.md).

**scanline fill**
A rasterization algorithm that fills a polygon by iterating over each horizontal
row of pixels (scanline) and finding where the outline edges cross it. The pixels
between pairs of crossings are inside the shape. `stb-truetype-go` implements this
in `scanlineCrossings` and `accumulateSpans` in `sfnt.go`.
See [Lesson 10](10-truetype-3-rasterizing.md).

**nonzero winding rule**
A rule for deciding whether a point is inside a filled shape. For each pixel,
trace a ray in any direction and count edge crossings: clockwise edges add +1,
counter-clockwise edges add -1. If the sum is nonzero, the pixel is inside.
TrueType uses this rule; `stb-truetype-go`'s `fillCoverage` follows it.
See [Lesson 10](10-truetype-3-rasterizing.md).

**LRU cache (Least Recently Used)**
A fixed-capacity cache that evicts the entry that was accessed longest ago when
the cache is full. `stb-truetype-go` uses a `GlyphCache` LRU in
`stb_truetype.go` so rendered glyphs are not rasterized twice. Without it, a
line of text with repeated characters would re-rasterize each glyph on every
occurrence.

**composite glyph**
A TrueType glyph built from references to other glyphs (e.g. an accented letter
composed of a base letter plus a diacritic). References can be nested. Without a
budget cap the fan-out is exponential (a composite of composites of composites …).
`glyphBudget` in `sfnt.go` limits the total component count and point count to
prevent this.
See [Lesson 19](19-recursion-and-billion-laughs.md).

---

## Tooling and CI

**golangci-lint**
A meta-linter that runs many Go linters (staticcheck, errcheck, govet, gosec, and
50+ others) in one pass. This repo uses golangci-lint v2 with configuration in
`.golangci.yml`. CI blocks merges if any linter finding is reported.

**gosec**
A Go linter focused on security patterns: hardcoded credentials, unsafe integer
conversions, unhandled errors on security-sensitive calls, and more. Runs as part
of golangci-lint and as a standalone CI step.

**govulncheck**
An official Go tool that cross-references your module's dependencies against the
Go vulnerability database. It reports only vulnerabilities reachable from your
code, not every transitive CVE. Runs in CI on every PR.

**go.work (workspace)**
A file at the root of a multi-module repository that lists the local modules so
`go build` and `go test` can resolve them without publishing. SafeHeaders-Go uses
a `go.work` file to span all nine modules; you can run `go test ./...` from the
root and test all of them.
See [Lesson 02](02-modules-and-workspaces.md).

**race detector (`-race`)**
A flag passed to `go test`, `go build`, or `go run` that instruments memory
accesses at compile time. At runtime, if two goroutines touch the same location
without synchronization, the detector prints a detailed report and exits nonzero.
It has roughly 2–20× overhead, so it is used in CI but not in production binaries.
See [Lesson 15](15-data-races-and-mutexes.md).

**coverage gate**
A CI check that fails if test coverage drops below a threshold. This repo enforces
a 70% minimum per module. Coverage is measured with `go test -coverprofile` and
reported to Codecov.
See [Lesson 21](21-table-tests-race-coverage.md).

**CGO**
The mechanism that lets Go code call C functions and vice versa. It requires a C
compiler, breaks pure cross-compilation, and reintroduces C's memory safety
hazards at the boundary. SafeHeaders-Go deliberately avoids CGO: all nine modules
are pure Go.
See [Lesson 01](01-why-pure-go-ports.md).

**`io.Reader` / `io.Writer`**
The two fundamental interfaces in Go's I/O model. Any type with a
`Read([]byte) (int, error)` method satisfies `io.Reader`; any with
`Write([]byte) (int, error)` satisfies `io.Writer`. Functions that accept these
interfaces work with files, network connections, in-memory buffers, and anything
else — including the length-limited readers used to cap decompressed output in
`miniz-go`.
See [Lesson 04](04-io-reader-writer.md).
