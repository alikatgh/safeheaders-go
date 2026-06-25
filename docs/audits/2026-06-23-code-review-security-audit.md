# SafeHeaders-Go — Code Review & Security Audit (2026-06-23)

## Executive summary

This audit covered all 9 production Go modules in the `safeheaders-go` workspace
plus the cross-cutting repository infrastructure (`go.work`, module `go.mod`s,
GitHub Actions workflows, `Makefile`, example programs, `SECURITY.md`, `README`,
and `Dockerfile`). Ten reviewers audited in parallel; every reported finding was
then re-checked by an independent verifier who attempted concrete reproduction
before recording a verdict.

**Overall posture: solid.** The memory-safety and bounds-checking discipline
across the library is genuinely strong. The binary parsers (`dr-wav-go`,
`stb-truetype-go`) bounds-check every offset/length derived from untrusted input
before slicing, the JSON/XML/glTF parsers delegate tokenizing to hardened stdlib
packages (`encoding/json`, `encoding/xml`) rather than hand-rolling offset math,
and recently-landed anti-OOM / divide-by-zero fixes are present and correct.
**No memory-corruption, slice-out-of-range, nil-deref, or integer-overflow bug
was found in any module.** The confirmed defects fall into two buckets: (a)
concurrency hazards in hand-rolled worker pools and shared global state, and (b)
DoS-shaped resource-exhaustion gaps (missing aggregate caps, unbounded recursion,
unsynchronized-size allocations) consistent with the repo's stated
untrusted-input threat model.

### Confirmed-issue counts by severity

| Severity | Count |
|----------|------:|
| Critical | 0 |
| High     | 5 |
| Medium   | 10 |
| Low      | 8 |
| Info     | 2 |
| **Total confirmed** | **25** |

Additionally: **1 uncertain** finding (latent API-misuse race, not reproducible
within the unit) and **0 verified false positives**. There were **no confirmed
critical issues.**

### Top risks (the five confirmed High findings)

1. **`jsmn-go` parallel parser deadlocks on context cancellation** — under-buffered
   results channel; any `WithCancel`/`WithTimeout` firing mid-parse on a large
   top-level stream wedges the worker pool permanently and leaks goroutines.
2. **`stb-image-go` `LoadBatchConcurrent` deadlocks** — same shape: `errs` channel
   sized for per-job failures only, overflows when cancellation sends stack on top
   of decode-failure sends; reproduced deterministically.
3. **`linenoise-go` global history API is a data race** — the package-level
   `defaultState` singleton backs all global `AddHistory`/`LoadHistory`/etc. with
   zero synchronization; concurrent use trips `-race` on the shared slice.
4. **`stb-truetype-go` composite-glyph billion-laughs DoS** — the depth-8 recursion
   cap does not bound fan-out, so a ~2–3 KB crafted font drives `K^8`-scale
   expansion; tiny input, astronomical work.
5. **`Dockerfile` omits a `go.work`-listed module** — `linenoise-go` is not copied,
   so the workspace fails to load and every tagged release's `publish-docker` job
   breaks. Build/release correctness, not a runtime security issue.

Four of the five are concurrency/DoS defects reachable from documented public
APIs with untrusted input; the fifth is a release-pipeline breakage. None permits
memory corruption or code execution.

---

## Confirmed findings

Ordered by severity (High → Info). Every finding below was independently
reproduced or code-verified by the adversarial verifier.

### High

#### H1 — `jsmn-go`: deadlock / goroutine leak on context cancellation during parallel parsing
- **Module:** jsmn-go
- **Location:** `jsmn-go/parallel.go:51-64, 124-139` (cancel-branch send at line 127; `wg.Wait()` at line 61)
- **Category:** concurrency
- **What/why:** `resultsCh` is buffered to capacity `numJobs` (== `numWorkers` on this path), and it is drained only *after* `wg.Wait()` + `close`. Each worker can send a job result (filling the buffer) and then, on `<-ctx.Done()`, send an **extra** `chunkResult{err}`. Worst-case sends (`numJobs` job results + `numWorkers` cancel results) exceed the buffer; the cancel-branch send blocks forever, the worker never calls `wg.Done()`, `wg.Wait()` blocks permanently → deadlock plus leaked goroutines retaining the input slice.
- **Trigger:** `ParseWithConfig(ctx, largeTopLevelStream, cfg)` where `ctx` is canceled *after* the early short-circuit (`config.go:166-170`) but *during* worker execution. Reproduced concretely: a watchdog test deadlocked on iteration 1 (NumCPU=11), `wg.Wait()` never returned. The existing cancellation tests cancel *before* the call and are absorbed by the early short-circuit, so this path was untested.
- **Suggested fix:** Drain `resultsCh` concurrently with the workers (range over it while a separate goroutine does `wg.Wait()` + `close`), or size the buffer to `numJobs + numWorkers`, or `select` on `ctx.Done()` when sending. Prefer the drain-concurrently pattern.

#### H2 — `stb-image-go`: deadlock / goroutine leak in `LoadBatchConcurrent` under cancellation + decode failures
- **Module:** stb-image-go
- **Location:** `stb-image-go/stb_image.go:106` (buffer), `:131` (per-job decode-failure send), `:120/:137` (per-worker cancel send), `:144` (`wg.Wait()`)
- **Category:** concurrency
- **What/why:** `errs` is buffered to exactly `len(datas)` (N), but total sends are bounded by N per-job decode failures **plus** W=`numWorkers` cancellation sends. Decode-failure sends do not return the worker (it loops on); cancellation sends do. When the context is canceled concurrently with a batch of images that fail to decode, sends exceed the N-capacity buffer. Nothing drains `errs` until after `wg.Wait()`, so a worker blocks permanently on `errs <-`, `wg.Wait()` never returns, and all workers leak.
- **Trigger:** `LoadBatchConcurrent(ctxCanceledSoon, [][]byte{bad, bad, ...})`. Reproduced deterministically: 64 invalid 4-byte images + a concurrently-canceled context hung within ~5–13 trials; goroutine dump showed main blocked at `:144` and workers in `[chan send]` at `:120` and `:131`.
- **Suggested fix:** Size the buffer for the true worst case (`make(chan error, len(datas)+numWorkers)`), or emit at most one error per worker, or drain `errs` concurrently. Cleanest: replace the hand-rolled pattern with `golang.org/x/sync/errgroup` + ctx.

#### H3 — `linenoise-go`: global history API has unsynchronized shared mutable state (data race)
- **Module:** linenoise-go
- **Location:** `linenoise-go/linenoise.go:581` (`defaultState`), reads/writes at `:523/:528/:563/:568/:576`, navigation reads at `:459/:469/:476/:483`
- **Category:** concurrency
- **What/why:** A single package-level `var defaultState = New(DefaultConfig())` backs every global convenience function (`AddHistory`, `LoadHistory`, `SaveHistory`, `ClearHistory`, `Set*`). These mutate `defaultState.history` and `config` with no mutex/RWMutex/atomic anywhere in the file. Two goroutines (e.g. one `AddHistory`, one `LoadHistory`/`ReadLine`) race on the slice header and elements; `append` reallocation concurrent with a reslice yields torn reads, lost writes, or a stale header pointing past a freed array.
- **Trigger:** Concurrent calls through the documented global API. Reproduced directly with `go test -race`: WARNING: DATA RACE at read `:523`, write `:528`, write `:563`. (Per-goroutine-confined `State` use *is* safe; the defect is the undocumented hazard of the global singleton API.)
- **Suggested fix:** Either document that the global functions / `defaultState` are not safe for concurrent use, or guard `State` with a `sync.Mutex` around every read/write of `history`/`config`, including the navigation reads inside `ReadLine`.

#### H4 — `stb-truetype-go`: composite-glyph exponential-expansion DoS (billion-laughs amplification)
- **Module:** stb-truetype-go
- **Location:** `stb-truetype-go/sfnt.go:293-317` (`glyphContours`, depth cap at 294), `:432-450` (`compositeContours` loop), `:494-511` (`appendComponent` recursion)
- **Category:** security
- **What/why:** Composite recursion is guarded only by `if depth > 8`. There is no cap on the number of components per glyph, no global total-points / total-components budget, and no visited-set for cycle detection. Each level fans out to an unbounded number of components, so total work multiplies: a glyph referencing K children, each referencing K children, ... 8 levels deep, produces ~`K^8` expansions. For K=50 that is ~2e15 invocations from a ~2–3 KB `glyf` region. `loca` can point many glyph IDs at the same composite region, so the payload is tiny while the work is astronomical. Existing caps do not protect: `numPts<=20000` is per-simple-glyph; the `w/h<=4096` rasterizer guard runs only *after* `glyphContours` returns.
- **Trigger:** Load an adversarial `.ttf` via `LoadFontFromBytes` and render any rune mapping to the crafted composite (`GetGlyph` → `rasterizeGlyph` → `glyphContours(gid,0)`). Public, unauthenticated. The DoS realizes as pure CPU/time exhaustion from call-tree traversal alone — it does not require non-empty leaves.
- **Suggested fix:** Thread a shared total-points (or total-components) counter through `glyphContours`/`compositeContours`/`appendComponent` and abort with an error past a sane cap (hundreds of thousands of points / a few thousand components per top-level glyph). Optionally cap components per glyph in the loop, and add a path visited-set to close self/mutual-reference cycles. The depth cap alone is provably insufficient when fan-out > 1.

#### H5 — infra: `Dockerfile` omits the `linenoise-go` module that `go.work` requires, breaking the Docker build and `publish-docker` release job
- **Module:** cross-cutting infra
- **Location:** `Dockerfile:13` (copies `go.work`), `:17-25` (module COPY block), `:28/:32-33` (`go mod download` / `go build`); `go.work:10` lists `./linenoise-go`; `.github/workflows/release.yaml:184-193` (`publish-docker`)
- **Category:** correctness
- **What/why:** The `Dockerfile` copies `go.work` (whose `use()` block lists `./linenoise-go`) but the COPY statements copy only 8 of the 9 modules, omitting `linenoise-go/`. With the workspace active, the Go toolchain fails to load the missing module. Reproduced twice in a scratch layout: both `go mod download` and the example `go build` fail with `go: cannot load module ../linenoise-go listed in go.work file: open ../linenoise-go/go.mod: no such file or directory`. The example `replace` directives do not help because workspace loading fails before module resolution. `make`/CI dodge this only via `GOWORK=off`; the `Dockerfile` sets no `GOWORK`. Net effect: every tagged release fails to publish the Docker image.
- **Trigger:** Any `docker build` against the repo context (e.g. the release pipeline).
- **Suggested fix:** Add `COPY linenoise-go/ linenoise-go/` alongside the other module copies, or set `ENV GOWORK=off` / pass `GOWORK=off` to the build commands so the workspace file is ignored during the image build. Verify the full COPY set matches `go.work`'s `use()` block exactly.

### Medium

#### M1 — `cgltf-go`: `ValidateGLTF` accepts negative reference indices (`node.Mesh < 0`)
- **Module:** cgltf-go
- **Location:** `cgltf-go/cgltf.go:153`
- **Category:** correctness
- **What/why:** The only reference check is `if node.Mesh >= len(gltf.Meshes) && node.Mesh != 0`. A negative index (e.g. `"mesh": -5`) makes `-5 >= len(Meshes)` false, so the invalid reference is accepted. The `&& node.Mesh != 0` clause additionally exempts `mesh: 0` even when zero meshes are defined, with no compensating guard like the scene check at `:143-149`. Nothing in this file dereferences `node.Mesh`, so there is no panic *within* `cgltf-go`; the risk is a downstream consumer trusting a `ValidateGLTF` pass and then indexing `Meshes[node.Mesh]`, which would panic on a negative/oversized value.
- **Trigger:** `ValidateGLTF` on a model with `node.Mesh = -5` (one mesh defined) returns `nil`; same for `node.Mesh = 0` with zero meshes. Reproduced concretely (both returned `err=nil`).
- **Suggested fix:** Range-check fully and stop special-casing zero: `if node.Mesh < 0 || node.Mesh >= len(gltf.Meshes) { return fmt.Errorf(...) }`. To preserve the legitimate "mesh omitted" case, change `Node.Mesh` (and other optional index fields) to `*int` so absent is `nil` rather than an ambiguous `0`.

#### M2 — `cjson-go`: `UnmarshalArrayParallel` sizes a buffered channel and slices directly from untrusted input length (memory-amplification DoS)
- **Module:** cjson-go
- **Location:** `cjson-go/cjson.go:100-109`
- **Category:** security
- **What/why:** `len(rawArray)` is attacker-controlled and used to eagerly size three structures with no cap: `results := make([]map[string]interface{}, len(rawArray))` (`:100`), `jobs := make(chan int, len(rawArray))` (`:102`), and the fill loop (`:107-109`) — plus the `[]json.RawMessage` slice itself. A ~10 MB body of `[0,0,0,...]` (~2 bytes/element) yields ~5M elements; per-element eager commitment (RawMessage header ~24B + map-pointer slot 8B + `jobs` int 8B ≈ 40B) gives roughly an order-of-magnitude small-bytes→committed-memory amplification before any work begins. No `MaxItems`/`MaxBytes` guard exists.
- **Trigger:** Feeding untrusted JSON arrays to this function. *(Verifier correction to the original report's figures: a Go map value is a single 8-byte pointer, so the "~120 MB map-header slice" estimate is imprecise — that magnitude better fits the `[]json.RawMessage` slice with 24B headers. The amplification, missing cap, and trigger are all real.)*
- **Suggested fix:** Reject inputs whose element count exceeds a configurable limit before processing, and/or bound input byte length before `json.Unmarshal`. Make `jobs` a fixed small buffer (e.g. `numWorkers`) fed by a producer goroutine so channel memory is O(workers), not O(items).

#### M3 — `jsmn-go`: `ParentIdx` not rebased during parallel chunk merge → corrupted parent pointers (diverges from serial parser)
- **Module:** jsmn-go
- **Location:** `jsmn-go/parallel.go:156-159` (`processChunk` rebases only Start/End), `:182-186` (`mergeChunkResults` concatenation)
- **Category:** correctness
- **What/why:** `Token.ParentIdx` is an index into the token array (set from `p.toksuper`; dereferenced at `jsmn.go:78`). `processChunk` rebases only `Start` and `End` by `j.offset`, leaving `ParentIdx` chunk-local. `mergeChunkResults` then concatenates chunk slices in order without shifting `ParentIdx` by the running count of prior tokens, so every nested token in chunk N>0 points into chunk-local index space and references the wrong token after merge. The parallel path produces a structurally broken token graph diverging from the serial parser.
- **Trigger:** Reproduced: diffing `ParseParallel` vs serial `Parse` over a 2000-element top-level stream, 10914/14000 tokens had mismatched `ParentIdx`; first divergence at the start of chunk 2 (serial `1267` vs parallel `0`). The existing guard compares only Type/Start/End, never `ParentIdx`.
- **Suggested fix:** During merge, add the running base index (sum of lengths of previously-appended chunks) to each token's `ParentIdx` where `ParentIdx != -1`. Add a test asserting `ParseParallel` output equals serial `Parse` including `ParentIdx`.

#### M4 — `linenoise-go`: `LoadHistory` silently fails on any history line > 64 KB and destroys existing history
- **Module:** linenoise-go
- **Location:** `linenoise-go/linenoise.go:563-571`
- **Category:** correctness
- **What/why:** `LoadHistory` clears in-memory history first (`s.history = s.history[:0]`, `:563`) and then reads with a default `bufio.NewScanner(f)` (`:564`), whose token cap is `bufio.MaxScanTokenSize` (64 KB). Any line > 64 KB (a pasted command, base64 blob, or corrupted file) makes `scanner.Scan()` return false and `scanner.Err()` return `bufio.ErrTooLong`. The load fails **and** the previously-loaded in-memory history has already been wiped, so the user loses both the file's contents and prior history.
- **Trigger:** Reproduced: a 70 KB line returned `bufio.Scanner: token too long`, and the two pre-existing in-memory entries were destroyed.
- **Suggested fix:** Use `bufio.Reader` + `ReadString('\n')` (as `readLineNoTTY` already does), or call `scanner.Buffer(...)` with an explicit documented cap and skip over-long lines gracefully. Defer the `history[:0]` clear until after a successful read, or load into a temp slice and swap on success so a failed load does not destroy existing history.

#### M5 — `miniz-go`: `ExtractArchive` decompression-bomb guard is per-entry only; aggregate output is unbounded
- **Module:** miniz-go
- **Location:** `miniz-go/miniz.go:101-120` (loop), `:74-88` (`readAllLimited`), `:70` (`MaxDecompressedSize`)
- **Category:** security
- **What/why:** `readAllLimited` caps a single stream via `io.LimitReader(r, MaxDecompressedSize+1)` per call. `ExtractArchive` calls it once per `r.File` entry and appends every payload into the returned slice with no running total and no entry-count cap. The stdlib zip reader bounds only the `r.File` *preallocation* by input size; actual entries are appended by reading real central-directory headers, so entry count is bounded by `input_size/46`, not by an untrusted count field.
- **Trigger:** Reproduced: an 8-entry archive of 9,134 bytes on disk, per-stream cap set to 2 MiB, produced 8 MiB of retained output (4× the per-stream cap) with no error. At ~99 bytes/entry, a ~990 KB archive holds ~10,000 real entries; with the default 256 MiB cap each inflating near the limit, aggregate retained memory ≈ 2.5 TB from a sub-megabyte input.
- **Suggested fix:** Track a running total across all entries and fail once cumulative output exceeds `MaxDecompressedSize` (or a separate `MaxTotalDecompressedSize`). Optionally cap `len(r.File)`. Pass the remaining budget into `readAllLimited` so each entry's `LimitReader` shrinks as bytes are produced.

#### M6 — `stb-image-go`: `LoadStream` has no decode-bomb / pixel-count guard (unbounded allocation)
- **Module:** stb-image-go
- **Location:** `stb-image-go/stb_image.go:167-174` (`image.Decode(r)` at `:169`)
- **Category:** resource-leak
- **What/why:** `Load()` enforces `MaxImagePixels` via `checkPixelLimit()` before decoding, but `LoadStream()` calls `image.Decode(r)` directly with no dimension check and no `io.LimitReader`. A small malicious stream declaring enormous dimensions drives the stdlib decoder to allocate gigabytes — defeating the decode-bomb protection the package doc and README explicitly advertise. The streaming API is the natural path for large/untrusted inputs, so leaving it unguarded is the higher-risk surface.
- **Trigger:** Reproduced: with `MaxImagePixels=1Mpx`, `Load` rejected a real 2000×2000 (4Mpx) PNG while `LoadStream` decoded it fully.
- **Suggested fix:** Wrap the reader so the header can be peeked (`bufio.Reader` + `image.DecodeConfig` on a peeked prefix to enforce `MaxImagePixels` before `image.Decode`), or impose an `io.LimitReader` byte cap. At minimum, document loudly that `LoadStream` bypasses the `MaxImagePixels` guard.

#### M7 — `tinyxml2-go`: exported `Parse` recurses with no depth cap → unrecoverable fatal stack-overflow DoS
- **Module:** tinyxml2-go
- **Location:** `tinyxml2-go/tinyxml2.go:184-228` (`parseElement`, recursion at `:205`), reached from `Parse` at `:29-64` (call at `:54`); contrast `parseElementLimited` guard at `:126`
- **Category:** security
- **What/why:** `parseElement` recurses once per nested element with no nesting-depth limit, unlike `parseElementLimited`. The public `Parse()` uses `parseElement`, so adversarial `<a><a>...</a></a>` nested ~5M deep (~35 MB, under the default 100 MB size guard) drives the goroutine stack past the 1 GB limit and triggers `fatal error: stack overflow`. The crash is a runtime fatal error that `recover()` **cannot** catch — a server cannot survive it.
- **Trigger:** Reproduced in a re-exec'd subprocess: `Parse` on ~3M-deep XML died with `runtime: goroutine stack exceeds 1000000000-byte limit`; a wrapping `recover()` did not catch it (exit 2). `ParseWithConfig(DefaultConfig())` correctly rejects the same input with `ErrNestingTooDeep`. Severity is Medium (not High) because a safe path ships; the danger is that `Parse` is the most obvious entry point and its only protection is a doc comment.
- **Suggested fix:** Have `Parse` enforce a hard safety cap even when nominally unlimited — delegate to `ParseWithConfig` with a `MaxNestingDepth` backstop, or add an absolute internal depth ceiling in `parseElement` that returns an error instead of crashing.

#### M8 — infra: data race on `requestCount` in the production-usage example
- **Module:** cross-cutting infra
- **Location:** `examples/production-usage/main.go:110` (decl), `:114` (`requestCount++` in `rateLimit` middleware), `:217` (read in `handleStats`)
- **Category:** concurrency
- **What/why:** `requestCount` is a plain package-level `int64` incremented non-atomically inside middleware that wraps every handler (runs per inbound request goroutine) and read without synchronization in `handleStats`. `net/http` serves each request in its own goroutine, so this is an unsynchronized read/write race — undefined behavior in Go, flaggable by `go test -race`. The file is explicitly a "production" reference, so it is also a copy-worthy bad pattern.
- **Trigger:** Concurrent requests under load.
- **Suggested fix:** Use `atomic.AddInt64(&requestCount, 1)` and `atomic.LoadInt64(&requestCount)` (or an `atomic.Int64`).

#### M9 — infra: `handleParse` reads the body with a single `Read` into a fixed 10 MB buffer (truncation + per-request allocation DoS)
- **Module:** cross-cutting infra
- **Location:** `examples/production-usage/main.go:140-146`
- **Category:** correctness
- **What/why:** Two issues. (1) Correctness: `body := make([]byte, MaxRequestSize)`, `n, err := r.Body.Read(body)` (single `Read`), then `body = body[:n]`. `io.Reader.Read` is not guaranteed to fill the buffer; any body arriving in more than one TCP segment (larger JSON, slow/chunked clients) is silently truncated, so valid requests fail to parse. (2) Resource: the full 10 MB buffer is allocated unconditionally for every request; combined with the no-op rate limiter, N concurrent tiny requests each pin 10 MB. `MaxBytesReader` (`:104`) caps bytes read but does not prevent the upfront allocation.
- **Trigger:** Reproduced: a multi-segment reader returned only the first segment (1 of 29 bytes) from a single `Read`, while `io.ReadAll` returned the full body.
- **Suggested fix:** Replace lines 140-146 with `body, err := io.ReadAll(r.Body)` (the existing `MaxBytesReader` enforces the 10 MB cap and errors past it), then handle `err`. Optionally reject by `Content-Length` first.

#### M10 — infra: gosec GitHub Action pinned to `@master` in a workflow with `security-events: write`
- **Module:** cross-cutting infra
- **Location:** `.github/workflows/go-ci.yaml:153` (`uses: securego/gosec@master`); permissions at `:15-16`
- **Category:** security
- **What/why:** A third-party action is pinned to a moving branch ref rather than a tag or SHA, in a workflow granting `pull-requests: write` and `security-events: write`. A compromised or breaking commit to gosec's `master` flows directly into CI with those write scopes. Every other action in the file is pinned to a major-version tag, so gosec is the outlier.
- **Trigger:** Any CI run after an upstream `master` change.
- **Suggested fix:** Pin to a released tag (`securego/gosec@v2.x.y`) or, best practice, a full commit SHA with a version comment, and let Dependabot (already configured for `github-actions`) bump it.

### Low

#### L1 — `cgltf-go`: `ValidateGLTF` does not validate accessor/bufferView/buffer/primitive index references
- **Module:** cgltf-go
- **Location:** `cgltf-go/cgltf.go:129` (function body `:129-159`)
- **Category:** correctness
- **What/why:** Despite its name, `ValidateGLTF` checks only the default-scene index and (incompletely) `node.Mesh`. It never range-checks `Primitive.Attributes/Indices/Material`, `Accessor.BufferView`, `BufferView.Buffer`, `Scene.Nodes`, or `Node.Children`. An adversarial glTF with `bufferView.buffer = 9999`, `accessor.bufferView = -1`, or `scene.nodes = [9999]` passes validation. These are plain `int`s never dereferenced in this file (no crash within `cgltf-go`); the risk is callers treating a `ValidateGLTF` pass as a referential-integrity guarantee it does not provide.
- **Trigger:** Reproduced: a GLTF with every reference out of range (`scene.nodes=[9999]`, `node.children=[9999]`, `POSITION=9999`, `indices=-1`, `material=9999`, `accessor.bufferView=-1`, `bufferView.buffer=9999`) returned `nil`.
- **Suggested fix:** Rename/document the function as a minimal sanity check, or extend it to range-check every index field (including negatives) against its target slice. Add fuzz/round-trip tests asserting out-of-range references are rejected. *(Shares a root cause with M1 but covers a distinct, broader set of unchecked references.)*

#### L2 — `cjson-go`: `UnmarshalStream` decodes from an untrusted `io.Reader` with no size limit
- **Module:** cjson-go
- **Location:** `cjson-go/cjson.go:64-69`
- **Category:** security
- **What/why:** `UnmarshalStream` wraps `json.NewDecoder(r).Decode(v)` directly on a caller-supplied reader with no `io.LimitReader`. The README frames this entry point as for "memory efficiency" over untrusted input, so callers reasonably assume a guard exists. The realistic risk is bounded — `json.NewDecoder` streams incrementally and does not buffer the whole input, so the exposure is a single very large token/value plus the fully-decoded `v` held in memory, not the entire stream. Callers can wrap their own `io.LimitReader`, so this is a hardening/documentation gap rather than a hard bug.
- **Trigger:** A malicious/unbounded reader (e.g. an HTTP body) producing one gigantic value.
- **Suggested fix:** Accept a max-bytes parameter (or a documented variant) and wrap `r` in `io.LimitReader(r, maxBytes)` before the decoder; at minimum document that callers MUST pre-limit the reader.

#### L3 — `linenoise-go`: completion state not reset after editing; next Tab discards user edits
- **Module:** linenoise-go
- **Location:** `linenoise-go/linenoise.go:336-341, 438-453`
- **Category:** correctness
- **What/why:** After the first Tab, `completionActive` is set true (`:443`). `handleEditKey`/`insertChar` modify `s.buf` but never reset `completionActive`. On the next Tab, `handleCompletion` takes the else branch and cycles the **stale** completions list computed from the pre-edit buffer, then overwrites the user's edited buffer at `:452`, silently discarding everything typed since the first Tab. This diverges from antirez/linenoise, which exits completion mode on the first non-completion keypress.
- **Trigger:** Reproduced: callback returns `[foobar, foobaz]`; Tab on `foo` → `foobar`; edit to `foobarX`; next Tab overwrote it with stale `foobaz`.
- **Suggested fix:** Set `completionActive = false` at the start of `insertChar`/`handleEditKey` for any key other than Tab (and on cursor moves).

#### L4 — `linenoise-go`: cursor repositioning emits CUF with argument 0 (off-by-one on empty prompt + pos)
- **Module:** linenoise-go
- **Location:** `linenoise-go/linenoise.go:426-428`
- **Category:** correctness
- **What/why:** `refresh()` computes `targetCol = promptLen + s.pos` and always emits `\r\x1b[%dC` (CUF). When `targetCol == 0` (empty prompt, pos 0), this produces `ESC[0C`. Per ECMA-48, a CUF parameter of 0 is treated as 1 by most terminals, so the cursor lands one column right of where it should be. The preceding `\r` already homes the cursor, so the sequence is both redundant and an off-by-one for this edge case. Cosmetic only — no safety impact.
- **Trigger:** Reproduced: empty prompt + pos 0 emitted `\rESC[K\rESC[0C`.
- **Suggested fix:** `if targetCol > 0 { fmt.Fprintf(out, "\r\x1b[%dC", targetCol) } else { fmt.Fprint(out, "\r") }`.

#### L5 — `tinyxml2-go`: `UnlimitedConfig` disables the nesting-depth guard, re-exposing the fatal stack overflow
- **Module:** tinyxml2-go
- **Location:** `tinyxml2-go/config.go:60-66` (`MaxNestingDepth:0`); guard bypassed at `tinyxml2.go:126`
- **Category:** security
- **What/why:** `UnlimitedConfig()` sets `MaxNestingDepth:0`, and the depth check is gated on `config.MaxNestingDepth > 0`, so `ParseWithConfig(data, UnlimitedConfig())` takes the unbounded recursion path and hits the same unrecoverable `fatal error: stack overflow` as `Parse` (M7). This is by-design — the constructor is named/documented "use with caution." The substantive note: an "unlimited" setting yields a process-killing fatal error (strictly worse than the OOM/CPU a user might expect), and `recover()` cannot catch it.
- **Trigger:** `ParseWithConfig(deeplyNested, UnlimitedConfig())`.
- **Suggested fix:** Give even `UnlimitedConfig` a high-but-finite `MaxNestingDepth` backstop, or enforce an absolute internal ceiling in `parseElementLimited` that no config can disable. At minimum, expand the doc to state depth=0 risks a non-recoverable crash.

#### L6 — `tinyxml2-go`: `FindDeep`/`FindAllDeep` recurse to tree depth with no cap
- **Module:** tinyxml2-go
- **Location:** `tinyxml2-go/tinyxml2.go:258-271` (`FindDeep`, recursion at `:266`), `:274-286` (`FindAllDeep`, recursion at `:283`)
- **Category:** security
- **What/why:** Both recurse once per DOM level with no depth budget, so on a sufficiently deep tree they can themselves overflow the stack and fatally crash the process, independent of how the tree was parsed. Reachability nuance (acknowledged): a tree parsed via `Parse`/`UnlimitedConfig` overflows at *parse* time before these helpers run, so their independent exposure requires a hand-constructed deep tree or input crafted to survive parse yet overflow traversal. With a sane `MaxNestingDepth`, traversal is bounded and safe.
- **Trigger:** Reproduced: `FindDeep` on a hand-built linear tree did not crash at 3M depth but did at 30M depth (`fatal error: stack overflow`).
- **Suggested fix:** Primarily addressed by capping parse depth (M7). For defense-in-depth, convert `FindDeep`/`FindAllDeep` to explicit-stack (iterative) traversal or carry a depth budget.

#### L7 — infra: CI/Makefile install security tools from `@latest` (govulncheck, gosec)
- **Module:** cross-cutting infra
- **Location:** `.github/workflows/go-ci.yaml:166` (`govulncheck@latest`); `Makefile:144-145` (`gosec@latest`, `govulncheck@latest`)
- **Category:** security
- **What/why:** These resolve a moving target at run time, making CI non-reproducible and pulling whatever upstream publishes. `golangci-lint` is pinned to `v2.2.2`, so the `@latest` tools are a genuine inconsistency and minor supply-chain risk.
- **Trigger:** Any CI run after an upstream release of these tools.
- **Suggested fix:** Pin to explicit versions (e.g. `govulncheck@v1.x.y`, `gosec@v2.x.y`) the way `golangci-lint` is pinned, and route updates through Dependabot/review.

#### L8 — infra: `curl | sh` install of golangci-lint fetches an unverified script from a raw URL
- **Module:** cross-cutting infra
- **Location:** `.github/workflows/go-ci.yaml:120` (duplicated in `release.yaml:42`)
- **Category:** security
- **What/why:** `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v2.2.2/install.sh | sh ...` pipes a remotely-fetched script straight into a shell with no checksum/signature verification. The URL path is pinned to the immutable `v2.2.2` git tag, which substantially mitigates the realistic risk (a tag cannot be silently moved on GitHub), so this is a hardening recommendation rather than an active vulnerability.
- **Trigger:** Theoretical — tampering with GitHub raw serving or the tag.
- **Suggested fix:** Prefer the official `golangci/golangci-lint-action` (pinned by tag/SHA), or download the script, verify a known SHA256, then execute. At minimum keep the immutable tag pin (already done).

### Info

#### I1 — `dr-wav-go`: `Parse` accepts a truncated trailing data chunk without surfacing the shortfall (by design)
- **Module:** dr-wav-go
- **Location:** `dr-wav-go/dr_wav.go:134-142` (anti-OOM design documented at `:117-120`)
- **Category:** correctness
- **What/why:** In `readDataChunk`, when the `data` subchunk declares a size larger than the bytes actually present, `allocSize` is clamped to `r.Len()` and `Parse` returns the truncated payload rather than reporting incompleteness. This is the deliberate anti-OOM design (covered by `TestParseOversizedDataChunkIsCapped`) and is the correct security trade-off: it eliminates the declared-size over-allocation/OOM vector. The only consequence is that a caller cannot distinguish a legitimately small file from an over-declared truncated one via the error channel. No memory-safety, panic, or DoS impact. **Listed for record only — no change required.** (Minor precision note: because `allocSize` is clamped to exactly `r.Len()`, `io.ReadFull` always fills the buffer and the `err != io.EOF` branch at `:139` is effectively unreachable in the over-declared case.)
- **Suggested fix:** Optional, non-security: if strict integrity is ever desired, record whether `subchunkSize` exceeded `r.Len()` and expose it (e.g. a `Truncated` bool or a distinct sentinel error). Keep the allocation cap exactly as-is regardless.

#### I2 — `cjson-go`: only the first worker error is surfaced; reported item index is nondeterministic
- **Module:** cjson-go
- **Location:** `cjson-go/cjson.go:129-134`
- **Category:** quality
- **What/why:** After `wg.Wait()`, a single `<-errs` reads at most one buffered error; the rest are discarded when the channel is GC'd. This is not a leak or deadlock (each worker sends at most one error, `errs` is buffered to `numWorkers`, so no send blocks; a drained closed channel yields `nil`). But when multiple items are malformed, the "item N" index in the returned error is nondeterministic under concurrency, which can confuse callers diagnosing which element failed.
- **Trigger:** A batch with multiple malformed items.
- **Suggested fix:** Document that the returned error is the first-observed (nondeterministic) failure, or use `errors.Join` / context cancellation for deterministic reporting.

---

## Uncertain findings (noteworthy, not confirmed)

#### U1 — `miniz-go`: mutable package-global `MaxDecompressedSize` read by decompressors without synchronization
- **Module:** miniz-go
- **Location:** `miniz-go/miniz.go:70` (decl), read at `:76/:84` (`readAllLimited`) and `:360/:367` (`DecompressStream`)
- **Category:** concurrency · **Verifier verdict:** uncertain (severity low)
- **Why uncertain:** The technical premise is correct — an unsynchronized exported `int64` read during decompression while another goroutine writes it would be a data race flagged by `-race`. **But the verifier could not trigger it within the unit:** the package never mutates this var during decompression (it is read-only at runtime inside the library); the only writers are tests, which set it from a single goroutine before calling. The race materializes only if an external caller mutates the global mid-decompression — API-misuse hardening, not a defect in this unit's own code paths. The original finding itself rated confidence "low." Left as uncertain rather than confirmed because no concrete in-unit reproduction exists.
- **Suggested fix (defensive):** Document that `MaxDecompressedSize` must only be set before any concurrent decompression begins, or back it with `atomic.Int64` + `Set/Get` accessors so runtime adjustment is race-free.

---

## Verified false positives

**None.** Every finding submitted by the ten reviewers was either confirmed by the
independent verifier (25 findings) or downgraded to *uncertain* for lack of an
in-unit reproduction (1 finding, U1). No finding was overturned as incorrect.

The verifier also explicitly *cleared* several adjacent code paths while
reproducing the confirmed findings, which raises confidence in coverage:
- `cgltf-go`: the default-scene index check (`cgltf.go:143-149`) is correct and was not flagged.
- `cjson-go` / `dr-wav-go` / `miniz-go` / `tinyxml2-go`: the concurrency in `UnmarshalArrayParallel`, `ParseBatch`, `CreateArchiveConcurrent`, and `TraverseConcurrent` was checked for send-on-closed, double-close, WaitGroup misuse, and goroutine leaks and found **sound** on the happy path (the confirmed concurrency defects are specifically the cancellation/under-buffering edges, not the steady state).
- `dr-wav-go`: the anti-OOM data-chunk cap, all divide-by-zero guards, and the `ExtractChannels` deinterleave bounds were verified safe; a 15s/~1.8M-exec fuzz run found no new crashes.
- `stb-truetype-go`: all four cmap-format readers, simple-glyph parsing, the rasterizer coverage indexing, and the LRU cache locking were verified bounds-/race-safe; the composite-glyph fan-out (H4) is the sole defect.

---

## Per-module posture (one line each)

| Module | Posture |
|--------|---------|
| **cgltf-go** | Memory-safe; `ValidateGLTF` under-validates references (accepts negative/out-of-range indices) — correctness gaps, no crash within the unit. |
| **cjson-go** | Thin, mostly-safe stdlib wrapper; the parallel path has an untrusted-length allocation amplification (M2) and an unlimited stream reader (L2). |
| **dr-wav-go** | Genuinely solid — anti-OOM cap, divide-by-zero guards, and deinterleave bounds all correct; clean under fuzz; one by-design info note. |
| **jsmn-go** | Serial tokenizer bounds-safe; the parallel path has a cancellation deadlock (H1) and corrupted parent pointers after merge (M3). |
| **linenoise-go** | Input-editing core bounds-safe; the global history API is a data race (H3) and `LoadHistory` destroys history on >64 KB lines (M4). |
| **miniz-go** | Largely solid (leans on stdlib); the only real DoS is the per-entry-only decompression-bomb cap (M5); one uncertain global-var race (U1). |
| **stb-image-go** | Memory-safe via stdlib; `LoadBatchConcurrent` deadlocks under cancellation (H2) and `LoadStream` bypasses the decode-bomb guard (M6). |
| **stb-truetype-go** | Strong bounds/cache discipline throughout; the sole defect is the composite-glyph exponential-expansion DoS (H4). |
| **tinyxml2-go** | Guarded `ParseWithConfig` path robust; the unguarded public `Parse` (and `UnlimitedConfig`) allow an unrecoverable stack-overflow DoS (M7/L5/L6). |
| **infra (CI/build/examples)** | CI mostly solid; broken release `Dockerfile` (H5), two example bugs (M8/M9), and unpinned/mutable action refs (M10/L7/L8). |

---

*Methodology: 10 parallel reviewers, each scoped to one module/surface, followed by
an independent adversarial verifier that attempted concrete reproduction of every
reported finding before recording a verdict. Only verifier-confirmed findings appear
in "Confirmed findings"; reproduction details are noted per finding.*

---

## Remediation status (2026-06-23)

**All 25 confirmed findings have been fixed**, plus U1 and I2 documented; each fix
ships with a regression test (watchdogs for the deadlocks, a `-race` test for the
history race, a fan-out bomb for H4, an aggregate-bomb test for M5, a depth-ceiling
test for M7, etc.). I1 is by-design (no change). L8 was already mitigated by the
immutable tag pin. After remediation every module is gofmt/vet/lint-clean, passes
`-race`, and stays above the 70% coverage gate.

| Finding | Fix commit |
|---------|-----------|
| H1, M3 (jsmn deadlock + ParentIdx) | `a5ee995` |
| H2, M6 (stb-image deadlock + LoadStream) | `18beee5` |
| H3, M4, L3, L4 (linenoise race + 3 bugs) | `01620eb` |
| H4 (stb-truetype composite bomb) | `8f1096a` |
| H5, M8, M9, M10, L7 (infra) | `13d68c9` |
| M1, L1 (cgltf validation) | `9bc583b` |
| M2, L2, I2 (cjson) | `4cc9e90` |
| M5, U1 (miniz aggregate bomb) | `5a77896` |
| M7, L5, L6 (tinyxml2 recursion) | `15e9803` |
