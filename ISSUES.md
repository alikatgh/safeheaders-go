# SafeHeaders-Go - Known Issues & Improvement Tracker

This document tracks known issues, technical debt, and planned improvements for the SafeHeaders-Go project.

**Last Updated**: 2026-06-23
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
- **dr-wav-go** - RIFF/PCM parse + serialize, concurrent batch decode (no float/ADPCM)
- **cgltf-go** - full glTF 2.0 JSON parse/serialize, concurrent batch parse
- **cjson-go** - marshaling helpers + parallel array unmarshal over `encoding/json`
- **miniz-go** - ZIP create/extract + DEFLATE, parallel entry compression
- **tinyxml2-go** - DOM parse + traversal queries (no XPath)
- **stb-image-go** - PNG/JPEG/GIF decode + concurrent batch over the `image` stdlib

---

### 2. Inconsistent Error Handling
**Priority**: Medium
**Affected Modules**: All

**Examples**:
1. **stb-image-go** - Returns formatted string of errors (now fixed for context cancellation, but still inconsistent for other errors)
2. **jsmn-go** - Fails fast on first error (good for correctness)
3. **tinyxml2-go** - No error aggregation in concurrent operations
4. **Most modules** - Mix of `errors.New()`, `fmt.Errorf()`, and direct returns

**Impact**: Makes debugging harder, inconsistent API experience.

**Recommendation**:
- Document error handling strategy in CONTRIBUTING.md
- Standardize on one approach:
  - Option A: Always fail-fast on first error
  - Option B: Aggregate errors using `errors.Join()` (Go 1.20+)
- Add error wrapping consistently with `fmt.Errorf("context: %w", err)`

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
**Status**: Partial

**Current State**:
- Only jsmn-go and select modules have README.md
- Most modules lack usage examples
- No API documentation beyond godoc comments
- Performance characteristics undocumented

**Missing for Each Module**:
- [ ] API usage examples (basic + concurrent)
- [ ] Performance benchmarks and comparisons
- [ ] Known limitations section
- [ ] Compatibility notes with original C libraries
- [ ] Thread-safety guarantees

**Recommendation**:
Create template: `.github/MODULE_README_TEMPLATE.md` with sections:
1. Overview
2. Installation
3. Basic Usage
4. Concurrent Usage
5. Performance Characteristics
6. Limitations
7. API Reference
8. Contributing

---

### 5. Input Size Limits (DoS Risk)
**Priority**: Medium (Security)
**Status**: ✅ Largely implemented (jsmn-go, tinyxml2-go, dr-wav-go)

**Implemented**:
- **jsmn-go** - `ParseWithConfig` enforces `MaxInputSize` and `MaxTokens`
  (`DefaultConfig`/`StrictConfig`/`UnlimitedConfig`)
- **tinyxml2-go** - `ParseWithConfig` enforces input size, node count, and
  nesting depth
- **dr-wav-go** - data-chunk allocation is capped to the bytes actually present
  (a malicious size header can no longer trigger an OOM)

**Still open**:
- **stb-image-go** - no explicit batch-size limit (bounded in practice by caller)
- **cjson-go** - relies on `encoding/json`'s own limits

**Impact**:
- An attacker could send 1GB JSON → OOM crash
- Batch processing 10,000 images → memory exhaustion
- Deeply nested XML → stack overflow or memory exhaustion

**Recommendation**:
Add configurable limits with sensible defaults:
```go
type ParserConfig struct {
    MaxTokens      int // Default: 1,000,000
    MaxInputSize   int // Default: 100MB
    MaxBatchSize   int // Default: 1,000 items
    MaxNestingDepth int // Default: 1,000
}
```

Example:
```go
// jsmn-go
func NewParserWithConfig(cfg ParserConfig) *Parser
func ParseParallel(json []byte, cfg ParserConfig) ([]Token, error)

// stb-image-go
func LoadBatchConcurrent(ctx context.Context, datas [][]byte, maxBatch int) ([]image.Image, error)
```

---

## 🔵 Minor Issues (P2)

### 6. Test Coverage
**Priority**: Low
**Status**: ✅ Every module is above the 70% CI gate (measured `go test -cover` totals)

- cgltf 93% · tinyxml2 89% · stb-image 89% · jsmn 88% · cjson 83% · dr-wav 82% ·
  stb-truetype 81% · miniz 79% · linenoise 77%

Fuzz tests exist for jsmn-go and tinyxml2-go, and the 70% threshold is enforced
in CI. Still useful: cross-module integration tests and more error-injection
and edge-case coverage.

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
**Status**: Not present

**Needed Files**:
- [ ] CONTRIBUTING.md - PR process, code standards, testing requirements
- [ ] ARCHITECTURE.md - Design patterns, concurrency patterns, error handling
- [ ] SECURITY.md - Vulnerability reporting, security policy
- [ ] CODE_OF_CONDUCT.md - Community guidelines
- [ ] CHANGELOG.md - Version history

---

### 9. No Semantic Versioning / Releases
**Priority**: Low
**Issue**: No git tags, no GitHub releases, no version tracking.

**Impact**: Users can't track breaking changes or pin to stable versions.

**Recommendation**:
- Add semantic versioning: `v0.1.0` for alpha modules, `v1.0.0` for stable
- Create GitHub releases with changelog
- Use git tags: `git tag -a v0.1.0 -m "Initial alpha release"`
- Document breaking changes clearly

---

### 10. Duplicate Code Patterns
**Priority**: Low
**Issue**: Worker pool pattern duplicated across 6+ modules.

**Examples**:
- jsmn-go:246-271 (worker pool)
- stb-image-go:43-67 (nearly identical)
- tinyxml2-go:131-148 (similar pattern)

**Recommendation**:
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

### 11. License Compatibility Not Verified
**Priority**: Low (Legal)
**Issue**: README says "MIT" but doesn't verify compatibility with original C libraries.

**Original Library Licenses**:
- jsmn.h - MIT ✅
- stb_*.h - Public Domain / MIT ✅
- cJSON.h - MIT ✅
- tinyxml2.h - zlib ⚠️ (need to verify compatibility)
- cgltf.h - MIT ✅
- nuklear.h - Public Domain / MIT ✅
- dr_wav.h - Public Domain ✅

**Recommendation**:
- Add LICENSE file with full MIT text
- Add LICENSES/ directory with original library licenses
- Verify zlib license compatibility with MIT
- Add attribution section to README

---

### 12. No Code Owners / Review Requirements
**Priority**: Low
**Issue**: No CODEOWNERS file, unclear who reviews PRs.

**Recommendation**:
```
# .github/CODEOWNERS
* @alikatgh

# Specific modules
/jsmn-go/ @alikatgh
/stb-truetype-go/ @alikatgh
```

---

### 13. golangci-lint Version Too Old
**Priority**: Low
**Issue**: CI uses v2.1.6 (very old, from 2020), many new linters missed.

**Current**: v2.1.6 (2020)
**Latest**: v1.61.0 (2024)

**Impact**: Missing modern linters (exhaustruct, nilerr, nilaway, etc.)

**Recommendation**: Update to v1.61.0 or later.

---

## 📋 Enhancement Ideas

### Future Enhancements (P3)

1. **Streaming API for jsmn-go**
   - Current: Loads entire JSON into memory
   - Proposed: `ParseStream(r io.Reader) <-chan Token`
   - Benefit: Handle multi-GB JSON files

2. **WebAssembly Support**
   - Compile modules to WASM for browser usage
   - Example: JSON parsing in web workers
   - Target: `GOOS=js GOARCH=wasm go build`

3. **Performance Monitoring**
   - Add pprof endpoints for benchmarking
   - Memory allocation profiling
   - CPU profiling for hot paths

4. **Benchmark Comparison Tool**
   - Script to compare against C libraries
   - Automated benchstat reports in CI
   - Performance regression detection

5. **Module-Specific READMEs**
   - Detailed API docs for each module
   - More usage examples
   - Performance tuning guides

6. **Add More C Libraries**
   - stb_vorbis.h (audio decoding)
   - tinyobjloader.h (OBJ model loading)
   - utf8.h (UTF-8 handling)
   - See README wishlist for full list

---

## 🎯 Roadmap

### Phase 1: Stability (Current)
- [x] Fix failing tests
- [x] Extend CI to all modules
- [x] Fix Go version mismatch
- [x] Remove duplicate CI steps
- [ ] Add maturity badges
- [ ] Add CONTRIBUTING.md
- [ ] Add LICENSE file

### Phase 2: Completeness (Next)
- [ ] Complete tinyxml2-go (add XPath queries)
- [ ] Complete dr-wav-go (multi-channel support)
- [ ] Improve error handling consistency
- [ ] Add input size limits
- [ ] Increase test coverage to 80%+

### Phase 3: Optimization (Future)
- [ ] Implement smart chunking for jsmn-go
- [ ] Optimize memory allocations
- [ ] Add fuzz testing
- [ ] Performance benchmarks vs C

### Phase 4: Ecosystem (Long-term)
- [ ] Add streaming APIs
- [ ] WebAssembly support
- [ ] Create examples/ directory
- [ ] Write blog posts / tutorials
- [ ] Present at conferences

---

## 📊 Module Maturity Matrix

Coverage = measured `go test -cover` total. Tests/Docs/Lint reflect current CI.

| Module | Coverage | Tests | Docs | Lint | Status |
|--------|----------|-------|------|------|--------|
| cgltf-go | 93% | ✅ race | ✅ | ✅ | 🟢 Stable |
| tinyxml2-go | 89% | ✅ race | ✅ | ✅ | 🟢 Stable |
| stb-image-go | 89% | ✅ race | ✅ | ✅ | 🟢 Stable |
| jsmn-go | 88% | ✅ race | ✅ | ✅ | 🟢 Stable |
| cjson-go | 83% | ✅ race | ✅ | ✅ | 🟢 Stable |
| dr-wav-go | 82% | ✅ race | ✅ | ✅ | 🟢 Stable |
| stb-truetype-go | 81% | ✅ race | ✅ | ✅ | 🟢 Stable |
| miniz-go | 79% | ✅ race | ✅ | ✅ | 🟢 Stable |
| linenoise-go | 77% | ✅ race | ✅ | ✅ | 🟢 Stable |

**Legend**: ✅ = passing / present · 🟢 Stable. All 9 modules are Stable,
race-tested, lint-clean (golangci-lint v2, 0 issues), and above the 70% gate.

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
- Link to GitHub issues when created

**Maintainer**: @alikatgh
**Last Review**: 2025-10-31
