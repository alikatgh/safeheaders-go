# 21 · Table tests, -race and coverage

> **Objectives:** Understand idiomatic Go test structure — `func TestXxx`, table-driven
> subtests, and helper utilities like `t.Helper` and `t.TempDir`. Learn how to run the
> race detector and read coverage numbers. See every technique demonstrated in the real
> `linenoise-go` test file.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **`func TestXxx(t *testing.T)`** = "a door labelled 'Test' that Go automatically opens — no sign-up sheet required." Any exported function whose name starts with `Test` in a `_test.go` file is discovered and run by `go test` without any registration step.
- **Table-driven tests** = "a spreadsheet of scenarios fed through a single recipe." Instead of copy-pasting one assertion block per case, you declare inputs and expected outputs as a slice of structs and drive them all through one shared loop — adding a new case is one struct literal.
- **`t.Run("name", func(t *testing.T) {...})`** = "a labelled lane in a race, each judged independently." Each subtest gets its own pass/fail verdict and can be targeted directly from the command line with `-run TestFoo/name`.
- **`t.Helper()`** = "a stagehand who steps aside so the spotlight hits the actor, not the crew." It marks a function as infrastructure so that, when an assertion fails inside it, the error message points to the *caller's* line, not to the helper's internals.
- **`t.TempDir()`** = "a hotel room that housekeeping auto-cleans the moment you check out." It creates an OS temp directory that is removed automatically when the test ends — no manual cleanup, no files left behind on CI.
- **`go test -race`** = "a sensor that trips an alarm the instant two workers grab the same tool at once." It instruments every memory access and reports actual concurrent conflicts at runtime; it cannot prove races absent, but it catches every race that occurs during the test run.

**Why it matters:** a data race is undefined behaviour — it silently corrupts state in
production but only crashes your program occasionally. The race detector turns "sometimes
wrong" into "always caught in CI".

**See it — table-driven subtests flowing into the race detector.**

<svg viewBox="0 0 700 310" role="img" aria-labelledby="t21 d21" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t21">Table-driven subtests and the race detector</title>
  <desc id="d21">A block diagram showing a slice of test structs fanning out into parallel t.Run subtests, each instrumented by go test -race, producing a single pass/fail result.</desc>
  <defs>
    <marker id="l21-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-default-fg-color--light)"/>
    </marker>
  </defs>
  <!-- Table (slice of structs) -->
  <rect x="20" y="100" width="160" height="110" rx="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="100" y="122" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">[]struct{ … }</text>
  <rect x="32" y="132" width="136" height="20" rx="4" fill="var(--md-accent-fg-color,#00897b)" opacity="0.15" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1"/>
  <text x="100" y="146" text-anchor="middle" font-size="11" fill="currentColor">{"insert printable", 'X', …}</text>
  <rect x="32" y="157" width="136" height="20" rx="4" fill="var(--md-accent-fg-color,#00897b)" opacity="0.15" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1"/>
  <text x="100" y="171" text-anchor="middle" font-size="11" fill="currentColor">{"backspace", '\x7f', …}</text>
  <rect x="32" y="182" width="136" height="20" rx="4" fill="var(--md-accent-fg-color,#00897b)" opacity="0.15" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1"/>
  <text x="100" y="196" text-anchor="middle" font-size="11" fill="currentColor">{"ctrl-a home", '\x01', …}</text>
  <!-- Arrow: table -> loop -->
  <line x1="180" y1="155" x2="232" y2="155" stroke="var(--md-default-fg-color--light)" stroke-width="1.5" marker-end="url(#l21-arrow)"/>
  <!-- for loop box -->
  <rect x="234" y="120" width="120" height="70" rx="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="294" y="148" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">for _, tt :=</text>
  <text x="294" y="165" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">range tests</text>
  <text x="294" y="182" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">one loop, N cases</text>
  <!-- Arrow: loop -> t.Run -->
  <line x1="354" y1="155" x2="406" y2="155" stroke="var(--md-default-fg-color--light)" stroke-width="1.5" marker-end="url(#l21-arrow)"/>
  <!-- t.Run subtests -->
  <rect x="408" y="60" width="130" height="30" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="473" y="80" text-anchor="middle" font-size="11" fill="currentColor">t.Run("insert printable")</text>
  <rect x="408" y="103" width="130" height="30" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="473" y="123" text-anchor="middle" font-size="11" fill="currentColor">t.Run("backspace")</text>
  <rect x="408" y="146" width="130" height="30" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="473" y="166" text-anchor="middle" font-size="11" fill="currentColor">t.Run("ctrl-a home")</text>
  <rect x="408" y="189" width="130" height="30" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="4,3"/>
  <text x="473" y="209" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">… 6 more …</text>
  <!-- fan-out lines from loop to subtests -->
  <line x1="354" y1="155" x2="406" y2="75" stroke="var(--md-default-fg-color--light)" stroke-width="1" marker-end="url(#l21-arrow)"/>
  <line x1="354" y1="155" x2="406" y2="118" stroke="var(--md-default-fg-color--light)" stroke-width="1" marker-end="url(#l21-arrow)"/>
  <line x1="354" y1="155" x2="406" y2="161" stroke="var(--md-default-fg-color--light)" stroke-width="1" marker-end="url(#l21-arrow)"/>
  <line x1="354" y1="155" x2="406" y2="204" stroke="var(--md-default-fg-color--light)" stroke-width="1" marker-end="url(#l21-arrow)"/>
  <!-- race detector badge -->
  <rect x="408" y="240" width="130" height="34" rx="6" fill="#e5484d" opacity="0.12" stroke="#e5484d" stroke-width="1.5"/>
  <text x="473" y="258" text-anchor="middle" font-size="11" font-weight="600" fill="#e5484d">go test -race</text>
  <text x="473" y="270" text-anchor="middle" font-size="10" fill="#e5484d">instruments every access</text>
  <!-- label above table -->
  <text x="100" y="90" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">test table</text>
</svg>

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

The Makefile has a dedicated target (from [`Makefile`](src/makefile.md)):

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
