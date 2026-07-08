# SafeHeaders-Go — Parser Entrypoint Security Audit (Round 1)

**Audit ID:** `2026-07-06-r1`  
**Date:** 2026-07-06  
**Scope:** Parser entrypoints in `tinyxml2-go`, `jsmn-go`, `cgltf-go`, `cjson-go`  
**Mode:** Read-only (no code changes)  
**Cross-reference:** Prior audit `2026-06-23-code-review-security-audit.md` (findings M7, L5, L6, M2, M3); `docs/BUG_JOURNAL.md` patterns on unbounded `Parse()`, `ParseParallel` + `UnlimitedConfig`, and legacy vs `ParseWithConfig` split.

---

## Executive summary

This round re-audited **only** the four parser libraries named above, with emphasis on **DoS limits** and the **legacy vs configured entrypoint** split introduced after the June 2026 audit.

**Remediation since 2026-06-23 (verified in source):**

| Prior finding | Module | Status on 2026-07-06 |
|---------------|--------|----------------------|
| M7 — unbounded `Parse()` stack overflow | tinyxml2-go | **Fixed** — absolute `maxNestingDepth=10000` in `parseElement` (`tinyxml2.go:193-198`) |
| L5 — `UnlimitedConfig` bypassed depth guard | tinyxml2-go | **Fixed** — same absolute ceiling in `parseElementLimited` (`tinyxml2.go:129-133`) |
| L6 — recursive `FindDeep`/`FindAllDeep` | tinyxml2-go | **Fixed** — iterative stack traversal (`tinyxml2.go:274-311`) |
| M2 — `UnmarshalArrayParallel` O(n) amplification | cjson-go | **Partially fixed** — `MaxArrayItems` guard added (`cjson.go:87,101-104`) but first-pass `json.Unmarshal` still allocates before the check |
| M3 — `ParseParallel` + `UnlimitedConfig` | jsmn-go | **Open** — `ParseParallel` still hard-wires `UnlimitedConfig()` (`jsmn.go:217-221`) |
| H1 — parallel cancellation deadlock | jsmn-go | **Fixed** — `resultsCh` buffer `numJobs+numWorkers` (`parallel.go:55`) + `ctx.Err()` pre-check (`parallel.go:132-135`) |

**Overall posture:** Configured paths (`ParseWithConfig`, `MaxArrayItems`, `MaxBatchSize`) are materially stronger than legacy convenience APIs. The highest residual risk is **callers using documented “quick start” entrypoints** (`Parse`, `ParseParallel`, `Unmarshal`) on untrusted input without wrapping size checks themselves. No memory-corruption or stack-overflow fatal was found in current code; remaining issues are **resource-exhaustion and documentation gaps**.

### Finding counts

| Severity | Count |
|----------|------:|
| High | 2 |
| Medium | 6 |
| Low | 4 |
| Info | 1 |
| **Total** | **13** |

---

## Entrypoint matrix (legacy vs configured)

| Module | Legacy / convenience API | Limits on legacy path | Configured / guarded API | Limits on configured path |
|--------|--------------------------|----------------------|--------------------------|---------------------------|
| **tinyxml2-go** | `Parse(data)` | Depth only (`maxNestingDepth=10000`); **no** input size or node count | `ParseWithConfig(data, cfg)` | `MaxInputSize`, `MaxNodeCount`, `MaxNestingDepth` + absolute depth ceiling |
| **jsmn-go** | `NewParser` + `Parser.Parse`; `ParseParallel` | **None** (`ParseParallel` → `UnlimitedConfig`) | `ParseWithConfig(ctx, data, cfg)`; `ParseParallelWithContext` → `DefaultConfig` | `MaxInputSize`, `MaxTokens`; parallel path also honors `ctx` |
| **cgltf-go** | `Parse(data)` | **None** (relies on `encoding/json`) | *No `ParseWithConfig`* — only batch aggregate cap | `ParseBatch` → `MaxBatchSize` (file count, not bytes) |
| **cjson-go** | `Unmarshal`, `UnmarshalToMap`, `UnmarshalToSlice` | **None** | `UnmarshalArrayParallel` → `MaxArrayItems`; `UnmarshalStream` documented caller-limit | Item count cap (post-unmarshal); stream requires caller `LimitReader` |

---

## Confirmed findings

Ordered by severity. Every location was verified against source on 2026-07-06.

### High

#### H1 — `jsmn-go`: `ParseParallel` hard-wires `UnlimitedConfig` (no input/token caps)

- **Location:** `jsmn-go/jsmn.go:217-221`
- **Category:** DoS / resource exhaustion
- **What:** `ParseParallel` delegates to `parseParallelWithConfig(context.Background(), json, UnlimitedConfig())`. `UnlimitedConfig` sets `MaxInputSize: 0` and `MaxTokens: 0` (`jsmn-go/config.go:61-67`), disabling both guards.
- **Trigger:** Untrusted large JSON (e.g. multi‑MB `[[[...` nesting or a huge top-level array) passed to `ParseParallel`. Token slice grows via `allocToken` append (`jsmn.go:115-118`) without cap.
- **Contrast:** `ParseParallelWithContext` uses `DefaultConfig()` (`parallel.go:212-213`) — 100 MB / 1 M token limits. README quick-start and `examples/jsmn-demo/main.go:46` promote the unbounded path.
- **Cross-ref:** Same shape as prior audit M3 / `SECURITY.md:50-57` worked example.
- **Suggested fix:** Default `ParseParallel` to `DefaultConfig()` (breaking change) or add `ParseParallelWithConfig`; deprecate unlimited default in docs.

#### H2 — `jsmn-go`: `UnlimitedConfig` has no absolute backstop for tokens/input (unlike tinyxml2 depth ceiling)

- **Location:** `jsmn-go/config.go:61-67`; enforcement gated at `config.go:93-95`, `config.go:187-189`, `parallel.go:47-49`
- **Category:** DoS / design gap
- **What:** When `MaxInputSize` or `MaxTokens` is `0`, all size checks are skipped. Unlike `tinyxml2-go`, which enforces `maxNestingDepth=10000` even for `UnlimitedConfig` (`tinyxml2.go:129-133`), jsmn has **no internal ceiling** that survives a zero config.
- **Trigger:** `ParseWithConfig(ctx, data, UnlimitedConfig())` on adversarial input — unbounded memory for tokens and input retention.
- **Cross-ref:** `BUG_JOURNAL.md` pattern: *"an unlimited (0) config value must still hit an internal absolute ceiling."*
- **Suggested fix:** Add package-level absolute caps (e.g. `maxTokensAbsolute`, `maxInputAbsolute`) checked even when config fields are zero.

### Medium

#### M1 — `tinyxml2-go`: legacy `Parse()` accepts unbounded input size

- **Location:** `tinyxml2-go/tinyxml2.go:29-67` (no size check); contrast `tinyxml2.go:82-84` (`config.validateInput`)
- **Category:** DoS / OOM
- **What:** `Parse` builds a full in-memory DOM with no `MaxInputSize`. Only nesting is capped at 10 000 levels.
- **Trigger:** Multi‑GB XML within available RAM — decoder and DOM allocation proceed until process memory is exhausted.
- **Suggested fix:** Document loudly (README is partially stale — see L1) or have `Parse` delegate to `ParseWithConfig(DefaultConfig())`.

#### M2 — `tinyxml2-go`: legacy `Parse()` has no `MaxNodeCount` guard

- **Location:** `tinyxml2-go/tinyxml2.go:29-57` → `parseElement` at `tinyxml2.go:195-223` (unlimited `append` to `Children`)
- **Category:** DoS / OOM
- **What:** A **wide** document (millions of sibling elements under one parent) stays under the depth ceiling but allocates one `*Node` per element with no counter. `parseElementLimited` enforces `MaxNodeCount` (`tinyxml2.go:138-141`); `parseElement` does not.
- **Trigger:** Shallow XML with ~10⁶ `<item/>` siblings — large heap, no error.
- **Suggested fix:** Share node-count tracking in `parseElement` or route `Parse` through `ParseWithConfig`.

#### M3 — `cgltf-go`: `Parse()` has no byte-size limit and no `ParseWithConfig` pattern

- **Location:** `cgltf-go/cgltf.go:110-118`
- **Category:** DoS / OOM
- **What:** Single-file entrypoint calls `json.Unmarshal` directly with no `len(data)` cap. Unlike batch parsing (`ParseBatch` + `MaxBatchSize` at `cgltf.go:251-254`), there is no symmetric single-file guard.
- **Trigger:** One enormous `.gltf` JSON (huge `accessors` / `bufferViews` arrays) — stdlib allocates full structure.
- **Suggested fix:** Add `MaxInputSize` / `ParseWithConfig` mirroring jsmn-go and tinyxml2-go.

#### M4 — `cjson-go`: `UnmarshalArrayParallel` allocates `[]json.RawMessage` before `MaxArrayItems` check

- **Location:** `cjson-go/cjson.go:96-104`
- **Category:** DoS / memory amplification
- **What:** First `json.Unmarshal(data, &rawArray)` (`:96-99`) materializes a slice with one `RawMessage` header per array element. `MaxArrayItems` is checked only **after** that allocation (`:101-104`).
- **Trigger:** ~20 MB body `[0,0,0,...]` (~10 M elements) — millions of slice slots allocated; then rejected by cap (if enabled). Attacker still forces large transient allocation.
- **Cross-ref:** Prior audit M2 — guard added but ordering leaves a residual vector.
- **Suggested fix:** Pre-scan / `json.Decoder` token count, or reject `len(data)` before full array decode; stream elements without materializing full `rawArray`.

#### M5 — `cjson-go`: `UnmarshalArrayParallel` uses O(n) `jobs` channel buffer

- **Location:** `cjson-go/cjson.go:118-125`
- **Category:** DoS / memory
- **What:** After passing `MaxArrayItems`, `jobs := make(chan int, len(rawArray))` and a loop sends every index (`:123-125`). For `MaxArrayItems = 1<<20` (default), this commits up to ~8 MB of channel buffer plus worker state, in addition to `results` slice (`:116`).
- **Trigger:** Valid 1 M-element array within cap — high fixed memory before any per-item unmarshaling.
- **Suggested fix:** Fixed worker-buffered channel (size `numWorkers`) with a producer goroutine, matching jsmn-go's `jobCh` pattern (`parallel.go:38-42`).

#### M6 — `cjson-go`: core `Unmarshal` / `UnmarshalToMap` / `UnmarshalToSlice` lack byte-size guards

- **Location:** `cjson-go/cjson.go:17-43`
- **Category:** DoS / OOM
- **What:** All thin wrappers call `json.Unmarshal` with only an empty-input check. No `MaxInputBytes` or `ParseWithConfig`-style API exists in this module.
- **Trigger:** `Unmarshal(hugeJSON, &v)` — full decode into caller type with no library-level cap.
- **Suggested fix:** Export `MaxInputBytes` + check in `Unmarshal`, or document mandatory caller-side limits in README (currently absent).

#### M7 — `jsmn-go`: low-level `Parser.Parse` bypasses all `Config` limits

- **Location:** `jsmn-go/jsmn.go:55-105`; `jsmn-go/config.go:142-149` (`ParserWithConfig` wraps but is not used by `ParseParallel`)
- **Category:** DoS / API hazard
- **What:** `Parser.Parse` tokenizes with unbounded `allocToken` growth. README quick-start (`README.md:36-40`) teaches this path. Only `ParseWithConfig` applies `MaxInputSize` / `MaxTokens`.
- **Trigger:** `NewParser(10).Parse(multiMBNestedJSON)` from example code — no limits.
- **Suggested fix:** Document that `Parser.Parse` is for trusted input only; steer untrusted use to `ParseWithConfig`.

### Low

#### L1 — `tinyxml2-go`: README claims `Parse` has no depth limits (stale; code enforces 10 000)

- **Location:** `tinyxml2-go/README.md:224-225` vs `tinyxml2-go/tinyxml2.go:193-198`
- **Category:** Documentation / security misdirection
- **What:** Limitations section states *"`Parse` applies no size/depth limits"* but `parseElement` returns an error at depth > `maxNestingDepth` (10 000). Size and node-count claims remain true.
- **Risk:** Operators may underestimate depth protection on legacy `Parse`, or over-trust it for size/node limits.
- **Cross-ref:** `docs/audits/2026-06-26-101-accuracy-audit.md` item 13 (same stale claim).
- **Suggested fix:** Update README to: depth hard-capped at 10 000; no input size / node count on `Parse`.

#### L2 — `jsmn-go`: README omits `ParseWithConfig` / `StrictConfig`; promotes `ParseParallel`

- **Location:** `jsmn-go/README.md:56-70`, `jsmn-go/README.md:122-123`
- **Category:** Documentation
- **What:** Parallel section documents only `ParseParallel` (unbounded). No mention of `ParseWithConfig`, `DefaultConfig`, `StrictConfig`, or `ParseParallelWithContext`.
- **Risk:** Copy-paste deployments parse untrusted JSON without caps.
- **Suggested fix:** Add a “Untrusted input” section mirroring `tinyxml2-go` and `SECURITY.md`.

#### L3 — `cgltf-go`: `ValidateGLTF` still omits accessor / bufferView / buffer / primitive checks

- **Location:** `cgltf-go/cgltf.go:128-133` (explicit doc omission); `cgltf.go:156-181` (only scene/node/mesh/children)
- **Category:** Correctness / downstream DoS
- **What:** Function name implies structural validation, but `Primitive.Attributes`, `Indices`, `Material`, `Accessor.BufferView`, and `BufferView.Buffer` are never range-checked. Prior audit L1; partially improved for mesh/scene refs (M1 fixed).
- **Trigger:** `ValidateGLTF` passes; caller indexes `gltf.Accessors[9999]` → panic in consumer code.
- **Suggested fix:** Extend validation or rename to `ValidateSceneGraph` and document scope.

#### L4 — `cgltf-go`: `MaxBatchSize` disable convention (`0`) removes aggregate batch guard

- **Location:** `cgltf-go/cgltf.go:207`, `cgltf.go:251-254`
- **Category:** Configuration hazard
- **What:** Setting `MaxBatchSize = 0` disables the only DoS guard on `ParseBatch`. Same pattern exists in peer modules (documented), but cgltf has **no** per-file byte cap to fall back on.
- **Trigger:** `MaxBatchSize = 0` + `ParseBatch(ctx, millionSmallGLTFs)` — unbounded concurrent `json.Unmarshal`.
- **Suggested fix:** Separate “unlimited” from “disabled” or require explicit `DisableBatchLimit` bool.

### Info

#### I1 — `tinyxml2-go`: configured vs legacy depth indexing aligned (positive verification)

- **Location:** `tinyxml2-go/tinyxml2.go:54-57` (`Parse` root depth=1); `tinyxml2.go:107` (`ParseWithConfig` root depth=1); `tinyxml2.go:129-136` (shared ceiling)
- **Category:** Verification note
- **What:** Both entrypoints start depth at 1 and hit `maxNestingDepth` at the same nesting level. Regression tests exist (`tinyxml2_audit_test.go:8-21`). This closes the post-M7 divergence noted in `BUG_JOURNAL.md` (2026-06-25 follow-up).

---

## Per-module summary

| Module | Legacy risk | Configured risk | Priority action |
|--------|-------------|-----------------|-----------------|
| **tinyxml2-go** | Size + node count unbounded on `Parse` | Low — `ParseWithConfig` robust | Update README; consider defaulting `Parse` to `DefaultConfig` |
| **jsmn-go** | **`ParseParallel` + `Parser.Parse` unbounded** | Low — `ParseWithConfig` / `ParseParallelWithContext` | Fix `ParseParallel` default; add absolute backstop to `UnlimitedConfig` |
| **cgltf-go** | Single-file parse unbounded | Batch count only | Add `ParseWithConfig` + input byte cap |
| **cjson-go** | `Unmarshal*` unbounded | Partial — item cap after first alloc | Reorder/limit before `json.Unmarshal` of full array |

---

## Methodology

1. Read `docs/BUG_JOURNAL.md` and prior audit remediation notes.
2. Grep all `Parse`, `ParseWithConfig`, `ParseParallel`, `UnlimitedConfig`, `Unmarshal*`, `ParseBatch` entrypoints in the four modules.
3. Line-by-line comparison of legacy vs configured code paths.
4. Cross-check regression tests in `*_audit_test.go` files.
5. No code changes; no dynamic reproduction runs in this round (static verification only).

---

## References

- `docs/BUG_JOURNAL.md` — unbounded recursion, `UnlimitedConfig` backstop, aggregate batch caps
- `docs/audits/2026-06-23-code-review-security-audit.md` — M7, L5, L6, M2, M3 baseline
- `ISSUES.md` §5 — project tracker claiming “fully implemented” limits (this audit narrows that claim to **configured** entrypoints)
- `SECURITY.md` — documents `ParseParallel` token-bomb scenario (still applicable)

*Next round: see `docs/audits/ROUND-STATUS.md`.*