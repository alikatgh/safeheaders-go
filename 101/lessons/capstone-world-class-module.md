# Capstone · Take a Module to World-Class

> **Objectives:** Tie together every lesson in this course by walking through a
> repeatable checklist that takes a Go package from "it compiles" to
> production-grade — formatted, lint-clean, race-tested, fuzzed, DoS-guarded,
> CI-wired, and audited.  By the end you will have a concrete gate you can apply
> to any Go library before shipping it.
> Estimated time: 30 minutes.

---

## What this actually means (plain English)

- **"It compiles" is table stakes.** A library that compiles but panics on a
  crafted input, deadlocks under cancellation, or leaks memory on a 1 MB input
  is not production-ready.
- **Think of this checklist as a passport control.** Each stamp is a gate. A
  package is allowed to fly only when every stamp is present.
- **The bugs in this repo were not obscure.** The deadlocks in `jsmn-go` and
  `stb-image-go`, the data race in `linenoise-go`, and the stack-overflow in
  `tinyxml2-go` were found by a straightforward audit and standard tooling —
  not by luck.
- **Each gate is cheap relative to the incident it prevents.** A five-second
  `go test -race` run catches races that, in production, corrupt a slice or
  return wrong data to a concurrent caller.
- **Audit is not a replacement for the gates — it is a final check that all
  gates are holding.** When the 10-agent audit ran on this repo it found 25
  issues; all 25 had already been assigned to a gate on this checklist that was
  missing or incomplete.

**Why it matters:** A production library that handles untrusted input must be
verifiably safe, not just probably safe.

---

## The checklist at a glance

```
[ ] 1. gofmt / goimports clean
[ ] 2. go vet passes
[ ] 3. golangci-lint clean (all configured linters)
[ ] 4. Tests pass — table-driven, with sub-tests
[ ] 5. go test -race passes
[ ] 6. Fuzz targets exist with regression seeds
[ ] 7. DoS guards: Config, size caps, depth ceilings, aggregate budgets
[ ] 8. CI wired: every gate runs on every PR
[ ] 9. Audited: independent review, all confirmed findings fixed
```

Work through these in order. Each section below shows what "done" looks like in
this repository.

---

## Gate 1–3: Formatting, vet, and lint

### Format and imports

```bash
# from the repo root — touches every module
go fmt ./...
```

The `Makefile` (from `Makefile`) exposes this as `make fmt`.  It is a no-op on
clean code and a fast sanity check before any commit.

### go vet

```bash
# from Makefile: iterates over all nine modules
make vet
```

The underlying shell loop (from `Makefile`) is:

```bash
for dir in cgltf-go cjson-go dr-wav-go jsmn-go linenoise-go \
           miniz-go stb-image-go stb-truetype-go tinyxml2-go; do
  (cd "$dir" && go vet ./...)
done
```

`go vet` catches a family of bugs at zero cost: wrong `Printf` format strings,
unreachable code, suspicious struct tags, and more.  It is not a substitute for
lint, but it runs in seconds and is always worth running first.

### golangci-lint

The repo ships `.golangci.yml` with an expanded linter set on top of the
default `standard` group.  A few settings worth noting (from `.golangci.yml`):

```yaml
version: "2"

run:
  timeout: 5m
  tests: false          # lint production code; tests are covered by -race

issues:
  max-issues-per-linter: 0   # never silently cap findings
  max-same-issues: 0

linters:
  default: standard           # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - gosec                   # security patterns
    - gocyclo                 # cyclomatic complexity gate
    - gocognit                # cognitive complexity gate
    - wrapcheck               # enforce error wrapping discipline
    - nolintlint              # require-explanation on every //nolint directive
    - prealloc                # flag missing pre-allocations
    - nestif                  # flag deeply nested conditionals
```

`nolintlint` with `require-explanation: true` means every suppression must
justify itself.  This prevents the common pattern where a `//nolint` silences a
real finding and nobody notices.

```bash
make lint
```

!!! warning "Do not cap issues"
    The config sets `max-issues-per-linter: 0` deliberately.  The default
    non-zero cap means CI can show "0 new issues" while hiding dozens of
    pre-existing ones.  Always report the true state.

---

## Gate 4–5: Tests with `-race`

### Table-driven tests

Every module uses the standard Go pattern: a `[]struct{ name, input, want }`
slice iterated with `t.Run(tc.name, ...)`.  Sub-tests let you run a single case
with `-run TestParse/empty_input` and pinpoint failures instantly.

### The race detector

```bash
make test-race
```

From `Makefile`:

```bash
for dir in $(MODULES); do
  (cd "$dir" && go test -race -v ./...)
done
```

The race detector found the `linenoise-go` data race (bug H3 from the audit):
concurrent `AddHistory` and `LoadHistory` callers raced on `defaultState.history`
with no mutex.  The fix was a `sync.Mutex` around every read and write of the
history slice — see [Lesson on the data race](15-data-races-and-mutexes.md) for the full
walkthrough.

!!! note "Try it"
    ```bash
    cd linenoise-go && go test -race -v ./...
    ```
    Expected: all tests pass and `go test` reports no races (the fix is in place).
    Before the fix, `-race` printed a `WARNING: DATA RACE` block pointing to the
    exact lines; after the fix, silence is the success signal.

### The watchdog pattern for deadlock tests

A race test cannot catch a deadlock by itself — the process just hangs.  The
modules that had deadlocks (`jsmn-go` H1, `stb-image-go` H2) now ship watchdog
tests: cancel the context mid-parse, then assert the function returned within
a deadline.

```go
// from jsmn-go — the shape of a watchdog test
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
_, err := jsmn.ParseWithConfig(ctx, largeInput, jsmn.DefaultConfig())
// If the deadlock is present, this line is never reached — the test times out.
if err == nil {
    t.Fatal("expected cancellation error")
}
```

See [Lesson on the deadlock bug](14-the-deadlock-bug.md) for the channel-buffer
root cause and fix.

---

## Gate 6: Fuzz targets with regression seeds

Fuzzing found two crashes in `dr-wav-go`: a slice allocation driven by an
untrusted `size` field in the RIFF header, causing OOM.  After the fix, the
crashing inputs became regression seeds committed under `testdata/fuzz/`.

The `Makefile` runs a smoke-fuzz across all fuzz-capable modules (from
`Makefile`):

```bash
# 15-second smoke run per target — override with FUZZTIME=2m
make fuzz FUZZTIME=30s
```

The underlying commands (from `Makefile`):

```bash
(cd jsmn-go     && go test -run='^$' -fuzz='^FuzzParse$'   -fuzztime=15s .)
(cd tinyxml2-go && go test -run='^$' -fuzz='^FuzzParse$'   -fuzztime=15s .)
(cd dr-wav-go   && go test -run='^$' -fuzz='^FuzzParse$'   -fuzztime=15s .)
(cd miniz-go    && go test -run='^$' -fuzz='^FuzzExtract$' -fuzztime=15s .)
(cd cgltf-go    && go test -run='^$' -fuzz='^FuzzParse$'   -fuzztime=15s .)
(cd cjson-go    && go test -run='^$' -fuzz='^FuzzUnmarshal$' -fuzztime=15s .)
(cd stb-truetype-go && go test -run='^$' -fuzz='^FuzzLoadFont$' -fuzztime=15s .)
```

!!! note "Try it"
    ```bash
    cd dr-wav-go && go test -run='^$' -fuzz='^FuzzParse$' -fuzztime=30s .
    ```
    Expected: the fuzzer runs for 30 seconds and exits cleanly.  The known-bad
    inputs are already in `testdata/fuzz/FuzzParse/` and are replayed as unit
    tests on every ordinary `go test` run — no `-fuzz` flag needed for
    regression coverage.

!!! tip "Seed corpus first, then fuzz"
    Always commit at least one valid input as a seed before fuzzing a new
    target.  The fuzzer mutates seeds; an empty corpus produces mostly garbage
    and finds surface bugs slowly.

---

## Gate 7: DoS guards — Config, caps, and ceilings

This gate is the hardest to retrofit and the easiest to skip.  Every module
in this repo that accepts untrusted input exposes a `Config` type with explicit
resource limits.  The pattern is consistent:

| Module | Config type | Key limits |
|---|---|---|
| `jsmn-go` | `Config` | `MaxInputBytes`, `MaxTokens` |
| `tinyxml2-go` | `Config` | `MaxInputBytes`, `MaxNestingDepth` |
| `miniz-go` | global `MaxDecompressedSize` | per-stream + aggregate cap |
| `stb-image-go` | global `MaxImagePixels` | checked before decode |
| `stb-truetype-go` | `glyphBudget` | component + point caps per glyph |
| `cjson-go` | `MaxArrayItems` | element count cap |

The audit found that several of these caps were present but incomplete.  For
example, `miniz-go`'s `ExtractArchive` capped each zip entry individually but
not the aggregate output — a 10,000-entry archive could produce terabytes even
with the per-stream limit in place (audit finding M5).  The fix tracks a
running total across entries and fails when it exceeds `MaxDecompressedSize`.

Similarly, `tinyxml2-go`'s exported `Parse` function used the unbounded
`parseElement` (audit finding M7).  The fix routes `Parse` through
`ParseWithConfig` with an absolute depth ceiling so a crafted deeply-nested XML
cannot crash the server with an unrecoverable `fatal error: stack overflow` —
one that `recover()` cannot catch.

!!! warning "A `recover()` cannot save you from a stack overflow"
    `fatal error: stack overflow` is a runtime fatal — it kills the entire
    process.  The only defense is not reaching it.  An absolute depth ceiling
    must return an error, not panic, before the stack is exhausted.

**The checklist for Gate 7:**

- Every exported parser/loader has a `Config` or equivalent limit.
- Limits are aggregate, not just per-item (zip entries, array elements, glyph
  components).
- Recursion depth is bounded by an integer counter, not a Go call stack.
- The `UnlimitedConfig` or equivalent exists for trusted callers, but the
  default is conservative.

---

## Gate 8: CI — every gate runs on every PR

The repo's `.github/workflows/go-ci.yaml` runs a matrix of jobs:

| Job | What it runs |
|---|---|
| `test` | `make test` |
| `race` | `make test-race` |
| `lint` | `make lint` |
| `security` | `gosec` + `govulncheck` |
| `fuzz` | `make fuzz FUZZTIME=15s` (smoke) |
| `examples` | `make examples` |
| `build` | matrix of OS/arch |

Weekly scheduled runs extend the fuzz time to catch slower-emerging crashes.

The `Makefile` `ci` target replicates what Actions runs, so you can gate-check
locally before pushing (from `Makefile`):

```bash
make ci        # lint + test-race + test-coverage + security
make pre-commit # fmt + vet + test-race (faster, for the commit hook)
```

!!! note "Try it"
    ```bash
    make pre-commit
    ```
    Expected: format check, `go vet`, and race-detector tests all pass, then
    `✅ Pre-commit checks passed!` is printed.  If any step fails, the make
    target exits non-zero — wire this into your git pre-commit hook.

---

## Gate 9: Audit — an independent review

After all eight mechanical gates are clean, a fresh set of eyes (or agents)
reads the code looking for issues the automated tools miss.  The 2026-06-23
audit of this repo (from `docs/audits/2026-06-23-code-review-security-audit.md`)
used 10 reviewers working in parallel and an adversarial verifier who attempted
to reproduce every reported finding before recording a verdict.

**Confirmed findings by severity:**

| Severity | Count |
|---|---:|
| Critical | 0 |
| High | 5 |
| Medium | 10 |
| Low | 8 |
| Info | 2 |
| **Total** | **25** |

The audit report opens with a key observation worth quoting directly:

> *"No memory-corruption, slice-out-of-range, nil-deref, or integer-overflow bug
> was found in any module.  The confirmed defects fall into two buckets: (a)
> concurrency hazards in hand-rolled worker pools and shared global state, and (b)
> DoS-shaped resource-exhaustion gaps (missing aggregate caps, unbounded recursion,
> unsynchronized-size allocations) consistent with the repo's stated
> untrusted-input threat model."*

This is the audit telling you exactly which gates were incomplete: Gates 5
(race detector) and 7 (DoS guards).  Every High finding maps back to one of
those two gates.

**What the five High findings were and which gate failed:**

| Finding | Gate that should have caught it |
|---|---|
| H1 `jsmn-go` deadlock on context cancellation | Gate 5 — watchdog test missing |
| H2 `stb-image-go` deadlock under cancellation | Gate 5 — watchdog test missing |
| H3 `linenoise-go` data race on global history | Gate 5 — `-race` not in CI |
| H4 `stb-truetype-go` billion-laughs composite glyphs | Gate 7 — per-component budget only |
| H5 `Dockerfile` missing `linenoise-go` module | Gate 8 — docker build not in CI |

Every finding was fixed with a regression test committed in the same PR as the
fix.  The audit report itself lives in `docs/audits/` so future reviewers can
see the history without re-deriving it.

!!! tip "Audit findings are a gift"
    Each finding is a lesson about which gate needs tightening.  An audit with
    zero findings on a non-trivial library means either the library is very well
    hardened or the audit was not thorough.  Either way, the gate checkboxes tell
    you which.

---

## Putting it together: the full quality ladder

The `Makefile` exposes the full ladder as single commands (from `Makefile`):

```bash
make fmt        # Gate 1 — format
make vet        # Gate 2 — go vet
make lint       # Gate 3 — golangci-lint
make test       # Gate 4 — table tests
make test-race  # Gate 5 — race detector
make fuzz       # Gate 6 — fuzz smoke
# Gate 7 is in your code, not a make target
make ci         # Gates 3+5+4+security in one shot
```

The install step for all tools used across gates:

```bash
make install-tools
# installs: golangci-lint v2.2.2, gosec v2.21.4, govulncheck v1.1.4, benchstat
```

The single `MODULES` variable in the `Makefile` is the one source of truth for
which modules are in scope — all targets iterate it:

```makefile
# from Makefile
MODULES := cgltf-go cjson-go dr-wav-go jsmn-go linenoise-go \
           miniz-go stb-image-go stb-truetype-go tinyxml2-go
```

Adding a new module means adding it to this list and all nine gates apply
automatically.

---

## Key takeaways

- **The nine-gate checklist is cumulative.** Skipping Gate 5 (`-race`) means
  Gate 9 (audit) will find concurrency bugs that should have been caught for
  free by `go test -race`.
- **DoS guards must be aggregate, not just per-item.** Per-entry caps on zip
  decompression, array items, and glyph components are all necessary but not
  sufficient — you need a running total across all items too.
- **Watchdog tests are the only way to test for deadlocks.** A test that hangs
  is not a failure until you add a timeout or a watchdog goroutine that fails
  the test when the deadline passes.
- **Regression seeds from fuzzing are permanent tests.** Once the fuzzer finds
  a crash and you commit the input to `testdata/fuzz/`, every future `go test`
  run replays that input — no `-fuzz` flag required.
- **An audit tells you which gates are thin.** The five High findings in this
  repo all pointed back to Gates 5 and 7.  Fix the gate, not just the finding.
