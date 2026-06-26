# 15 - Data races and the -race detector

> **Objectives:** Understand what a data race is and why Go treats it as
> undefined behavior; learn to find races with the `-race` detector; see a real
> race from this repository (H3 in `linenoise-go`) and the `sync.Mutex` pattern
> that fixed it.
> Estimated time: 20 minutes.

## What this actually means (plain English)

- **Two cooks, one shopping list.** A data race happens when two goroutines
  read and write the same memory at the same time without taking turns. Like
  two cooks both scribbling on the same notepad — the result is illegible.
- **"Undefined behavior" is not a polite way of saying "probably fine."**
  In Go, a race means the compiler and runtime can produce *any* outcome: torn
  writes, stale reads, silent data loss, or a crash. The program may even appear
  to work most of the time and fail only under load.
- **The `-race` detector is a referee.** Compile with `-race` and Go instruments
  every memory access; if two goroutines touch the same variable without
  synchronization, you get a clear `WARNING: DATA RACE` report pointing to the
  exact lines.
- **A `sync.Mutex` is a token.** Only the goroutine holding the token may touch
  the protected data. All others wait at `Lock()` until it is returned via
  `Unlock()`.
- **Snapshot-under-lock** is a pattern: take the lock, copy what you need into
  a local variable, release the lock, then do slow work (like file I/O) on the
  copy. This keeps critical sections short.
- **Global convenience functions hide shared state.** A package-level singleton
  that multiple callers use concurrently is a race waiting to happen — and the
  callers have no idea.

**Why it matters:** A data race on a `[]string` history slice can corrupt the
slice header itself — length, capacity, pointer — turning a safe append into a
write past the end of a GC'd array.

---

## The real bug: H3 in `linenoise-go`

`linenoise-go` is a CLI line-editor (a Go port of
[antirez/linenoise](https://github.com/antirez/linenoise)). It exposes two
tiers of API: a full `*State` object for programs that manage their own
instance, and a set of package-level convenience functions for simple scripts:

```go
// from linenoise-go/linenoise.go
var defaultState = New(DefaultConfig())

func AddHistory(line string)         { defaultState.AddHistory(line) }
func SaveHistory(filename string) error { return defaultState.SaveHistory(filename) }
func LoadHistory(filename string) error { return defaultState.LoadHistory(filename) }
func ClearHistory()                  { defaultState.ClearHistory() }
```

Every call goes through the same singleton `defaultState`. If two goroutines
call `AddHistory` and `LoadHistory` concurrently, they both read and write
`defaultState.history` — a plain `[]string` — at the same time.

### What goes wrong

`history` is a slice. In Go, a slice header is three words: pointer, length,
capacity. If goroutine A is in the middle of `append` (which may allocate a new
backing array and update all three words) while goroutine B is reading the
slice, B can see a torn header: a new pointer with the old length, or the old
pointer with the new length. Either can produce a read past the end of the
array — undefined behavior.

The 10-agent audit (recorded in
`docs/audits/2026-06-23-code-review-security-audit.md`, finding H3) reproduced
this directly:

> `go test -race`: WARNING: DATA RACE at read `:523`, write `:528`, write `:563`.

Lines 523/528/563 in the pre-fix file were `LoadHistory` clearing the slice and
`AddHistory` appending to it — both touching `s.history` bare, no lock.

---

## The fix: `sync.Mutex` on every history access

The `State` struct now carries a mutex:

```go
// from linenoise-go/linenoise.go
type State struct {
    // mu guards history (and historyIndex/draftLine) so the package-level
    // convenience functions, which all share one defaultState, are safe to call
    // from multiple goroutines. A per-goroutine State needs no external locking.
    mu      sync.Mutex
    config  *Config
    history []string
    // ...
}
```

Every method that touches `history` locks before reading or writing:

```go
// from linenoise-go/linenoise.go
func (s *State) AddHistory(line string) {
    line = strings.TrimSpace(line)
    if line == "" {
        return
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    if len(s.history) > 0 && s.history[len(s.history)-1] == line {
        return
    }
    s.history = append(s.history, line)
    if s.config.HistoryMaxLen > 0 && len(s.history) > s.config.HistoryMaxLen {
        s.history = s.history[len(s.history)-s.config.HistoryMaxLen:]
    }
}
```

`LoadHistory` is similarly guarded:

```go
// from linenoise-go/linenoise.go
func (s *State) LoadHistory(filename string) error {
    // ... read file into `loaded` slice without holding the lock ...

    s.mu.Lock()
    defer s.mu.Unlock()
    s.history = loaded
    // ...
    return nil
}
```

Notice that `LoadHistory` does the file I/O *before* acquiring the lock, then
swaps the result in while holding it. That is the snapshot-under-lock pattern
in reverse: do slow I/O outside the lock, hold the lock only for the final
in-memory update.

`SaveHistory` applies the classic forward direction — snapshot under lock, write
outside:

```go
// from linenoise-go/linenoise.go
func (s *State) SaveHistory(filename string) error {
    // Snapshot under lock, then do file I/O without holding it.
    s.mu.Lock()
    snapshot := append([]string(nil), s.history...)
    s.mu.Unlock()

    f, err := os.Create(filename)
    // ... write snapshot to f ...
    return nil
}
```

The lock is held only long enough to copy the slice. All the disk I/O happens
outside, so other goroutines are not blocked waiting for a slow write.

### History navigation also locks

`historyPrev` and `historyNext` (called from `ReadLine` while a goroutine
navigates history with the arrow keys) also acquire the lock before reading
`s.history`:

```go
// from linenoise-go/linenoise.go
func (s *State) historyPrev() {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.history) == 0 {
        return
    }
    // ...
}
```

This matters because a background goroutine could call `AddHistory` while the
user is pressing the up-arrow key.

---

## Why `-race` catches what code review misses

Code review could notice the missing lock in `AddHistory`, but it is easy to
miss the navigation methods buried deeper in the file. The race detector finds
*all* of them automatically, because it instruments every memory access at
runtime.

!!! warning "The race detector requires real concurrency to fire"
    `-race` only catches races that *actually execute* during the test run. A
    test that calls `AddHistory` from a single goroutine will pass cleanly even
    on the unfixed code. The test has to exercise the concurrent path. See the
    "Try it" box below.

!!! note "The race detector has a runtime cost"
    Binaries compiled with `-race` run 2–20× slower and use more memory. Use
    it in CI and development, not in production builds. In this repo the
    `go-ci.yaml` workflow runs a dedicated `race` job.

---

## Writing a test that proves the race is gone

A race test has to actually run two goroutines against the shared state
simultaneously. The audit's fix commit (`01620eb`) added exactly this:

```go
// Illustrative — based on the fix described in the audit report.
func TestAddHistoryRace(t *testing.T) {
    s := linenoise.New(linenoise.DefaultConfig())
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            s.AddHistory(fmt.Sprintf("cmd-%d", n))
        }(i)
    }
    wg.Wait()
}
```

Run it without `-race` and it will likely pass. Run it *with* `-race` and the
pre-fix code would print a `DATA RACE` report immediately. After the fix, both
modes pass.

!!! note "Try it"
    From the repo root, run the linenoise race test:

    ```bash
    cd linenoise-go && go test -race -run TestHistory -v ./...
    ```

    **Expected outcome:** all tests pass, no `WARNING: DATA RACE` output. If you
    revert the mutex from `AddHistory` and rerun, you should see a race warning
    within a few iterations. Predict before you run: without `-race` the test
    will still pass (races are not detected), but with it the detector will catch
    the unsynchronized concurrent writes.

    To fuzz-test the parser rather than the history:
    ```bash
    cd linenoise-go && go test -race ./...
    ```

---

## Connecting to the other concurrency bugs

The deadlock bugs (H1 in `jsmn-go`, H2 in `stb-image-go`, covered in
[Lesson 14](14-the-deadlock-bug.md)) and the data race here (H3 in
`linenoise-go`) are both concurrency defects, but they fail in different ways:

| Bug | Symptom | Detector |
|-----|---------|---------|
| Deadlock (H1, H2) | Program hangs forever | Watchdog test / timeout |
| Data race (H3) | Corrupted data or crash — non-deterministic | `-race` detector |

Deadlocks are usually reproducible given the right input. Races can be silent
for months and then corrupt production data under load.

---

## Key takeaways

- A **data race** happens when two goroutines access shared memory concurrently
  and at least one access is a write, with no synchronization between them. Go
  treats this as undefined behavior.
- The **`-race` detector** instruments every memory access at compile time and
  reports races the moment they occur at runtime. Run it in CI on every PR.
- Use **`sync.Mutex`** to guard shared mutable state. Every read *and* write of
  the protected data must happen while holding the lock.
- The **snapshot-under-lock** pattern keeps critical sections short: lock, copy
  the data locally, unlock, then do slow work (I/O, computation) on the copy.
- Global convenience functions that share a singleton (like `linenoise`'s
  `defaultState`) are a hidden concurrency hazard — callers cannot tell that
  their "simple" function call shares state with every other caller.
