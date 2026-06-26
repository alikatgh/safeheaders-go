# 12 · Worker pools: fan-out / fan-in

> **Objectives:** Understand how to spread independent work across a fixed number of goroutines using a jobs channel and a results channel. Learn the fan-out / fan-in pattern in full — feeder goroutine, worker pool, WaitGroup, and ordered result collection — by reading it directly from this repository's production code.
> Estimated time: 25 minutes.

## What this actually means (plain English)

- **Fan-out** is giving the same pile of work to several workers at once, like
  a dispatcher handing envelopes to a team of couriers simultaneously instead
  of sending one courier and waiting for them to come back.
- **Fan-in** is collecting whatever each worker produces into a single stream,
  like all couriers dropping their receipts into one inbox when they return.
- A **jobs channel** is the inbox on the dispatcher's desk — workers pull jobs
  from it when they are free.
- A **results channel** is the inbox on your desk — workers drop answers into
  it as soon as they finish.
- A **`sync.WaitGroup`** is the foreman's tally sheet — each worker signs in
  (`wg.Add(1)`) and signs out (`wg.Done()`), so the foreman knows the
  exact moment all workers are done and closes the results inbox.
- **Ordering** is the tricky part: workers finish in whatever order they feel
  like, but the caller usually wants results in the same order as the input.
  Tagging each job with its original index, and writing results into a
  pre-allocated slice by that index, is the standard fix.

**Why it matters:** parsing audio files, 3D models, or ZIP entries one at a
time leaves most CPU cores idle; a worker pool can saturate all cores while
keeping memory usage bounded and result order predictable.

---

## The skeleton: four moving parts

Every worker pool in this repo follows the same four-part shape. Here it is
written out abstractly before we look at the real code:

```go
// 1. Channels — sized so senders never block.
jobCh    := make(chan job,    len(inputs))
resultCh := make(chan result, len(inputs))

// 2. Workers — a fixed pool, each running until jobCh is closed.
var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for j := range jobCh {
            resultCh <- process(j)
        }
    }()
}

// 3. Feeder — runs in its own goroutine so it does not block the caller.
go func() {
    defer close(jobCh)       // closing the channel signals workers to stop
    for i, v := range inputs {
        jobCh <- job{v, i}
    }
}()

// 4. Closer — once every worker is done, close the results channel.
go func() {
    wg.Wait()
    close(resultCh)
}()

// Collect results (main goroutine — blocks until resultCh is closed).
out := make([]*Result, len(inputs))
for r := range resultCh {
    out[r.index] = r.value   // restore input order via the tagged index
}
```

Each number above maps to a concrete code location you can read right now.

---

## Seeing it in dr-wav-go: ParseBatch

`dr-wav-go/dr_wav.go` contains `ParseBatch`, which decodes multiple WAV files
concurrently. It is a clean, self-contained example of all four parts.

### Part 1 — the channels

```go
// from dr-wav-go/dr_wav.go
dataChan := make(chan struct {
    data  []byte
    index int
}, len(dataList))
resultChan := make(chan result, len(dataList))
```

Both channels are buffered to `len(dataList)`. That size matters: if
`resultChan` were unbuffered, a worker would block trying to send a result while
the main goroutine was still sending jobs — instant deadlock. Buffering to the
number of jobs guarantees every worker can always send without waiting.

### Part 2 — the workers

```go
// from dr-wav-go/dr_wav.go
numWorkers := runtime.NumCPU()
if numWorkers > len(dataList) {
    numWorkers = len(dataList)
}

var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
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
}
```

The `runtime.NumCPU()` cap keeps you from spawning 10,000 goroutines for
10,000 files; goroutine creation is cheap but not free. The `select` on
`ctx.Done()` means the caller can cancel mid-batch and all workers exit
promptly — no goroutine leaks.

### Part 3 — the feeder goroutine

```go
// from dr-wav-go/dr_wav.go
go func() {
    for i, data := range dataList {
        select {
        case <-ctx.Done():
            close(dataChan)
            return
        case dataChan <- struct {
            data  []byte
            index int
        }{data, i}:
        }
    }
    close(dataChan)
}()
```

The feeder runs in its own goroutine. This is essential: if the feeder ran
inline (before starting workers), it would fill `dataChan` and then block if
there were more items than buffer slots, with no workers yet reading. Putting
the feeder in a goroutine lets it run concurrently with the workers that are
already pulling from the channel.

Closing `dataChan` at the end is the shutdown signal: workers see `ok == false`
on the next receive and return, decrementing the WaitGroup.

### Part 4 — the closer and collection

```go
// from dr-wav-go/dr_wav.go
go func() {
    wg.Wait()
    close(resultChan)
}()

results := make([]*WAV, len(dataList))
for res := range resultChan {
    if res.err != nil {
        return nil, fmt.Errorf("failed to parse WAV at index %d: %w", res.index, res.err)
    }
    results[res.index] = res.wav   // <-- ordered by index, not by arrival
}
```

`wg.Wait()` in a goroutine is the pattern for "close `resultChan` only after
every worker has finished". Without it you would close `resultChan` while
workers might still be sending — a panic. The `for res := range resultChan`
loop drains everything and exits when the channel is closed.

The line `results[res.index] = res.wav` is how you recover input order even
though results arrive in arbitrary order. Worker A might finish file 7 before
worker B finishes file 2; the index routes each result to the right slot.

---

## The same pattern in cgltf-go: ParseBatch

`cgltf-go/cgltf.go` contains an identical four-part structure for 3D model
files. Compare the result struct:

```go
// from cgltf-go/cgltf.go
type result struct {
    gltf  *GLTF
    err   error
    index int
}
```

And the collection loop:

```go
// from cgltf-go/cgltf.go
results := make([]*GLTF, len(dataList))
for res := range resultChan {
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }
    if res.err != nil {
        return nil, fmt.Errorf("failed to parse glTF at index %d: %w", res.index, res.err)
    }
    results[res.index] = res.gltf
}
```

The pattern is identical to `dr-wav-go`. Once you read one, you have read them
all. This is intentional: a predictable, repeatable pattern is easier to audit
and test than creative one-offs.

---

## A twist in miniz-go: parallel compression then raw assembly

`miniz-go/miniz.go` has `CreateArchiveConcurrent`, which compresses ZIP entries
in parallel. It follows the same fan-out / fan-in skeleton, but the assembly
step is more interesting because the work products (raw DEFLATE streams) must
be written into the ZIP file in a specific order.

```go
// from miniz-go/miniz.go
type job struct {
    entry FileEntry
    index int
}
jobCh    := make(chan job,            len(files))
resultCh := make(chan compressedFile, len(files))
```

`compressedFile` carries the pre-compressed bytes, CRC, uncompressed size, and
the original `index`:

```go
// from miniz-go/miniz.go
type compressedFile struct {
    name       string
    compressed []byte
    crc        uint32
    rawSize    uint64
    index      int
    err        error
}
```

After collecting results by index into a slice, the function calls
`buildRawZip`:

```go
// from miniz-go/miniz.go
results := make([]compressedFile, len(files))
for r := range resultCh {
    if r.err != nil {
        return nil, fmt.Errorf("failed to compress %q: %w", r.name, r.err)
    }
    results[r.index] = r
}
// ...
return buildRawZip(results)
```

`buildRawZip` uses `zip.CreateRaw` to write the already-compressed bytes
straight into the ZIP entry without re-compressing them:

```go
// from miniz-go/miniz.go
w, err := zw.CreateRaw(fh)
// ...
w.Write(r.compressed)
```

!!! warning "The double-compression bug"
    An earlier version wrote the pre-deflated `compressed` bytes via a normal
    `zip.Create` entry (method = Deflate). The ZIP library then *deflated them
    again*, producing a valid-but-corrupt archive that could not round-trip.
    `zip.CreateRaw` bypasses the second compression pass.  A round-trip test
    (`CreateArchiveConcurrent` → `ExtractArchive` → compare bytes) is the
    regression guard. If you ever change the assembly step, run that test first.

---

## Channel sizing and the deadlock risk

!!! warning "Buffer size must cover every possible send"
    `resultChan` must be large enough that no worker ever blocks on send while
    the collector goroutine is not yet reading. In the simple case — N jobs,
    each producing exactly one result — `make(chan result, N)` is correct.

    This repo had a real deadlock in `jsmn-go/parallel.go`: the results channel
    was buffered to `numJobs`, but workers could send an *extra* result on the
    cancel branch, so the last worker blocked forever on send while
    `wg.Wait()` hung. The fix was `numJobs + numWorkers`. See
    [Lesson 14](14-the-deadlock-bug.md) for the full story.

The `ParseBatch` functions sidestep this by buffering `resultChan` to
`len(dataList)` — one slot per input — so every worker can always send
without waiting, regardless of how ctx cancellation plays out.

---

## Context cancellation: don't leak goroutines

Every worker loop in this repo selects on `ctx.Done()`:

```go
// from dr-wav-go/dr_wav.go (workers section)
select {
case <-ctx.Done():
    return
case work, ok := <-dataChan:
    // ...
}
```

And the feeder also checks:

```go
// from dr-wav-go/dr_wav.go (feeder section)
select {
case <-ctx.Done():
    close(dataChan)
    return
case dataChan <- struct{ ... }{data, i}:
}
```

Without these checks, cancelling the context would leave goroutines blocked on
`dataChan` or `resultChan` forever — a goroutine leak that accumulates until
the process runs out of memory. Both the feeder and each worker must observe
the context.

---

!!! note "Try it"
    Run the dr-wav-go tests with the race detector enabled:

    ```bash
    cd /path/to/safeheaders-go
    go test -race ./dr-wav-go/...
    ```

    Expected outcome: all tests pass with no data race reports. The race
    detector instruments every channel send/receive and memory access across
    goroutines, so a buffer-size mistake or a missing `ctx.Done()` check would
    surface here rather than in a production crash.

    To see the concurrency working, add `-v` and watch the test names scroll by
    in parallel:

    ```bash
    go test -race -v ./dr-wav-go/... ./cgltf-go/... ./miniz-go/...
    ```

---

## Key takeaways

- A worker pool needs four coordinated parts: **buffered channels**, a fixed
  **worker loop** (select on job channel + ctx), a **feeder goroutine** that
  closes the job channel when done, and a **closer goroutine** that calls
  `wg.Wait()` then closes the results channel.
- **Buffer sizes matter.** Size the results channel to the maximum number of
  sends any worker could produce, not just the number of jobs; under-buffering
  causes silent deadlocks.
- **Tag every job with its original index** and write results into a
  pre-allocated slice by that index. Workers finish in any order; the index
  restores the order the caller expects.
- **Every goroutine must select on `ctx.Done()`** — feeder and workers alike —
  otherwise context cancellation leaks goroutines.
- **Assembly order matters beyond parsing.** In `miniz-go`, collecting results
  by index ensures the ZIP file entries appear in the declared order, and using
  `zip.CreateRaw` avoids the double-compression trap that an earlier version
  fell into.
