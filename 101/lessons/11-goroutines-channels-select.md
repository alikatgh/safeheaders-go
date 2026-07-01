# 11 · Goroutines, Channels and Select

> **Objectives:** Understand how Go expresses concurrency with goroutines and channels,
> learn how buffered channels differ from unbuffered ones, and see how `select` lets a
> worker react to multiple events at once.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Goroutine** = "a worker you hire on a sticky note — costs almost nothing to write, runs in the background, and you can hire thousands." A goroutine launched with `go f()` runs concurrently with the caller for only a few kilobytes of stack, unlike a full OS thread.
- **Channel** = "a pneumatic tube connecting two rooms — one side posts a value in, the other pulls it out, and no one needs to reach into the other room's drawers." Channels let goroutines exchange typed values without shared memory.
- **Buffered channel** = "a tube with a small waiting room at the end — the sender can drop several items in the queue before anyone picks them up." `make(chan T, N)` holds up to N values so the sender does not block until the buffer is full; an unbuffered `make(chan T)` forces sender and receiver to be present at exactly the same moment.
- **`select`** = "a dispatcher watching several inboxes at once — whichever tray has mail first gets handled, and if two arrive together it picks one at random." `select` lets a worker react to whichever of its channels is ready, preventing starvation through that built-in randomness.
- **`close(ch)`** = "hanging a 'no more deliveries today' sign on the tube — anyone still listening can drain what's left and then leave." Producers call `close(ch)` to signal completion; receivers detect it with `range ch` or the `v, ok := <-ch` idiom.

**Why it matters:** every hardening problem in this repo — concurrency, cancellation,
deadlines — is solved with this handful of primitives. If you can read a `select`
loop, you can read all of it.

**See it — goroutines, a buffered jobs channel, and a select worker loop.**

<svg viewBox="0 0 700 320" role="img" aria-labelledby="t11 d11" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t11">Goroutines, buffered channel, and select worker loop</title>
  <desc id="d11">Diagram showing a producer filling a buffered jobs channel, then three goroutine workers each running a select loop that reads from jobs or reacts to ctx.Done(), and finally a buffered errs channel collecting results.</desc>
  <defs>
    <marker id="l11-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
  </defs>
  <rect x="20" y="120" width="130" height="48" rx="6" ry="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="85" y="140" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Producer</text>
  <text x="85" y="157" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">fills &amp; closes jobs</text>
  <rect x="195" y="110" width="140" height="68" rx="6" ry="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="265" y="132" text-anchor="middle" font-size="12" fill="var(--md-accent-fg-color,#00897b)" font-weight="600">jobs channel</text>
  <text x="265" y="150" text-anchor="middle" font-size="10" fill="currentColor">make(chan int, N)</text>
  <text x="265" y="167" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">buffered · closed after fill</text>
  <line x1="150" y1="144" x2="193" y2="144" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.6" marker-end="url(#l11-arrow)"/>
  <rect x="195" y="240" width="140" height="44" rx="6" ry="6" fill="none" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.2"/>
  <text x="265" y="258" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">ctx.Done()</text>
  <text x="265" y="274" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">cancellation signal</text>
  <rect x="395" y="30" width="130" height="64" rx="6" ry="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="460" y="52" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Worker 1</text>
  <text x="460" y="68" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">go func() · select</text>
  <text x="460" y="83" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">{ jobs | ctx.Done }</text>
  <rect x="395" y="118" width="130" height="64" rx="6" ry="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="460" y="140" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Worker 2</text>
  <text x="460" y="156" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">go func() · select</text>
  <text x="460" y="171" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">{ jobs | ctx.Done }</text>
  <rect x="395" y="206" width="130" height="64" rx="6" ry="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="460" y="228" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Worker 3</text>
  <text x="460" y="244" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">go func() · select</text>
  <text x="460" y="259" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">{ jobs | ctx.Done }</text>
  <line x1="335" y1="134" x2="393" y2="62" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l11-arrow)"/>
  <line x1="335" y1="144" x2="393" y2="150" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l11-arrow)"/>
  <line x1="335" y1="155" x2="393" y2="238" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l11-arrow)"/>
  <line x1="335" y1="258" x2="393" y2="88" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.2" stroke-dasharray="4 3" marker-end="url(#l11-arrow)"/>
  <line x1="335" y1="262" x2="393" y2="176" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.2" stroke-dasharray="4 3" marker-end="url(#l11-arrow)"/>
  <line x1="335" y1="262" x2="393" y2="252" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.2" stroke-dasharray="4 3" marker-end="url(#l11-arrow)"/>
  <rect x="560" y="118" width="120" height="64" rx="6" ry="6" fill="none" stroke="#e5484d" stroke-width="1.4"/>
  <text x="620" y="140" text-anchor="middle" font-size="12" fill="#e5484d" font-weight="600">errs channel</text>
  <text x="620" y="156" text-anchor="middle" font-size="10" fill="currentColor">make(chan error,</text>
  <text x="620" y="171" text-anchor="middle" font-size="10" fill="currentColor">N+workers)</text>
  <line x1="525" y1="70" x2="558" y2="138" stroke="#e5484d" stroke-width="1.2" marker-end="url(#l11-arrow)"/>
  <line x1="525" y1="150" x2="558" y2="150" stroke="#e5484d" stroke-width="1.2" marker-end="url(#l11-arrow)"/>
  <line x1="525" y1="245" x2="558" y2="165" stroke="#e5484d" stroke-width="1.2" marker-end="url(#l11-arrow)"/>
  <line x1="20" y1="295" x2="45" y2="295" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.6" marker-end="url(#l11-arrow)"/>
  <text x="50" y="299" font-size="10" fill="var(--md-default-fg-color--light)">work flow</text>
  <line x1="120" y1="295" x2="145" y2="295" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.2" stroke-dasharray="4 3" marker-end="url(#l11-arrow)"/>
  <text x="150" y="299" font-size="10" fill="var(--md-default-fg-color--light)">cancellation</text>
  <line x1="240" y1="295" x2="265" y2="295" stroke="#e5484d" stroke-width="1.2" marker-end="url(#l11-arrow)"/>
  <text x="270" y="299" font-size="10" fill="var(--md-default-fg-color--light)">errors</text></svg>

---

## The simplest goroutine

```go
go fmt.Println("hello from a goroutine")
```

**In plain terms:** this line starts a tiny background task that prints a message, and does not make the rest of the program wait for it to finish.

`go` before any function call (a named, reusable chunk of instructions you can run — "call" or "invoke" a function means "run it") spins it into the background. The caller (the code that started it running) keeps running
immediately. If `main` returns first — that is, if the program's entry-point function finishes and hands control back before the background task is done — the goroutine (a lightweight, independently-running task; Go's version of a "worker" that costs almost nothing to start, as opposed to a full operating-system thread) is silently killed, so production
code always synchronises with `sync.WaitGroup` (a counter object that lets one part of the program wait until a group of background tasks has finished) or channels (typed pipes that let two goroutines pass values safely between them).

```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    fmt.Println("working…")
}()
wg.Wait() // block until the goroutine calls Done()
```

**In plain terms:** `wg.Add(1)` says "I'm about to start one background task," the goroutine runs and calls `Done()` when it finishes (`defer` means "run this line automatically right before the surrounding function exits, no matter how it exits"), and `wg.Wait()` makes the main program pause — "block" means the line simply waits there and does nothing else — until that task reports it is done.

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

**In plain terms:** `make(chan int)` creates a pipe that can carry whole numbers between goroutines. The background task tries to send the number 42 through it, but that send simply waits until something on the other end is ready to receive; the main code's `v := <-ch` line does that receiving, and both sides unblock together the instant they meet.

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

**In plain terms:** the code drops five numbers into the pipe, then closes it to mean "nothing more is coming." The `for v := range ch` loop pulls values out one at a time and stops automatically once the pipe is both empty and closed — no explicit "are we done?" check needed.

`range` (a loop keyword that steps through every item in something, one at a time) on a channel drains it and exits the loop when the channel is closed.
Sending to a closed channel panics (the program hits an unrecoverable error and stops, unless something explicitly catches it), so only the producer should call `close`.

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

**In plain terms:** `select` waits on two pipes at once. If a message arrives on `inbox` first, it gets processed; if instead the cancellation signal `ctx.Done()` fires first, the function returns (finishes and hands its result — here, an error — back to whoever called it) immediately. Whichever pipe has something ready first wins.

If both channels are ready simultaneously, Go picks one uniformly at random.
That randomness is a feature: it prevents starvation (a situation where one task never gets its turn because something else is always chosen ahead of it).

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

## Reading the real worker loop — [`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md)

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

**In plain terms:** figure out how many CPU cores the machine has, then cap the number of workers at however many images there actually are (a slice — an ordered, resizable list of values, here `datas` — holds those images; `len(datas)` counts how many items are in it).

No point spawning more workers than there are images.

### Step 2 — pre-fill a buffered jobs channel, then close it

```go
jobs := make(chan int, len(datas))
for i := 0; i < len(datas); i++ {
    jobs <- i          // send the image *index*, not the image itself
}
close(jobs)            // signal: no more work will arrive
```

**In plain terms:** create a buffered pipe big enough to hold one number per image, drop in each image's position number (its "index" — its position, counting from 0, in the list) rather than the image data itself, then close the pipe to announce that all the work has been posted.

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

**In plain terms:** make one more buffered pipe, this time for errors, sized large enough to hold a failure message from every single image PLUS a cancellation message from every worker — the worst case where everything goes wrong at once.

This comment is the most important line in the file. If the buffer were only
`len(datas)`, a cancelled context (Go's built-in mechanism for signalling "stop now" — a deadline or a cancel — across goroutines) could cause every worker to *also* try to
send `ctx.Err()` on the same channel. With N workers all trying to send and
nobody reading yet, the `N`th send blocks. The goroutine is stuck. `wg.Wait`
waits for it. Deadlock (every remaining goroutine is permanently stuck waiting on something that will never happen, so the program hangs forever). The fix is to reserve space for both failure kinds.

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

**In plain terms:** launch `numWorkers` background tasks. Each one loops forever until it decides to stop: first it checks whether cancellation has already happened; then it waits on `select` for either a job index to pull from `jobs`, or a cancellation signal. If it gets a job, it loads that image and records either the result or an error; if `jobs` is closed and empty, or cancellation fires, it exits the loop and reports it is done.

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
4. **Write to `results[idx]`** — no mutex (a lock that lets only one goroutine touch a
   shared piece of data at a time, preventing two workers from corrupting it by writing
   at once) needed because each goroutine owns
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
