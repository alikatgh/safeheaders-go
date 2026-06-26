# 21 · Table tests, -race and coverage

> **Objectives:** Understand idiomatic Go test structure — `func TestXxx`, table-driven
> subtests, and helper utilities like `t.Helper` and `t.TempDir`. Learn how to run the
> race detector and read coverage numbers. See every technique demonstrated in the real
> `linenoise-go` test file.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **`func TestXxx(t *testing.T)`** is the entry point Go's test runner looks for. Any
  exported function starting with `Test` in a `_test.go` file gets run automatically by
  `go test`.
- **Table-driven tests** are just a slice of structs, one per scenario, looped over.
  Instead of copy-pasting the same assertion ten times, you describe the inputs and
  expected outputs as data, then drive them through one shared block of code.
- **`t.Run("name", func(t *testing.T) {...})`** creates a named subtest. Each subtest
  gets its own pass/fail verdict and can be run in isolation with `-run TestFoo/name`.
- **`t.Helper()`** marks a function as a test helper so that, when it fails, the error
  points to the *caller*, not to the internals of your helper — makes output readable.
- **`t.TempDir()`** gives you an OS temp directory that is automatically removed when
  the test finishes. No manual cleanup, no leftover files on CI.
- **`go test -race`** instruments every memory access and reports data races at runtime.
  It cannot prove the absence of races, but it catches the ones that actually occur.

**Why it matters:** a data race is undefined behaviour — it silently corrupts state in
production but only crashes your program occasionally. The race detector turns "sometimes
wrong" into "always caught in CI".

---

## The test file at a glance

`linenoise-go/linenoise_engine_test.go` is a clean example of all three patterns used
together. The file starts with two helper functions:

```go
// from linenoise-go/linenoise_engine_test.go

func newSinkState(t *testing.T) *State {
    t.Helper()
    out, err := os.CreateTemp(t.TempDir(), "out-*")
    if err != nil {
        t.Fatalf("temp output: %v", err)
    }
    t.Cleanup(func() { _ = out.Close() })
    cfg := DefaultConfig()
    cfg.Output = out
    return New(cfg)
}
```

`t.Helper()` on line one means that if `os.CreateTemp` fails, the test output says
"FAIL at TestFoo:42" (the caller's line) instead of "FAIL at linenoise_engine_test.go:18"
(the helper's line). That one call makes every future failure message useful.

`t.TempDir()` is called once in each of the two helpers shown here (it appears in several places across the file). Both temp
directories are cleaned up automatically when the parent test finishes.

```go
func tempInput(t *testing.T, content string) *os.File {
    t.Helper()
    path := filepath.Join(t.TempDir(), "in")
    if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
        t.Fatalf("write input: %v", err)
    }
    f, err := os.Open(path)
    if err != nil {
        t.Fatalf("open input: %v", err)
    }
    t.Cleanup(func() { _ = f.Close() })
    return f
}
```

Notice the pattern: `t.Cleanup` registers a teardown closure. Go runs all registered
cleanups when the test ends, even if the test panics.

---

## Simple subtests — `t.Run`

`TestProcessCharTerminators` groups four related checks under one parent test by
calling `t.Run` for each:

```go
// from linenoise-go/linenoise_engine_test.go

func TestProcessCharTerminators(t *testing.T) {
    t.Run("enter returns the line", func(t *testing.T) {
        s := newSinkState(t)
        s.buf = []rune("done")
        cont, result, err := s.processChar('\r')
        if cont || err != nil || result != "done" {
            t.Fatalf("got cont=%v result=%q err=%v", cont, result, err)
        }
    })

    t.Run("ctrl-c interrupts", func(t *testing.T) {
        s := newSinkState(t)
        s.buf = []rune("x")
        cont, _, err := s.processChar('\x03')
        if cont || !errors.Is(err, ErrInterrupted) {
            t.Fatalf("got cont=%v err=%v", cont, err)
        }
    })
    // ... two more subtests
}
```

You can run just one subtest from the command line:

```bash
go test -run TestProcessCharTerminators/ctrl-c ./linenoise-go/
```

The `/` separates the parent name from the subtest name. Partial matching works too —
`-run TestProcess` would run all four subtests.

---

## Table-driven subtests

When the test logic is identical but the inputs vary, a slice of structs is cleaner than
repeating `t.Run` calls. `TestHandleEditKeys` is the canonical example from this file:

```go
// from linenoise-go/linenoise_engine_test.go

func TestHandleEditKeys(t *testing.T) {
    mk := func() *State {
        s := newSinkState(t)
        s.buf = []rune("hello world")
        s.pos = len(s.buf)
        return s
    }

    tests := []struct {
        name    string
        key     rune
        wantBuf string
        wantPos int
    }{
        {"insert printable", 'X', "hello worldX", 12},
        {"backspace", '\x7f', "hello worl", 10},
        {"ctrl-a home", '\x01', "hello world", 0},
        {"home key", keyHome, "hello world", 0},
        {"ctrl-k kill to end", '\x0b', "hello world", 11},
        {"ctrl-u kill line", '\x15', "", 0},
        {"ctrl-w del word", '\x17', "hello ", 6},
        {"left arrow", keyLeft, "hello world", 10},
        {"delete key", keyDelete, "hello world", 11},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := mk()
            s.handleEditKey(tt.key)
            if string(s.buf) != tt.wantBuf {
                t.Errorf("buf = %q, want %q", string(s.buf), tt.wantBuf)
            }
            if s.pos != tt.wantPos {
                t.Errorf("pos = %d, want %d", s.pos, tt.wantPos)
            }
        })
    }
}
```

Nine scenarios, one loop. Adding a tenth means appending one struct literal — no new
test function, no duplicated assertion code.

!!! tip "t.Errorf vs t.Fatalf"
    `t.Errorf` marks the test as failed but keeps running, so you see *all* failures in
    one pass. `t.Fatalf` stops the subtest immediately — useful when continuing makes no
    sense (e.g. the object you are about to inspect is nil).

---

## Testing error identity with `errors.Is`

```go
// from linenoise-go/linenoise_engine_test.go

t.Run("ctrl-d on empty buffer is EOF", func(t *testing.T) {
    s := newSinkState(t)
    cont, _, err := s.processChar('\x04')
    if cont || !errors.Is(err, io.EOF) {
        t.Fatalf("got cont=%v err=%v", cont, err)
    }
})
```

Always compare errors with `errors.Is`, not `==`. Wrapped errors (e.g.
`fmt.Errorf("read: %w", io.EOF)`) compare equal with `errors.Is` but not with `==`.

---

## File I/O in tests — `t.TempDir` in practice

`TestHistoryPersistence` writes a history file and reloads it, all inside a temp
directory:

```go
// from linenoise-go/linenoise_engine_test.go

func TestHistoryPersistence(t *testing.T) {
    path := filepath.Join(t.TempDir(), "history")

    s := New(DefaultConfig())
    s.AddHistory("alpha")
    s.AddHistory("beta")
    s.AddHistory("beta") // duplicate of last, ignored
    if err := s.SaveHistory(path); err != nil {
        t.Fatalf("SaveHistory: %v", err)
    }

    loaded := New(DefaultConfig())
    if err := loaded.LoadHistory(path); err != nil {
        t.Fatalf("LoadHistory: %v", err)
    }
    if got := strings.Join(loaded.history, ","); got != "alpha,beta" {
        t.Fatalf("loaded history = %q, want %q", got, "alpha,beta")
    }

    // Loading a missing file is not an error.
    if err := loaded.LoadHistory(filepath.Join(t.TempDir(), "absent")); err != nil {
        t.Fatalf("LoadHistory(missing) = %v, want nil", err)
    }
}
```

The test also exercises the "missing file is a no-op" case — a real edge case that a
user hitting the feature for the first time will always trigger.

---

## Running with the race detector

`go test -race` recompiles the package with race instrumentation and runs every test.
Any concurrent access to shared memory without proper synchronisation is reported
immediately with a full goroutine stack trace.

The Makefile has a dedicated target (from `Makefile`):

```makefile
# from Makefile
test-race:
    @for dir in $(MODULES); do \
        echo "Testing $$dir (with -race)..."; \
        (cd $$dir && go test -race -v ./...) || exit 1; \
    done
```

The pre-commit target also runs it:

```makefile
pre-commit: fmt vet test-race
```

And the CI simulation target chains lint, race, coverage, and security:

```makefile
ci: lint test-race test-coverage security
```

!!! warning "Race detection has a cost"
    `-race` adds roughly 5–10× overhead to execution time and 2–5× to memory.
    Run it in CI on every push, but skip it for interactive `go test` during development
    if the suite is slow. Never skip it before merging.

The linenoise global state bug ([Lesson 14](14-the-deadlock-bug.md)) was exactly the
kind of problem the race detector catches: two goroutines calling `AddHistory` and
`LoadHistory` on a shared slice with no lock. `go test -race` fails immediately on that
pattern.

---

## Coverage

```makefile
# from Makefile
test-coverage:
    @for dir in $(MODULES); do \
        (cd $$dir && go test -coverprofile=coverage.out -covermode=atomic ./...) || exit 1; \
        (cd $$dir && go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'); \
    done
```

`-covermode=atomic` is required alongside `-race` because it uses atomic operations
to count coverage hits safely across goroutines. `-covermode=count` would race.

The CI matrix enforces a 70% gate: if total coverage falls below 70%, the build fails.
That number is not a target to hit and forget — it is a floor that prevents silent
regression when someone deletes tests.

!!! tip "Read the HTML report"
    ```bash
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out
    ```
    The HTML report colours every source line green (covered) or red (not covered).
    Red clusters in error-handling paths are the most common gap — they show you exactly
    where to add tests next.

---

## The ANSI escape test — a nice table trick

`TestReadCharEscapeSequences` uses the table to cover many different byte sequences
in one pass, and sanitises the test name so control bytes do not corrupt terminal output:

```go
// from linenoise-go/linenoise_engine_test.go

tests := []struct {
    in   string
    want rune
}{
    {"a", 'a'},
    {"\x1b[A", keyUp},
    {"\x1b[B", keyDown},
    // ...
    {"\x1bZ", '\x1b'}, // unknown sequence falls back to ESC
    {"é", 'é'},        // multi-byte UTF-8
}
for _, tt := range tests {
    t.Run(strings.ReplaceAll(tt.in, "\x1b", "ESC"), func(t *testing.T) {
        // ...
    })
}
```

`strings.ReplaceAll(tt.in, "\x1b", "ESC")` turns the escape byte into a printable
label. Without that, `go test -v` output would contain raw `ESC` bytes that confuse
terminal emulators.

---

!!! note "Try it"
    From the repository root, predict what you will see, then run:

    ```bash
    cd linenoise-go && go test -v -race -run TestHandleEditKeys ./...
    ```

    **Expected outcome:**

    ```
    === RUN   TestHandleEditKeys
    === RUN   TestHandleEditKeys/insert_printable
    --- PASS: TestHandleEditKeys/insert_printable (0.00s)
    === RUN   TestHandleEditKeys/backspace
    --- PASS: TestHandleEditKeys/backspace (0.00s)
    ... (nine subtests total) ...
    --- PASS: TestHandleEditKeys (0.00s)
    PASS
    ok      linenoise   0.XXXs
    ```

    Nine lines of `PASS`, no `RACE` lines. The `-race` flag is silent when there is
    nothing to report.

    Now run coverage for just this package:

    ```bash
    go test -coverprofile=coverage.out -covermode=atomic ./... && \
    go tool cover -func=coverage.out | grep total
    ```

    You should see something like `total: (statements) 78.X%`.

---

## Key takeaways

- Name every test function `TestXxx` and place it in a `_test.go` file. The test
  runner finds them automatically — no registration needed.
- Use `t.Run("scenario name", ...)` and a table of structs for any test that varies only
  by input/output. It scales without repetition and gives each case an addressable name.
- Mark helper functions with `t.Helper()` so failure messages point to the caller, not
  the helper's internals. Use `t.TempDir()` for any file I/O — it cleans up automatically.
- Run `go test -race` in CI on every push. It is the only reliable way to catch data
  races before they bite users. The linenoise global-state bug and the parallel-parser
  deadlock were both classes of bugs the race detector surfaces.
- Track coverage with `-coverprofile` and set a floor (70% in this repo). The HTML
  report shows exactly which lines lack tests; error-handling paths are usually first.
