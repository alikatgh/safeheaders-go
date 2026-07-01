# 25 · CI: GitHub Actions, govulncheck, gosec

> **Objectives:** Understand how the safeheaders-go pipeline is structured as a
> matrix of jobs, learn what govulncheck and gosec actually check (and why they
> differ), and see how the 70 % coverage gate is enforced in shell rather than a
> third-party action.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Matrix job** = "one kitchen, nine ovens running at once." GitHub spins up nine parallel VMs, one per Go module, so a failure in `dr-wav-go` does not cancel the rest because `fail-fast: false` keeps every oven going.
- **govulncheck** = "a librarian who checks whether you actually read the dangerous chapter, not just whether the book is on your shelf." It traces your real call graph against known CVEs and ignores vulnerable packages you import but never invoke.
- **gosec** = "a building inspector scanning blueprints for code violations before anything is built." It analyses your source patterns — `exec.Command` with user input, `math/rand` as a CSPRNG, hardcoded secrets, unhandled `os.Create` errors — without ever looking at your dependencies.
- **SARIF** = "a standardised incident report form that every security scanner fills out the same way." GitHub reads it and turns gosec findings into inline PR annotations so reviewers see the exact line without leaving the diff.
- **Coverage gate** = "a physical turnstile, not a scoreboard." A shell `if` calling `exit 1` when coverage drops below 70 % makes the whole job red and blocks merges on a protected branch — the number has teeth, not just paint.
- **Pinning to a tag, not `@master`** = "ordering a sealed, numbered bottle rather than pouring from an open tap." `uses: securego/gosec@v2.21.4` locks you to one audited release; `@master` silently runs whatever was pushed last, including a supply-chain compromise.

**Why it matters:** every fix in this repo — the deadlock, the OOM, the decode
bomb — was caught either by a test or by a fuzzer. The CI pipeline is the
machine that runs those tests on every push so humans do not have to remember.

**See it — seven CI jobs, two security tools, one 70 % gate.**

<svg viewBox="0 0 700 370" role="img" aria-labelledby="t25 d25" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t25">GitHub Actions CI pipeline for safeheaders-go</title>
  <desc id="d25">A block-and-arrow diagram showing a push or PR event triggering seven parallel jobs: test (with race detector and 70% coverage gate), lint, security (gosec SARIF + govulncheck), benchmark, fuzz (schedule/dispatch only), examples, and build. The security job fans out to gosec and govulncheck separately.</desc>
  <defs>
    <marker id="l25-arr" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L0,7 L8,3.5 Z" fill="var(--md-default-fg-color,#333)"/>
    </marker>
  </defs>
  <rect x="260" y="16" width="180" height="38" rx="6" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="350" y="40" text-anchor="middle" font-size="13" font-weight="600" fill="#fff">push / pull_request</text>
  <line x1="350" y1="54" x2="350" y2="76" stroke="var(--md-default-fg-color,#333)" stroke-width="1.5" marker-end="url(#l25-arr)"/>
  <text x="14" y="104" font-size="10" fill="var(--md-default-fg-color--light,#666)">Jobs (parallel)</text>
  <rect x="14" y="82" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="68" y="99" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">test</text>
  <text x="68" y="113" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">-race · coverage</text>
  <rect x="134" y="82" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="188" y="99" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">lint</text>
  <text x="188" y="113" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">golangci-lint v2</text>
  <rect x="254" y="82" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="308" y="99" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">security</text>
  <text x="308" y="113" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">gosec · govulncheck</text>
  <rect x="374" y="82" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="428" y="99" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">benchmark</text>
  <text x="428" y="113" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">PR only</text>
  <rect x="494" y="82" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="548" y="99" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">fuzz</text>
  <text x="548" y="113" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">schedule/dispatch</text>
  <rect x="134" y="138" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="188" y="155" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">examples</text>
  <text x="188" y="169" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">make examples</text>
  <rect x="254" y="138" width="108" height="38" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="308" y="155" text-anchor="middle" font-size="11" font-weight="600" fill="var(--md-default-fg-color,#333)">build</text>
  <text x="308" y="169" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">Linux · macOS · Win</text>
  <line x1="14" y1="200" x2="686" y2="200" stroke="var(--md-default-fg-color--lightest,#ccc)" stroke-width="1" stroke-dasharray="4 3"/>
  <text x="14" y="220" font-size="10" fill="var(--md-default-fg-color--light,#666)">Inside test (per module × 9):</text>
  <rect x="14" y="228" width="130" height="32" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="79" y="248" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">go test -race ./...</text>
  <line x1="144" y1="244" x2="168" y2="244" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2" marker-end="url(#l25-arr)"/>
  <rect x="170" y="228" width="130" height="32" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="235" y="248" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">coverage &lt; 70%?</text>
  <line x1="300" y1="244" x2="324" y2="244" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2" marker-end="url(#l25-arr)"/>
  <rect x="326" y="228" width="80" height="32" rx="5" fill="#e5484d" stroke="none"/>
  <text x="366" y="248" text-anchor="middle" font-size="10" font-weight="600" fill="#fff">exit 1 ✗</text>
  <line x1="300" y1="244" x2="300" y2="278" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2"/>
  <line x1="300" y1="278" x2="434" y2="278" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2"/>
  <text x="350" y="274" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light,#666)">≥ 70%</text>
  <line x1="434" y1="278" x2="434" y2="260" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2" marker-end="url(#l25-arr)"/>
  <rect x="420" y="228" width="80" height="32" rx="5" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="460" y="248" text-anchor="middle" font-size="10" font-weight="600" fill="#fff">pass ✓</text>
  <text x="14" y="310" font-size="10" fill="var(--md-default-fg-color--light,#666)">Inside security:</text>
  <rect x="14" y="318" width="130" height="32" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="79" y="333" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">gosec (source</text>
  <text x="79" y="346" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">patterns) → SARIF</text>
  <line x1="144" y1="334" x2="168" y2="334" stroke="var(--md-default-fg-color,#333)" stroke-width="1.2" marker-end="url(#l25-arr)"/>
  <rect x="170" y="318" width="140" height="32" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="240" y="333" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">GitHub Code Scanning</text>
  <text x="240" y="346" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">(inline PR annotations)</text>
  <rect x="340" y="318" width="160" height="32" rx="5" fill="none" stroke="var(--md-default-fg-color--light,#888)" stroke-width="1.2"/>
  <text x="420" y="333" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">govulncheck (call graph</text>
  <text x="420" y="346" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color,#333)">vs CVE database)</text></svg>

---

## The seven jobs at a glance

The file is [`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md) (in plain terms: a configuration
file, written in a format called YAML, that tells GitHub's automation
service — "GitHub Actions" — exactly what commands to run and when; it is
not itself a program, just a recipe the service follows). It defines seven jobs (a "job" here
is one self-contained batch of commands, like "run the tests" or "check
for security problems") that run on
every push or pull request to `main`, plus a weekly schedule and a
`workflow_dispatch` so you can trigger fuzzing manually (fuzzing means
bombarding a program with huge numbers of randomly-generated or mutated
inputs to see if any of them crash it or make it misbehave — a way of
searching for bugs that a human tester would never think to try).

```yaml
# from .github/workflows/go-ci.yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    # Run weekly security scans and fuzzing on Mondays at 9 AM UTC
    - cron: '0 9 * * 1'
  workflow_dispatch: # allow manual fuzz/CI runs
```

**In plain terms:** this says "run the pipeline automatically whenever code is
pushed to the `main` branch, whenever someone opens a pull request against
`main`, every Monday at 9 AM UTC on a timer, and also whenever a person
manually clicks a button to trigger it."

| Job | Trigger | What it does |
|-----|---------|--------------|
| `test` | every push / PR | `go test -race`, coverage gate, Codecov |
| `lint` | every push / PR | golangci-lint v2 |
| `security` | every push / PR | gosec (SARIF) + govulncheck |
| `benchmark` | PR only | benchmarks posted as PR comment |
| `fuzz` | schedule + dispatch | 120 s fuzzing per target |
| `examples` | every push / PR | `make examples` — every example program builds and runs |
| `build` | every push / PR | compile on Linux / macOS / Windows |

---

## The test job: `-race`, coverage gate, and module matrix

The test job is the most instructive because it shows three practices in one
place. (A "test" here is a small piece of code, written by a programmer, whose
only purpose is to run a bit of the real program and check that it produced
the answer it was supposed to — tests catch mistakes automatically, before a
human ever notices something is wrong.)

```yaml
# from .github/workflows/go-ci.yaml
jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        module:
          - jsmn-go
          - stb-image-go
          - stb-truetype-go
          - tinyxml2-go
          - cjson-go
          - cgltf-go
          - dr-wav-go
          - miniz-go
          - linenoise-go
```

**In plain terms:** this list defines a "matrix" — nine separate, independent
copies of the same job, one per code module (a module is a self-contained
package of related code, roughly like a folder that holds one piece of the
overall project), all run in parallel rather than one after another.

`fail-fast: false` means a crash in `miniz-go` does not cancel the
`linenoise-go` run. You see all failures in one pass, not one per push.

### Race detector

```yaml
# from .github/workflows/go-ci.yaml
      - name: Run tests
        working-directory: ${{ matrix.module }}
        run: go test -v -race -timeout 5m ./...
```

**In plain terms:** this line runs the automated test suite with an extra
safety check turned on (`-race`), and gives it at most 5 minutes to finish.

`-race` instruments every memory access (in plain terms: it watches every
single spot in the program's reserved chunk of working memory — its
"memory" — that gets read from or written to while the program runs, so it can
catch two independent, simultaneously-running pieces of the program, called
goroutines, trying to touch the same piece of data at the same time in an
unsafe way — this kind of clash is called a "data race"). Without it, the history-slice race in
`linenoise-go` (two goroutines writing the same slice — a "slice" is Go's
name for a resizable list of values) would have passed all
tests silently. See [Lesson 22](15-data-races-and-mutexes.md) for how that bug was found
and fixed with a `sync.Mutex` (a mutex is a lock: a mechanism that lets only
one goroutine touch a piece of data at a time, so the others must wait their
turn instead of colliding).

### Coverage gate

```yaml
# from .github/workflows/go-ci.yaml
      - name: Run tests with coverage
        working-directory: ${{ matrix.module }}
        run: go test -coverprofile=coverage.txt -covermode=atomic -v ./...

      - name: Check coverage threshold
        working-directory: ${{ matrix.module }}
        run: |
          coverage=$(go tool cover -func=coverage.txt | grep total | awk '{print $3}' | sed 's/%//')
          echo "Coverage: $coverage%"
          if (( $(echo "$coverage < 70.0" | bc -l) )); then
            echo "❌ Coverage $coverage% is below 70% threshold"
            exit 1
          else
            echo "✅ Coverage $coverage% meets threshold"
          fi
```

**In plain terms:** run the tests while measuring "coverage" — the percentage
of the program's actual lines that got executed at least once by some test —
then pull that percentage out of the report, and if it's under 70%, stop the
whole job and mark it as failed (`exit 1` is the standard way a program tells
whatever ran it "something went wrong"); otherwise print a success message.

This is deliberate shell arithmetic, not a badge. `go tool cover -func` lists
per-function coverage (a "function" is a named, reusable block of code that
performs one task and can be run — "called" — from elsewhere in the program;
"per-function" coverage means the percentage is broken down function by
function); `grep total` picks the summary line; `awk` and `sed`
strip the `%` sign so `bc` can compare floats (numbers with decimal points).
The `exit 1` is what GitHub
turns into a red X.

`-covermode=atomic` matters when tests run goroutines (goroutines are Go's
lightweight units of independent, simultaneously-running work — a way of
letting a program do more than one thing at once): it uses atomic
increments (updates to a shared counter that are guaranteed to happen as one
uninterruptible step, so two goroutines updating it at once can't corrupt the
count) so the coverage counters are themselves race-free.

!!! warning "The coverage gate is per module, not per workspace"
    Each of the nine modules must independently clear 70 %. A module with 95 %
    coverage cannot "donate" to one at 65 %. This prevents a large, well-tested
    module from hiding a module with almost no tests. (A "workspace" here just
    means the whole collection of modules treated as one combined project.)

---

## The security job: gosec and govulncheck

```yaml
# from .github/workflows/go-ci.yaml
  security:
    name: Security Scan
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        module: [jsmn-go, stb-image-go, stb-truetype-go, tinyxml2-go,
                 cjson-go, cgltf-go, dr-wav-go, miniz-go, linenoise-go]
    steps:
      - name: Run gosec security scanner
        uses: securego/gosec@v2.21.4 # pinned (not @master); Dependabot bumps it
        with:
          args: '-fmt sarif -out results.sarif ${{ matrix.module }}/...'

      - name: Upload SARIF file
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: results.sarif
          category: ${{ matrix.module }}

      - name: Run govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
          cd ${{ matrix.module }}
          govulncheck ./...
```

**In plain terms:** two separate scans run on each module. First, gosec reads
the source code text itself looking for risky patterns and writes its
findings to a standard report file (SARIF), which then gets uploaded to
GitHub's dashboard. Second, a tool called `govulncheck` is installed and run;
it checks the outside code this project depends on (its "dependencies" —
other people's code that gets pulled in and used, rather than written from
scratch) for known security vulnerabilities.

### gosec vs. govulncheck — two different questions

| Tool | Question it answers |
|------|---------------------|
| **gosec** | Does this _source code_ contain dangerous patterns? |
| **govulncheck** | Do my _dependencies_ have CVEs I actually call into? |

gosec never looks at your `go.sum` (a file that pins down the exact version of
every outside dependency the project uses). govulncheck never looks at your source
patterns. You need both.

`-fmt sarif` tells gosec to emit
[SARIF](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
instead of plain text. The Upload SARIF step sends it to GitHub's Code Scanning
dashboard, where findings appear as inline comments on the relevant lines of the
PR diff.

`if: always()` on the upload step ensures the SARIF file is pushed even when
gosec exits non-zero (i.e., when it actually found something). Without
`always()`, a finding would cause the step to be skipped and you would never see
the annotation.

### Why pin the action version?

```yaml
uses: securego/gosec@v2.21.4
```

Compare with the dangerous alternative:

```yaml
uses: securego/gosec@master   # ← never do this
```

`@master` resolves to whatever commit (a saved snapshot of the code at one
point in time) the maintainer has pushed most recently.
A compromised maintainer account — or a dependency of the action — could push
malicious code that runs inside your CI with write access to your repository.
Pinning to a tag ties you to an audited release. Dependabot will open a PR when
a new version is available, so you do not miss security updates.

!!! tip "Pin to a SHA for maximum safety"
    Some teams go further and pin to the commit SHA rather than the tag, because
    tags can be force-pushed. Example:
    ```yaml
    uses: securego/gosec@1a79d73df1078bf5b2e4561d02b5e3b1d7abe7b2
    ```
    The tag approach used here is a reasonable trade-off between readability and
    security for an open-source library project.

---

## The fuzz job: scheduled, not per-push

Fuzzing is computationally expensive. Running it on every push would consume
minutes of CI time (computer time rented from GitHub to run these jobs) for the common case (nothing new to find). The pipeline
gates it behind `schedule` and `workflow_dispatch`:

```yaml
# from .github/workflows/go-ci.yaml
  fuzz:
    name: Fuzz
    runs-on: ubuntu-latest
    if: github.event_name == 'schedule' || github.event_name == 'workflow_dispatch'
    strategy:
      fail-fast: false
      matrix:
        include:
          - module: jsmn-go
            target: FuzzParse
          - module: dr-wav-go
            target: FuzzParse
          - module: miniz-go
            target: FuzzExtract
          # … four more targets …
    steps:
      - name: Fuzz ${{ matrix.module }} (${{ matrix.target }})
        working-directory: ${{ matrix.module }}
        run: go test -run='^$' -fuzz="^${{ matrix.target }}$" -fuzztime=120s .

      - name: Upload crash corpus on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: fuzz-crash-${{ matrix.module }}
          path: ${{ matrix.module }}/testdata/fuzz/
```

**In plain terms:** for each of several chosen entry points (a "fuzz target"
is the specific function the fuzzer is told to hammer with random input),
run the fuzzer for 120 seconds; if it manages to crash the program, save the
exact inputs that caused the crash as a downloadable file so a person can
look at them afterward.

`-run='^$'` skips all unit tests (a "unit test" is a test that checks one
small piece of the program in isolation) so only the fuzz target runs. `-fuzztime=120s`
gives the fuzzer two minutes per target per week. When a crash is found, the
`if: failure()` step uploads the corpus (a "corpus" is the collection of
saved input samples the fuzzer generated or found interesting — including the
one that caused the crash) so you can reproduce it locally — this
is exactly how the OOM (out-of-memory: the program tried to reserve more
working memory than was available and crashed) in `dr-wav-go` was caught and reproduced (see
[Lesson 19](22-fuzzing.md)).

---

## The build job: three OSes, three modules

```yaml
# from .github/workflows/go-ci.yaml
  build:
    name: Build
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        module:
          - jsmn-go
          - stb-image-go
          - stb-truetype-go
```

`matrix.os` crossed with `matrix.module` produces 3 × 3 = 9 build jobs. The
`runs-on` key accepts the matrix variable directly. This catches Windows-specific
path separator bugs and macOS-specific syscall differences (a "syscall" is a
request a program makes to the operating system itself, such as "open this
file" — different operating systems sometimes handle the same request
slightly differently) without any extra
scripting.

`go test -short` in the build job skips long-running tests. The intent is
compile-correctness (in plain terms: just confirming that the human-written
source code successfully turns into a runnable program — "compiles" — on
each operating system, without errors), not full coverage (the `test` job already has that on
Linux).

---

## The lint job: golangci-lint v2

```yaml
# from .github/workflows/go-ci.yaml
      - name: Install golangci-lint
        # golangci-lint v2 — must match the schema version of .golangci.yml.
        # The install script is pinned to the same tag so its CLI contract matches.
        run: |
          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v2.2.2/install.sh \
            | sh -s -- -b $(go env GOPATH)/bin v2.2.2

      - name: Run golangci-lint
        working-directory: ${{ matrix.module }}
        run: golangci-lint run --config ../.golangci.yml --timeout 5m
```

**In plain terms:** download and install a specific version of a "linter" — a
tool that reads source code without running it and flags style problems,
likely bugs, or bad practices, the way a spell-checker flags typos — then run
it against every module using one shared rulebook.

The comment explains why the install script itself is pinned to `v2.2.2`: the
v2 linter has a different `.golangci.yml` schema than v1. Installing v1 and
pointing it at a v2 config silently ignores unknown keys, producing no warnings
and no errors — a false green. The version pins are coupled.

`--config ../.golangci.yml` reads the config from the workspace root (one level
up from the module directory), so all nine modules share a single lint
configuration.

---

## Minimal permissions

```yaml
# from .github/workflows/go-ci.yaml
permissions:
  contents: read
  pull-requests: write
  security-events: write
```

**In plain terms:** this grants the automated pipeline a limited set of
permissions — it may read the repository's files, post comments on pull
requests, and write security-scan results, but nothing beyond that.

The workflow declares the minimum GitHub token (a token is a piece of secret
text that proves to GitHub's servers "this request is authorized to act on
the repository's behalf," standing in for a username and password) permissions it needs:

- `contents: read` — checkout (downloading a copy of the repository's files
  onto the machine running the job).
- `pull-requests: write` — the benchmark job posts a PR comment (a
  "benchmark" is a test that measures how fast or resource-hungry a piece of
  code is, rather than whether it's correct).
- `security-events: write` — the SARIF upload writes to Code Scanning.

If a step in the workflow is compromised (e.g., a malicious dependency run
during `go mod download`), the token it steals can only do what these three
scopes allow. It cannot push commits, create releases, or modify secrets.

!!! warning "Default permissions are too broad"
    If you omit the `permissions` block, GitHub defaults to `contents: write`
    for the token, meaning a supply-chain compromise could push code to your
    repository. Declaring `permissions` explicitly is a one-line defense.

---

!!! note "Try it"
    Run the coverage check locally against any module:

    ```bash
    cd jsmn-go
    go test -coverprofile=coverage.txt -covermode=atomic ./...
    go tool cover -func=coverage.txt | grep total
    ```

    **Predicted output:**
    ```
    total:   (statements)   82.4%
    ```
    The number will vary, but it should be above 70 %. If you delete a test
    function and re-run, you will see the percentage drop. The CI gate would
    reject a PR with that deletion.

    To run govulncheck locally (install once):

    ```bash
    go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
    cd jsmn-go && govulncheck ./...
    ```

    **Predicted output:** `No vulnerabilities found.` (as of the lesson date;
    the answer may change if a future CVE is disclosed against a dependency).

---

## Key takeaways

- **Matrix + `fail-fast: false`** lets all nine modules run in parallel and
  report all failures at once, not one per push cycle.
- **govulncheck and gosec answer different questions** — one checks your
  dependency graph against CVEs, the other checks your source code for dangerous
  patterns. Both run on every PR.
- **The 70 % coverage gate is enforced with `exit 1`** in a shell step, not a
  badge. A PR that drops coverage below the threshold fails to merge.
- **Pin action versions to tags** (`@v2.21.4`, not `@master`) to prevent
  supply-chain attacks from silently replacing the binary your CI runs.
- **Fuzzing runs weekly, not per-push**, because it is expensive; crash corpora
  are uploaded as artifacts (files saved by the pipeline that a person can
  download afterward) so failures are reproducible locally.
