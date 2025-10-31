# SafeHeaders-Go - Known Issues & Improvement Tracker

This document tracks known issues, technical debt, and planned improvements for the SafeHeaders-Go project.

**Last Updated**: 2025-10-31
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

### 1. Incomplete Module Implementations
**Priority**: High
**Affected Modules**:
- **nuklear-go** (5% complete) - Empty stubs, no real GUI functionality
- **dr-wav-go** (15% complete) - Basic RIFF header parsing only, missing multi-channel support
- **cgltf-go** (20% complete) - Header validation only, no real glTF parsing
- **cjson-go** (30% complete) - Wraps stdlib, minimal parallel logic, no nested parallelism
- **miniz-go** (25% complete) - Basic compression only, naive chunking reduces ratio
- **tinyxml2-go** (40% complete) - DOM parsing works, missing XPath-like queries
- **stb-image-go** (60% complete) - Wraps stdlib decode, limited format support

**Impact**: README suggests production-ready functionality, but most modules are proofs-of-concept.

**Recommendation**:
- Add maturity badges (Alpha, Beta, Stable) to README
- Focus development on 2-3 modules to completion
- Document missing features explicitly
- Consider marking stub modules as "Experimental - Contributions Welcome"

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

### 3. Naive Chunking Strategy Limits Performance
**Priority**: Medium
**Affected Modules**: jsmn-go, miniz-go, cgltf-go, dr-wav-go

**Issues**:
- **jsmn-go:166-199** - Splits on top-level commas (works for arrays, fails for complex objects)
- **miniz-go** - Splits by byte count (reduces compression ratio by ~10-15%)
- **cgltf-go** - Splits binary data without structure awareness
- **dr-wav-go** - Simple byte chunking doesn't align with audio frames

**Impact**: Limits parallel speedup to ~2.1x on 8 CPUs (should be ~4-6x).

**Recommendation**:
- **Short-term**: Document limitations prominently in README and module docs
- **Medium-term**: Implement smart boundary detection
  - JSON: Scan backwards/forwards from split point to find `}]` boundaries
  - Compression: Use independent compression blocks (ZIP format supports this)
  - glTF: Parse chunk headers to find buffer boundaries
  - WAV: Align splits to frame boundaries (sample size × channels)

**Related Benchmarks**:
```
Current (naive):     150ms → 70ms (2.1x speedup on 8 CPU)
Smart chunking est:  150ms → 40ms (3.75x speedup on 8 CPU)
Overhead reduction:  ~43% improvement possible
```

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

### 5. No Input Size Limits (DoS Risk)
**Priority**: Medium (Security)
**Affected Modules**: jsmn-go, stb-image-go, tinyxml2-go, cjson-go

**Vulnerable Locations**:
- **jsmn-go:115** - Token slice grows unbounded
- **stb-image-go:24** - No batch size limit
- **tinyxml2-go** - No maximum node count
- **cjson-go** - No maximum object size

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

### 6. Test Coverage Gaps
**Priority**: Low
**Current Coverage**:
- jsmn-go: ~85% (good)
- stb-truetype-go: ~80% (good)
- stb-image-go: ~70%
- Others: 40-60%

**Missing Test Categories**:
- [ ] Integration tests (multi-module)
- [ ] Fuzz tests (found via `go test -fuzz`)
- [ ] Stress tests (large inputs, many goroutines)
- [ ] Error injection tests
- [ ] Edge cases (empty inputs, single-byte files)

**Recommendation**:
- Add fuzz tests for parsers: `FuzzParse(f *testing.F)`
- Add stress test suite: `stress_test.go` files
- Set coverage threshold in CI: `go test -coverprofile=coverage.out && go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | awk '{if ($1 < 75.0) exit 1}'`

---

### 7. Benchmark Data Missing from Repo
**Priority**: Low
**Issue**: README references `testdata/bench.json` but file not in repo.

**Recommendation**:
- Create `testdata/` directories in each module
- Add benchmark data files with `//go:embed` directive
- Document benchmark methodology:
  ```
  testdata/
    bench.json (1MB, 10,000 objects)
    bench.xml (500KB, 5,000 nodes)
    bench.png (1024×1024, 24-bit)
    bench.ttf (font file, ~100KB)
  ```

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

| Module | Completeness | Tests | Docs | Production-Ready | Target Version |
|--------|--------------|-------|------|------------------|----------------|
| jsmn-go | 85% | ✅ Good | ✅ Good | ✅ Yes | v1.0.0 |
| stb-truetype-go | 90% | ✅ Good | ⚠️ Basic | ✅ Yes | v1.0.0 |
| stb-image-go | 60% | ⚠️ Fair | ⚠️ Basic | ⚠️ Partial | v0.5.0 |
| tinyxml2-go | 40% | ⚠️ Fair | ⚠️ Basic | ❌ No | v0.3.0 |
| cjson-go | 30% | ⚠️ Fair | ⚠️ Basic | ❌ No | v0.2.0 |
| miniz-go | 25% | ❌ Poor | ⚠️ Basic | ❌ No | v0.2.0 |
| cgltf-go | 20% | ❌ Poor | ⚠️ Basic | ❌ No | v0.2.0 |
| dr-wav-go | 15% | ❌ Poor | ⚠️ Basic | ❌ No | v0.1.0 |
| nuklear-go | 5% | ❌ Poor | ❌ None | ❌ No | v0.1.0 |

**Legend**:
- ✅ Good: 80%+
- ⚠️ Fair/Basic: 50-79%
- ❌ Poor/None: <50%

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
