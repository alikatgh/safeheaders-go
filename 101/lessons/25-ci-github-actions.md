# 25 · CI: GitHub Actions, govulncheck, gosec

> **Objectives:** Understand how the safeheaders-go pipeline is structured as a
> matrix of jobs, learn what govulncheck and gosec actually check (and why they
> differ), and see how the 70 % coverage gate is enforced in shell rather than a
> third-party action.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **Matrix job** — run the same recipe once per module (or OS). GitHub spins up
  nine parallel VMs, each handling one Go module. If `dr-wav-go` fails, the
  others keep running (`fail-fast: false`).
- **govulncheck** — reads your exact dependency graph and checks whether any
  function you _actually call_ is reachable through a known CVE. It ignores
  vulnerable packages you import but never touch.
- **gosec** — a static analyser that looks for dangerous Go patterns: calls to
  `exec.Command` with user-controlled input, `math/rand` used as a CSPRNG,
  hardcoded secrets, unhandled errors from `os.Create`, etc.
- **SARIF** — a standard JSON format for security findings. GitHub reads it and
  shows inline annotations in the Security tab so reviewers see gosec findings
  without leaving the PR.
- **Coverage gate** — not a percentage displayed on a badge. A shell `if`
  statement that calls `exit 1` when coverage drops below 70 %, which marks the
  whole job red and blocks merges on a protected branch.
- **Pinning to a tag, not `@master`** — `uses: securego/gosec@v2.21.4` means
  the exact released binary. `@master` would silently pick up any commit pushed
  since — including supply-chain attacks.

**Why it matters:** every fix in this repo — the deadlock, the OOM, the decode
bomb — was caught either by a test or by a fuzzer. The CI pipeline is the
machine that runs those tests on every push so humans do not have to remember.

---

## The seven jobs at a glance

The file is `.github/workflows/go-ci.yaml`. It defines seven jobs that run on
every push or pull request to `main`, plus a weekly schedule and a
`workflow_dispatch` so you can trigger fuzzing manually.

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
place.

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

`fail-fast: false` means a crash in `miniz-go` does not cancel the
`linenoise-go` run. You see all failures in one pass, not one per push.

### Race detector

```yaml
# from .github/workflows/go-ci.yaml
      - name: Run tests
        working-directory: ${{ matrix.module }}
        run: go test -v -race -timeout 5m ./...
```

`-race` instruments every memory access. Without it, the history-slice race in
`linenoise-go` (two goroutines writing the same slice) would have passed all
tests silently. See [Lesson 22](15-data-races-and-mutexes.md) for how that bug was found
and fixed with a `sync.Mutex`.

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

This is deliberate shell arithmetic, not a badge. `go tool cover -func` lists
per-function coverage; `grep total` picks the summary line; `awk` and `sed`
strip the `%` sign so `bc` can compare floats. The `exit 1` is what GitHub
turns into a red X.

`-covermode=atomic` matters when tests run goroutines: it uses atomic
increments so the coverage counters are themselves race-free.

!!! warning "The coverage gate is per module, not per workspace"
    Each of the nine modules must independently clear 70 %. A module with 95 %
    coverage cannot "donate" to one at 65 %. This prevents a large, well-tested
    module from hiding a module with almost no tests.

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

### gosec vs. govulncheck — two different questions

| Tool | Question it answers |
|------|---------------------|
| **gosec** | Does this _source code_ contain dangerous patterns? |
| **govulncheck** | Do my _dependencies_ have CVEs I actually call into? |

gosec never looks at your `go.sum`. govulncheck never looks at your source
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

`@master` resolves to whatever commit the maintainer has pushed most recently.
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
minutes of CI time for the common case (nothing new to find). The pipeline
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

`-run='^$'` skips all unit tests so only the fuzz target runs. `-fuzztime=120s`
gives the fuzzer two minutes per target per week. When a crash is found, the
`if: failure()` step uploads the corpus so you can reproduce it locally — this
is exactly how the OOM in `dr-wav-go` was caught and reproduced (see
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
path separator bugs and macOS-specific syscall differences without any extra
scripting.

`go test -short` in the build job skips long-running tests. The intent is
compile-correctness, not full coverage (the `test` job already has that on
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

The workflow declares the minimum GitHub token permissions it needs:

- `contents: read` — checkout.
- `pull-requests: write` — the benchmark job posts a PR comment.
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
  are uploaded as artifacts so failures are reproducible locally.
