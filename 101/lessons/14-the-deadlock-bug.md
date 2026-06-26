# 14 · The deadlock we shipped (and the fix)

> **Objectives:** Understand exactly why an under-buffered channel can deadlock a
> worker pool — not as an abstract rule, but as a real bug with a goroutine dump
> and a one-line fix. See the same shape appear in two different modules so you
> can recognise it in your own code.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

- A **goroutine** is a lightweight thread. Sending on a channel blocks the
  goroutine until someone on the other end receives.
- A **buffered channel** `make(chan T, N)` lets up to N sends proceed without a
  matching receive. Send number N+1 blocks.
- A **`sync.WaitGroup`** is a counter. `wg.Wait()` blocks until the counter
  reaches zero; each worker decrements it with `defer wg.Done()`.
- The trap: if a goroutine blocks on a channel send _before_ it calls
  `wg.Done()`, `wg.Wait()` waits for `wg.Done()` that never comes. Both sides
  are stuck. That is a **deadlock** (in practice: a process that hangs forever).
- **Goroutine leak**: a goroutine that is stuck on a channel send is never
  collected by the GC. It keeps its stack and any slice it references alive. Over
  time — or on a single bad request — this drains memory.

**Why it matters:** a deadlock in a parser library means one crafted input (or a
canceled request) can freeze a server permanently, requiring a restart to recover.

**See it — the deadlock.** The pool can send `numJobs + numWorkers` messages (one
result per job, plus one cancel-ack per worker), but the channel buffers only
`numJobs`. Once the buffer fills, the extra cancel-acks block on send — and
`wg.Wait()` waits for goroutines that can never finish. The fix sizes the buffer
`numJobs + numWorkers`.

<svg viewBox="0 0 720 360" role="img" aria-labelledby="dl-t dl-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:680px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="dl-t">The under-sized results channel deadlock</title>
  <desc id="dl-d">Workers can send numJobs plus numWorkers messages but the channel buffers only numJobs, so the extra cancel-acks block and wg.Wait never returns.</desc>
  <defs>
    <marker id="dl-ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto"><path d="M0,0 L7,3 L0,6 Z" fill="var(--md-accent-fg-color,#00897b)"/></marker>
    <marker id="dl-mut" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto"><path d="M0,0 L7,3 L0,6 Z" fill="var(--md-default-fg-color--light)"/></marker>
    <marker id="dl-rh" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto"><path d="M0,0 L7,3 L0,6 Z" fill="#e5484d"/></marker>
  </defs>
  <rect x="28" y="150" width="160" height="84" rx="6" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="108" y="184" text-anchor="middle" font-size="14" font-weight="600" fill="currentColor">worker pool</text>
  <text x="108" y="205" text-anchor="middle" font-size="12" fill="var(--md-default-fg-color--light)">N goroutines</text>
  <text x="108" y="222" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">each sends 2 kinds</text>
  <text x="470" y="92" text-anchor="middle" font-size="13" fill="currentColor"><tspan font-family="ui-monospace,monospace">resultsCh</tspan>  ·  cap = numJobs (4)  ·  <tspan fill="#e5484d" font-weight="700">FULL</tspan></text>
  <rect x="342" y="106" width="58" height="44" rx="4" fill="var(--md-accent-fg-color,#00897b)"/><text x="371" y="133" text-anchor="middle" font-size="12" fill="#fff">r1</text>
  <rect x="408" y="106" width="58" height="44" rx="4" fill="var(--md-accent-fg-color,#00897b)"/><text x="437" y="133" text-anchor="middle" font-size="12" fill="#fff">r2</text>
  <rect x="474" y="106" width="58" height="44" rx="4" fill="var(--md-accent-fg-color,#00897b)"/><text x="503" y="133" text-anchor="middle" font-size="12" fill="#fff">r3</text>
  <rect x="540" y="106" width="58" height="44" rx="4" fill="var(--md-accent-fg-color,#00897b)"/><text x="569" y="133" text-anchor="middle" font-size="12" fill="#fff">r4</text>
  <path d="M190,168 L334,140" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.6" marker-end="url(#dl-ah)"/>
  <text x="256" y="148" text-anchor="middle" font-size="11" fill="var(--md-accent-fg-color,#00897b)">results × numJobs</text>
  <path d="M190,210 L322,196" fill="none" stroke="#e5484d" stroke-width="1.6" stroke-dasharray="5 4" marker-end="url(#dl-rh)"/>
  <line x1="334" y1="168" x2="334" y2="234" stroke="#e5484d" stroke-width="3"/>
  <text x="252" y="232" text-anchor="middle" font-size="11" fill="#e5484d">cancel-ack × numWorkers</text>
  <text x="346" y="205" font-size="11" fill="#e5484d" font-weight="600">✗ no free slot — send blocks</text>
  <rect x="342" y="266" width="256" height="62" rx="6" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="470" y="292" text-anchor="middle" font-size="13" fill="currentColor"><tspan font-family="ui-monospace,monospace">main(): wg.Wait()</tspan></text>
  <text x="470" y="312" text-anchor="middle" font-size="11" fill="#e5484d" font-weight="600">hangs forever — workers never return</text>
  <path d="M470,150 L470,264" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2" stroke-dasharray="3 4" marker-end="url(#dl-mut)"/>
  <path d="M340,300 C250,300 184,272 156,238" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2" stroke-dasharray="3 4" marker-end="url(#dl-mut)"/>
  <text x="250" y="292" text-anchor="middle" font-size="10.5" fill="var(--md-default-fg-color--light)">circular wait</text>
</svg>

---

## The bug in `jsmn-go/parallel.go`

`parseParallelWithConfig` tokenises a large JSON input by splitting it into chunks
and fanning out to `numWorkers` goroutines. Each goroutine pulls jobs from `jobCh`
and sends results to `resultsCh`.

### How the channel was sized (the broken version)

The audit finding H1 (from `docs/audits/2026-06-23-code-review-security-audit.md`)
describes what the original code looked like:

```
resultsCh := make(chan chunkResult, numJobs)   // ← original, broken
```

`numJobs` is the number of chunks. The thinking was: one result per chunk, so
buffer `numJobs`. Reasonable at first glance.

### Why that reasoning is wrong

Look at `chunkWorker` in `jsmn-go/parallel.go`. Each worker runs a `select` loop:

```go
// from jsmn-go/parallel.go
func chunkWorker(
    ctx context.Context,
    json []byte,
    jobCh <-chan chunkJob,
    resultsCh chan<- chunkResult,
    maxTokensPerChunk int,
    config *Config,
) {
    for {
        select {
        case <-ctx.Done():
            resultsCh <- chunkResult{err: ctx.Err()}   // cancel send
            return
        case j, ok := <-jobCh:
            if !ok {
                return
            }
            toks, err := processChunk(json, j, maxTokensPerChunk, config)
            resultsCh <- chunkResult{id: j.id, toks: toks, err: err}  // job send
            if err != nil {
                return
            }
        }
    }
}
```

A worker can send to `resultsCh` in **two places**:

1. When it finishes a job — that is the "job send".
2. When `ctx.Done()` fires — that is the "cancel send".

In a normal run with no cancellation, the total sends equal `numJobs`. The
buffer holds them all. Fine.

Now consider what happens when a `context.WithTimeout` fires after the workers
have already sent their job results but before `wg.Wait()` has drained anything:

```
Scenario (numWorkers = 4, numJobs = 4):

  resultsCh buffer: [r0][r1][r2][r3]   ← all 4 slots filled by job sends
  ctx fires ──────────────────────────► each worker tries to send a cancel result
  worker-0 sends cancel   → BLOCKS (buffer full, nobody is receiving yet)
  worker-0 never reaches wg.Done()
  wg.Wait() waits for wg.Done() that never comes
  ─────────────────────────────────────────── DEADLOCK
```

`wg.Wait()` is the only thing guarding the `close(resultsCh)` call that would
let `mergeChunkResults` start draining. Nobody is draining, so the send blocks.
Nobody can unblock the send because draining hasn't started. Classic deadlock.

!!! warning "recover() cannot help here"
    A deadlocked `wg.Wait()` is not a panic — it is a goroutine blocked on a
    mutex/semaphore. `recover()` only catches panics. The process simply hangs.

---

## The fix — one extra term in the buffer size

The current code in `jsmn-go/parallel.go` (line 55) reads:

```go
// from jsmn-go/parallel.go
// Buffer for the worst case so no worker can block on send (and thus never
// reach wg.Done): every job produces one result (numJobs) and, on context
// cancellation, each worker may emit one extra cancel result (numWorkers).
// An under-sized buffer here deadlocks wg.Wait on mid-parse cancellation.
resultsCh := make(chan chunkResult, numJobs+numWorkers)
```

Worst-case sends = `numJobs` (one per chunk) + `numWorkers` (one cancel result
per goroutine). Size the buffer to that ceiling and every send is guaranteed to
proceed without blocking, so every goroutine reaches `wg.Done()`, so `wg.Wait()`
always returns.

The comment in the code is the reasoning you just read — it was added as part of
the fix so the next reader does not have to re-derive it.

---

## The same shape in `stb-image-go/stb_image.go`

The audit found the identical pattern in `LoadBatchConcurrent` (finding H2). The
`errs` channel was originally sized to `len(datas)` — one slot per image. But
each worker has two send sites: a per-job decode-failure send and a per-worker
cancellation send.

The fixed code in `stb-image-go/stb_image.go` (line 109) mirrors the jsmn fix:

```go
// from stb-image-go/stb_image.go
// Buffer the worst case so no worker blocks on send: up to len(datas) decode
// failures plus up to numWorkers cancellation sends. An under-sized buffer
// deadlocks wg.Wait when cancellation coincides with decode failures.
errs := make(chan error, len(datas)+numWorkers)
```

The pattern:

| Module | Channel | Old size | Correct size |
|---|---|---|---|
| `jsmn-go` | `resultsCh` | `numJobs` | `numJobs + numWorkers` |
| `stb-image-go` | `errs` | `len(datas)` | `len(datas) + numWorkers` |

The rule generalises: **if a worker has K distinct send sites that could all fire
in one execution path, the channel buffer must accommodate K sends per worker.**

---

## How to detect this before it ships: the watchdog test

A deadlock does not produce an error — the test just hangs. Standard test
timeouts catch this, but they are slow (the default `go test` timeout is ten
minutes). A faster approach is a watchdog: cancel a context mid-parse and assert
the call returns within a short deadline.

The repo's tests follow this pattern (the real watchdog tests are
`TestParseParallelCancellationNoDeadlock` in `jsmn-go/jsmn_parallel_test.go` and
`TestLoadBatchConcurrentCancellationNoDeadlock` in `stb-image-go/stb_image_audit_test.go`):

```go
// Watchdog pattern — assert the call does not hang on cancellation.
ctx, cancel := context.WithCancel(context.Background())

done := make(chan struct{})
go func() {
    // Cancel mid-parse by firing the context from another goroutine.
    cancel()
}()

go func() {
    defer close(done)
    ParseWithConfig(ctx, largeInput, DefaultConfig())
}()

select {
case <-done:
    // Good — the call returned.
case <-time.After(5 * time.Second):
    t.Fatal("ParseWithConfig blocked after context cancellation — goroutine leak / deadlock")
}
```

Before the buffer fix, this test deadlocked on iteration 1 with `NumCPU=11`.
After the fix, it returns promptly every time.

!!! note "Try it"
    Run the parallel parser tests with the race detector:

    ```bash
    cd jsmn-go && go test -race -count=1 -run TestParse ./...
    ```

    Expected: all tests pass, no `DATA RACE` output, no hang. If you revert the
    buffer size to `numJobs` and re-run, the test will hang (or time out if your
    system has enough CPUs to trigger the race quickly).

---

## Why `numJobs` and `numWorkers` differ

It is worth being precise about these two numbers because the bug hinges on them
being different.

- `numWorkers = runtime.NumCPU()` — the number of goroutines in the pool.
- `numJobs = len(jobs)` — the number of work items.

`buildChunkJobs` caps `numJobs` at `numWorkers` (you cannot have more chunks than
workers), so on the parallel path `numJobs == numWorkers`. In that case the worst
case is `2 × numWorkers` sends, and a buffer of `numJobs + numWorkers =
2 × numWorkers` is exactly right.

If you ever change the code so chunks outnumber workers (a reasonable
optimisation), the analysis still holds: worst case is `numJobs` job sends plus
`numWorkers` cancel sends. The fix is already correct for that scenario too.

---

## The audit that found it

This bug was found by the 10-agent Opus security audit documented in
`docs/audits/2026-06-23-code-review-security-audit.md`. It was classified as
**High** severity (finding H1 for `jsmn-go`, H2 for `stb-image-go`) because:

- The trigger is any caller using `context.WithTimeout` or `context.WithCancel`,
  which is idiomatic Go for HTTP handlers and CLI tools.
- The effect is permanent: the goroutines never exit, the call never returns,
  and the leaked goroutines keep the input slice alive.
- The existing cancellation tests happened to cancel *before* the call entered
  the worker loop, so the bug was never exercised.

!!! tip "Lesson from the audit"
    Test cancellation *during* execution, not just before. A context canceled
    before a function is called is caught by a cheap early-exit check
    (`config.go` line 166–170 in `jsmn-go`). The dangerous case is cancellation
    that races with in-flight workers.

---

## Key takeaways

- **Count every send path per worker.** If a worker has two send sites (job
  result + cancellation result), the channel buffer must accommodate both. A
  buffer sized only for the "happy path" deadlocks under cancellation.
- **`wg.Wait()` + channel drain must not form a cycle.** The classic safe pattern
  is: drain the channel in a goroutine while `wg.Wait()` runs, then `close` the
  channel after `Wait` returns. Alternatively, size the buffer large enough that
  no send ever blocks.
- **Deadlocks do not panic.** They hang. Write watchdog tests that assert your
  concurrent functions return within a timeout when the context is canceled
  mid-flight.
- **The same bug appeared in two independent modules.** When you find a
  structural pattern bug (wrong buffer size in a worker pool), grep for the same
  pattern everywhere it could appear. Do not fix it in one place and move on.
- **Buffer size = max sends across all code paths, not just the happy path.**
  `numJobs + numWorkers` is the formula for a pool where each worker can fire one
  cancel send on top of its normal job sends.
