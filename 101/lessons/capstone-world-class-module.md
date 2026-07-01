# Capstone · Take a Module to World-Class

> **Objectives:** Tie together every lesson in this course by walking through a
> repeatable checklist that takes a Go package from "it compiles" to
> production-grade — formatted, lint-clean, race-tested, fuzzed, DoS-guarded,
> CI-wired, and audited.  By the end you will have a concrete gate you can apply
> to any Go library before shipping it.
> Estimated time: 30 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **"It compiles" is table stakes** = "a car that starts is not a car that is road-legal." Compiling proves syntax; it says nothing about panics on crafted input, deadlocks under cancellation, or memory exhaustion on a 1 MB payload.
- **The nine-gate checklist** = "a passport with nine required stamps — the plane does not depart until every box is inked." Each gate (format, vet, lint, tests, race, fuzz, DoS caps, CI, audit) is an independent barrier; missing one lets a whole class of defect through.
- **The bugs were not obscure** = "they were the equivalent of an unlocked front door, not a secret tunnel." The deadlocks in `jsmn-go` and `stb-image-go`, the data race in `linenoise-go`, and the stack-overflow in `tinyxml2-go` all fell to standard tooling applied methodically — no exotic knowledge required.
- **Each gate is cheap relative to the incident it prevents** = "a smoke detector costs less than the fire brigade." A five-second `go test -race` run catches data races that, in production, silently corrupt a slice or return wrong data to a concurrent caller.
- **Audit is a final check, not a replacement for the gates** = "a building inspector who finds problems the blueprint review was supposed to prevent." When the 10-agent audit ran on this repo it found 25 issues; all 25 mapped back to a gate on this checklist that was missing or incomplete — fix the gate, not just the finding.

**Why it matters:** A production library that handles untrusted input must be
verifiably safe, not just probably safe.

**See it — the nine quality gates a package must clear before shipping.**

<svg viewBox="0 0 700 400" role="img" aria-labelledby="txcapsto dxcapsto" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="txcapsto">Nine quality gates from "it compiles" to production-grade</title>
  <desc id="dxcapsto">A left-to-right flow diagram showing a Go package passing through nine gates: fmt/vet/lint, tests, race detector, fuzz, DoS guards, CI, and audit, arriving at a "Ship it" checkmark.</desc>
  <defs>
    <marker id="xcapsto-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="currentColor"/>
    </marker>
  </defs>
  <rect x="10" y="170" width="90" height="44" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="55" y="188" text-anchor="middle" font-size="10" fill="currentColor">Go package</text>
  <text x="55" y="202" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">"it compiles"</text>
  <line x1="100" y1="192" x2="122" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="124" y="162" width="72" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="160" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G1–3</text>
  <text x="160" y="194" text-anchor="middle" font-size="9" fill="currentColor">fmt / vet</text>
  <text x="160" y="207" text-anchor="middle" font-size="9" fill="currentColor">lint</text>
  <line x1="196" y1="192" x2="218" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="220" y="162" width="68" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="254" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G4</text>
  <text x="254" y="194" text-anchor="middle" font-size="9" fill="currentColor">table tests</text>
  <text x="254" y="207" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">sub-tests</text>
  <line x1="288" y1="192" x2="310" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="312" y="162" width="68" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="346" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G5</text>
  <text x="346" y="194" text-anchor="middle" font-size="9" fill="currentColor">-race</text>
  <text x="346" y="207" text-anchor="middle" font-size="9" fill="currentColor">watchdog</text>
  <line x1="380" y1="192" x2="402" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="404" y="162" width="66" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="437" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G6–7</text>
  <text x="437" y="194" text-anchor="middle" font-size="9" fill="currentColor">fuzz</text>
  <text x="437" y="207" text-anchor="middle" font-size="9" fill="currentColor">DoS caps</text>
  <line x1="470" y1="192" x2="492" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="494" y="162" width="60" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="524" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G8</text>
  <text x="524" y="194" text-anchor="middle" font-size="9" fill="currentColor">CI</text>
  <text x="524" y="207" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">every PR</text>
  <line x1="554" y1="192" x2="576" y2="192" stroke="currentColor" stroke-width="1.4" marker-end="url(#xcapsto-arrow)"/>
  <rect x="578" y="162" width="60" height="60" rx="6" fill="none" stroke="currentColor" stroke-width="1.4"/>
  <text x="608" y="180" text-anchor="middle" font-size="10" font-weight="bold" fill="currentColor">G9</text>
  <text x="608" y="194" text-anchor="middle" font-size="9" fill="currentColor">audit</text>
  <text x="608" y="207" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">independent</text>
  <line x1="638" y1="192" x2="658" y2="192" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8" marker-end="url(#xcapsto-arrow)"/>
  <circle cx="676" cy="192" r="18" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="676" y="197" text-anchor="middle" font-size="15" fill="#fff">✓</text>
  <text x="160" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">static</text>
  <text x="254" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">correctness</text>
  <text x="346" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">concurrency</text>
  <text x="437" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">safety</text>
  <text x="524" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">automation</text>
  <text x="608" y="150" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">verification</text>
  <line x1="346" y1="222" x2="346" y2="310" stroke="#e5484d" stroke-width="1.2" stroke-dasharray="4,3"/>
  <line x1="346" y1="310" x2="160" y2="310" stroke="#e5484d" stroke-width="1.2" stroke-dasharray="4,3" marker-end="url(#xcapsto-arrow)"/>
  <text x="253" y="303" text-anchor="middle" font-size="9" fill="#e5484d">✗ race / deadlock found → fix + rerun</text>
  <text x="676" y="225" text-anchor="middle" font-size="9" fill="var(--md-accent-fg-color,#00897b)">ship it</text></svg>

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

**In plain terms:** this is a nine-item checklist. Each `[ ]` is one box to
tick before the code is trustworthy enough to ship. The rest of this lesson
walks through each item one at a time.

Work through these in order. Each section below shows what "done" looks like in
this repository. (A "repository," or "repo," is just the folder-plus-history
that holds all the project's code — think of it as the project's filing
cabinet, tracked by a version-control tool so every past change is saved.)

---

## Gate 1–3: Formatting, vet, and lint

### Format and imports

```bash
# from the repo root — touches every module
go fmt ./...
```

**In plain terms:** this command automatically rewrites every source file in
the project so the spacing and layout follow one consistent house style — no
human has to nitpick indentation by hand. A "module," here, is one
self-contained chunk of the codebase — like one folder that does one job and
can be built and shared on its own (you'll see several named below, like
`jsmn-go` and `tinyxml2-go`).

The [`Makefile`](src/makefile.md) (from `Makefile`) exposes this as `make fmt`.  It is a no-op on
clean code and a fast sanity check before any commit. (A `Makefile` is a
plain-text list of named shortcuts for common commands — typing `make fmt`
just runs the longer command written next to that name, so nobody has to
remember or retype it. A "commit" is a saved, timestamped snapshot of the
project's files, recorded so you can always go back to it later.)

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

**In plain terms:** this repeats the same check — "go into this module's
folder and run `go vet`" — once for each of the nine modules in the list, so
every one of them gets vetted, not just the first.

`go vet` catches a family of bugs at zero cost: wrong `Printf` format strings,
unreachable code, suspicious struct tags, and more.  It is not a substitute for
lint, but it runs in seconds and is always worth running first. (A "struct" is
a custom data shape you define — a bundle of named, labeled slots grouped
under one name, similar to a form with labeled fields. A "tag" on a struct is
a small piece of text attached to one of those slots that tells other tools
how to treat it. "Lint" — short for a linter/linting tool — is a program that
reads your source code without running it and flags patterns that are legal
but risky or sloppy, the way a spell-checker flags a word that's technically
real but probably wrong here.)

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

**In plain terms:** this is a configuration file — a settings document, not
program logic — that turns on a specific set of automated checks
("linters") beyond the default set, each one hunting for a different kind of
risky pattern (insecure code, overly tangled logic, missing memory
pre-reservations, and so on). "Pre-allocation," mentioned by the `prealloc`
linter, means reserving a chunk of the computer's memory for something up
front, once, instead of asking for a little more memory repeatedly as the
work grows — the single upfront reservation is faster.

`nolintlint` with `require-explanation: true` means every suppression must
justify itself.  This prevents the common pattern where a `//nolint` silences a
real finding and nobody notices. (A `//nolint` is a one-line comment a
programmer adds directly above a piece of code to tell the linter "ignore
this specific warning here" — useful when the warning is a false alarm, risky
when used to hide a real problem.)

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
with `-run TestParse/empty_input` and pinpoint failures instantly. (A "test,"
in programming, is a small piece of code that runs another piece of code with
a known input and automatically checks whether the result matches what's
expected — it replaces a human manually re-checking behavior every time
something changes. Here, `[]struct{ name, input, want }` is a list — Go calls
an ordered list a "slice" — where each entry is one of those labeled-field
struct bundles: a test's name, the input to feed in, and the answer ("want")
it should produce. Running one such list entry through the same check is
called a "table-driven test," and each individual run of it is a "sub-test.")

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

**In plain terms:** this runs every module's tests again, but with a special
"race detector" watchdog switched on that watches for two pieces of code
touching the same shared data at the same time in an unsafe way (explained
just below).

The race detector found the `linenoise-go` data race (bug H3 from the audit):
concurrent `AddHistory` and `LoadHistory` callers raced on `defaultState.history`
with no mutex.  The fix was a `sync.Mutex` around every read and write of the
history slice — see [Lesson on the data race](15-data-races-and-mutexes.md) for the full
walkthrough. (Modern programs often do several things at once — separate
independent "workers," in Go called goroutines, running concurrently. A "data
race" happens when two of these workers read and write the same piece of
memory at the same time with no coordination, so the result depends on
unpredictable timing and can silently corrupt data. A "mutex" — short for
mutual exclusion, and `sync.Mutex` is Go's built-in version — is a lock: only
one worker may hold it at a time, so whoever has it can safely touch the
shared data while every other worker trying to grab the same lock simply
waits — pauses there, doing nothing else — until it's released.)

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

The repo's [`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md) runs a matrix of jobs:

| Job | What it runs |
|---|---|
| `test` | `go test -race` + coverage gate |
| `lint` | `golangci-lint` v2 |
| `security` | `gosec` + `govulncheck` |
| `benchmark` | benchmarks posted on PRs |
| `fuzz` | per-target fuzzing (schedule + dispatch) |
| `examples` | `make examples` |
| `build` | matrix of OS / arch |

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
| H5 [`Dockerfile`](src/dockerfile.md) missing `linenoise-go` module | Gate 8 — docker build not in CI |

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
