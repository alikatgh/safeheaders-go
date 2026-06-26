# 24 · gofmt, vet, golangci-lint and the Makefile

> **Objectives:** Understand the three-layer Go quality toolbelt (format, vet,
> lint) and how SafeHeaders-Go wires them together in a single Makefile.
> Learn what each `.golangci.yml` entry catches, why the v1→v2 schema change
> matters, and how to run every check locally in one command.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **gofmt** = "the one spell-checker the whole office agreed to use." Go's community settled on a single layout; `gofmt` rewrites your source to match it automatically, so code review is never about tabs versus spaces.
- **go vet** = "a second pair of eyes that reads the code after it compiles." It finds mistakes the compiler allows but that are almost certainly bugs — mismatched `printf` verbs, copied `sync.Mutex` values, unreachable code.
- **golangci-lint** = "a panel of specialist reviewers who all file their notes in one report." It runs dozens of analysers in a single pass, understanding Go modules, and produces one consolidated list of findings.
- **`.golangci.yml`** = "the written rulebook the whole team signs." Without it, every developer's local lint run produces different warnings; with it, every run — local or CI — checks the exact same set of rules.
- **The `MODULES` variable** = "the master guest list for every party." Adding a new module to that one line in the Makefile automatically includes it in every target — `lint`, `vet`, `test`, `fuzz` — with no copy-paste drift.
- **A Makefile target** = "a shortcut button on the microwave." `make lint` is easier to remember and type than the multi-flag `golangci-lint run` command it expands to, and it is literally the same command CI calls.

**Why it matters:** a linter caught the unused-variable that led to the silent
int-overflow; vet caught the race-prone global before the `-race` test exposed
it at runtime. Tools find bugs before users do.

**See it — three-layer quality toolbelt: fmt → vet → lint in sequence.**

<svg viewBox="0 0 700 220" role="img" aria-labelledby="t24 d24" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t24">Go quality toolbelt: gofmt, go vet, golangci-lint</title>
  <desc id="d24">Three boxes in sequence — gofmt (format), go vet (static analysis), golangci-lint (meta-linter) — connected by arrows, all driven by a Makefile target above them. A final arrow leads to a CI pass checkmark.</desc>
  <defs>
    <marker id="l24-arrow" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L8,3.5 L0,7 Z" fill="var(--md-default-fg-color,#333)"/>
    </marker>
  </defs>

  <!-- Makefile label at top -->
  <rect x="240" y="12" width="220" height="32" rx="6" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2" stroke-dasharray="5,3"/>
  <text x="350" y="32" text-anchor="middle" font-size="12" fill="var(--md-default-fg-color,#333)">Makefile: make all / make ci</text>

  <!-- Downward connector from Makefile to row -->
  <line x1="350" y1="44" x2="350" y2="68" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2" marker-end="url(#l24-arrow)"/>

  <!-- Box 1: gofmt -->
  <rect x="40" y="74" width="150" height="72" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="115" y="100" text-anchor="middle" font-size="13" font-weight="600" fill="var(--md-accent-fg-color,#00897b)">gofmt</text>
  <text x="115" y="118" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">Rewrites layout.</text>
  <text x="115" y="132" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">Idempotent.</text>

  <!-- Arrow gofmt → vet -->
  <line x1="191" y1="110" x2="268" y2="110" stroke="var(--md-default-fg-color,#333)" stroke-width="1.4" marker-end="url(#l24-arrow)"/>

  <!-- Box 2: go vet -->
  <rect x="270" y="74" width="160" height="72" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="350" y="100" text-anchor="middle" font-size="13" font-weight="600" fill="var(--md-accent-fg-color,#00897b)">go vet</text>
  <text x="350" y="118" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">Catches compiler-legal</text>
  <text x="350" y="132" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">but wrong code.</text>

  <!-- Arrow vet → golangci-lint -->
  <line x1="431" y1="110" x2="508" y2="110" stroke="var(--md-default-fg-color,#333)" stroke-width="1.4" marker-end="url(#l24-arrow)"/>

  <!-- Box 3: golangci-lint -->
  <rect x="510" y="74" width="160" height="72" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="590" y="100" text-anchor="middle" font-size="13" font-weight="600" fill="var(--md-accent-fg-color,#00897b)">golangci-lint</text>
  <text x="590" y="118" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">Many analysers,</text>
  <text x="590" y="132" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,#888)">one config file.</text>

  <!-- Arrow all → CI pass -->
  <line x1="590" y1="147" x2="590" y2="172" stroke="var(--md-default-fg-color,#333)" stroke-width="1.4" marker-end="url(#l24-arrow)"/>
  <rect x="510" y="174" width="160" height="32" rx="6" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="590" y="194" text-anchor="middle" font-size="12" font-weight="600" fill="#fff">CI pass</text>
</svg>

---

## The MODULES variable — one list to rule them all

SafeHeaders-Go is a Go workspace with nine independent modules. The Makefile
starts with a single variable (from [`Makefile`](src/makefile.md)):

```makefile
MODULES := cgltf-go cjson-go dr-wav-go jsmn-go linenoise-go miniz-go \
           stb-image-go stb-truetype-go tinyxml2-go
```

Every target that needs to iterate — `test`, `lint`, `vet`, `fuzz` — loops
over `$(MODULES)`. Add a new module to that one line and every target picks it
up automatically. This is the "single source of truth" pattern: no copy-paste
drift between CI and local dev.

The loop pattern looks like this (from `Makefile`, the `vet` target):

```makefile
vet:
	@for dir in $(MODULES); do \
		(cd $$dir && go vet ./...) || exit 1; \
	done
```

The `|| exit 1` is important: if any module fails, the whole loop stops
immediately and returns a non-zero exit code to CI. No silent partial success.

---

## gofmt — the formatter

`gofmt` ships with Go and rewrites source files in place. The Makefile wraps
it as (from `Makefile`, the `fmt` target):

```makefile
fmt:
	@go fmt ./...
```

`go fmt` is a thin wrapper around `gofmt` that works with the module system.
Running it is idempotent — running it twice changes nothing.

!!! note "Try it"
    From the repo root:

    ```bash
    make fmt
    ```

    Expected output: `✅ Code formatted!` — and no file changes if the repo is
    already clean. If any file was reformatted, `git diff` will show the
    whitespace-only delta.

golangci-lint also enforces formatting via its `formatters` section — so even
if you forget `make fmt`, `make lint` will catch it.

---

## go vet — the compiler's safety net

`go vet` performs deeper analysis than the compiler without actually running
the code. Common catches:

| What vet checks | Real mistake it prevents |
|-----------------|--------------------------|
| `Printf` verb matches argument type | `fmt.Sprintf("%d", str)` silently prints `%!d(string=…)` |
| Struct literal has correct field count | `Point{1, 2, 3}` when `Point` has two fields |
| `sync.Mutex` copied by value | Copying a mutex breaks its invariant (see [Lesson 17](15-data-races-and-mutexes.md)) |
| `context.WithCancel` result unused | Leaked goroutines |

Run it in isolation (from `Makefile`):

```makefile
vet:
	@for dir in $(MODULES); do \
		(cd $$dir && go vet ./...) || exit 1; \
	done
```

!!! warning "vet vs. compiler"
    The compiler rejects programs that cannot possibly work. `go vet` rejects
    programs that *compile* but are almost certainly wrong. Both must pass —
    vet is not optional even when tests are green.

---

## golangci-lint v2 — the config file in full

### The schema version bump: v1 → v2

The config at `.golangci.yml` opens with:

```yaml
version: "2"
```

This is not a lint version — it is the **config-schema version**. golangci-lint
v2 introduced a new YAML layout (top-level `linters`, `formatters`,
`exclusions` sections) that is incompatible with the v1 layout (flat
`linters.enable-all`, `linters-settings`, `issues` keys). If you copy a v1
config into a v2 binary you get cryptic parse errors; the schema version field
makes the binary fail fast with a clear message instead.

To verify your config at any time:

```bash
golangci-lint config verify --config .golangci.yml
```

### The linter baseline

`.golangci.yml` starts from the curated `standard` default set:

```yaml
linters:
  default: standard
  enable:
    - bodyclose
    - containedctx
    ...
```

`default: standard` includes `errcheck`, `govet`, `ineffassign`, `staticcheck`,
and `unused`. These are the "you should never ship without these" linters.
Everything in the `enable:` list is an addition on top.

### The linters and why they were chosen

Here is what each enabled linter catches, grounded in this repo's own bug history:

**Correctness and safety**

- `bodyclose` — HTTP response bodies that are never `Close()`d leak connections;
  this fires on every unclosed `resp.Body`.
- `errcheck` — an unchecked error is a hidden failure path. The config adds
  `check-type-assertions: true` so a bare `x.(T)` (without the `ok` guard) is
  also flagged — it panics if the assertion fails.
- `gosec` — security-focused: flags hardcoded credentials, integer overflow
  from `math/rand`, unbounded decompression, and similar. This is the same
  analyser the `make security` target runs standalone.
- `makezero` — catches `make([]T, n)` followed by `append`, which silently
  produces leading zeros in the slice.
- `prealloc` — suggests pre-allocating slices where the length is known,
  avoiding repeated `append` reallocations (relevant in dr-wav-go and
  jsmn-go's hot paths).
- `rowserrcheck` — `sql.Rows.Err()` must be checked after iteration; easy to
  forget, silent data loss.
- `wrapcheck` — errors returned from external packages should be wrapped with
  context (e.g. `fmt.Errorf("parse: %w", err)`). The `ignore-sigs` section
  carves out idiomatic exceptions:

```yaml
wrapcheck:
  ignore-sigs:
    - .Errorf(
    - errors.New(
    - errors.Unwrap(
    - errors.Join(
    - .Err()     # context.Canceled, bufio.Scanner.Err — must stay unwrapped
```

The comment explains the `.Err()` carve-out: `context.Context.Err()` returns
sentinel values (`context.Canceled`, `context.DeadlineExceeded`) that callers
compare with `errors.Is`. Wrapping them would break those comparisons.

**Code quality**

- `gocyclo` / `gocognit` — cyclomatic and cognitive complexity ceilings (20
  and 30 respectively). If a function is too tangled to lint cleanly, it is
  too tangled to review safely.
- `dupl` — detects copy-paste duplications; the signal to extract a shared
  helper.
- `nestif` — deeply nested `if` trees are flagged. The stb-truetype-go glyph
  parser would trip this without its iterative redesign.
- `nakedret` — named return values with a bare `return` are forbidden past a
  certain function length; they obscure what is being returned.
- `unparam` — parameters that are always passed the same constant can be
  removed or replaced with a constant, making the API clearer.

**Maintenance and hygiene**

- `godox` — `TODO` / `FIXME` / `HACK` comments are reported. They are not
  banned, but they show up in the lint output so they cannot pile up silently.
- `misspell` (locale: US) — catches `recieve`, `seperator`, `lenght` in
  comments and identifiers.
- `nolintlint` — when you write `//nolint:somecheck`, this linter requires a
  reason comment and a specific linter name:

```yaml
nolintlint:
  require-explanation: true
  require-specific: true
  allow-unused: false
```

A bare `//nolint` or `//nolint:all` is itself a lint error. This prevents
silencing rules wholesale and forgetting why.

### Exclusion presets

```yaml
exclusions:
  generated: lax
  presets:
    - comments
    - common-false-positives
    - legacy
    - std-error-handling
```

These presets suppress known false-positive patterns that golangci-lint ships
with. `generated: lax` means generated files get softer treatment but are not
completely exempt — the `lax` level still catches security issues in generated
code.

### Formatters section (v2 feature)

In golangci-lint v2, formatters are separate from linters:

```yaml
formatters:
  enable:
    - gofmt
    - goimports
```

`goimports` is `gofmt` plus automatic import grouping and removal of unused
imports. Running `make lint` therefore catches formatting problems too, not
just logic issues — so `make fmt` and `make lint` are complementary, not
redundant.

!!! note "Try it"
    Run the linter against one module to see real output:

    ```bash
    cd jsmn-go && golangci-lint run --config ../.golangci.yml
    ```

    Expected outcome: no output and exit code 0 on the current clean branch.
    To see a finding, temporarily add `x := 1` (unused variable) to any `.go`
    file (not a `_test.go`) and rerun — `unused` fires immediately.

---

## The Makefile targets and when to use them

| Target | What it does | When to run |
|--------|-------------|-------------|
| `make fmt` | `go fmt ./...` on all modules | Before committing |
| `make vet` | `go vet ./...` on all modules | Before committing |
| `make lint` | golangci-lint on all modules | Before opening a PR |
| `make test` | `go test -v ./...` on all modules | After any change |
| `make test-race` | same with `-race` flag | Before merging (catches data races) |
| `make security` | gosec + govulncheck on all modules | Weekly / before release |
| `make fuzz` | smoke fuzz for 15 s per fuzzer | Periodic / before release |
| `make examples` | builds + vets + runs the demo binaries | After API changes |
| `make all` | fmt → vet → lint → test | Full local check |
| `make pre-commit` | fmt → vet → test-race | Quick gate before `git commit` |
| `make ci` | lint → test-race → test-coverage → security | Mirrors GitHub Actions |

The `fuzz` target (from `Makefile`):

```makefile
FUZZTIME ?= 15s
fuzz:
	@(cd jsmn-go    && go test -run='^$$' -fuzz='^FuzzParse$$'   -fuzztime=$(FUZZTIME) .) || exit 1
	@(cd tinyxml2-go && go test -run='^$$' -fuzz='^FuzzParse$$'  -fuzztime=$(FUZZTIME) .) || exit 1
	@(cd dr-wav-go  && go test -run='^$$' -fuzz='^FuzzParse$$'   -fuzztime=$(FUZZTIME) .) || exit 1
	@(cd miniz-go   && go test -run='^$$' -fuzz='^FuzzExtract$$' -fuzztime=$(FUZZTIME) .) || exit 1
	...
```

`-run='^$$'` tells `go test` to skip all regular tests; only the fuzzer runs.
The default 15 s is a smoke check. For real discovery, override:

```bash
make fuzz FUZZTIME=5m
```

The fuzzer in dr-wav-go is the one that found the OOM: it generated a WAV
header with a huge `dataSize` field and watched the allocation fail. Those
seeds now live in `testdata/fuzz/` and replay on every `make test`.

!!! tip "install-tools"
    Before running lint or security scans, install the exact pinned versions:

    ```bash
    make install-tools
    ```

    This installs `golangci-lint v2.2.2`, `gosec v2.21.4`, and `govulncheck
    v1.1.4` via `go install`. Pinning prevents "it worked on my machine"
    failures caused by a linter update changing which rules fire.

---

## The `examples` target — GOWORK=off matters

```makefile
examples:
	@for ex in examples/json-parser examples/jsmn-demo \
	            examples/production-usage examples/linenoise-repl; do \
		(cd $$ex && GOWORK=off go build ./...) || exit 1; \
		(cd $$ex && GOWORK=off go vet ./...)  || exit 1; \
	done
```

`GOWORK=off` disables the workspace for the duration of that shell command.
Each example is a standalone module with a `replace` directive pointing at the
parent library. With the workspace enabled, Go ignores the `replace` directive
and uses the workspace version instead — which means the example would test
the workspace copy, not the published-module path. With `GOWORK=off` the
example resolves exactly the way an end user who cloned only that directory
would resolve it. This is a subtle but important correctness distinction.

---

## How CI uses the same Makefile

[`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md) has separate jobs (`test`, `lint`, `security`,
`fuzz`, `examples`, `build`) that each call the corresponding Makefile target.
The Makefile is the single implementation; CI is just a caller. This means:

- Running `make ci` locally gives you the same result CI will give — no
  "passes locally, fails in CI" surprises.
- Adding a new module to `MODULES` in the Makefile is the only change needed
  to include it in CI.

!!! note "Try it"
    Run the full local check in one shot:

    ```bash
    make all
    ```

    Expected sequence: formatting runs (no output if already clean), vet runs
    (silent if no issues), lint runs (per-module output), then all tests run.
    Final line: `✅ All checks passed!`

---

## Key takeaways

- The `MODULES` variable in `Makefile` is the single source of truth for the
  list of modules; every target iterates over it so nothing gets missed.
- `go vet` runs inside the Go toolchain and catches logic bugs the compiler
  allows. Run it before every commit; `make pre-commit` does it automatically.
- golangci-lint v2 uses a new YAML schema (`version: "2"`); a v1 config in a
  v2 binary fails with a parse error — always verify with
  `golangci-lint config verify`.
- `nolintlint` with `require-explanation: true` prevents suppression debt:
  every `//nolint` directive must name the specific linter and explain why.
- `GOWORK=off` in the `examples` target ensures examples resolve dependencies
  the same way an end user would — not via the workspace shortcut.
