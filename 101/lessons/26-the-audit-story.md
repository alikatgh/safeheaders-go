# 26 · The 10-agent audit (a war story)

> **Objectives:** Understand how a structured multi-agent security audit
> surfaced 25 real bugs — deadlocks, data races, resource bombs — across all
> 9 modules in one day, and why the same bug classes appear again and again.
> Walk away with a concrete checklist you can run on any new Go library.
>
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **"10 parallel reviewers"** — ten independent AI agents each focused on
  one module, the same way a code-review team splits a large PR. They can
  find things in parallel, but each one misses context the others have.
- **"Adversarial verifier"** — a separate agent whose only job is to *doubt*
  each finding and try to reproduce it with a real test. If it cannot
  reproduce the bug, the finding is downgraded. Think of it as a referee.
- **"25 confirmed findings"** — every item in the report was reproduced
  concretely before being written down. Zero verified false positives.
- **"Severity buckets"** — High means "exploitable with a tiny crafted input
  from a public API"; Medium means "real bug but harder to trigger or lower
  impact"; Low and Info are hardening notes.
- **"All fixed in the same week"** — the audit was run on 2026-06-23; every
  fix landed with a regression test in the same branch.

**Why it matters:** the bugs the audit found are not exotic. Deadlocks from
under-buffered channels, data races on a shared global, decompression bombs
from a missing aggregate cap — these are the *canonical* Go pitfalls, and
they can survive months of local testing before a structured review catches
them.

---

## How the audit was structured

The methodology is described in the audit's closing paragraph
(docs/audits/2026-06-23-code-review-security-audit.md):

> *"10 parallel reviewers, each scoped to one module/surface, followed by an
> independent adversarial verifier that attempted concrete reproduction of
> every reported finding before recording a verdict."*

That structure matters. The two-phase design — find, then verify — is what
kept the false-positive count at zero. A single reviewer reading their own
work tends to convince themselves a plausible-sounding bug is real. A
separate verifier who must write a failing test is much harder to fool.

The count breakdown:

| Severity | Confirmed |
|----------|----------:|
| Critical | 0 |
| High     | 5 |
| Medium   | 10 |
| Low      | 8 |
| Info     | 2 |
| **Total**| **25** |

One additional finding (U1) was *uncertain* — technically plausible but not
reproducible within the unit under test. Zero were overturned as wrong.

!!! note "The overall verdict"
    The audit's executive summary opened with: *"Overall posture: solid. The
    memory-safety and bounds-checking discipline across the library is genuinely
    strong … No memory-corruption, slice-out-of-range, nil-deref, or
    integer-overflow bug was found in any module."*

    That is a meaningful statement for a library that parses untrusted binary
    and text formats. The confirmed bugs fell into two tighter categories:
    concurrency hazards and resource-exhaustion gaps — not memory safety.

---

## Bug class 1: the under-buffered channel deadlock

Two of the five High findings were the *same bug in two different modules*,
found within hours of each other.

**H1 — `jsmn-go` parallel parser** (parallel.go lines 51-64, 124-139):

```go
// The channel was sized for exactly numJobs results.
resultsCh := make(chan chunkResult, numJobs)

// But each worker could send TWO things:
// 1. the normal job result
// 2. an extra result on the ctx.Done() cancel branch
// worst case: numJobs + numWorkers sends > numJobs capacity
// → one send blocks → worker never calls wg.Done() → wg.Wait() hangs forever
```

**H2 — `stb-image-go` batch loader** (stb_image.go line 109):

```go
// errs was sized for exactly len(datas) per-job failures.
errs := make(chan error, len(datas))

// But a canceled context adds up to numWorkers extra sends on top.
// Same shape: capacity N, worst-case sends N+W → deadlock.
```

The fix in both cases was the same: size the buffer for the true worst case.

```go
// jsmn-go fix: numJobs + numWorkers
resultsCh := make(chan chunkResult, numJobs+numWorkers)

// stb-image-go fix: len(datas) + numWorkers
errs := make(chan error, len(datas)+numWorkers)
```

Each fix shipped with a watchdog test that cancels the context mid-parse and
fails if `wg.Wait()` does not return within two seconds. These tests now live
as permanent regression guards.

!!! warning "The pattern that hides this bug"
    The existing tests canceled the context *before* calling the function.
    That hits an early short-circuit path and never reaches the channel.
    The cancellation-during-execution path was completely untested. If you
    write a cancellation test, make sure the context fires *while workers are
    running*, not before the call starts.

See [Lesson 14](14-the-deadlock-bug.md) and [Lesson 13](13-context-cancellation.md)
for the full channel-sizing and cancellation mechanics.

---

## Bug class 2: the data race on shared global state

**H3 — `linenoise-go` history API** (linenoise.go line 631):

```go
// A single package-level singleton backs all the "convenience" functions.
var defaultState = New(DefaultConfig())

// AddHistory, LoadHistory, SaveHistory, ClearHistory all call into it.
// No mutex anywhere in the file.
// Two goroutines → race on the history slice header → torn reads / lost writes.
```

The race was reproduced immediately with `go test -race`:

```
WARNING: DATA RACE
Read at 0x... by goroutine ...:
  linenoise-go.(*State).AddHistory(...)
Write at 0x... by goroutine ...:
  linenoise-go.(*State).LoadHistory(...)
```

The fix added a `sync.Mutex` around every read and write of `State.history`
and `State.config`, including the navigation reads inside `ReadLine`.

!!! note "Try it"
    Run the race detector against the linenoise module:

    ```bash
    cd linenoise-go && go test -race ./...
    ```

    Expected (after the fix): `ok  linenoise-go [no test files]` or a passing
    test run with no `DATA RACE` output. Before the fix, this reliably printed
    a race warning pointing at the history slice.

See [Lesson 15](15-data-races-and-mutexes.md) for a full treatment of the
race detector and mutex patterns.

---

## Bug class 3: resource-exhaustion bombs

Three findings fit the "small input, catastrophic output" shape.

### The composite-glyph billion-laughs (H4)

**`stb-truetype-go`** (sfnt.go lines 293-317):

```go
// glyphContours recurses once per composite component.
// The only guard was a depth cap of 8.
// But with no fan-out cap, a glyph referencing K children
// each referencing K children, 8 levels deep → K^8 expansions.
// K=50 → ~2e15 invocations from a ~2-3 KB font file.
```

The fix threaded a shared budget counter through the recursion:

```go
// sfnt.go (after fix): a glyphBudget struct caps total components AND points
// across the entire composite tree, not just the depth.
```

The depth cap was not enough. Fan-out without a total-work budget is the
billion-laughs shape regardless of depth limit.

See [Lesson 19](19-recursion-and-billion-laughs.md) for why depth alone does
not bound work when fan-out > 1.

### The aggregate decompression bomb (M5)

**`miniz-go`** (miniz.go lines 101-120):

```go
// readAllLimited caps a SINGLE zip entry at MaxDecompressedSize.
// But ExtractArchive loops over ALL entries with no running total.
// 10,000 entries × 256 MiB cap each = 2.5 TB aggregate output
// from a ~990 KB input archive.
```

The fix tracks a running byte total across entries and fails when the
cumulative output crosses `MaxDecompressedSize`.

### The XML stack overflow (M7)

**`tinyxml2-go`** (tinyxml2.go line 195):

```go
// parseElement recurses once per nested XML element.
// ParseWithConfig enforces MaxNestingDepth via parseElementLimited.
// But Parse() used parseElement — NO depth cap.
// A 35 MB file of <a><a><a>...<a/></a></a></a> → goroutine stack > 1 GB
// → fatal error: stack overflow
// recover() CANNOT catch this. The process dies.
```

The fix: an absolute `maxNestingDepth = 10000` ceiling was added *inside*
`parseElement` itself (and `parseElementLimited`), so even `Parse` and
`UnlimitedConfig` hit a finite backstop that no caller can disable (L5).

!!! warning "recover() does not save you from a stack overflow"
    A Go stack overflow is a runtime fatal error, not a panic. Wrapping
    `Parse` in `recover()` gives you nothing — the process exits with code 2.
    The only defense is an explicit depth limit *before* the recursion runs
    out of stack space.

See [Lesson 18](18-decode-and-decompression-bombs.md) for the aggregate-cap
pattern and [Lesson 19](19-recursion-and-billion-laughs.md) for the recursion
ceiling pattern.

---

## Bug class 4: correctness gaps that look like security

Not every finding was a dramatic crash. Some were quiet correctness bugs that
matter precisely because callers trust the library to be right.

**M3 — `jsmn-go` corrupted parent pointers** (parallel.go lines 194-195):

```go
// processChunk rebased Token.Start and Token.End by the chunk offset.
// It did NOT rebase Token.ParentIdx — an index into the token array.
// After mergeChunkResults concatenated all chunks, every token in chunk
// N>1 had a ParentIdx pointing into chunk-local space, not the merged array.
// Result: 10,914 out of 14,000 tokens had wrong ParentIdx after a
// parallel parse. The serial parser was correct; the parallel path was
// silently broken.
```

**M1 — `cgltf-go` negative mesh index accepted** (cgltf.go line 170):

```go
// The only check was: node.Mesh >= len(gltf.Meshes) && node.Mesh != 0
// A negative value like -5 makes -5 >= len(Meshes) false → accepted.
// ValidateGLTF returned nil for node.Mesh = -5.
// A downstream caller trusting ValidateGLTF would then panic on Meshes[-5].
```

Both were correctness bugs, not crashes in the library itself. But a
function called `ValidateGLTF` that silently accepts `mesh: -5` is
misinforming its callers.

---

## The checklist: what the audit teaches you to look for

The 25 findings cluster into a short list of recurring patterns. Before
shipping any Go library that processes untrusted input, run through these:

### Concurrency checklist

- [ ] **Channel sizing.** For every `make(chan T, N)` in a worker pool: list
  every goroutine that can send on it and every path each goroutine can take.
  Is the buffer large enough for the true worst case (normal sends + cancel
  sends)?
- [ ] **Drain order.** Is the channel drained *concurrently* with workers, or
  only *after* `wg.Wait()`? The latter blocks if any worker is stuck trying
  to send.
- [ ] **Global singletons.** Any package-level `var` that multiple goroutines
  read or write needs a mutex. This includes exported "convenience" wrappers
  that secretly share a single state object.
- [ ] **Race detector.** Run `go test -race ./...` for every package. If CI
  does not run `-race`, add it.

### Resource-exhaustion checklist

- [ ] **Aggregate caps.** A per-entry limit does not protect against many
  entries. Sum outputs across all entries and enforce a total cap.
- [ ] **Fan-out bounds.** A recursion depth cap does not bound work when each
  level fans out to multiple children. Add a total-work counter that the
  entire call tree shares.
- [ ] **Stack-safe recursion.** Unbounded recursion on untrusted input
  eventually hits `fatal error: stack overflow`. Either enforce a depth cap
  that stays well below Go's goroutine stack limit, or convert to an
  iterative algorithm with an explicit stack.
- [ ] **All entry points covered.** If you add a limit to `ParseWithConfig`,
  does `Parse` also enforce it? Does `LoadStream` have the same guard as
  `Load`?

### Correctness checklist

- [ ] **Index rebasing in parallel merges.** Any token or reference that
  indexes into an array must be rebased when chunks are concatenated.
- [ ] **Full range checks.** `x >= len(slice)` does not catch `x < 0`. Check
  both bounds.
- [ ] **Validation function scope.** If a function is named `ValidateFoo`,
  does it actually validate all fields? Document what it does *not* check.

---

## Running the regression tests that caught these bugs

!!! note "Try it"
    Run the full test suite with the race detector enabled:

    ```bash
    cd /path/to/safeheaders-go
    go test -race ./...
    ```

    Expected outcome: all packages pass with zero `DATA RACE` warnings.
    The watchdog tests for the deadlocks (H1, H2) will time out and fail
    loudly if the channel-sizing fix is reverted.

!!! note "Try it"
    Run the fuzz targets that originally found the dr-wav OOM:

    ```bash
    cd dr-wav-go
    go test -fuzz=FuzzParse -fuzztime=30s
    ```

    Expected outcome: no crashes. The regression seeds under
    `testdata/fuzz/FuzzParse/` are replayed automatically every time
    `go test` runs, even without `-fuzz`.

---

## Key takeaways

- **The same five bug classes recur:** under-buffered channels, unguarded
  global state, missing aggregate caps, unbounded fan-out, and incomplete
  validation. Learn their shapes and you can spot them before an audit does.
- **A depth limit is not a work limit.** When each recursive level fans out
  to K children, depth-8 means up to K^8 total calls. The fix is a shared
  total-work counter, not a deeper depth cap.
- **Per-entry resource limits are not aggregate limits.** A 256 MiB
  per-stream decompression cap does nothing if 10,000 streams can each reach
  that cap. Always track the running total.
- **The most dangerous path is the one not covered by existing tests.** Both
  deadlocks survived because tests canceled the context before the call, not
  during worker execution. Verify that your cancellation tests actually race
  the cancellation against the work.
- **An adversarial verifier is worth the overhead.** 10 reviewers found
  candidates; the independent verifier turned 26 candidates into 25 confirmed
  bugs and 0 false positives. The reproduce-first rule is what makes an audit
  report trustworthy.
