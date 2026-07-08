# Security Audit Round Status

Tracker for read-only and remediation audit rounds across `safeheaders-go`.

**Last updated:** 2026-07-06

---

## Round index

| Round ID | Date | Scope | Mode | Report | Findings | Status |
|----------|------|-------|------|--------|----------|--------|
| `2026-07-06-r2` | 2026-07-06 | **go test coverage gaps** vs r1 parser DoS findings (4 modules + CI fuzz matrix) | Read-only | [2026-07-06-r2-safeheaders-go-tests.md](./2026-07-06-r2-safeheaders-go-tests.md) | 11 (2 High, 5 Medium, 4 Low) | 🟡 Open — test gaps documented |
| `2026-06-23` | 2026-06-23 | All 9 modules + infra | Review + verify + **remediate** | [2026-06-23-code-review-security-audit.md](./2026-06-23-code-review-security-audit.md) | 25 confirmed (+1 uncertain) | ✅ Remediated (per audit footer) |
| `2026-07-06-r1` | 2026-07-06 | **Parser entrypoints only:** `tinyxml2-go`, `jsmn-go`, `cgltf-go`, `cjson-go` | Read-only | [2026-07-06-r1-safeheaders-go-security.md](./2026-07-06-r1-safeheaders-go-security.md) | 13 (2 High, 6 Medium, 4 Low, 1 Info) | 🟡 Open — findings documented, no fixes this round |

---

## Cross-reference: prior audit themes (2026-06-23 → 2026-07-06)

Themes requested for re-check in round `2026-07-06-r1` (unbounded `Parse()`, `ParseParallel` + `UnlimitedConfig`, legacy vs `ParseWithConfig`):

| Theme | Module | 2026-06-23 ID | 2026-07-06 status |
|-------|--------|---------------|-------------------|
| Unbounded recursive `Parse()` stack overflow | tinyxml2-go | M7, L5, L6 | ✅ **Fixed** — `maxNestingDepth=10000`; iterative `FindDeep` |
| `ParseParallel` uses `UnlimitedConfig` | jsmn-go | M3 (related) | ❌ **Still open** — `jsmn.go:221` |
| `UnlimitedConfig` without hard backstop | tinyxml2-go | L5 | ✅ **Fixed** (tinyxml2); ❌ **Still open** (jsmn-go) |
| `UnmarshalArrayParallel` memory amplification | cjson-go | M2 | 🟡 **Partial** — `MaxArrayItems` added; pre-check alloc remains |
| No `ParseWithConfig` on glTF / cJSON single-file parse | cgltf-go, cjson-go | — | ❌ **Still open** |

---

## Open findings from `2026-07-06-r1` (remediation backlog)

### High (fix first)

| ID | Module | Summary | Location |
|----|--------|---------|----------|
| H1 | jsmn-go | `ParseParallel` → `UnlimitedConfig`, no size/token caps | `jsmn-go/jsmn.go:217-221` |
| H2 | jsmn-go | `UnlimitedConfig` lacks absolute token/input ceiling | `jsmn-go/config.go:61-67` |

### Medium

| ID | Module | Summary | Location |
|----|--------|---------|----------|
| M1 | tinyxml2-go | `Parse()` no `MaxInputSize` | `tinyxml2-go/tinyxml2.go:29-67` |
| M2 | tinyxml2-go | `Parse()` no `MaxNodeCount` | `tinyxml2-go/tinyxml2.go:195-223` |
| M3 | cgltf-go | `Parse()` no byte-size cap / no `ParseWithConfig` | `cgltf-go/cgltf.go:110-118` |
| M4 | cjson-go | `[]json.RawMessage` allocated before `MaxArrayItems` | `cjson-go/cjson.go:96-104` |
| M5 | cjson-go | O(n) `jobs` channel buffer | `cjson-go/cjson.go:118-125` |
| M6 | cjson-go | `Unmarshal*` no byte-size guard | `cjson-go/cjson.go:17-43` |
| M7 | jsmn-go | `Parser.Parse` bypasses `Config` | `jsmn-go/jsmn.go:55-105` |

### Low / docs

| ID | Module | Summary | Location |
|----|--------|---------|----------|
| L1 | tinyxml2-go | README stale on `Parse` depth limits | `tinyxml2-go/README.md:224-225` |
| L2 | jsmn-go | README omits `ParseWithConfig` | `jsmn-go/README.md:56-70` |
| L3 | cgltf-go | `ValidateGLTF` incomplete reference checks | `cgltf-go/cgltf.go:128-181` |
| L4 | cgltf-go | `MaxBatchSize=0` disables only batch guard | `cgltf-go/cgltf.go:251-254` |

---

## Open test gaps from `2026-07-06-r2` (priority)

| ID | Module | Summary |
|----|--------|---------|
| TST-R2-H1 | jsmn-go | `ParseParallel` → `UnlimitedConfig` — no security regression test |
| TST-R2-H2 | jsmn-go | `UnlimitedConfig` lacks absolute-backstop negative test |
| TST-R2-M1 | tinyxml2-go | Legacy `Parse()` — no size / wide-sibling tests |
| TST-R2-M2 | cgltf-go | Single-file `Parse()` — no byte-size cap test |
| TST-R2-M4 | CI | `FuzzParseParallel` not in weekly fuzz matrix |

Full list: [2026-07-06-r2-safeheaders-go-tests.md](./2026-07-06-r2-safeheaders-go-tests.md).

---

## Suggested next round

| Round ID | Proposed scope | Goal |
|----------|----------------|------|
| `2026-07-06-r3` | Remediate r1 H1/H2 + add TST-R2-H1/H2 tests | Default safe `ParseParallel`; absolute backstop |
| `2026-07-xx-r1` | `dr-wav-go`, `stb-image-go`, `miniz-go`, `stb-truetype-go` | Repeat entrypoint / DoS limit audit on remaining parsers |

---

## How to update this file

1. Add a row to **Round index** when a new audit completes.
2. Move findings from **Open** to **Closed** with fix commit SHA when remediated.
3. Keep cross-reference table aligned with `docs/BUG_JOURNAL.md` patterns.