---
title: "Tests Audit — safeheaders-go"
repo: safeheaders-go
lens: tests
date: 2026-07-06
round: 2
mode: read-only
audience: Claude implementation sessions
---

# Tests Audit — SafeHeaders-Go (Round 2)

**Scope:** Go test coverage across 9 modules (38 `*_test.go` files), CI (`go-ci.yaml`: race, 70% coverage gate, weekly fuzz), `*_audit_test.go` regression suites, BUG_JOURNAL patterns (unbounded `Parse`, `UnlimitedConfig` backstop, size-gated parallel paths, cancellation `select` race, aggregate batch caps). Cross-referenced r1 security (`2026-07-06-r1-safeheaders-go-security.md`, 13 findings).  
**Method:** Read-only grep of test entrypoints vs legacy `Parse`/`ParseParallel`/`Unmarshal` APIs; compare audit tests to open r1 items; review CI fuzz matrix.  
**Prior work:** June–July 2026 added `*_audit_test.go`, decompression bomb tests, `MaxBatchSize` guards, depth-ceiling alignment tests. This round focuses on **go test coverage gaps that leave r1 DoS findings unguarded**.

---

## Executive Summary

| Severity | Count | Top risk |
|----------|------:|----------|
| **HIGH** | 2 | `ParseParallel` → `UnlimitedConfig` has correctness tests but **no security regression** |
| **MEDIUM** | 5 | Legacy `Parse()` size/node gaps; cjson pre-alloc ordering; fuzz matrix omissions |
| **LOW** | 4 | README staleness; legacy `Parser.Parse` token bomb; coverage blind spots on convenience APIs |

**Total verified findings:** 11

CI is **strong** (matrix all 9 modules, `-race`, 70% threshold, gosec, weekly fuzz). The gap is **semantic**: many tests prove parsers work on valid input but do not assert that **documented unsafe entrypoints reject adversarial input** — the exact r1 theme.

---

## Cross-Reference: r1 Security → r2 Test Coverage

| r1 ID | Finding | r2 test status |
|-------|---------|----------------|
| H1 | `ParseParallel` → `UnlimitedConfig` | **OPEN** — `TestParseParallel` checks token equality only (`jsmn_test.go:59-92`) |
| H2 | No absolute backstop in `UnlimitedConfig` | **OPEN** — no test expects rejection of huge input on unlimited config |
| M1 | `tinyxml2` legacy `Parse()` no `MaxInputSize` | **OPEN** — `ParseWithConfig` has size tests; legacy `Parse` does not |
| M2 | `tinyxml2` legacy `Parse()` no `MaxNodeCount` | **OPEN** — no wide-sibling XML test on `Parse()` |
| M3 | `cgltf` `Parse()` no byte cap | **OPEN** — `cgltf_audit_test.go` covers batch count + `ValidateGLTF` refs only |
| M4 | `cjson` alloc before `MaxArrayItems` | **PARTIAL** — `cjson_audit_test.go` checks post-hoc rejection, not alloc ordering |
| M5 | `cjson` O(n) jobs channel | **OPEN** — no memory/regression test for 1M-element channel buffer |
| M6 | `cjson` `Unmarshal*` no byte guard | **OPEN** — fuzz hits `Unmarshal`; no explicit size-cap test |
| M7 | `jsmn` `Parser.Parse` bypasses Config | **OPEN** — quick-start path tested for correctness, not limits |

---

## HIGH

### H1 — `ParseParallel` security regression unguarded (r1 H1)

- **Files:** `jsmn-go/jsmn.go:217-221`; tests: `jsmn_test.go:59-92`, `jsmn_parallel_test.go`
- **Symptom:** `ParseParallel` hard-wires `UnlimitedConfig()` — no input/token caps. r1 HIGH.
- **Root cause:** `TestParseParallel` asserts **output matches** serial parse on ~50-element fixtures. `TestParseWithConfigInputLimit` exercises **configured** path only (`jsmn_parallel_test.go:126-131`). No test fails if `ParseParallel` suddenly honors `DefaultConfig`.
- **Fix direction:** `TestParseParallelUsesBoundedConfig` — pass multi-MB nested JSON to `ParseParallel`; expect `ErrInputTooLarge` or `ErrTooManyTokens` once default is fixed; until fix, document as `// TODO: flip when H1 remediated`.
- **Tags:** `tests` `verified` `dos` `regression` `STILL-PRESENT`

### H2 — `UnlimitedConfig` lacks negative “must still bound” test (r1 H2)

- **Files:** `jsmn-go/config.go:61-67`; contrast: `tinyxml2-go/tinyxml2_audit_test.go:8-21` (depth ceiling even for unlimited)
- **Symptom:** jsmn `UnlimitedConfig` disables all size checks with **no internal ceiling** — unlike tinyxml2's `maxNestingDepth=10000` backstop.
- **Root cause:** `TestConfigValidation` includes `{"unlimited valid", UnlimitedConfig(), false}` (`jsmn_test.go:206`) — treats unlimited as **valid**, not **dangerous**. No adversarial payload expects eventual rejection.
- **Fix direction:** After absolute backstop lands, test `ParseWithConfig(ctx, huge, UnlimitedConfig())` errors before OOM; mirror `tinyxml2_audit_test.go` pattern.
- **Tags:** `tests` `verified` `dos` `STILL-PRESENT`

---

## MEDIUM

### M1 — Legacy `tinyxml2.Parse()` missing size and node-count tests (r1 M1/M2)

- **Files:** `tinyxml2-go/tinyxml2.go:29-67`; tests: `config_test.go` (configured path), `tinyxml2_audit_test.go` (depth only)
- **Symptom:** Wide shallow XML (millions of siblings) and huge byte inputs are unbounded on `Parse()` but guarded on `ParseWithConfig`.
- **Root cause:** `TestParseDepthCeiling` covers nesting on **both** paths; no `TestParseRejectsWideDocument` or `TestParseRejectsOversizeInput` on legacy API.
- **Fix direction:** Parametrize entrypoint tests: `Parse` vs `ParseWithConfig(DefaultConfig())` — same rejection expectations where policy demands.
- **Tags:** `tests` `verified` `dos` `tinyxml2`

### M2 — `cgltf.Parse()` has no single-file byte-size regression (r1 M3)

- **Files:** `cgltf-go/cgltf.go:110-118`; `cgltf_audit_test.go` — batch + validation only
- **Symptom:** One enormous `.gltf` JSON can OOM via stdlib `json.Unmarshal`; batch path has `MaxBatchSize` only.
- **Fix direction:** `TestParseRejectsOversizeGLTF` with generated huge `accessors` array; fails when `ParseWithConfig` lands.
- **Tags:** `tests` `verified` `dos` `cgltf`

### M3 — `cjson` `MaxArrayItems` test does not guard pre-allocation ordering (r1 M4)

- **File:** `cjson-go/cjson_audit_test.go:7-19`
- **Symptom:** `json.Unmarshal` into `[]json.RawMessage` runs **before** item cap check — attacker forces large transient alloc.
- **Root cause:** Test only asserts final error on 10-element array with cap=5; does not measure alloc ordering or cap disabled behavior beyond success path.
- **Fix direction:** After fix, test rejects on byte length pre-scan; optional `testing.AllocsPerRun` regression.
- **Tags:** `tests` `verified` `dos` `cjson`

### M4 — `FuzzParseParallel` excluded from weekly CI fuzz matrix

- **Files:** `jsmn-go/jsmn_fuzz_test.go:64`; `.github/workflows/go-ci.yaml:228-230` (only `FuzzParse`)
- **Symptom:** Parallel unbounded path gets 120s weekly fuzz on serial `FuzzParse` only; `FuzzParseParallel` runs only if author runs locally.
- **Fix direction:** Add matrix entry `{ module: jsmn-go, target: FuzzParseParallel }`.
- **Tags:** `tests` `verified` `ci` `fuzz`

### M5 — `cjson.Unmarshal` / `UnmarshalToMap` lack explicit byte-limit tests (r1 M6)

- **Files:** `cjson-go/cjson.go:17-43`; fuzz: `FuzzUnmarshal` (weekly)
- **Symptom:** Thin wrappers have empty-input check only; no `MaxInputBytes` API tested.
- **Fix direction:** Mirror `jsmn` `TestParseWithConfigInputLimit` for `Unmarshal` once guard exists.
- **Tags:** `tests` `verified` `dos` `cjson`

---

## LOW

### L1 — README security claims not validated by tests (r1 L1/L2)

- **Files:** `tinyxml2-go/README.md:224-225` (stale “no depth limits”); `jsmn-go/README.md` omits `ParseWithConfig`
- **Symptom:** Documentation drift misleads operators; `2026-06-26-101-accuracy-audit` already flagged stale tinyxml2 claim.
- **Fix direction:** Optional `docs_test.go` grep or CI markdown check; not a runtime test gap but contributes to misuse.
- **Tags:** `tests` `verified` `docs`

### L2 — Legacy `Parser.Parse` token growth untested on adversarial nesting

- **Files:** `jsmn-go/jsmn.go:55-105`; `README.md:36-40` quick-start
- **Symptom:** r1 M7 — low-level API bypasses all `Config` limits; examples promote it.
- **Root cause:** Fuzz and unit tests treat `Parser.Parse` as reference implementation, not policy violation.
- **Tags:** `tests` `verified` `api-hazard`

### L3 — 70% coverage gate does not distinguish entrypoint branches

- **File:** `.github/workflows/go-ci.yaml:69-78`
- **Symptom:** Coverage is line-based; legacy `Parse` and `ParseWithConfig` can share helpers such that unsafe branch stays thin but “covered.”
- **Fix direction:** Require `*_audit_test.go` per parser module in CI checklist (jsmn/cjson/tinyxml2/cgltf already have them — extend assertions per M1–M3).
- **Tags:** `tests` `verified` `coverage`

### L4 — `cgltf` `ValidateGLTF` incomplete accessor checks untested as security note (r1 L3)

- **File:** `cgltf-go/cgltf.go:128-181`
- **Symptom:** `ValidateGLTF` passes scene graph but not accessor/bufferView indices — downstream panic risk.
- **Root cause:** `TestValidateGLTFReferences` covers mesh/scene/children; no test for out-of-range `Accessor.BufferView`.
- **Tags:** `tests` `verified` `correctness`

---

## Controls Verified Sound (no finding)

| Control | Location | Notes |
|---------|----------|-------|
| Depth ceiling both tinyxml2 paths | `tinyxml2_audit_test.go:8-21` | Parse + UnlimitedConfig reject >10k nesting |
| `ParseWithConfig` size/node/depth | `tinyxml2-go/config_test.go` | Boundary tests at limit±1 |
| Parallel cancellation no deadlock | `jsmn_parallel_test.go:93-112` | Post-BUG_JOURNAL fix |
| `ctx.Err()` pre-check in workers | BUG_JOURNAL 2026-07-02 | stb-image pattern propagated |
| Aggregate `MaxBatchSize` | `cgltf_audit_test.go:34-52`, stb-image/dr-wav/miniz tests | Per-module `TestMaxBatchSize` |
| Decompression bomb | `miniz-go/miniz_bomb_test.go` | Crafted high-ratio stream |
| dr-wav OOM guards | `dr_wav_security_test.go` | fmt skip + channel zero |
| CI race + fuzz + examples | `go-ci.yaml` | `-race`, weekly fuzz, `make examples` |
| Fuzz token bounds on error | `jsmn_fuzz_test.go` | BUG_JOURNAL pattern applied |

---

## Test / CI Inventory Snapshot

| Module | `*_test.go` files | Audit tests | CI coverage ≥70% | Weekly fuzz |
|--------|-------------------|-------------|------------------|-------------|
| jsmn-go | 5 | partial (parallel) | ✅ | `FuzzParse` only |
| tinyxml2-go | 5 | ✅ depth | ✅ | `FuzzParse` |
| cgltf-go | 4 | batch/validate | ✅ | `FuzzParse` |
| cjson-go | 4 | item cap | ✅ | `FuzzUnmarshal` |
| dr-wav-go | 4 | security | ✅ | `FuzzParse` |
| stb-image-go | 4 | dosguard | ✅ | — |
| miniz-go | 5 | bomb/roundtrip | ✅ | `FuzzExtract` |
| stb-truetype-go | 4 | fuzz font | ✅ | `FuzzLoadFont` |
| linenoise-go | 3 | audit | ✅ | — |

---

## Remediation Priority (for Claude)

1. **P0 — H1:** Add `TestParseParallelRejectsOversizeInput` (fails today; documents r1 H1).
2. **P0 — H2:** Absolute backstop test for jsmn `UnlimitedConfig` (after code fix).
3. **P1 — M1 + M2:** Legacy `Parse` wide/size tests (tinyxml2 + cgltf).
4. **P1 — M4:** Add `FuzzParseParallel` to CI fuzz matrix.
5. **P2 — M3 + M5:** cjson alloc-ordering + `Unmarshal` byte-cap tests.

---

*Read-only audit. No application source modified.*