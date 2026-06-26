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

- **Context is a "stop light" you pass into long-running calls.** The caller
  creates a context, hands it to a function, and can cancel it at any moment —
  the function's job is to notice and stop promptly.
- **`ctx.Done()` is a channel that closes when cancellation happens.** You
  can `select` on it alongside your real work channels.
- **`ctx.Err()` tells you *why* it was cancelled** — `context.Canceled` (someone
  called `cancel()`) or `context.DeadlineExceeded` (a timeout fired).
- **A bare `select` between a ready work item and `ctx.Done()` is not
  reliable.** When both branches are ready simultaneously, Go picks one at
  random. A cancelled context can be ignored for many iterations.
- **The fix is a cheap guard at the top of the loop:** check `ctx.Err()` before
  entering the `select`. If it is non-nil, the context is already done — stop
  immediately.
- **Why it matters:** without this guard, a batch job on a deadline might
  happily process thousands of items after its deadline has passed, wasting CPU
  and memory and returning results the caller will discard anyway.

---

## The two functions we are reading

This lesson grounds every example in two real files from this repository:

| File | Function |
|------|----------|
| `stb-image-go/stb_image.go` | `LoadBatchConcurrent` — decodes a slice of images in parallel |
| `dr-wav-go/dr_wav.go` | `ParseBatch` — parses a slice of WAV files in parallel |

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
    go test -race -v -run TestLoadBatchCancelMidParse -timeout 10s ./...
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
