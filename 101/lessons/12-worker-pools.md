# 12 · Worker pools: fan-out / fan-in

> **Objectives:** Understand how to spread independent work across a fixed number of goroutines using a jobs channel and a results channel. Learn the fan-out / fan-in pattern in full — feeder goroutine, worker pool, WaitGroup, and ordered result collection — by reading it directly from this repository's production code.
> Estimated time: 25 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Fan-out** = "one dispatcher, many couriers — all running at once." The same pile of WAV or glTF chunks is handed to a fixed pool of goroutines simultaneously rather than processed one at a time, so every CPU core stays busy.
- **Fan-in** = "all couriers drop their receipts into one shared inbox." Every worker sends its parsed result onto `resultCh`; the main goroutine drains that single channel to collect everything.
- **Jobs channel** = "the dispatcher's conveyor belt." Workers pull from `dataChan` / `jobCh` whenever they are free; the channel delivers work at whatever pace each worker can consume.
- **Results channel** = "the inbox on your desk, pre-sized so nobody waits." `resultCh` is buffered to the number of inputs so any worker can drop its answer in immediately without blocking on the receiver.
- **`sync.WaitGroup`** = "a foreman's sign-in sheet." Each goroutine calls `wg.Add(1)` when it starts and `wg.Done()` when it finishes; `wg.Wait()` in a separate goroutine fires `close(resultCh)` the instant the last worker signs out.
- **Ordering** = "numbered luggage tags." Because workers finish in arbitrary order, each job carries its original slice index; results are written to `results[res.index]` so the output matches the input sequence regardless of which worker finished first.

**Why it matters:** parsing audio files, 3D models, or ZIP entries one at a
time leaves most CPU cores idle; a worker pool can saturate all cores while
keeping memory usage bounded and result order predictable.

**See it — fan-out / fan-in.** One feeder tags each chunk with its index and
drops it on `jobCh`. A fixed set of workers pull, parse, and push answers onto
`resultCh`. Because every result carries its index, the collector writes it into
`slice[idx]` — so the output is in input order even though workers finish in any
order.

<svg viewBox="0 0 720 300" role="img" aria-labelledby="wp-t wp-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="wp-t">Worker pool fan-out and fan-in</title>
  <desc id="wp-d">A jobs channel feeds a fixed set of workers; each worker pushes an indexed result onto a results channel, which is reassembled in order.</desc>
  <defs><marker id="wp-ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto"><path d="M0,0 L7,3 L0,6 Z" fill="var(--md-accent-fg-color,#00897b)"/></marker></defs>
  <text x="100" y="36" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">feeder: one job per chunk</text>
  <rect x="36" y="48" width="128" height="150" rx="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="100" y="68" text-anchor="middle" font-size="12" fill="currentColor" font-family="ui-monospace,monospace">jobCh</text>
  <rect x="52" y="78" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="100" y="93" text-anchor="middle" font-size="10" fill="currentColor">j0 · idx 0</text>
  <rect x="52" y="106" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="100" y="121" text-anchor="middle" font-size="10" fill="currentColor">j1 · idx 1</text>
  <rect x="52" y="134" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="100" y="149" text-anchor="middle" font-size="10" fill="currentColor">j2 · idx 2</text>
  <rect x="52" y="162" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="100" y="177" text-anchor="middle" font-size="10" fill="currentColor">j3 · idx 3</text>
  <text x="232" y="64" text-anchor="middle" font-size="11" fill="var(--md-accent-fg-color,#00897b)">fan-out</text>
  <rect x="298" y="70" width="132" height="44" rx="6" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="364" y="90" text-anchor="middle" font-size="12" fill="currentColor">worker 1</text><text x="364" y="106" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">parse chunk</text>
  <rect x="298" y="126" width="132" height="44" rx="6" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="364" y="146" text-anchor="middle" font-size="12" fill="currentColor">worker 2</text><text x="364" y="162" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">parse chunk</text>
  <rect x="298" y="182" width="132" height="44" rx="6" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="364" y="202" text-anchor="middle" font-size="12" fill="currentColor">worker 3</text><text x="364" y="218" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">parse chunk</text>
  <path d="M164,120 L294,92" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <path d="M164,128 L294,148" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <path d="M164,136 L294,204" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <text x="494" y="64" text-anchor="middle" font-size="11" fill="var(--md-accent-fg-color,#00897b)">fan-in</text>
  <path d="M430,92 L552,120" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <path d="M430,148 L552,128" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <path d="M430,204 L552,136" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#wp-ah)"/>
  <text x="620" y="36" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">results[idx] — ordered</text>
  <rect x="556" y="48" width="128" height="150" rx="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="620" y="68" text-anchor="middle" font-size="12" fill="currentColor" font-family="ui-monospace,monospace">resultCh</text>
  <rect x="572" y="78" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="620" y="93" text-anchor="middle" font-size="10" fill="currentColor">r0 · idx 0</text>
  <rect x="572" y="106" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="620" y="121" text-anchor="middle" font-size="10" fill="currentColor">r1 · idx 1</text>
  <rect x="572" y="134" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="620" y="149" text-anchor="middle" font-size="10" fill="currentColor">r2 · idx 2</text>
  <rect x="572" y="162" width="96" height="22" rx="3" fill="none" stroke="var(--md-default-fg-color--lighter)"/><text x="620" y="177" text-anchor="middle" font-size="10" fill="currentColor">r3 · idx 3</text>
  <text x="360" y="262" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)"><tspan font-family="ui-monospace,monospace">wg.Wait()</tspan> closes resultCh once every worker signs out (wg.Done)</text>
  <text x="360" y="280" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">each result written to slice[idx] → input order preserved</text>
</svg>

---

## The skeleton: four moving parts

Every worker pool in this repo follows the same four-part shape. Here it is
written out abstractly before we look at the real code (this is pseudocode —
a simplified sketch meant to show the shape of the pattern, not a real,
runnable file):

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

**In plain terms:** a "channel" (`chan`) is a pipe that one part of the program can drop values into and another part can pull values out of, safely, even when many goroutines are doing this at the same time — it is Go's built-in way of passing work and results between independently-running pieces of code. A "goroutine" is a lightweight, independently-running piece of a program — Go can run thousands of them at once, and the `go` keyword in front of a function call is what starts one. This skeleton creates two channels (one for incoming jobs, one for outgoing results), starts a fixed number of worker goroutines that each loop "pull a job, do the work, drop the result," starts a separate "feeder" goroutine that hands out the jobs, and starts one more goroutine whose only task is to wait for every worker to finish and then seal the results channel shut (`close`) so the part collecting results below knows there is nothing more coming. `sync.WaitGroup` (met in full below) is the counter used to track how many workers are still running.

Each number above maps to a concrete code location you can read right now.

---

## Seeing it in dr-wav-go: ParseBatch

[`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md) is a file inside this project — a chunk of source code with its own name, grouped with related code into what Go calls a "package" (a folder of `.go` files that work together and can be reused by other code via `import`). It contains `ParseBatch`, a function — a named, reusable block of code that you can "call" (run) by writing its name, optionally handing it some inputs, and it optionally hands back ("returns") a result to whoever called it. `ParseBatch` decodes multiple WAV files
concurrently (concurrently means several of these tasks are in progress over the same stretch of time, rather than strictly one after another). It is a clean, self-contained example of all four parts.

### Part 1 — the channels

```go
// from dr-wav-go/dr_wav.go
dataChan := make(chan struct {
    data  []byte
    index int
}, len(dataList))
resultChan := make(chan result, len(dataList))
```

**In plain terms:** this creates two channels. `struct { data []byte; index int }` is a "struct" — a small custom bundle that groups several named pieces of data together (here: some raw file bytes, plus the position that file had in the original list). "Fields" is the term for those named pieces inside a struct (`data` and `index` are fields). `[]byte` is a "slice" of "bytes" — a byte is one small unit of computer memory (think: one letter of raw data), and a slice is a resizable, ordered list of them, here holding the file's contents. `len(dataList)` reads the number of items currently in `dataList` and uses that number to size the channel's buffer — how many items it can hold before anyone has to wait to add more.

Both channels are buffered to `len(dataList)`. That size matters: if
`resultChan` were unbuffered, a worker would block trying to send a result while
the main goroutine was still sending jobs — instant deadlock. (To "block" means the line of code simply waits — that goroutine pauses right there and does nothing else — until some condition is satisfied, in this case until there is room to place the result on the channel.) A "deadlock" is when two or more goroutines are each stuck waiting on the other, so nothing ever moves forward again. Buffering to the
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

**In plain terms:** `runtime.NumCPU()` asks the machine how many processor cores it has, and the code uses that number as how many worker goroutines to start (capped so it never starts more workers than there are files). Each worker runs an endless loop that uses `select` — a Go construct that waits on several channels at once and proceeds with whichever one has something ready first. Here a worker either notices the shared context has been cancelled (`ctx.Done()`) and stops, or it receives the next job off `dataChan`, runs `Parse` on it, and sends the outcome to `resultChan`. `ok` reports whether the channel is still open — once the feeder closes `dataChan`, further receives report `ok == false` and the worker returns (a function "returning" means it finishes running and hands control, and optionally a value, back to whatever invoked it).

The `runtime.NumCPU()` cap keeps you from spawning 10,000 goroutines for
10,000 files; goroutine creation is cheap but not free. The `select` on
`ctx.Done()` means the caller can cancel mid-batch and all workers exit
promptly — no goroutine leaks. (A "context", `ctx`, is a small object Go code passes around to signal things like "please stop now" or "time's up" to every piece of work that's in flight. A "leak" here means a goroutine that never gets cleaned up and just sits there forever, quietly wasting memory.)

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

**In plain terms:** the feeder walks through every item in `dataList` and tries to place each one, tagged with its original position `i`, onto `dataChan`. Each attempt is itself a `select`: either the send onto `dataChan` succeeds, or `ctx.Done()` fires first (someone cancelled), in which case the feeder closes the channel and stops early.

The feeder runs in its own goroutine. This is essential: if the feeder ran
inline (before starting workers) — meaning as an ordinary, directly-run part of the same sequence of steps, rather than kicked off separately with `go` — it would fill `dataChan` and then block if
there were more items than buffer slots, with no workers yet reading. Putting
the feeder in a goroutine lets it run concurrently with the workers that are
already pulling from the channel.

Closing `dataChan` at the end is the shutdown signal: workers see `ok == false`
on the next receive and return, decrementing the WaitGroup (lowering its internal count of "still running" goroutines by one).

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

**In plain terms:** `wg.Wait()` blocks (pauses) that goroutine until every worker has called `wg.Done()` — i.e., until the count `sync.WaitGroup` is tracking drops to zero — and only then closes `resultChan`. Meanwhile, `make([]*WAV, len(dataList))` "allocates" a slice — reserves a chunk of the computer's memory — sized to hold one result per input file, pre-filled with empty placeholders, and the loop below fills in each slot.

`wg.Wait()` in a goroutine is the pattern for "close `resultChan` only after
every worker has finished". Without it you would close `resultChan` while
workers might still be sending — a panic (Go's term for a crash: the program stops abruptly because something went wrong that it doesn't know how to recover from). The `for res := range resultChan`
loop drains everything and exits when the channel is closed.

The line `results[res.index] = res.wav` is how you recover input order even
though results arrive in arbitrary order. Worker A might finish file 7 before
worker B finishes file 2; the index routes each result to the right slot.

---

## The same pattern in cgltf-go: ParseBatch

[`cgltf-go/cgltf.go`](src/cgltf-go-cgltf-go.md) contains an identical four-part structure for 3D model
files (glTF is a common file format for 3D models). Compare the result struct:

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
and test — verify with automated code that checks the real behavior is correct — than creative one-offs.

---

## A twist in miniz-go: parallel compression then raw assembly

[`miniz-go/miniz.go`](src/miniz-go-miniz-go.md) has `CreateArchiveConcurrent`, which compresses ZIP entries
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

`compressedFile` carries the pre-compressed bytes, CRC (a small checksum number used to detect if data got corrupted), uncompressed size, and
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

**In plain terms:** this opens a "raw" entry in the ZIP file being built and writes the already-compressed bytes straight in, unchanged — skipping the normal step where the ZIP library would compress the data itself.

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

    This repo had a real deadlock in [`jsmn-go/parallel.go`](src/jsmn-go-parallel-go.md): the results channel
    was buffered to `numJobs`, but workers could send an *extra* result on the
    cancel branch, so the last worker blocked forever on send while
    `wg.Wait()` hung. The fix was `numJobs + numWorkers`. See
    [Lesson 14](14-the-deadlock-bug.md) for the full story.

The `ParseBatch` functions sidestep this by buffering `resultChan` to
`len(dataList)` — one slot per input — so every worker can always send
without waiting, regardless of how ctx cancellation plays out (ctx cancellation = the context signaling "stop now," as covered above).

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
    Run the dr-wav-go tests (a test is a small piece of code written to automatically check that another piece of code behaves correctly) with the race detector enabled:

    ```bash
    cd /path/to/safeheaders-go
    go test -race ./dr-wav-go/...
    ```

    **In plain terms:** `go test` compiles (turns the human-written source text into a program the machine can run) and runs the tests in the `dr-wav-go` package, and `-race` turns on Go's "race detector" — a tool that watches for two goroutines touching the same piece of memory at the same time in an unsafe way.

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
