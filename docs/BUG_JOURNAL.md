# Bug Journal

## Patterns to scan for FIRST

- **Multi-error context flattening**: When collecting errors from goroutines into a slice and formatting as a string, context sentinel errors (context.Canceled, context.DeadlineExceeded) lose their identity. Always check errors.Is() before formatting. Applies to any function that fans out work into goroutines and collects errors.
- **Fuzz test token bounds check on error**: When a parser fails partway through, partial tokens with End=-1 remain accessible via Tokens(). Fuzz tests that validate token bounds must skip the check when Parse() returns an error, or the test will report false positives.
- **Size-gated parallel code path in tests**: If parallel processing only activates above a byte threshold (e.g. >4096 bytes), test data must exceed that threshold or the parallel path is never exercised and coverage is misleadingly low.
- **OO API fuzz test referencing wrong API**: Auto-generated fuzz tests can reference an OO-style API (NewDocument/doc.Parse) when the actual package exposes a functional API (Parse(data) (*Doc, error)). Always compile-check fuzz tests before merging.
- **Unbounded recursive parser = stack-overflow DoS**: Recursive descent parsers (XML elements, JSON values) that recurse once per nesting level will exhaust the stack on adversarial deeply-nested input. Guard with a MaxNestingDepth limit checked at the top of the recursive function, plus MaxInputSize and a shared MaxNodeCount counter. Follow the jsmn-go Config pattern (DefaultConfig/StrictConfig/UnlimitedConfig + sentinel errors via errors.Is); add limits as a NEW ParseWithConfig entrypoint so the existing Parse signature stays back-compatible. Critically: an "unlimited" (0) config value must still hit an internal absolute ceiling — a fatal stack overflow bypasses `recover()` entirely, so disabling the configurable limit must never disable the hard backstop too.
- **Shared depth/count ceiling checked with inconsistent base indexing**: if two entry points recurse toward the same hard-coded limit constant but start counting depth from different bases (root=0 in one, root=1 in the other), the "same" ceiling actually triggers one level apart in each path — a silent divergence between paths documented as sharing one invariant. When multiple code paths enforce a shared safety limit, run a boundary test (`limit-1`, `limit`, `limit+1`) through EVERY entry point, not just one.
- **`make([]byte, untrustedSize)` from a header field = OOM DoS**: Any binary parser that reads a length/size field from input and allocates that many bytes (WAV data chunk, image dimensions, ZIP entry size) can be driven to allocate gigabytes by a tiny malicious file. Cap the allocation to the bytes actually remaining in the reader (`if n > r.Len() { n = r.Len() }`) before `make`, then `io.ReadFull`.
- **Image decode bomb**: `image.Decode` allocates `width*height*bytesPerPixel` from header-declared dimensions, so a tiny file claiming 50000x50000 forces a multi-GB allocation. Read `image.DecodeConfig` first (header only) and reject images over a pixel cap before the full decode. Guard the `w*h` multiply against int overflow (`int64(w)*int64(h)`).
- **Decompression bomb**: `io.ReadAll(flate.NewReader(...))` (or per-ZIP-entry `io.ReadAll`) is unbounded — a few KB of compressed zeros inflate to gigabytes (~1000x). Wrap the reader in `io.LimitReader(r, max+1)` and error if the result exceeds `max`. Random fuzzing won't find this (it can't synthesize a valid high-ratio stream); craft a `Compress(make([]byte, N))` test instead.
- **Pre-compressed data written into a recompressing container = double compression**: Writing an already-`flate`-compressed stream into an `archive/zip` entry with `Method: zip.Deflate` deflates it AGAIN, so the archive does not round-trip (extraction yields the intermediate stream). To keep pre-compression, use `zip.Writer.CreateRaw` with a `FileHeader` carrying `Method`, `CRC32` (of the *uncompressed* data), `CompressedSize64`, `UncompressedSize64`. ALWAYS add a round-trip test (`Create → Extract → bytes.Equal`), not just a "no error / non-empty" assertion.
- **Config schema-version mismatch silently disables all settings**: A `.golangci.yml` declaring `version: "2"` while using v1-style `linters-settings:` is *invalid* — `golangci-lint run` tolerates it but ignores every setting (thresholds, ignore-sigs, etc.), so the linter runs on defaults and CI that installs the wrong major version fails to parse it entirely. Validate with `golangci-lint config verify` and keep the installed CLI major version (CI/Makefile/pre-commit/release) in lockstep with the config's `version:`.
- **`testing` imported into a non-`_test.go` file**: Benchmark helpers placed in `foo_bench.go` (not `foo_bench_test.go`) compile `testing` into the production binary and dodge `tests: false` lint scope. Benchmarks belong in `*_test.go` files.
- **`select { case <-jobs: ...; case <-ctx.Done(): ... }` honors cancellation only ~50% of the time**: when both a job and `ctx.Done()` are ready, Go picks a case at random, so a pre-canceled context is observed intermittently → flaky cancellation tests. Add a leading `if err := ctx.Err(); err != nil { … }` check at the top of the worker loop (or a leading `ctx.Err()` check in the result collector) so cancellation is deterministic.
- **A blank line OR a standalone `<!-- comment -->` line inside a raw `<svg>...</svg>` block in Markdown silently truncates it**: Python-Markdown's `md_in_html` extension closes the "raw HTML" block early at that line, injecting a stray `</svg></p>` there — everything after becomes inert `<p><rect .../></p>` fragments outside any SVG context, so the diagram renders as visually EMPTY with no build error or warning. `mkdocs build --strict` does not catch this (the HTML is well-formed, just semantically broken). Verify inline SVGs by checking the BUILT HTML for real shape elements (`grep -c '<rect\|<text' site/page/index.html`), not just source XML validity. Fix: never put a blank line or a comment-only line inside an inline `<svg>` block — keep every line non-blank and non-comment-only from `<svg>` to `</svg>`.
- **Per-item limit is not an aggregate limit, for batch APIs too**: a guard that caps each individual item (image pixel count, archive entry size, array element count) does nothing to stop a caller from passing many items that each pass the check but exhaust memory/CPU in aggregate (e.g. 100,000 small-but-valid images to a batch decode call). Any `LoadBatchXxx(items []T)`-shaped API needs its OWN cap on `len(items)`, independent of and in addition to any per-item cap. Same family as the ZIP aggregate-cap bug, just for in-process batch APIs rather than archive entries.

## Chronological Log

### 2026-07-02 — stb-image-go: LoadBatchConcurrent had no aggregate batch-size cap

- **File**: `stb-image-go/stb_image.go` (`LoadBatchConcurrent`)
- **Symptom**: found while correcting stale claims in `ISSUES.md` — the doc had (correctly, for once) flagged that `MaxImagePixels` caps each image's decoded size individually, but nothing capped how many images `LoadBatchConcurrent` would accept in one call.
- **Cause**: a caller passing e.g. 100,000 small-but-individually-valid images could still exhaust memory/CPU in aggregate; no guard existed at the batch-count level.
- **Fix**: added `var MaxBatchSize = 10_000` (same adjustable/disable-with-0 convention as `MaxImagePixels`), checked via a new `checkBatchLimit` helper before any decoding work begins. Extracting the per-worker loop into a standalone `batchWorker` function was required to keep `LoadBatchConcurrent` under golangci-lint's `gocognit` threshold after adding the check — adding a guard clause to an already-complex function needs a compensating simplification, not just the new `if`.
- **Verification**: new `TestMaxBatchSize` (matches `TestMaxImagePixels`'s save/restore/disable pattern); full suite + `-race` pass; `golangci-lint` 0 issues; coverage 87.7%→88.6%.
- **Lesson**: see the matching "Patterns to scan for FIRST" bullet above.

### 2026-07-01 — 101 course: 19 of 31 inline SVG diagrams rendered visually empty

- **File**: `101/lessons/*.md` (19 files: mostly the second and third authoring passes)
- **Symptom**: diagrams generated by later sonnet fan-outs appeared completely blank in the browser (confirmed by inspecting the live DOM: `svg[role="img"].querySelectorAll('rect').length === 0`), while diagrams from the first pass rendered fine. `mkdocs build --strict` reported zero errors or warnings for any of them.
- **Cause**: the affected SVGs had a blank line and/or a standalone `<!-- comment -->` line somewhere in the middle of the block (a natural authoring habit for readability). Python-Markdown's `md_in_html` extension treats either as the end of the raw-HTML block, inserts `</svg></p>` right there, and reprocesses everything after as ordinary Markdown — which wraps each subsequent `<rect>`/`<text>`/`<path>` line in its own orphaned `<p>` tag, outside any SVG rendering context.
- **Fix**: stripped all blank lines and standalone comment-only lines from every `<svg>...</svg>` block across all 31 lesson files (mechanical, whitespace/comment-only — verified via a stripped-content diff against the pre-fix version that nothing else changed). Verified by grepping the BUILT HTML of all 31 pages for real shape-element counts (`<rect>`/`<text>`/`<path>`), not just source XML validity.
- **Lesson**: this generalizes — see the matching "Patterns to scan for FIRST" bullet above. `mkdocs build --strict` and XML well-formedness checks both passed on every affected file; the bug was only visible by inspecting the rendered DOM.

### 2026-06-25 — tinyxml2-go: unbounded recursion in Parse/FindDeep — fatal stack overflow (audit M7, L5, L6)

- **File**: `tinyxml2-go/tinyxml2.go` (`parseElement`, `FindDeep`, `FindAllDeep`)
- **Symptom**: `Parse` (and `ParseWithConfig(UnlimitedConfig())`) had no depth limit, so a ~35MB file of deeply nested `<a><a>...` (under the 100MB size guard) recursed until the goroutine stack overflowed — a fatal error `recover()` cannot catch. `FindDeep`/`FindAllDeep` had the same risk walking a deep result tree.
- **Cause**: the pre-fix `parseElement` had no depth parameter at all; `Config.MaxNestingDepth=0` ("unlimited") bypassed the only existing guard.
- **Fix**: added an absolute `maxNestingDepth=10000` ceiling inside `parseElement`/`parseElementLimited` that no config can disable; rewrote `FindDeep`/`FindAllDeep` to use an explicit stack instead of recursion.
- **Lesson**: an "unlimited" config option must still have an internal hard backstop — the fix must prevent recursion from ever reaching a dangerous depth, not just make the limit configurable. (Follow-up: a later review found `parseElement`'s root call started depth at 0 while `parseElementLimited`'s started at 1, so the "same" 10000 ceiling fired one level apart between the two paths — fixed by aligning both to root==1.)

### 2026-06-23 — stb-truetype-go: implemented a real glyf rasterizer (was a stub)

- **File**: `stb-truetype-go/sfnt.go` (new), `stb_truetype.go`
- **Change**: replaced the placeholder rasterizer (blank bitmap + fake sleep) with a real pure-Go TrueType pipeline — sfnt directory + head/maxp/hhea/loca/cmap/glyf/hmtx parsing, cmap formats 0/4/6/12, simple + composite glyf decode, quadratic-Bézier flattening, and nonzero-winding anti-aliased scan-fill. `LoadFontFromBytes` now validates the font (rejects non-/CFF fonts).
- **Verification**: renders real glyphs from the embedded test font (correct relative widths/heights, descenders, blank space); 81% coverage; `FuzzLoadFont` clean over 266k execs (font files are untrusted input).
- **Lesson**: bounds-check every offset/length read from a font before slicing — a font parser is an untrusted-input parser like any other. Validate the sfnt version up front (reject OTTO/CFF) rather than mis-parsing it as glyf.

### 2026-06-23 — dr-wav-go: two fuzz-found crashes (divide-by-zero, OOM)

- **File**: `dr-wav-go/dr_wav.go` (`GetSampleCount`, `Parse`)
- **Symptom**: (1) `GetSampleCount` panicked `integer divide by zero` on a parsed WAV with `NumChannels == 0`; (2) `Parse` allocated `make([]byte, subchunk1Size-16)` from an untrusted ~4GB `subchunk1Size`, an OOM vector found by `FuzzParse` after ~15s.
- **Cause**: `Parse` does not validate header fields (only `ValidateWAV` does), so malformed values reach the accessors and the fmt-chunk skip.
- **Fix**: guard `NumChannels == 0` in `GetSampleCount`; `Seek` past the extra fmt bytes instead of allocating. Added `FuzzParse` (exercises Parse + every accessor) and checked in the crash-regression seed. 9M+ execs clean after the fix.
- **Lesson**: every field read from untrusted binary input is an attack surface — fuzz the parser AND its accessors, not just the entry point. Treat size fields as hints (Seek/cap), never as allocation lengths. (Same family as the data-chunk OOM above.)

### 2026-06-23 — cgltf-go: ValidateGLTF rejected a valid scene-less glTF

- **File**: `cgltf-go/cgltf.go` (`ValidateGLTF`)
- **Symptom**: a glTF with no `scenes` and `scene` omitted (zero value) failed validation ("invalid scene index: 0"), even though it is valid (e.g. a mesh library).
- **Cause**: `if gltf.Scene >= len(gltf.Scenes)` treats `0 >= 0` as out-of-range; the `omitempty` zero value can't distinguish an omitted `scene` from `scene: 0`.
- **Fix**: only range-check the index when scenes exist; otherwise only a *non-zero* scene index is an error. Still rejects `scene: 5` with empty scenes.
- **Lesson**: with `omitempty` numeric fields, a 0 can mean "omitted" — validation must special-case it instead of treating 0 as a real index.

### 2026-06-23 — jsmn-go: parallel tokenizer slower than serial (chunk granularity)

- **File**: `jsmn-go/parallel.go` (`buildChunkJobs`)
- **Symptom**: `BenchmarkParseParallel` was ~5.7x SLOWER than serial on a 1MB input (9.97ms vs 1.74ms) with 20,073 allocs.
- **Cause**: chunking created ONE chunk per top-level split point — 20,000 objects → 20,000 chunks, each allocating its own parser + token slice. The per-chunk setup dwarfed the parsing.
- **Fix**: group values into ~`numWorkers` balanced chunks (each still begins/ends on a top-level boundary). Allocs dropped 20,073 → 79 (~250x); parallel time fell ~27%.
- **Lesson**: fan-out granularity matters as much as fan-out itself — one unit of work per item, each re-paying a fixed per-unit cost, is pathological. Batch items into ~`numWorkers` groups. (Same shape as the §4c fan-out economics note.) Verify token-for-token equality vs serial after changing chunk boundaries.

### 2026-06-23 — stb-image-go: flaky cancellation (select race) and broken examples

- **File**: `stb-image-go/stb_image.go` (`LoadBatchConcurrent`)
- **Symptom**: `TestLoadBatchConcurrent_Cancellation` failed ~1 in 5 race runs: "Expected context.Canceled, got: <nil>".
- **Cause**: The worker's `select` raced `<-jobs` against `<-ctx.Done()`; with one ready job and an already-canceled context, Go picked the job ~50% of the time, decoded it successfully, and reported no error.
- **Fix**: Check `ctx.Err()` at the top of the worker loop before the select. Verified 20/20 race runs.
- **Lesson**: A bare select does not prioritize cancellation; check `ctx.Err()` explicitly before pulling work. Separately, the README's example programs had rotted (wrong API arity, missing go.mod) because no CI job built them — added a `make examples` target and an Examples CI job.

### 2026-06-23 — miniz-go: CreateArchiveConcurrent double-compresses → archives don't round-trip

- **File**: `miniz-go/miniz.go` (`CreateArchiveConcurrent`)
- **Symptom**: `Create → Extract` returns the intermediate flate stream, not the original bytes (e.g. 35-byte input extracts as 20 bytes). Existing tests missed it — they only checked for no error / non-empty output.
- **Cause**: Each file was pre-compressed with raw `flate`, then written into a `zip.Deflate` entry, which deflates it a second time.
- **Fix**: Assemble entries with `zip.Writer.CreateRaw` (Method/CRC32/CompressedSize64/UncompressedSize64 set from the worker), preserving parallel compression while round-tripping correctly. Added an explicit round-trip test.
- **Lesson**: Never feed pre-compressed data into a recompressing writer; assert round-trip equality, not just "no error".

### 2026-06-23 — dr-wav-go: Parse allocates an attacker-controlled size (OOM DoS)

- **File**: `dr-wav-go/dr_wav.go` (`readDataChunk`, extracted from `Parse`)
- **Symptom**: A 44-byte file declaring a 4 GB `data` subchunk drives `make([]byte, subchunkSize)` to allocate 4 GB before `io.ReadFull` fails.
- **Cause**: The declared 32-bit subchunk size was trusted as the allocation length.
- **Fix**: Cap the allocation to `r.Len()` (bytes actually remaining) before `make`.
- **Lesson**: Treat every size field from untrusted input as a hint, never an allocation length.

### 2026-06-22 — tinyxml2-go: fuzz test references non-existent OO API

- **File**: `tinyxml2-go/tinyxml2_fuzz_test.go:36`
- **Symptom**: `go vet` fails with `undefined: NewDocument`; fuzz tests are completely broken
- **Cause**: Fuzz test was generated against an OO-style API (`NewDocument()`, `doc.Parse()`, etc.) but the actual package uses a functional API (`Parse(data []byte) (*XMLDocument, error)`)
- **Fix**: Rewrote fuzz test to use the actual `Parse()` function and `*Node` struct fields
- **Lesson**: Always run `go vet` on fuzz tests before merging; OO vs functional API mismatches are common in generated test code

### 2026-06-22 — jsmn-go: fuzz seed test reports false positives on parse errors

- **File**: `jsmn-go/jsmn_fuzz_test.go:52`
- **Symptom**: `FuzzParse/seed#15` and `seed#16` fail: "Token 0 has invalid bounds: Start=0 End=-1"
- **Cause**: Fuzz test validates token bounds unconditionally; on failed parses, partial tokens with `End=-1` remain in `p.Tokens()`, causing the bounds check to trigger falsely
- **Fix**: Skip the bounds check when `Parse()` returns a non-nil error; partial tokens are expected on failure
- **Lesson**: Fuzz tests checking output invariants must guard on successful parse only; error paths leave parser state partially populated

### 2026-06-22 — stb-image-go: LoadBatchConcurrent returns string instead of context.Canceled

- **File**: `stb-image-go/stb_image.go:78`
- **Symptom**: `TestLoadBatchConcurrent_Cancellation` failed: expected `context.Canceled`, got `[context canceled context canceled ...]`
- **Cause**: Multiple workers all sent `ctx.Err()` to the errors channel; the collector formatted the entire slice as a string, losing the sentinel error identity
- **Fix**: Before formatting a multi-error, check each error with `errors.Is(e, context.Canceled)` and return it directly if matched
- **Lesson**: When aggregating goroutine errors, always preserve context sentinel errors; formatting loses `errors.Is` identity
