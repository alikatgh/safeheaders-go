# Bug Journal

## Patterns to scan for FIRST

- **Multi-error context flattening**: When collecting errors from goroutines into a slice and formatting as a string, context sentinel errors (context.Canceled, context.DeadlineExceeded) lose their identity. Always check errors.Is() before formatting. Applies to any function that fans out work into goroutines and collects errors.
- **Fuzz test token bounds check on error**: When a parser fails partway through, partial tokens with End=-1 remain accessible via Tokens(). Fuzz tests that validate token bounds must skip the check when Parse() returns an error, or the test will report false positives.
- **Size-gated parallel code path in tests**: If parallel processing only activates above a byte threshold (e.g. >4096 bytes), test data must exceed that threshold or the parallel path is never exercised and coverage is misleadingly low.
- **OO API fuzz test referencing wrong API**: Auto-generated fuzz tests can reference an OO-style API (NewDocument/doc.Parse) when the actual package exposes a functional API (Parse(data) (*Doc, error)). Always compile-check fuzz tests before merging.
- **Unbounded recursive parser = stack-overflow DoS**: Recursive descent parsers (XML elements, JSON values) that recurse once per nesting level will exhaust the stack on adversarial deeply-nested input. Guard with a MaxNestingDepth limit checked at the top of the recursive function, plus MaxInputSize and a shared MaxNodeCount counter. Follow the jsmn-go Config pattern (DefaultConfig/StrictConfig/UnlimitedConfig + sentinel errors via errors.Is); add limits as a NEW ParseWithConfig entrypoint so the existing Parse signature stays back-compatible.
- **`make([]byte, untrustedSize)` from a header field = OOM DoS**: Any binary parser that reads a length/size field from input and allocates that many bytes (WAV data chunk, image dimensions, ZIP entry size) can be driven to allocate gigabytes by a tiny malicious file. Cap the allocation to the bytes actually remaining in the reader (`if n > r.Len() { n = r.Len() }`) before `make`, then `io.ReadFull`.
- **Pre-compressed data written into a recompressing container = double compression**: Writing an already-`flate`-compressed stream into an `archive/zip` entry with `Method: zip.Deflate` deflates it AGAIN, so the archive does not round-trip (extraction yields the intermediate stream). To keep pre-compression, use `zip.Writer.CreateRaw` with a `FileHeader` carrying `Method`, `CRC32` (of the *uncompressed* data), `CompressedSize64`, `UncompressedSize64`. ALWAYS add a round-trip test (`Create → Extract → bytes.Equal`), not just a "no error / non-empty" assertion.
- **Config schema-version mismatch silently disables all settings**: A `.golangci.yml` declaring `version: "2"` while using v1-style `linters-settings:` is *invalid* — `golangci-lint run` tolerates it but ignores every setting (thresholds, ignore-sigs, etc.), so the linter runs on defaults and CI that installs the wrong major version fails to parse it entirely. Validate with `golangci-lint config verify` and keep the installed CLI major version (CI/Makefile/pre-commit/release) in lockstep with the config's `version:`.
- **`testing` imported into a non-`_test.go` file**: Benchmark helpers placed in `foo_bench.go` (not `foo_bench_test.go`) compile `testing` into the production binary and dodge `tests: false` lint scope. Benchmarks belong in `*_test.go` files.
- **`select { case <-jobs: ...; case <-ctx.Done(): ... }` honors cancellation only ~50% of the time**: when both a job and `ctx.Done()` are ready, Go picks a case at random, so a pre-canceled context is observed intermittently → flaky cancellation tests. Add a leading `if err := ctx.Err(); err != nil { … }` check at the top of the worker loop (or a leading `ctx.Err()` check in the result collector) so cancellation is deterministic.

## Chronological Log

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
