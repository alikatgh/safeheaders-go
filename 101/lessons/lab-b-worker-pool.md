# Lab B · A worker pool that cancels cleanly

> **Objectives:** Build a small fan-out worker pool from scratch, wire it to a
> `context.Context` so it stops promptly on cancellation, and size its results
> channel so it can never deadlock — then verify it under `go test -race`.
> Estimated time: 35 minutes.

This is a hands-on lab. You will write the pool in stages, break it
deliberately, then fix it using the same reasoning the safeheaders-go team
applied when they fixed real deadlocks in [`jsmn-go/parallel.go`](src/jsmn-go-parallel-go.md) and
[`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md).

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Worker pool** — instead of launching one goroutine per item (which can
  flood the CPU and exhaust memory for large inputs), you launch a fixed number
  of goroutines. They each pull work from a shared queue and process items one
  at a time until the queue is empty.
- **Fan-out** — the coordinator sends every item into the queue ("fans out")
  and waits for all workers to finish.
- **Context cancellation** — a `context.Context` is a signal you can fire
  from outside: "stop what you're doing, we're done." Workers check this
  signal and exit early instead of churning through work nobody needs.
- **Channel buffering** — a Go channel can hold a fixed number of values
  without a receiver waiting. If the buffer is too small, a sender blocks
  forever waiting for space; if the buffer is sized for the worst case, sends
  always succeed immediately.
- **`sync.WaitGroup`** — a counter the coordinator increments for each
  goroutine it starts and each goroutine decrements when it finishes. When the
  counter hits zero the coordinator unblocks and knows all work is done.

**Why it matters:** the deadlock bug in jsmn-go ([`parallel.go`](src/jsmn-go-parallel-go.md)) was caused by
an under-sized channel buffer — safe to fix by understanding exactly how many
sends can happen in the worst case.

---

## The bug template: why under-sizing the results channel deadlocks

Before building your own pool, look at the comment in
`jsmn-go/parallel.go` (lines 51-55) that documents the real deadlock
the team hit:

```go
// Buffer for the worst case so no worker can block on send (and thus never
// reach wg.Done): every job produces one result (numJobs) and, on context
// cancellation, each worker may emit one extra cancel result (numWorkers).
// An under-sized buffer here deadlocks wg.Wait on mid-parse cancellation.
resultsCh := make(chan chunkResult, numJobs+numWorkers)
```

The same pattern — for the same reason — appears in
`stb-image-go/stb_image.go` (lines 106-109):

```go
// Buffer the worst case so no worker blocks on send: up to len(datas) decode
// failures plus up to numWorkers cancellation sends. An under-sized buffer
// deadlocks wg.Wait when cancellation coincides with decode failures.
errs := make(chan error, len(datas)+numWorkers)
```

The rule is:

> `buffer size = (max sends on the happy path) + (max sends on the cancel path)`

For a pool with `numJobs` jobs and `numWorkers` workers where each worker
sends exactly one result per job *and* one extra send if the context fires:

```
buffer = numJobs + numWorkers
```

That's the formula you will use in this lab.

!!! warning "The silent failure mode"
    An under-sized `resultsCh` does not panic. The program just hangs at
    `wg.Wait()` forever, with no error message. This is why the jsmn-go team
    added a watchdog test that cancels mid-parse and asserts the function
    returns promptly — the deadlock was invisible without it.

---

## Step 1 — scaffold the package

Create a new directory outside the workspace so you can iterate freely:

```bash
mkdir -p /tmp/pool-lab && cd /tmp/pool-lab
go mod init pool-lab
```

Create `pool.go`:

```go
package poollab

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// Result carries the outcome of processing one item.
type Result struct {
	Index int
	Value string
	Err   error
}

// Process runs fn on each item in parallel, using up to runtime.NumCPU()
// workers. It respects ctx: if the context is cancelled, workers stop early
// and Process returns ctx.Err().
func Process(ctx context.Context, items []string, fn func(string) (string, error)) ([]Result, error) {
	numWorkers := runtime.NumCPU()
	if len(items) < numWorkers {
		numWorkers = len(items)
	}
	if numWorkers == 0 {
		return nil, nil
	}

	// Fan out all job indices into a closed buffered channel so workers can
	// drain it without a coordinator goroutine.
	jobs := make(chan int, len(items))
	for i := range items {
		jobs <- i
	}
	close(jobs)

	// BUG (intentional): buffer is too small — fix in Step 3.
	out := make(chan Result, len(items)) // <-- will deadlock on cancel

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if err := ctx.Err(); err != nil {
					out <- Result{Err: err} // cancel send
					return
				}
				select {
				case idx, ok := <-jobs:
					if !ok {
						return
					}
					v, err := fn(items[idx])
					out <- Result{Index: idx, Value: v, Err: err} // job send
				case <-ctx.Done():
					out <- Result{Err: ctx.Err()} // cancel send
					return
				}
			}
		}()
	}

	wg.Wait()
	close(out)

	// Collect.
	var results []Result
	var firstErr error
	for r := range out {
		if r.Err != nil {
			if firstErr == nil {
				firstErr = r.Err
			}
			continue
		}
		results = append(results, r)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// identity is a trivial fn for tests.
func identity(s string) (string, error) {
	return fmt.Sprintf("done:%s", s), nil
}
```

!!! note "Try it (happy path, should pass)"
    ```bash
    cd /tmp/pool-lab
    go test -v -run TestHappyPath ./...
    ```
    You need a test file first — create it in Step 2.

---

## Step 2 — write the tests (happy path + cancel path)

Create `pool_test.go`:

```go
package poollab

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHappyPath(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	results, err := Process(context.Background(), items, identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != len(items) {
		t.Fatalf("got %d results, want %d", len(results), len(items))
	}
}

// TestCancelDeadlock deliberately cancels the context mid-pool.
// With the buggy buffer size it hangs forever; the test has a 2-second timeout
// to surface the deadlock as a failure instead of a mystery hang.
func TestCancelDeadlock(t *testing.T) {
	t.Parallel()
	items := make([]string, 100)
	for i := range items {
		items[i] = "x"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_, _ = Process(ctx, items, func(s string) (string, error) {
			time.Sleep(10 * time.Millisecond) // slow enough for the timeout to fire
			return identity(s)
		})
		close(done)
	}()

	select {
	case <-done:
		// good — returned promptly after cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("Process did not return after context cancellation — likely deadlock (buffer too small)")
	}
}

func TestErrPropagation(t *testing.T) {
	boom := errors.New("boom")
	_, err := Process(context.Background(), []string{"a", "b"}, func(s string) (string, error) {
		return "", boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}
```

Run the happy path:

```bash
go test -v -run TestHappyPath ./...
```

Expected output:
```
--- PASS: TestHappyPath (0.00s)
PASS
```

Now run the cancel test — it should *fail* (hang and time out) because the
buffer is too small:

```bash
go test -v -run TestCancelDeadlock -timeout 5s ./...
```

Expected output (the test catches the bug):
```
--- FAIL: TestCancelDeadlock (2.00s)
    pool_test.go:45: Process did not return after context cancellation — likely deadlock (buffer too small)
FAIL
```

!!! note "Predict-then-run"
    Before running the cancel test, answer for yourself: how many sends can
    land in `out` in the worst case? Count: up to `len(items)` job sends,
    **plus** up to `numWorkers` cancel sends. The current buffer holds only
    `len(items)`. When cancellation fires before all jobs are drained, one or
    more workers blocks on `out <- Result{Err: ctx.Err()}` and never reaches
    `wg.Done()`. The coordinator hangs at `wg.Wait()` forever.

---

## Step 3 — apply the fix

The fix mirrors the formula used in both `jsmn-go/parallel.go` and
`stb-image-go/stb_image.go`. In `pool.go`, change one line:

```go
// Before (buggy):
out := make(chan Result, len(items))

// After (correct):
out := make(chan Result, len(items)+numWorkers)
```

The reasoning:
- Every item produces at most one send (`out <- Result{...}` inside the
  `select` case).
- Every worker produces at most one cancel send (`out <- Result{Err: ctx.Err()}`
  in the cancel branch).
- Total worst-case sends: `len(items) + numWorkers`.
- A buffer of that size guarantees every send completes immediately, so every
  worker reaches `wg.Done()`, and `wg.Wait()` unblocks.

Run the full suite again:

```bash
go test -v -timeout 5s ./...
```

Expected output:
```
--- PASS: TestHappyPath (0.00s)
--- PASS: TestCancelDeadlock (0.06s)
--- PASS: TestErrPropagation (0.00s)
PASS
```

---

## Step 4 — prove there is no data race

The pool writes `results` from multiple goroutines — the only shared mutable
state is the `out` channel, which Go's channel implementation protects
internally. But let the race detector confirm it:

!!! note "Try it"
    ```bash
    go test -race -count=3 ./...
    ```
    Expected output: `PASS` with no `DATA RACE` report.

    If you accidentally wrote to a shared slice without a lock you would see
    something like:
    ```
    WARNING: DATA RACE
    Write at 0x... by goroutine N:
    ...
    ```

The approach used in both safeheaders-go pools — write results into an
**index-keyed slice allocated up front** rather than appending to a shared
slice — eliminates the need for a mutex over the output. Each worker writes
only to `results[idx]`, its own slot, which no other worker touches. In this
lab the `out` channel serves the same isolation purpose.

---

## Step 5 — optional: a pre-allocated output slice variant

The jsmn-go parallel path allocates `jobResults := make([]chunkResult, numJobs)` once
and lets each worker write to its own slot. That removes the channel entirely
for the output, at the cost of one allocation up front. The `stb-image-go`
pool does the same thing (line 105 in [`stb_image.go`](src/stb-image-go-stb-image-go.md)):

```go
results := make([]image.Image, len(datas))
// ...
results[idx] = img   // worker writes to its own slot — no lock needed
```

You can refactor your pool to this pattern if you want zero channel sends for
the happy path. The `errs` channel then only carries errors and cancel signals,
so its buffer is `numWorkers` (one per worker, worst case), not
`len(items) + numWorkers`. The trade-off: you need to allocate the output slice
up front and accept that failed slots remain their zero value.

!!! tip "Which pattern to use?"
    Use the **pre-allocated slot pattern** when items are independent and you
    always want all results regardless of partial errors. Use the **channel
    collection pattern** (this lab) when you want to stop at the first error or
    stream results as they arrive.

---

## How the real pools compare

| Detail | `jsmn-go/parallel.go` | `stb-image-go/stb_image.go` | This lab |
|--------|----------------------|----------------------------|----------|
| Jobs channel | pre-filled, closed | pre-filled, closed | pre-filled, closed |
| Results channel buffer | `numJobs + numWorkers` | `len(datas) + numWorkers` | `len(items) + numWorkers` |
| Output storage | pre-allocated `[]chunkResult` | pre-allocated `[]image.Image` | collected from channel |
| Cancel check | bare `select` (`ctx.Done()` case) | `ctx.Err()` before `select` | `ctx.Err()` before `select` |
| Race-safe? | yes (`-race` in CI) | yes (`-race` in CI) | verified in Step 4 |

The channel buffer formula is identical in both production modules and this
lab. Once you see the shape — `(one send per job) + (one send per worker on
cancel)` — it becomes mechanical to apply.

---

## Key takeaways

- **Size your results channel for the worst case:** `numJobs + numWorkers`,
  where the extra `numWorkers` slots absorb the cancel-path sends that each
  worker may emit before exiting.
- **An under-sized channel deadlocks silently** at `wg.Wait()` — no panic, no
  error, just a hang. A timeout-gated test is the only reliable detector.
- **Check `ctx.Err()` before the `select`** so an already-cancelled context is
  honored immediately; a bare `select` between a ready job and `ctx.Done()`
  is non-deterministic.
- **`-race` is not optional for concurrent code** — the Go race detector catches
  categories of bugs that are invisible to functional tests and code review.
- **Pre-allocate the output slice when possible** — writing to `results[idx]`
  from each worker needs no lock because each worker owns its own slot.
