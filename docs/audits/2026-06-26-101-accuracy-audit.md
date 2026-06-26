# SafeHeaders-Go 101 — Accuracy Audit

**Date:** 2026-06-26
**Scope:** All 33 documents of the "SafeHeaders-Go 101" course (`101/lessons/` + index, glossary, capstone, and Labs A–D).
**Method:** Per-lesson citation verification against the live repository source. Each lesson was scored on three axes — does it teach genuine Go (`teachesGo`), how is it grounded (`grounding`), and how many cited claims survived verification (`citationsChecked` vs `citationsValid`).

---

## Bottom line

**Is the course all about Go?** Yes. 32 of 33 documents teach genuine Go language or tooling concepts — goroutines, channels, `select`, `sync.Mutex`, `context` cancellation, the race detector, native fuzzing (`testing.F`), `io.Reader`/`Writer`, `encoding/binary`, modules/workspaces, error wrapping, table-driven tests, and CI. The single exception is the course landing page (`index.md`), which is an orientation page that teaches no Go directly (correctly classified `teachesGo: no`). No lesson teaches fake or incorrect Go semantics. There are no "partial" lessons.

**Is it all about how THIS project was built?** Overwhelmingly, yes. 29 of 33 documents are **project-specific** — they dissect this repository's actual production code (real functions, structs, constants, line ranges, audit findings, and real bugs from the git history). 3 documents are **general-anchored-to-repo** (the glossary and two of the four hands-on labs — Lab A and Lab B — teach a general pattern but anchor every claim back to real repo code, with clearly-labelled synthetic exercise scaffolding). 1 document is **non-code** (the index). **Zero lessons are generic-toy** — no lesson presents invented/textbook code as if it were the repository's.

**Was any fabrication found?** No file paths were fabricated and no function/struct/constant names were invented anywhere in the course. The grounding is genuine. However, **the course is not error-free**: 414 citations were checked and 390 verified clean (94.2%), leaving **28 issues — 3 high, 14 medium, 11 low**. The high-severity issues are concentrated in the "Try it" exercise blocks of the rasterizer lesson and the capstone, where readers are told to run test/fuzz commands that reference **non-existent symbols** (`TestGlyphBudget`, `FuzzRasterize`, a standalone `race` CI job) — these commands silently match nothing, so a reader following along gets false confirmation. The medium issues are mostly inverted-logic descriptions, simplified code snippets that drop error handling, wrong test-name references, and a miscount of CI jobs (six claimed, seven actual). None of these undermine the core grounding claim, but the runnable-command errors should be fixed before the course is considered authoritative.

**Verdict:** The course delivers on both promises — it is a real Go course, taught almost entirely through how this specific project was built and hardened — but it needs a correction pass on its runnable commands and a handful of stale line numbers and inverted descriptions.

---

## Totals

| Metric | Value |
|---|---|
| Total documents | 33 |
| Teaches Go — yes / partial / no | 32 / 0 / 1 |
| Grounding — project-specific | 29 |
| Grounding — general-anchored-to-repo | 3 |
| Grounding — generic-toy | 0 |
| Grounding — non-code | 1 |
| Citations checked (sum) | 414 |
| Citations valid (sum) | 390 |
| Citation accuracy | 94.2% |
| Issues — high | 3 |
| Issues — medium | 14 |
| Issues — low | 11 |
| Issues — total | 28 |

---

## Per-lesson table

| Lesson | Teaches Go | Grounding | Issues |
|---|---|---|---|
| 01 — why pure-Go ports | yes | project-specific | 1 |
| 02 — modules and workspaces | yes | project-specific | — |
| 03 — errors and wrapping | yes | project-specific | — |
| 04 — io Reader/Writer | yes | project-specific | 1 |
| 05 — tokenizing JSON (jsmn) | yes | project-specific | 2 |
| 06 — wrapping the stdlib | yes | project-specific | 1 |
| 07 — binary parsing RIFF/WAV | yes | project-specific | 1 |
| 08 — truetype 1: sfnt tables | yes | project-specific | — |
| 09 — truetype 2: glyph outlines | yes | project-specific | — |
| 10 — truetype 3: rasterizing | yes | project-specific | 2 |
| 11 — goroutines, channels, select | yes | project-specific | — |
| 12 — worker pools (fan-out/fan-in) | yes | project-specific | — |
| 13 — context cancellation | yes | project-specific | 1 |
| 14 — the deadlock bug | yes | project-specific | 3 |
| 15 — data races and mutexes | yes | project-specific | 1 |
| 16 — DoS threat modelling | yes | project-specific | 2 |
| 17 — untrusted size fields (WAV) | yes | project-specific | — |
| 18 — decode and decompression bombs | yes | project-specific | — |
| 19 — recursion and billion-laughs | yes | project-specific | 1 |
| 20 — configurable-limits pattern | yes | project-specific | — |
| 21 — table tests, race, coverage | yes | project-specific | 1 |
| 22 — Go fuzzing for binary parsers | yes | project-specific | — |
| 23 — round-trip / property testing | yes | project-specific | — |
| 24 — Go tooling fundamentals | yes | project-specific | — |
| 25 — CI/CD pipeline | yes | project-specific | 1 |
| 26 — audit war story | yes | project-specific | 5 |
| capstone — world-class module | yes | project-specific | 1 |
| glossary | yes | general-anchored-to-repo | — |
| index | no | non-code | — |
| Lab A — bounds-checked binary parser | yes | general-anchored-to-repo | — |
| Lab B — worker pool channel sizing | yes | general-anchored-to-repo | 2 |
| Lab C — TeeReader / limits | yes | project-specific | 1 |
| Lab D — Go fuzzing | yes | project-specific | 1 |

---

## High- and medium-severity issues

### HIGH (3)

1. **Lesson 10 (rasterizing) — phantom test name.** Lesson (line 364) instructs `go test -v -run TestGlyphBudget ./...`, but no `TestGlyphBudget` exists in `stb-truetype-go`. The real test is `TestCompositeBudgetAborts` (`sfnt_test.go:102`). The `-run` filter matches nothing, so the command silently runs 0 tests and gives no confirmation the guard works.

2. **Lesson 10 (rasterizing) — phantom fuzz target.** Lesson (line 375) instructs `go test -fuzz=FuzzRasterize -fuzztime=30s ./...`, but no `FuzzRasterize` exists. The only fuzz entry point in the module is `FuzzLoadFont` (`truetype_fuzz_test.go:7`). The command errors / matches nothing.

3. **Capstone (world-class module) — fabricated CI job.** Gate 8's CI table (line 281) lists a standalone `race` job running `make test-race`. No such job exists in `.github/workflows/go-ci.yaml` (jobs are: test, lint, security, benchmark, fuzz, examples, build). The `-race` flag is embedded inside the `test` job at line 63, not a separate job.

### MEDIUM (14)

4. **Lesson 01 — wrong version claim.** Lesson states all nine modules are at v0.5.0 and Stable; in fact `linenoise-go` is at v0.1.0 per `README.md` line 107. The other eight are v0.5.0.

5. **Lesson 04 — simplified Unmarshal body.** The `cjson-go/cjson.go` snippet shows `return json.Unmarshal(data, v)`, but the real code wraps the error: `if err := json.Unmarshal(data, v); err != nil { return fmt.Errorf("unmarshal error: %w", err) }; return nil`.

6. **Lesson 05 (jsmn) — phantom test name.** The "Try it" output references `--- PASS: TestParallelVsSerial`; no such test exists. The actual test is `TestParallelTokensMatchSerial` (`jsmn_parallel_test.go:64`).

7. **Lesson 06 (wrapping the stdlib) — inverted check order.** Lesson claims the config limit fires before the hard ceiling, so `ErrNestingTooDeep` reaches the caller first. The actual code in `parseElementLimited` (`tinyxml2-go/tinyxml2.go:128`) checks the hard ceiling first (line 128), then the config limit (line 131) — order is reversed.

8. **Lesson 07 (dr-wav) — phantom test name.** The "Try it" block expects `TestGetSampleCount`; no such test exists. `GetSampleCount` exists (`dr_wav.go:163-168`) but has no dedicated unit test.

9. **Lesson 13 (context cancellation) — wrong test name.** Lesson tells readers to run `TestLoadBatchCancelMidParse`; the real test is `TestLoadBatchConcurrent_Cancellation` (`stb_image_test.go:146`). The named test matches nothing.

10. **Lesson 14 (deadlock) — watchdog attributed to wrong file (jsmn).** Lesson says the watchdog test is in `jsmn-go/jsmn_test.go`; it actually lives in `jsmn-go/jsmn_parallel_test.go` (`TestParseParallelCancellationNoDeadlock`, lines 93-112). `jsmn_test.go` has only a pre-cancellation test.

11. **Lesson 14 (deadlock) — watchdog attributed to wrong file (stb-image).** Lesson says the watchdog test is in `stb-image-go/stb_image_test.go`; it actually lives in `stb-image-go/stb_image_audit_test.go` (lines 11-29). `stb_image_test.go` has only a pre-cancellation test.

12. **Lesson 25 (CI/CD) — wrong job count.** Lesson says "The six jobs at a glance" and tables six jobs (test, lint, security, benchmark, fuzz, build). The real `go-ci.yaml` defines seven — the `examples` job (runs `make examples`, no matrix) is omitted.

13. **Lesson 26 (audit war story) — wrong fix description (M7 tinyxml2).** Lesson says "Parse now delegates to ParseWithConfig with a hard internal ceiling", but `Parse()` (`tinyxml2.go:54`) still calls `parseElement()` directly. The actual fix added a `maxNestingDepth=10000` guard inside `parseElement` itself.

14. **Lesson 26 (audit war story) — wrong line cited for mesh check (M1).** Lesson cites `cgltf.go:153` for the negative mesh-index check, but line 153 is a scene-index error return. The mesh check is at lines 169-174 (`node.Mesh < 0 || node.Mesh >= len(gltf.Meshes)`).

15. **Lab B (worker pool sizing) — wrong variable name/type.** Step 5 states the jsmn parallel path allocates `results := make([]Token, numJobs)`. The real code (`parallel.go:170`) is `jobResults := make([]chunkResult, numJobs)` — wrong name and wrong element type.

16. **Lab B (worker pool sizing) — wrong cancel-pattern attribution.** The comparison table claims both jsmn-go and stb-image-go use a `ctx.Err()` check before `select`. In jsmn-go's `chunkWorker` the `select` is bare (`case <-ctx.Done()`) with no pre-select `ctx.Err()` guard; only stb-image-go performs `if err := ctx.Err(); err != nil` before its select.

17. **Lab D (fuzzing) — de-templated CI snippet.** The CI YAML is presented as a direct quote with hardcoded `dr-wav-go` step name, working-directory, and artifact name/path. The actual `go-ci.yaml` (lines 253-262) uses a matrix strategy with `${{ matrix.module }}` / `${{ matrix.target }}` template variables — the lesson shows a reconstruction, not the literal file text.

---

## Low-severity issues (11, summarized)

- **Lesson 05:** duplicate broken cross-refs — both "Lesson 06" and "Lesson 07" links point to `14-the-deadlock-bug.md` with wrong lesson numbers.
- **Lesson 14:** watchdog code snippet is a pedagogical paraphrase (two goroutines) of the real test (cancel from main goroutine), presented as "the repo's tests follow this pattern".
- **Lesson 15:** claims a "dedicated race job" in `go-ci.yaml`; `-race` actually runs as a step inside the single `test` matrix job (line 63).
- **Lesson 16 (×2):** `checkPixelLimit` error string drops the trailing `(adjust MaxImagePixels)`; `maxNestingDepth` const shown as if inside `parseElement` when it is a package-level const above it.
- **Lesson 19:** `parseElementLimited` snippet omits the `nodeCount *int` parameter (shows 4 of 5 params, `// ...` implies truncation).
- **Lesson 21:** claims `t.TempDir()` is called exactly twice in `linenoise_engine_test.go`; it appears 5 times (lines 17, 30, 266, 285, 301).
- **Lesson 26 (×3):** stale line numbers — `var defaultState` cited at 581 (actual 631); `errs` channel cited at 106 (actual 109); ParentIdx rebase cited at 156-159 (actual 194-195).
- **Lab C:** `ExtractArchive` snippet shows `rc, _ := f.Open()` discarding the error; real code (`miniz.go:117`) is `rc, err := f.Open()` with a proper error return — also a poor Go-teaching example.

---

## Recommendations

1. **Fix the three high-severity runnable commands first** (Lessons 10 ×2, Capstone). These actively mislead a reader following along: `TestGlyphBudget` → `TestCompositeBudgetAborts`, `FuzzRasterize` → `FuzzLoadFont`, and remove/relabel the standalone `race` CI row.
2. **Sweep all "Try it" blocks for test/fuzz names** (Lessons 05, 07, 13 also have phantom names) — these are the highest-frequency error class and the most damaging to learner trust.
3. **Re-derive line-number citations** in Lesson 26 (and spot-check others) — they have drifted since the fixes landed.
4. **Correct the two inverted descriptions** (Lesson 06 check order, Lesson 26 M7 fix) — these teach the wrong mental model even though the code is real.
5. **Restore dropped error handling** in simplified snippets (Lessons 04 and Lab C) so the course never models error-swallowing as idiomatic Go.

---

## Remediation (applied 2026-06-26, same change as this update)

All actionable findings were fixed against verified repo ground truth, and a
follow-up regex-aware sweep caught five phantom commands the per-lesson audit had
missed.

**High (3/3 fixed):** L10 `TestGlyphBudget`→`TestCompositeBudgetAborts`, L10
`FuzzRasterize`→`FuzzLoadFont` (and `./...`→`.` for single-package fuzzing), capstone
CI table rebuilt to the seven real jobs (no standalone `race` job; `-race` is a step
in `test`).

**Medium (fixed):** L01 version (linenoise-go is v0.1.0), L04 restored the real
`fmt.Errorf("unmarshal error: %w", …)` wrapping, L05 `TestParallelVsSerial`→
`TestParallelTokensMatchSerial`, L07 `TestGetSampleCount`→`TestWAV_GetSampleCount`
(the audit was wrong that it was untested), L13 `TestLoadBatchCancelMidParse`→
`TestLoadBatchConcurrent_Cancellation`, L14 watchdog tests re-attributed to
`jsmn_parallel_test.go`/`stb_image_audit_test.go`, L25 six→seven jobs (+`examples`),
L26 M7 description rewritten (the fix is a `maxNestingDepth=10000` guard inside
`parseElement`, not `Parse`→`ParseWithConfig`), L26 M1 line 153→170, Lab B variable
`results := make([]Token, …)`→`jobResults := make([]chunkResult, …)` and the cancel-check
row (jsmn uses a bare `select`; only stb-image guards with `ctx.Err()` first), Lab D CI
snippet shown as the real `matrix` template.

**Low (fixed):** L05 two broken cross-refs (→ L12/L14), L15 "dedicated race job"→"`-race`
step in the `test` job", L16 error string restored `(adjust MaxImagePixels)`, L21 `t.TempDir`
count, L26 stale line numbers (`stb_image.go` 106→109, `linenoise.go` 581→631, `tinyxml2.go`
205→195, `parallel.go` 156-159→194-195). Also fixed a **leaked home-directory path** in L07
(`cd /Users/.../dr-wav-go`→`cd dr-wav-go`).

**Phantom commands the audit MISSED (found by the follow-up sweep, fixed):** L09 (second
copy of `TestGlyphBudget`), L06 & L19 `-run TestNesting` (matches no tinyxml2 test →
`-run 'Nesting|DepthCeiling'`), L23 `-run TestCreateArchiveConcurrentRoundTrip`→
`TestConcurrentArchiveRoundTrip`, L23 `-fuzz=FuzzParallelEqualsSerial`→`FuzzParseConsistency`,
and L15 `-run TestHistory`→`-run History` (the old filter excluded
`TestGlobalHistoryConcurrent`, so the lesson's "revert the mutex to see a race" claim would
have silently run no concurrent test).

**Findings reclassified as FALSE POSITIVES (left unchanged, with reason):**
- **L06 check order** — the lesson describes correct *observable* behavior: with a configured
  limit (e.g. 1000) far below the 10000 hard ceiling, `ErrNestingTooDeep` is what the caller
  sees. The audit misread code-line order as behavior order.
- **L19 `parseElementLimited` params** — the snippet already shows `nodeCount *int`; its `// ...`
  is intentional truncation.
- **L16 `maxNestingDepth` const placement** — the lesson correctly shows the package-level const
  above the `parseElement` check, matching the source.

**Verification:** `mkdocs build --strict` passes; an automated sweep confirms every non-lab
`-run`/`-fuzz` command now matches a real repo test (or is the intentional `-run='^$'`
"run-no-unit-tests" idiom). Net: the three audit recommendations about phantom commands,
drifted line numbers, and dropped error handling are resolved; recommendation 4's "L06 check
order" item was a false positive.
