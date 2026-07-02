# SafeHeaders-Go - Known Issues & Improvement Tracker

This document tracks known issues, technical debt, and planned improvements for the SafeHeaders-Go project.

**Last Updated**: 2026-07-02 (verified against the actual repo state — several entries below were stale and have been corrected)
**Status Legend**: 🔴 Critical | 🟡 Major | 🔵 Minor | ✅ Fixed

---

## 🔴 Critical Issues (P0)

### ✅ FIXED: Failing Test in stb-image-go
**Status**: Fixed
**Module**: stb-image-go
**Location**: `stb-image-go/stb_image_test.go:89`
**Issue**: Context cancellation test was failing because multiple workers reported `context.Canceled`, but test expected single error.
**Fix**: Updated error handling to detect context cancellation and return `context.Canceled` directly instead of aggregating.

### ✅ FIXED: CI/CD Only Tests One Module
**Status**: Fixed
**Location**: `.github/workflows/go-ci.yaml`
**Issue**: Workflow only tested jsmn-go, leaving 8 modules untested in CI.
**Fix**: Extended CI to use matrix strategy testing all 9 modules with separate lint and test jobs.

### ✅ FIXED: Go Version Mismatch
**Status**: Fixed
**Locations**: `go.work`, all `go.mod` files, CI workflow
**Issue**: CI used Go 1.22 while modules declared 1.24.4 (non-existent version).
**Fix**: Standardized on Go 1.23 across all files.

---

## 🟡 Major Issues (P1)

### 1. Module Maturity / Scope
**Priority**: Low
**Status**: ✅ Resolved — all 9 modules are Stable and maturity is labeled honestly

The former scope gap, **stb-truetype-go**'s placeholder rasterizer, is now a real
pure-Go glyf rasterizer (sfnt parsing, cmap formats, simple+composite outlines,
anti-aliased scan-fill; fuzzed). The remaining modules do real work, deliberately
scoped as ports/wrappers:
- **dr-wav-go** - RIFF/PCM parse + serialize, concurrent batch decode, multi-channel
  extraction (`ExtractChannels`) — no float/ADPCM sample formats
- **cgltf-go** - full glTF 2.0 JSON parse/serialize, concurrent batch parse
- **cjson-go** - marshaling helpers + parallel array unmarshal over `encoding/json`
- **miniz-go** - ZIP create/extract + DEFLATE, parallel entry compression, streaming
  compress/decompress
- **tinyxml2-go** - DOM parse + traversal queries (no XPath)
- **stb-image-go** - PNG/JPEG/GIF decode + concurrent batch over the `image` stdlib

---

### 2. Inconsistent Error Handling
**Priority**: Medium
**Status**: 🟡 Partially resolved — a standard is now documented; consistent enforcement across every call site was not independently re-audited

**What changed**: `CONTRIBUTING.md` now has a dedicated "Error Handling Standards"
section, and the 2026-06-23 security audit's fixes (see `docs/audits/`) added
consistent `fmt.Errorf("context: %w", err)` wrapping to several previously-bare
`return nil, err` sites (e.g. `tinyxml2-go/tinyxml2.go`'s `Parse`/`ParseWithConfig`
token loops).

**Still true**:
1. **stb-image-go** - context-cancellation error handling was fixed; other error
   paths are not necessarily uniform.
2. **jsmn-go** - fails fast on first error (a deliberate, documented choice, not a bug).
3. Some modules still mix `errors.New()`, `fmt.Errorf()`, and direct returns —
   no module uses `errors.Join()` for aggregation.

**Recommendation**: a full pass verifying every exported function follows the
documented standard has not been done; low urgency since the standard itself
is now written down for contributors to follow going forward.

---

### 3. Chunking Strategy
**Priority**: Medium
**Status**: ✅ jsmn-go fixed; others batch independent items (no chunk-boundary problem)

**jsmn-go (FIXED)**: previously created one chunk per top-level split point, so a
1MB / 20k-object input spawned 20k chunks, each allocating its own parser — the
parallel path was *slower* than serial. `buildChunkJobs` now groups values into
~`NumCPU` balanced chunks (allocations dropped ~250x). Splitting is still on
top-level boundaries, so a single large object correctly falls back to serial.

**miniz-go / cgltf-go / dr-wav-go / stb-image-go / cjson-go**: these parallelize
across *independent items* (files / models / images / array elements), one item
per job, which is the right granularity — there is no chunk-boundary correctness
issue. Compression ratio is preserved in miniz because entries are compressed
whole and assembled with `zip.CreateRaw` (no recompression).

**Remaining**: per-input boundary-aware splitting for a *single* large JSON
object/array is still not implemented (such inputs use the serial path).

---

### 4. Missing Module Documentation
**Priority**: Medium
**Status**: ✅ Largely resolved — this entire section was stale; all 9 modules have substantial READMEs

**Verified current state** (all 9 module READMEs re-checked directly, not assumed):
- **All 9 modules have a `README.md`** (149–459 lines each), not "only jsmn-go and select modules" as previously claimed.
- **8 of 9** have a `## Performance` section with real benchmark commands.
  `tinyxml2-go` was missing one — **fixed this pass**, added pointing at its
  real `BenchmarkParse`/`BenchmarkTraverseConcurrent` benchmarks (no fabricated numbers).
- **5 of 9** (cgltf, dr-wav, jsmn, miniz, and now tinyxml2) have a `## Limitations`
  section. `cjson-go`, `linenoise-go`, `stb-image-go`, `stb-truetype-go` still lack one —
  genuinely open, low priority.
- **Thread-safety** is documented in 8 of 9 READMEs. `linenoise-go` had zero
  mentions despite the code being mutex-guarded since the 2026-06-23 audit fix
  (H3) — **fixed this pass**, added a "Thread safety" note under Global Functions.
- Every module has godoc comments plus its own usage examples in its README.

**Still genuinely open**: a compatibility-notes-with-the-original-C-library
section is not standardized across modules (some READMEs have a "Comparison
with Original" section, others don't).

---

### 5. Input Size Limits (DoS Risk)
**Priority**: Medium (Security)
**Status**: ✅ Fully implemented in all 9 modules — no known open gap

**Implemented**:
- **jsmn-go** - `ParseWithConfig` enforces `MaxInputSize` and `MaxTokens`
  (`DefaultConfig`/`StrictConfig`/`UnlimitedConfig`)
- **tinyxml2-go** - `ParseWithConfig`/`Parse` enforce input size, node count, and
  nesting depth, including an absolute `maxNestingDepth` ceiling that applies
  even to the "unlimited" path
- **dr-wav-go** - data-chunk allocation is capped to the bytes actually present
  (a malicious size header can no longer trigger an OOM), **and**
  (2026-07-02) `MaxBatchSize` caps `ParseBatch` to 10,000 files
- **stb-image-go** - `MaxImagePixels` rejects any single image whose declared
  dimensions exceed the cap, checked from the header *before* the full decode
  (a decode-bomb guard), **and** (2026-07-02) `MaxBatchSize` rejects a
  `LoadBatchConcurrent` call outright if handed more than 10,000 images
- **cgltf-go** - (2026-07-02) `MaxBatchSize` caps `ParseBatch` to 10,000 files —
  the same aggregate gap as stb-image-go/dr-wav-go, found by checking every
  other `func.*Batch\|func.*Parallel` entry point in the codebase after fixing
  the first instance
- **cjson-go** - `MaxArrayItems` caps `UnmarshalArrayParallel` — this was also
  previously listed as open and is now fixed
- **miniz-go** - `MaxDecompressedSize` bounds both single-entry and (as of the
  2026-06-23 audit) aggregate archive output on the *extract* path, **and**
  (2026-07-02) `MaxBatchSize` caps `CreateArchiveConcurrent` to 10,000 files
  on the *create* path (a distinct gap — the existing guard didn't cover it)

**Note on 2026-07-02's fixes**: after closing the stb-image-go gap, the same
`LoadBatchXxx(items []T)`/`ParseBatch(dataList [][]byte)` shape was grepped for
across every module rather than assumed fixed elsewhere — it wasn't, in three
more places. See the "per-item limit is not an aggregate limit" pattern in
`docs/BUG_JOURNAL.md`.

---

## 🔵 Minor Issues (P2)

### 6. Test Coverage
**Priority**: Low
**Status**: ✅ Every module is above the 70% CI gate (re-measured this pass with `go test -cover`)

- cgltf 94.3% · cjson 93.9% · jsmn 93.6% · tinyxml2 89.8% · stb-image 87.7% ·
  dr-wav 86.2% · stb-truetype 81.3% · miniz 83.6% · linenoise 77.6%

(Several of these — cjson, jsmn, miniz in particular — are meaningfully higher
than the last-recorded numbers in this file, reflecting coverage work since the
prior update.) Fuzz tests exist for jsmn-go, tinyxml2-go, dr-wav-go, and
stb-truetype-go. Still useful: cross-module integration tests and more
error-injection/edge-case coverage.

---

### 7. Benchmark Data
**Priority**: Low
**Status**: ✅ Resolved — benchmarks no longer depend on committed fixtures.

The jsmn benchmark generates a representative ~1MB payload in-memory; the
tinyxml2 benchmark generates `bench.xml` on first run; the large `testdata/`
fixtures are regenerable with `make testdata`. None are committed — see
`testdata/README.md`.

---

### 8. Missing Contributing Guidelines
**Priority**: Low
**Status**: ✅ Resolved — this entire entry was stale

All five files this entry asked for already exist at the repo root:
`CONTRIBUTING.md`, `ARCHITECTURE.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, and
`CHANGELOG.md` (the last actively maintained — see its `[Unreleased]` section).

---

### 9. Semantic Versioning / Releases
**Priority**: Low
**Status**: 🟡 Partially resolved — this entry's "no git tags" claim was stale

Git tags exist: `v0.2.0` (repo-wide), plus per-module tags
`stb-image-go/v0.1.0`, `tinyxml2-go/v0.1.0`, `tinyxml2-go/v0.3.0`.

**Still open**: no GitHub Releases have been published from these tags, and
tagging is not yet consistent across all 9 modules (most don't have their own
tag). Low priority.

---

### 10. Duplicate Code Patterns
**Priority**: Low
**Status**: Still open — genuinely unaddressed, re-verified this pass (no `internal/` package exists)

**Issue**: Worker pool pattern duplicated across 6+ modules.

**Examples**:
- jsmn-go:246-271 (worker pool)
- stb-image-go:43-67 (nearly identical)
- tinyxml2-go:131-148 (similar pattern)

**Recommendation** (unchanged — deliberately not attempted without a decision
from a maintainer, since consolidating 6 modules' concurrency code is a
meaningful refactor with real regression risk, not a quick fix):
Extract common patterns to internal package:
```go
// internal/workers/pool.go
package workers

type Job[T any, R any] struct {
    ID   int
    Data T
}

type Result[R any] struct {
    ID   int
    Data R
    Err  error
}

func RunPool[T any, R any](
    ctx context.Context,
    jobs []Job[T, R],
    worker func(context.Context, Job[T, R]) (R, error),
) ([]Result[R], error)
```

---

### 11. License Compatibility
**Priority**: Low (Legal)
**Status**: 🟡 Partially resolved — the file exists; the legal question is still unverified

**Done**: a `LICENSE` file now exists at the repo root (this entry previously,
incorrectly, said it didn't).

**Original Library Licenses** (unchanged from the original entry):
- jsmn.h - MIT ✅
- stb_*.h - Public Domain / MIT ✅
- cJSON.h - MIT ✅
- tinyxml2.h - zlib ⚠️ (compatibility with the repo's MIT license not
  independently verified — this needs an actual legal read, not an AI guess)
- cgltf.h - MIT ✅
- dr_wav.h - Public Domain ✅

**Still open**: a `LICENSES/` directory with the original per-library license
texts and a README attribution section have not been added.

---

### 12. Code Owners
**Priority**: Low
**Status**: ✅ Resolved — this entry was stale

`.github/CODEOWNERS` already exists.

---

### 13. golangci-lint Version
**Priority**: Low
**Status**: ✅ Resolved — this entry's version numbers were both wrong

CI and the `Makefile` pin `golangci-lint v2.2.2` (not "v2.1.6 from 2020" as this
entry previously claimed), matching the `.golangci.yml` v2 schema. No action
needed beyond routine version bumps.

---

## 📋 Enhancement Ideas

### Future Enhancements (P3)

1. **Streaming APIs** — ✅ done, but not exactly as originally scoped to jsmn-go
   - `cjson-go`: `UnmarshalStream(r io.Reader, v any) error` / `MarshalStream(w io.Writer, v any) error`
   - `miniz-go`: `CompressStream(dst io.Writer, src io.Reader) error` / `DecompressStream(...)`
   - `stb-image-go`: `LoadStream(r io.Reader) (image.Image, error)`
   - `jsmn-go` itself does **not** have a `ParseStream`/channel-based API — its
     token model is a flat `[]Token` over the whole input, which doesn't map
     cleanly onto a streaming/channel interface without a larger redesign. If
     multi-GB JSON streaming is still wanted specifically for jsmn-go, that
     remains open.

2. **WebAssembly Support** — still just an idea, not started.
   - Compile modules to WASM for browser usage
   - Target: `GOOS=js GOARCH=wasm go build`

3. **Performance Monitoring** — still just an idea, not started.
   - pprof endpoints, allocation/CPU profiling for hot paths

4. **Benchmark Comparison Tool** — still just an idea, not started.
   - Script to compare against the original C libraries, automated benchstat in CI

5. **Module-Specific READMEs** — ✅ done (see #4 above); the remaining gap is
   per-module Limitations sections and consistent "Compatibility with original"
   sections, not full READMEs.

6. **Add More C Libraries** — still just an idea (stb_vorbis.h, tinyobjloader.h,
   utf8.h), not started.

---

## 🎯 Roadmap

### Phase 1: Stability — ✅ complete
- [x] Fix failing tests
- [x] Extend CI to all modules
- [x] Fix Go version mismatch
- [x] Remove duplicate CI steps
- [x] Add maturity badges (README.md has 7 status badges)
- [x] Add CONTRIBUTING.md
- [x] Add LICENSE file

### Phase 2: Completeness
- [ ] Complete tinyxml2-go (add XPath queries) — still open, genuinely not done
- [x] Complete dr-wav-go (multi-channel support) — `ExtractChannels()` exists
- [x] Document error handling consistency (CONTRIBUTING.md § Error Handling Standards) — enforcement not independently re-audited across every call site
- [x] Add input size limits — done in all 9 modules, including stb-image-go's `MaxBatchSize` aggregate cap (see #5)
- [x] Increase test coverage to 80%+ — 8 of 9 modules now clear 80%; `linenoise-go` is at 77.6%, still below

### Phase 3: Optimization
- [x] Implement smart chunking for jsmn-go
- [ ] Optimize memory allocations — no dedicated pass done
- [x] Add fuzz testing (jsmn-go, tinyxml2-go, dr-wav-go, stb-truetype-go)
- [ ] Performance benchmarks vs C — not started

### Phase 4: Ecosystem
- [x] Add streaming APIs (cjson-go, miniz-go, stb-image-go — see Enhancement #1)
- [ ] WebAssembly support
- [ ] Create examples/ directory (an `examples/` directory with 4 runnable programs already exists at the repo root — if "ecosystem examples" means something further, scope it explicitly)
- [ ] Write blog posts / tutorials (the `101/` course — a full 31-lesson learn-Go-from-this-repo curriculum — substantially covers this)
- [ ] Present at conferences

---

## 📊 Module Maturity Matrix

Coverage = measured `go test -cover` total, re-run 2026-07-02. Tests/Docs/Lint reflect current CI.

| Module | Coverage | Tests | Docs | Lint | Status |
|--------|----------|-------|------|------|--------|
| cgltf-go | 94.3% | ✅ race | ✅ | ✅ | 🟢 Stable |
| cjson-go | 93.9% | ✅ race | ✅ | ✅ | 🟢 Stable |
| jsmn-go | 93.6% | ✅ race | ✅ | ✅ | 🟢 Stable |
| tinyxml2-go | 89.8% | ✅ race | ✅ | ✅ | 🟢 Stable |
| stb-image-go | 87.7% | ✅ race | ✅ | ✅ | 🟢 Stable |
| dr-wav-go | 86.2% | ✅ race | ✅ | ✅ | 🟢 Stable |
| miniz-go | 83.6% | ✅ race | ✅ | ✅ | 🟢 Stable |
| stb-truetype-go | 81.3% | ✅ race | ✅ | ✅ | 🟢 Stable |
| linenoise-go | 77.6% | ✅ race | ✅ | ✅ | 🟢 Stable |

**Legend**: ✅ = passing / present · 🟢 Stable. All 9 modules are Stable,
race-tested, lint-clean (golangci-lint v2.2.2, 0 issues), and above the 70% gate.

---

## 💡 Contributing

Found a new issue? Want to fix one? Great!

1. **For existing issues**: Comment on the issue to claim it
2. **For new issues**: Add them to this file via PR
3. **For fixes**: Reference the issue number in your commit

**Priority Guide**:
- P0 (Critical): Fix immediately, blocks releases
- P1 (Major): Fix within 1-2 weeks, impacts quality
- P2 (Minor): Fix within 1-2 months, quality-of-life
- P3 (Enhancement): Nice-to-have, no timeline

---

## 📝 Notes

- This document is living and should be updated as issues are fixed
- When fixing an issue, move it to the "Fixed" section with date
- Add new issues as they're discovered
- **2026-07-02**: this file had drifted significantly from reality — several
  P1/P2 items claimed things were "not present" or "still open" (LICENSE,
  CODEOWNERS, CONTRIBUTING.md, git tags, module READMEs, golangci-lint version,
  cjson/stb-image size limits) when they had in fact already been done in
  earlier work. Every claim in this revision was re-verified directly against
  the repo (file existence checks, `git tag`, `go test -cover`, grep for the
  relevant guard code) rather than assumed. Re-verify before trusting old
  entries in the git history of this file.

**Maintainer**: @alikatgh
