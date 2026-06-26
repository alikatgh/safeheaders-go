# 11 · Goroutines, Channels and Select

> **Objectives:** Understand how Go expresses concurrency with goroutines and channels,
> learn how buffered channels differ from unbuffered ones, and see how `select` lets a
> worker react to multiple events at once.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

- **Goroutine** — a "green thread" launched with `go f()`. It runs concurrently with
  the caller, but costs only a few kilobytes of stack (not a full OS thread). You can
  run thousands of them.
- **Channel** — a typed pipe between goroutines. One side sends (`ch <- v`), the other
  receives (`v := <-ch`). No shared memory needed.
- **Buffered channel** — a pipe with a waiting room. `make(chan T, N)` lets up to N
  values sit in the buffer before the sender has to wait. An unbuffered channel
  (`make(chan T)`) forces sender and receiver to meet at the same instant.
- **`select`** — like a `switch` but for channels. Go picks whichever `case` has a
  ready channel, at random when several are ready simultaneously.
- **`close(ch)`** — signals "no more values coming." A `range ch` loop or an `ok`
  check (`v, ok := <-ch`) can detect the close.

**Why it matters:** every hardening problem in this repo — concurrency, cancellation,
deadlines — is solved with this handful of primitives. If you can read a `select`
loop, you can read all of it.

---

## The simplest goroutine

```go
go fmt.Println("hello from a goroutine")
```

`go` before any function call spins it into the background. The caller keeps running
immediately. If `main` returns first, the goroutine is silently killed, so production
code always synchronises with `sync.WaitGroup` or channels.

```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    fmt.Println("working…")
}()
wg.Wait() // block until the goroutine calls Done()
```

---

## Channels: the pipe metaphor

### Unbuffered — rendezvous

```go
ch := make(chan int) // no buffer

go func() {
    ch <- 42 // blocks until receiver is ready
}()

v := <-ch // blocks until sender sends
fmt.Println(v) // 42
```

Sender and receiver must both be ready at the same moment — a rendezvous.
Use this when timing matters: if one side isn't there yet, you want to wait,
not race ahead.

### Buffered — mailbox

```go
ch := make(chan int, 4) // room for 4 items

ch <- 1 // returns immediately, item sits in the buffer
ch <- 2
v := <-ch // 1
```

The sender is not blocked until the mailbox is full. This is the right choice
when you know the upper bound on outstanding work up front — and you often do.

### Closing and ranging

```go
ch := make(chan int, 5)
for i := 0; i < 5; i++ {
    ch <- i
}
close(ch) // tell readers "that's all"

for v := range ch { // drains until closed
    fmt.Println(v)
}
```

`range` on a channel drains it and exits the loop when the channel is closed.
Sending to a closed channel panics, so only the producer should call `close`.

---

## `select` — listening to multiple channels at once

```go
select {
case msg := <-inbox:
    process(msg)
case <-ctx.Done():
    return ctx.Err()
}
```

If both channels are ready simultaneously, Go picks one uniformly at random.
That randomness is a feature: it prevents starvation.

!!! warning "Priority matters sometimes"
    If you need a strict priority check (e.g. "always honour cancellation
    first"), don't rely on `select`'s random choice. Instead, check the
    high-priority channel *before* entering `select`:

    ```go
    if err := ctx.Err(); err != nil {
        return err
    }
    select {
    case idx := <-jobs: ...
    case <-ctx.Done(): ...
    }
    ```

    The real `LoadBatchConcurrent` worker does exactly this — see below.

---

## Reading the real worker loop — `stb-image-go/stb_image.go`

`LoadBatchConcurrent` decodes a batch of images in parallel. It uses every
primitive from above, so it is a perfect reading exercise.

### Step 1 — decide the worker count

```go
// stb-image-go/stb_image.go
numWorkers := runtime.NumCPU()
if len(datas) < numWorkers {
    numWorkers = len(datas)
}
```

No point spawning more workers than there are images.

### Step 2 — pre-fill a buffered jobs channel, then close it

```go
jobs := make(chan int, len(datas))
for i := 0; i < len(datas); i++ {
    jobs <- i          // send the image *index*, not the image itself
}
close(jobs)            // signal: no more work will arrive
```

Closing immediately after filling is the classic "fan-out" pattern. Workers
read indices from `jobs`; when the channel is drained and closed, `range jobs`
(or the `ok` check in `select`) exits the loop automatically.

### Step 3 — size the error channel to avoid a deadlock

```go
// Buffer the worst case so no worker blocks on send: up to len(datas) decode
// failures plus up to numWorkers cancellation sends. An under-sized buffer
// deadlocks wg.Wait when cancellation coincides with decode failures.
errs := make(chan error, len(datas)+numWorkers)
```

This comment is the most important line in the file. If the buffer were only
`len(datas)`, a cancelled context could cause every worker to *also* try to
send `ctx.Err()` on the same channel. With N workers all trying to send and
nobody reading yet, the `N`th send blocks. The goroutine is stuck. `wg.Wait`
waits for it. Deadlock. The fix is to reserve space for both failure kinds.

!!! note "This was a real bug"
    The original buffer was `len(datas)`. A cancellation during a full-error
    batch deadlocked `wg.Wait` indefinitely. The fix — `len(datas)+numWorkers`
    — was added after the 2026-06 audit. The jsmn-go parallel worker had the
    identical bug at the same time. See [Lesson 14](14-the-deadlock-bug.md)
    for the full post-mortem.

### Step 4 — launch workers

```go
var wg sync.WaitGroup

for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            // Priority check — don't even enter select if already cancelled.
            if err := ctx.Err(); err != nil {
                errs <- err
                return
            }
            select {
            case idx, ok := <-jobs:
                if !ok {
                    return          // jobs channel drained and closed
                }
                img, err := Load(datas[idx])
                if err != nil {
                    errs <- fmt.Errorf("failed to decode image at index %d: %w", idx, err)
                } else {
                    results[idx] = img   // safe: each goroutine writes a distinct index
                }
            case <-ctx.Done():
                errs <- ctx.Err()
                return
            }
        }
    }()
}
```

Key observations:

1. **`defer wg.Done()`** — fires however the goroutine exits (normal,
   cancellation, error). The `defer` removes the need for multiple `Done()`
   calls scattered across every return path.
2. **Priority check before `select`** — if `ctx` is already cancelled when
   the goroutine starts, `select` would randomly pick between `jobs` and
   `ctx.Done()`. The explicit `ctx.Err()` check before `select` guarantees
   cancellation is honoured immediately.
3. **`ok` sentinel** — `case idx, ok := <-jobs` detects a closed channel.
   When `ok` is false the worker returns cleanly.
4. **Write to `results[idx]`** — no mutex needed because each goroutine owns
   a distinct index; they never write the same slot.

### Step 5 — drain, then collect errors

```go
wg.Wait()
close(errs)   // safe now: all senders are done

for err := range errs {
    multiErr = append(multiErr, err)
}
```

`close(errs)` is called only after `wg.Wait()` confirms every worker has
returned. Once the channel is closed, the `range` loop drains whatever errors
accumulated and then exits.

---

## Buffered vs unbuffered — when to choose which

| Situation | Recommendation |
|---|---|
| You know the exact max number of sends before any receive | Buffered (`len(items)+headroom`) |
| Sender and receiver should synchronise step-by-step | Unbuffered |
| Fan-out: pre-fill all work, then let workers pull | Buffered, closed after fill |
| Fan-in: collect results from N workers | Buffered (`N`) or merge with `select` |
| Signal "done" with no data | `close(ch)` on a `chan struct{}` |

!!! tip "A zero-capacity channel is not broken"
    `make(chan T)` is deliberately strict — it forces you to think about who is
    waiting for whom. Unbuffered channels are excellent for one-shot signals and
    unit tests where you want to assert ordering.

---

## Try it

!!! note "Try it"
    Run the stb-image-go tests, including the race detector, from the repo root:

    ```bash
    cd stb-image-go && go test -race -v ./...
    ```

    Expected outcome: all tests pass with zero data-race warnings. The test
    suite includes a cancellation test that sends a context whose deadline
    expires mid-batch; before the buffer-size fix, that test would hang
    indefinitely. If it exits within a few seconds the fix is working.

    To watch the worker pool in action with a short benchmark:

    ```bash
    go test -bench=BenchmarkLoadBatchConcurrent -benchtime=3s ./...
    ```

    You should see throughput scale with the number of CPUs on your machine.

---

## Key takeaways

- Launch a goroutine with `go f()`. Synchronise with `sync.WaitGroup` or a
  channel — never assume timing.
- Buffered channels (`make(chan T, N)`) decouple sender and receiver up to N
  items. Choose N carefully: too small and you deadlock; too large and you
  hide backpressure.
- `select` reacts to whichever channel is ready first. When two cases are
  simultaneously ready it picks at random — add a priority check *before*
  `select` when ordering is critical.
- `close(ch)` is the producer's signal that no more values will arrive.
  Receivers detect it with `range` or the `_, ok := <-ch` idiom.
- The buffer size `len(datas)+numWorkers` in `LoadBatchConcurrent` is not
  arbitrary — it is the exact worst-case count of sends that can race `wg.Wait`,
  and getting it wrong causes a deadlock that only manifests under cancellation.
