# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **stb-image-go, dr-wav-go, cgltf-go, miniz-go**: `MaxBatchSize` (default
  10,000) caps the number of files each module's batch entry point
  (`LoadBatchConcurrent`, `ParseBatch` ×2, `CreateArchiveConcurrent`) accepts
  per call. Per-item guards already existed on some of these paths
  (`MaxImagePixels`, `MaxDecompressedSize`), but none capped the *count* of
  items in a batch — many small-but-valid items could still exhaust
  memory/CPU in aggregate. Found by grepping every `Batch`/`Parallel` entry
  point in the codebase after fixing the first instance in stb-image-go.
- **miniz-go**: streaming `CompressStream(dst, src)` / `DecompressStream(dst, src)`
  for large files — DEFLATE through an `io.Reader`/`io.Writer` without buffering
  the whole payload. `DecompressStream` honors `MaxDecompressedSize`. (Complements
  the existing stream APIs: cjson `UnmarshalStream`/`MarshalStream`, stb-image
  `LoadStream`.)
- **stb-truetype-go**: a real pure-Go glyf rasterizer replaces the previous
  placeholder. It parses the sfnt tables (head/maxp/hhea/loca/cmap/glyf/hmtx),
  resolves runes via cmap (formats 0/4/6/12), decodes simple and composite
  glyph outlines, flattens the quadratic Béziers, and scan-fills them with
  nonzero-winding anti-aliasing. The module is now Stable; `LoadFontFromBytes`
  validates the font and `FuzzLoadFont` guards the parser against malformed input.

### Fixed
- **miniz-go**: added a decompression-bomb guard — `DecompressData` and
  `ExtractArchive` now cap output at `MaxDecompressedSize` (default 256 MiB; set 0
  to disable) via `io.LimitReader`, so a small crafted stream (zeros compress
  ~1000x) can no longer inflate to gigabytes and exhaust memory.
- **stb-image-go**: added a decode-bomb guard — `Load` now rejects images whose
  header-declared dimensions exceed `MaxImagePixels` (default 64 MP; set 0 to
  disable) before the full `image.Decode`, so a tiny malicious file can no longer
  force a multi-gigabyte allocation. `LoadBatchConcurrent` inherits the guard.
- **dr-wav-go**: fixed two fuzz-found crashes on malformed input — a divide-by-zero
  in `GetSampleCount` (zero `NumChannels`) and an OOM in `Parse` (unbounded
  `make` from the fmt-chunk size); added `FuzzParse` + regression seed.
- **cgltf-go**: `ValidateGLTF` no longer rejects a valid scene-less glTF.
- **miniz-go**: `CreateArchiveConcurrent` double-compressed every file (pre-deflated
  data written into a `zip.Deflate` entry), producing archives that did not
  round-trip. Rebuilt with `zip.CreateRaw`; added a round-trip regression test.
- **dr-wav-go**: `Parse` allocated an attacker-controlled size from the WAV data
  chunk header (OOM DoS); the allocation is now capped to the bytes available.
  `Serialize` guards against payloads too large for the 32-bit RIFF size fields.
- **linenoise-go**: command-history Up/Down navigation was an unimplemented TODO
  stub; it now works (with draft-line stashing). Non-TTY reads no longer drop a
  final line that lacks a trailing newline.
- **golangci-lint config** was declared `version: "2"` but used v1-style
  `linters-settings:`, which is invalid for v2 — all linter settings were
  silently ignored and the v1.61 CI runner could not parse it. Rewritten as a
  valid, verified v2 config; CI/Makefile/release now install golangci-lint v2.
- All 9 modules are now genuinely lint-clean (golangci-lint, 0 issues) and gofmt
  clean; benchmarks moved into `*_test.go` so `testing` is no longer linked into
  production binaries; `linenoise-go` added to the CI test/lint/security matrices.
- Raised test coverage above the 70% gate on the two modules that were below it
  (`linenoise-go` 21%→77%, `jsmn-go` 54%→88%).

### Added
- SECURITY.md with vulnerability reporting guidelines
- CODE_OF_CONDUCT.md based on Contributor Covenant 2.1
- CHANGELOG.md for tracking changes
- GitHub templates for issues and pull requests
- Dependabot configuration for automated dependency updates
- Fuzz tests for parser modules (jsmn-go, tinyxml2-go)
- Input validation and configurable limits to prevent DoS attacks
- Comprehensive examples directory with real-world use cases
- Testdata directory with benchmark files
- Security scanning in CI/CD (gosec)
- Coverage threshold enforcement in CI (70% minimum)
- Integration tests across modules
- Root-level go.mod for easier imports
- Versioning and release automation

### Changed
- Updated golangci-lint to v2 (valid v2 config) in CI, Makefile, and release
- Standardized error handling across all modules using errors.Join
- Improved CI/CD pipeline with security scanning and coverage requirements
- Enhanced README with production-ready status badges
- Updated module documentation with input limits and security considerations

### Fixed
- Inconsistent error handling across modules
- Missing input size limits (DoS vulnerability)
- golangci-lint runner/config mismatch — a v2-schema config was paired with a
  v1.61 runner that could not parse it; both are now on v2 (see Fixed, above)

### Security
- Added configurable input size limits to all parser modules
- Added context timeout support for long-running operations
- Documented security best practices in SECURITY.md
- Added DoS prevention mechanisms

## [0.5.0] - 2025-10-31

### Added
- Comprehensive READMEs for all stable modules
- Module maturity badges (Alpha, Beta, Stable)
- Extended CI to test all 9 modules

### Changed
- Removed nuklear-go stub (was only 5% complete)
- Marked incomplete modules with appropriate status badges

### Fixed
- Failing context cancellation test in stb-image-go
- CI only testing jsmn-go (now tests all modules)
- Go version mismatch (standardized on 1.23)

## [0.4.0] - 2025-10-15

### Added
- dr-wav-go: WAV audio file parsing with concurrent decoding
- cgltf-go: glTF 3D model loading with parallel assets
- miniz-go: ZIP compression with concurrent chunking
- cjson-go: JSON marshaling/unmarshaling with parallel processing
- tinyxml2-go: XML DOM parsing

### Changed
- Improved parallel chunking strategies in jsmn-go
- Enhanced error messages across all modules

## [0.3.0] - 2025-09-20

### Added
- stb-image-go: Image loading with batch decoding (PNG, JPEG, GIF)
- stb-truetype-go: TrueType font parsing with LRU glyph cache
- Benchmarking suite with comparison to C libraries
- Context support for cancellation in long-running operations

### Changed
- Improved memory allocation patterns
- Optimized worker pool implementation

## [0.2.0] - 2025-08-10

### Added
- jsmn-go: JSON tokenizer with parallel parsing
- Go workspace setup with go.work
- Comprehensive test suite with race detector
- CI/CD pipeline with GitHub Actions
- golangci-lint configuration

### Changed
- Reorganized project into independent modules
- Added detailed CONTRIBUTING.md guidelines

## [0.1.0] - 2025-07-01

### Added
- Initial project structure
- README with project overview
- MIT License
- Basic module structure
- ISSUES.md for tracking known issues

---

## Version Naming Convention

We use [Semantic Versioning](https://semver.org/):

- **Major version (X.0.0)**: Breaking API changes
- **Minor version (0.X.0)**: New features, backward compatible
- **Patch version (0.0.X)**: Bug fixes, backward compatible

## Module Stability Levels

- **🔴 Alpha (0.x.x)**: Incomplete, API may change, not production-ready
- **🟡 Beta (0.x.x)**: Core features complete, API stabilizing, use with caution
- **🟢 Stable (1.x.x+)**: Production-ready, API stable, full test coverage

## Current Module Versions

| Module | Version | Status | Next Release |
|--------|---------|--------|--------------|
| jsmn-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q1 2026) |
| stb-truetype-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q1 2026) |
| stb-image-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q1 2026) |
| tinyxml2-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q2 2026) |
| cjson-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q2 2026) |
| miniz-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q2 2026) |
| cgltf-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q2 2026) |
| dr-wav-go | 0.5.0 | 🟢 Stable | 1.0.0 (Q2 2026) |

---

**Maintainer**: @alikatgh
**Last Updated**: 2025-11-16

[Unreleased]: https://github.com/alikatgh/safeheaders-go/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/alikatgh/safeheaders-go/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/alikatgh/safeheaders-go/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/alikatgh/safeheaders-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/alikatgh/safeheaders-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/alikatgh/safeheaders-go/releases/tag/v0.1.0
