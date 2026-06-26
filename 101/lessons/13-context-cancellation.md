# 13 · Context cancellation done right

> **Objectives:** Understand what `context.Context` is and why Go's concurrent
> workers need it. Learn how to write a worker loop that stops reliably when the
> caller cancels — and understand the subtle pitfall that makes a bare `select`
> miss cancellations intermittently. This lesson sets up [Lesson 14](14-the-deadlock-bug.md),
> where we look at what happens when the channel buffers are sized wrong.
>
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Context** = "a baton handed to every runner that the starter can yank back mid-race." The caller creates a `context.Context`, passes it into `LoadBatchConcurrent` or `ParseBatch`, and can cancel it at any moment — every goroutine that received it is responsible for noticing and stopping promptly.
- **`ctx.Done()`** = "a door that swings open the instant the race is called off." It is a channel that closes when the context is cancelled or its deadline fires; workers `select` on it alongside the jobs channel to hear the signal while blocked.
- **`ctx.Err()`** = "checking the scoreboard after the whistle — it tells you *why* the race stopped." It returns `nil` while the context is live, then returns `context.Canceled` (someone called `cancel()`) or `context.DeadlineExceeded` (the wall-clock budget ran out).
- **Bare `select` race** = "a coin flip between 'do one more item' and 'quit' — and the coin is biased toward work." When both the jobs channel and `ctx.Done()` are ready at the same time, Go picks a branch uniformly at random, so a cancelled context can be silently ignored for hundreds of iterations.
- **`ctx.Err()` guard at the top of the loop** = "checking the scoreboard *before* picking up the next baton, not after." The `if err := ctx.Err(); err != nil { return }` line in `LoadBatchConcurrent` runs before every `select`, so a worker that wakes up to an already-cancelled context exits on that very iteration rather than racing.
- **Why it matters:** without this guard, a batch job on a deadline might happily process thousands of items after its deadline has passed, wasting CPU and memory and returning results the caller will discard anyway.

**See it — worker loop: guard + select, two cancellation checkpoints.**

<svg viewBox="0 0 700 370" role="img" aria-labelledby="t13 d13" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t13">Worker loop cancellation checkpoints</title>
  <desc id="d13">A flow diagram showing the two places cancellation is checked in the worker loop: the ctx.Err() guard at the top of the loop, and the ctx.Done() case inside the select.</desc>
  <defs>
    <marker id="l13-arrow" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="currentColor"/>
    </marker>
  </defs>

  <!-- Start -->
  <rect x="270" y="20" width="160" height="38" rx="19" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="350" y="44" text-anchor="middle" font-size="13" fill="#fff" font-weight="600">loop iteration</text>

  <!-- Arrow: start → guard -->
  <line x1="350" y1="58" x2="350" y2="88" stroke="currentColor" stroke-width="1.5" marker-end="url(#l13-arrow)"/>

  <!-- Guard box -->
  <rect x="200" y="88" width="300" height="44" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2"/>
  <text x="350" y="107" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">① ctx.Err() guard</text>
  <text x="350" y="123" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light,currentColor)">if err := ctx.Err(); err != nil</text>

  <!-- Arrow: guard → cancelled (left) -->
  <line x1="200" y1="110" x2="110" y2="110" stroke="currentColor" stroke-width="1.5" marker-end="url(#l13-arrow)"/>
  <text x="155" y="103" text-anchor="middle" font-size="10" fill="currentColor">cancelled</text>

  <!-- Exit left -->
  <rect x="20" y="88" width="90" height="44" rx="8" fill="#e5484d" stroke="none"/>
  <text x="65" y="107" text-anchor="middle" font-size="12" fill="#fff" font-weight="600">return</text>
  <text x="65" y="122" text-anchor="middle" font-size="10" fill="#fff">errs &lt;- err</text>

  <!-- Arrow: guard → select (down, not cancelled) -->
  <line x1="350" y1="132" x2="350" y2="168" stroke="currentColor" stroke-width="1.5" marker-end="url(#l13-arrow)"/>
  <text x="372" y="155" text-anchor="start" font-size="10" fill="currentColor">not cancelled</text>

  <!-- Select box -->
  <rect x="165" y="168" width="370" height="50" rx="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5" stroke-dasharray="6,3"/>
  <text x="350" y="188" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">select  (blocks until one case fires)</text>
  <text x="260" y="207" text-anchor="middle" font-size="11" fill="currentColor">case &lt;-jobs</text>
  <text x="460" y="207" text-anchor="middle" font-size="11" fill="currentColor">② case &lt;-ctx.Done()</text>

  <!-- Arrow: jobs case → process -->
  <line x1="260" y1="218" x2="260" y2="258" stroke="currentColor" stroke-width="1.5" marker-end="url(#l13-arrow)"/>

  <!-- Process box -->
  <rect x="170" y="258" width="180" height="44" rx="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="260" y="278" text-anchor="middle" font-size="12" fill="currentColor">process item</text>
  <text x="260" y="294" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">decode image / parse WAV</text>

  <!-- Arrow: process → next iteration (loop back) -->
  <path d="M 350 280 Q 650 280 650 110 Q 650 58 510 44" stroke="currentColor" stroke-width="1.5" fill="none" marker-end="url(#l13-arrow)"/>
  <text x="620" y="190" text-anchor="middle" font-size="10" fill="currentColor">next</text>
  <text x="620" y="202" text-anchor="middle" font-size="10" fill="currentColor">iter.</text>

  <!-- Arrow: ctx.Done case → exit right -->
  <line x1="460" y1="218" x2="460" y2="258" stroke="currentColor" stroke-width="1.5" marker-end="url(#l13-arrow)"/>

  <!-- Exit right -->
  <rect x="400" y="258" width="120" height="44" rx="8" fill="#e5484d" stroke="none"/>
  <text x="460" y="278" text-anchor="middle" font-size="12" fill="#fff" font-weight="600">return</text>
  <text x="460" y="294" text-anchor="middle" font-size="10" fill="#fff">errs &lt;- ctx.Err()</text>

  <!-- Legend -->
  <rect x="20" y="320" width="14" height="14" rx="3" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="40" y="332" font-size="10" fill="currentColor">cancellation check</text>
  <rect x="170" y="320" width="14" height="14" rx="3" fill="#e5484d" stroke="none"/>
  <text x="190" y="332" font-size="10" fill="currentColor">exit (cancelled)</text>
  <rect x="310" y="320" width="14" height="14" rx="3" fill="none" stroke="currentColor" stroke-width="1.5" stroke-dasharray="4,2"/>
  <text x="330" y="332" font-size="10" fill="currentColor">select block</text>
</svg>

---

## The two functions we are reading

This lesson grounds every example in two real files from this repository:

| File | Function |
|------|----------|
| [`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md) | `LoadBatchConcurrent` — decodes a slice of images in parallel |
| [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md) | `ParseBatch` — parses a slice of WAV files in parallel |

Both functions spin up a worker pool, hand out jobs through a channel, and
accept a `context.Context` from the caller. They solve the cancellation problem
in slightly different ways, which makes comparing them instructive.

---

## What a context looks like at the call site

```go
// Caller creates a context with a 5-second deadline.
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel() // always release resources

images, err := stbimagego.LoadBatchConcurrent(ctx, rawImages)
```

`cancel` is a function. Calling it immediately signals every goroutine that
received `ctx` to stop. `defer cancel()` ensures the signal fires even if the
caller returns early due to an error.

!!! tip "WithTimeout vs WithCancel"
    `context.WithTimeout(parent, d)` is shorthand for `context.WithDeadline(parent, time.Now().Add(d))`.
    Use `WithCancel` when you want manual control; use `WithTimeout` /
    `WithDeadline` when you have a wall-clock budget.

---

## The worker loop in stb-image-go

From `stb-image-go/stb_image.go`, the goroutine launched inside
`LoadBatchConcurrent`:

```go
go func() {
    defer wg.Done()
    for {
        // Check cancellation first: a bare select races between a ready
        // job and ctx.Done() (Go picks randomly), so an already-canceled
        // context would only be honored intermittently.
        if err := ctx.Err(); err != nil {
            errs <- err
            return
        }
        select {
        case idx, ok := <-jobs:
            if !ok {
                return // jobs channel closed and empty — normal exit
            }
            img, err := Load(datas[idx])
            if err != nil {
                errs <- fmt.Errorf("failed to decode image at index %d: %w", idx, err)
            } else {
                results[idx] = img
            }
        case <-ctx.Done():
            errs <- ctx.Err()
            return
        }
    }
}()
```

Two lines do the critical work:

1. **`if err := ctx.Err(); err != nil`** — this runs *before* the `select`.
   If the context was already cancelled when we reach the top of the loop, we
   exit immediately without entering the `select` at all.
2. **`case <-ctx.Done():`** inside the `select` — this catches a cancellation
   that arrives *while* the worker is blocked waiting for a job. Both branches
   together give reliable, prompt cancellation.

### Why the top-of-loop check is not redundant

Imagine the jobs channel has 1000 items buffered and the context is cancelled.
Without the `ctx.Err()` guard, each iteration does a `select` with two ready
cases — `jobs` and `ctx.Done()`. Go picks randomly, so on average half the
iterations will drain a job instead of stopping. Statistically the worker would
process ~500 more images before it finally picks `ctx.Done()`. With the guard,
it exits on the next loop iteration.

!!! warning "The bare-select pitfall"
    This pattern is subtly wrong:

    ```go
    // BAD — do not do this
    for {
        select {
        case work := <-jobs:
            process(work)
        case <-ctx.Done():
            return
        }
    }
    ```

    When both `jobs` and `ctx.Done()` are ready, Go's runtime picks one branch
    uniformly at random. The worker may consume many more jobs before it
    happens to land on `ctx.Done()`. Always guard with `ctx.Err()` first.

---

## Contrast: the worker loop in dr-wav-go

From `dr-wav-go/dr_wav.go`, `ParseBatch` uses a structurally different
approach:

```go
go func() {
    defer wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case work, ok := <-dataChan:
            if !ok {
                return
            }
            wav, err := Parse(work.data)
            resultChan <- result{wav: wav, err: err, index: work.index}
        }
    }
}()
```

Notice what is missing: there is no `ctx.Err()` guard before the `select`.
The `ctx.Done()` case is listed *first* in the `select` block, but that does
not help — Go does not give priority to any case. Both cases get equal weight
when both are ready.

The WAV batch function compensates in a different place — the result collector
checks `ctx.Err()` after each result arrives:

```go
for res := range resultChan {
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }
    // ...
}
```

This stops collecting results, but the workers themselves may still process
extra items in the background. For a batch of large WAV files that is wasteful.
The stb-image pattern (guard at the top of the worker loop) is tighter.

!!! note "Neither is wrong for the repo's current use cases"
    The dr-wav approach is correct for correctness — it does eventually stop and
    never returns results after cancellation. The stb-image approach is more
    efficient under high cancellation pressure. The lesson is about understanding
    the trade-off so you can choose deliberately.

---

## How the caller receives the cancellation signal

Back in `stb-image-go/stb_image.go`, after all workers finish:

```go
wg.Wait()
close(errs)

multiErr := make([]error, 0, len(errs))
for err := range errs {
    multiErr = append(multiErr, err)
}

if len(multiErr) > 0 {
    if errors.Is(multiErr[0], context.Canceled) ||
        errors.Is(multiErr[0], context.DeadlineExceeded) {
        return nil, multiErr[0]
    }
    return nil, fmt.Errorf("multiple errors occurred: %v", multiErr)
}
```

Key points:

- `errors.Is` unwraps chains. If a worker wrapped `ctx.Err()` with `fmt.Errorf("…: %w", err)`, `errors.Is` still finds `context.Canceled` inside the chain.
- Context errors are surfaced directly, not merged into the multi-error string.
  This lets callers do `errors.Is(err, context.Canceled)` cleanly.

---

## Channel buffer sizing and why it matters here

Both functions pre-allocate the error channel with extra capacity:

```go
// stb-image-go/stb_image.go
errs := make(chan error, len(datas)+numWorkers)
```

The comment in the file explains why:

> Buffer the worst case so no worker blocks on send: up to len(datas) decode
> failures plus up to numWorkers cancellation sends. An under-sized buffer
> deadlocks wg.Wait when cancellation coincides with decode failures.

This is the exact bug explored in [Lesson 14](14-the-deadlock-bug.md). If the
buffer is `len(datas)` and all workers also send a cancellation error, those
sends block. `wg.Wait()` waits for the workers to finish. Deadlock.

!!! note "Try it"
    Run the stb-image tests with the race detector enabled:

    ```bash
    cd /path/to/safeheaders-go/stb-image-go
    go test -race -v -run TestLoadBatchConcurrent ./...
    ```

    Expected outcome: all tests pass with no data-race report. The `-race` flag
    instruments every goroutine memory access; if cancellation were racy, the
    detector would report it here. Try predicting before running: will you see
    output lines mentioning "PASS" or "DATA RACE"?

!!! note "Try it — cancel mid-batch"
    You can exercise the cancellation path directly:

    ```bash
    cd /path/to/safeheaders-go/stb-image-go
    go test -race -v -run TestLoadBatchConcurrent_Cancellation -timeout 10s ./...
    ```

    Expected outcome: the test passes and completes well within the 10-second
    timeout. If the channel buffer were too small, `wg.Wait()` would hang and
    the test would be killed by `-timeout`.

---

## Putting it all together — a minimal template

Here is the distilled pattern from the two real functions, stripped to its
essential shape:

```go
func ProcessBatch(ctx context.Context, items []Item) ([]Result, error) {
    numWorkers := runtime.NumCPU()
    if len(items) < numWorkers {
        numWorkers = len(items)
    }

    jobs := make(chan int, len(items))
    for i := range items {
        jobs <- i
    }
    close(jobs)

    results := make([]Result, len(items))
    // Size: decode errors (len(items)) + cancellation sends (numWorkers).
    errs := make(chan error, len(items)+numWorkers)

    var wg sync.WaitGroup
    for range numWorkers {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                // ① Guard: exit immediately if already cancelled.
                if err := ctx.Err(); err != nil {
                    errs <- err
                    return
                }
                select {
                case idx, ok := <-jobs:
                    if !ok {
                        return
                    }
                    r, err := process(items[idx])
                    if err != nil {
                        errs <- err
                    } else {
                        results[idx] = r
                    }
                case <-ctx.Done(): // ② catch cancellation while blocked
                    errs <- ctx.Err()
                    return
                }
            }
        }()
    }

    wg.Wait()
    close(errs)

    for err := range errs {
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return nil, err
        }
        return nil, err
    }
    return results, nil
}
```

The two numbered comments mark the two places cancellation is checked. Neither
alone is sufficient. Together they are.

---

## Key takeaways

- `context.Context` is Go's standard way to propagate cancellation and
  deadlines across API boundaries; always accept it as the first argument of
  long-running functions.
- A bare `select{case <-jobs: … case <-ctx.Done(): …}` is non-deterministic
  when both are ready — use `ctx.Err()` at the top of the loop as a hard check.
- `ctx.Err()` returns `nil` until cancelled, then returns `context.Canceled` or
  `context.DeadlineExceeded`; use `errors.Is` to check wrapped errors.
- Size your error channels for the worst case: job errors *plus* per-worker
  cancellation sends — an undersized buffer causes the deadlock described in
  [Lesson 14](14-the-deadlock-bug.md).
- Always `defer cancel()` at the call site to release context resources, even
  when you expect the deadline to fire naturally.
