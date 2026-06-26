# Lab D · Fuzz a Parser, Commit the Crash Seed

> **Objectives:** Write a `FuzzXxx` function for the dr-wav-go parser, run
> `go test -fuzz` to let the engine mutate inputs automatically, and commit any
> crash-producing seed so the regression is permanent. You will also learn how
> the existing `FuzzParse` in this repo was written and why its seed corpus is
> designed the way it is.
> Estimated time: 30 minutes.

!!! note "This is a hands-on lab"
    The lesson teaches by doing. Read each section, run the command, observe the
    output, then move to the next step. The expected output for each command is
    shown directly below it.

---

## What this actually means (plain English)

- **Fuzzing** is like hiring a robot to mash a parser with random inputs at
  machine speed — thousands of attempts per second — looking for panics,
  crashes, or hangs that a human-written test would never stumble into.
- **A seed corpus** is a bag of starting examples you hand the fuzzer. It
  mutates them (flip a bit, insert a byte, truncate) rather than starting from
  random noise, so it reaches interesting corners faster.
- **A crash seed** is the exact byte string that caused a panic. The fuzzer
  writes it into `testdata/fuzz/<FuzzName>/` automatically.
- **Committing the seed** turns a one-time discovery into a permanent
  regression test. Every future `go test` run (no `-fuzz` flag) replays the
  seed and the test fails if the bug comes back.
- **Corpus-driven coverage** means the fuzzer tracks which branches each input
  exercises and keeps inputs that open new branches, gradually mapping the
  parser's full state space.

**Why it matters:** the two real OOM bugs found in dr-wav-go were not caught by
any hand-written test — the fuzzer found them in minutes.

---

## The real `FuzzParse` — what it does and why

Open `dr-wav-go/dr_wav_fuzz_test.go`. Here is the complete function:

```go
// from dr-wav-go/dr_wav_fuzz_test.go
func FuzzParse(f *testing.F) {
    valid, _ := Serialize(&WAV{
        Header: WAVHeader{
            AudioFormat: 1, NumChannels: 2, SampleRate: 44100,
            ByteRate: 176400, BlockAlign: 4, BitsPerSample: 16,
        },
        Data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
    })
    f.Add(valid)
    f.Add([]byte{})
    f.Add([]byte("RIFF"))
    f.Add(make([]byte, 44))
    // A header that parses but has NumChannels = 0 (the divide-by-zero case).
    zeroCh, _ := Serialize(&WAV{Header: WAVHeader{AudioFormat: 1, BitsPerSample: 16}, Data: []byte{1, 2, 3, 4}})
    f.Add(zeroCh)

    f.Fuzz(func(_ *testing.T, data []byte) {
        wav, err := Parse(data)
        if err != nil || wav == nil {
            return
        }
        _ = ValidateWAV(wav)
        _ = wav.GetDuration()
        _ = wav.GetSampleCount()
        _, _ = wav.ExtractChannels()
        _, _ = Serialize(wav)
    })
}
```

### Why five seeds?

| Seed | Purpose |
|------|---------|
| `valid` | A well-formed WAV — the fuzzer mutates it to find edge cases close to real input. |
| `[]byte{}` | Empty input — tests the length guard (`< 44` check) immediately. |
| `[]byte("RIFF")` | Four-byte truncation — exercises the early-read failure paths. |
| `make([]byte, 44)` | Minimum-length all-zeros — forces the RIFF/WAVE/fmt magic-byte checks. |
| `zeroCh` | Valid structure, zero `NumChannels` — targets the divide-by-zero in `GetSampleCount`. |

Seeding is cheap and high-value: a seed that exercises a new branch saves the
fuzzer from having to discover that branch through random mutation.

### What the fuzz body checks

The `f.Fuzz` callback calls **every public method** on a successfully-parsed
WAV. The invariant is simple: *if `Parse` returns nil error, nothing else may
panic*. The fuzzer reports a crash if any call panics, not just `Parse` itself.

This is the same pattern used across the codebase — call every accessor so a
latent divide-by-zero or nil-deref in a downstream function gets caught too.

---

## The bug the fuzzer found: OOM in `readDataChunk`

The fuzzer discovered that a WAV whose `data` subchunk header claims a huge
size (e.g. 4 GB) caused `Parse` to call `make([]byte, 4294967295)` and crash
the process with out-of-memory.

The fix in [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md) caps the allocation to the bytes actually
remaining in the reader:

```go
// from dr-wav-go/dr_wav.go — readDataChunk
if string(subchunkID[:]) == "data" {
    allocSize := int(subchunkSize)
    if allocSize > r.Len() {
        allocSize = r.Len() // never trust the declared size past EOF
    }
    pcmData := make([]byte, allocSize)
    ...
}
```

The principle: **never allocate based on an untrusted integer from the input**.
Always bound it against something you already know is safe — here, the bytes
physically present in the reader. See [Lesson 17](17-unbounded-allocation-oom.md)
for the threat model behind this pattern.

---

## The committed crash seed

After the fuzzer found the OOM, it wrote the crash-triggering input to:

```
dr-wav-go/testdata/fuzz/FuzzParse/be78194ecdd4d533
```

The file content is:

```
go test fuzz v1
[]byte("RIFF000\x00WAVEfmt \x10\xde\x16\xc4\xd6\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x10\x00data")
```

This is a minimal valid RIFF/WAVE header with a `data` subchunk whose declared
size field is crafted to be much larger than the remaining bytes. After the fix,
replaying this seed passes: `Parse` returns a result with the data capped to the
bytes actually present, rather than panicking.

The seed is committed alongside the fix in the same git commit — it is now a
permanent regression test that runs on every `go test ./...` with no special
flags.

---

## Step-by-step lab

### Step 1 — run the existing seed corpus (no fuzzing)

!!! note "Try it"
    ```bash
    cd dr-wav-go
    go test -run=FuzzParse -v ./...
    ```

    Expected output (the seeds replay as unit tests):

    ```
    --- PASS: FuzzParse (0.00s)
    PASS
    ok      drwavgo    0.012s
    ```

    When there is no `-fuzz` flag, `go test` runs `FuzzParse` once for each
    seed in `testdata/fuzz/FuzzParse/` plus the seeds registered with `f.Add`.
    This is how committed seeds become regression tests.

### Step 2 — run the fuzzer for a short burst

!!! note "Try it"
    ```bash
    go test -run='^$' -fuzz=FuzzParse -fuzztime=20s ./...
    ```

    Expected output (abridged):

    ```
    fuzz: elapsed: 0s, gathering baseline coverage: 0/6 completed
    fuzz: elapsed: 0s, gathering baseline coverage: 6/6 completed, now fuzzing with 8 workers
    fuzz: elapsed: 3s, execs: 87432 (29143/sec), new interesting: 12 (total: 18)
    fuzz: elapsed: 6s, execs: 182901 (31820/sec), new interesting: 14 (total: 20)
    ...
    fuzz: elapsed: 20s, execs: 610233 (30511/sec), new interesting: 19 (total: 25)
    PASS
    ```

    `new interesting` counts inputs that opened new coverage branches. The
    fuzzer keeps those in a temporary corpus (under `$GOCACHE/fuzz/`) but
    does not commit them unless they cause a crash.

### Step 3 — replay the committed crash seed explicitly

!!! note "Try it"
    ```bash
    go test -run='FuzzParse/be78194ecdd4d533' -v ./...
    ```

    Expected output (the patched code handles it cleanly):

    ```
    --- PASS: FuzzParse/be78194ecdd4d533 (0.00s)
    PASS
    ok      drwavgo    0.008s
    ```

    If you were to revert the `readDataChunk` fix and re-run, this test would
    crash with an OOM or panic — that is exactly the regression guarantee.

### Step 4 — write your own fuzz target (optional extension)

!!! tip "Extension exercise"
    Write a second fuzz target that focuses only on `Serialize` → `Parse`
    round-trips. The invariant: for any `WAV` that `Serialize` accepts, the
    output must re-parse to the same header and data.

    ```go
    // dr-wav-go/dr_wav_roundtrip_fuzz_test.go
    package drwavgo

    import "testing"

    func FuzzRoundTrip(f *testing.F) {
        f.Add(uint16(1), uint16(2), uint32(44100), uint16(16), []byte{1, 2, 3, 4})

        f.Fuzz(func(t *testing.T, fmt uint16, ch uint16, sr uint32, bps uint16, data []byte) {
            wav := &WAV{
                Header: WAVHeader{
                    AudioFormat:   fmt,
                    NumChannels:   ch,
                    SampleRate:    sr,
                    BitsPerSample: bps,
                },
                Data: data,
            }
            raw, err := Serialize(wav)
            if err != nil {
                return // Serialize may reject invalid fields; that's fine
            }
            got, err := Parse(raw)
            if err != nil {
                t.Fatalf("Parse failed after Serialize succeeded: %v", err)
            }
            if got.Header != wav.Header {
                t.Fatalf("header round-trip mismatch: got %+v want %+v", got.Header, wav.Header)
            }
        })
    }
    ```

    Run it:

    ```bash
    go test -run='^$' -fuzz=FuzzRoundTrip -fuzztime=30s ./...
    ```

    The fuzzer will mutate the individual fields rather than raw bytes, letting
    it explore the parameter space of `Serialize`'s input validation quickly.

### Step 5 — committing a new crash seed

If `go test -fuzz` finds a crash, it prints:

```
--- FAIL: FuzzParse (3.21s)
    Fuzzing found a crashing input; minimizing...
    ...
    Failing input written to testdata/fuzz/FuzzParse/a1b2c3d4e5f6
    To re-run: go test -run=FuzzParse/a1b2c3d4e5f6 ./...
FAIL
```

The workflow after that is:

1. Fix the bug in the production code.
2. Confirm the seed now passes: `go test -run=FuzzParse/a1b2c3d4e5f6 -v ./...`
3. Commit the seed file **in the same commit as the fix** — one SHA, one story.
4. Update `docs/BUG_JOURNAL.md` in that same commit (per global rule §1).

!!! warning "Never delete crash seeds"
    A crash seed is proof a bug existed and a regression guard against it
    returning. The file is tiny (a few hundred bytes). There is no good reason
    to remove it.

---

## How CI picks this up

In [`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md), the weekly `fuzz` job runs every module's
target through a build matrix (shown here with the `dr-wav-go` row expanded):

```yaml
# from .github/workflows/go-ci.yaml — fuzz job (matrix)
strategy:
  matrix:
    include:
      - module: dr-wav-go
        target: FuzzParse
      # … jsmn-go/FuzzParse, miniz-go/FuzzExtract, stb-truetype-go/FuzzLoadFont, …
steps:
  - name: Fuzz ${{ matrix.module }} (${{ matrix.target }})
    working-directory: ${{ matrix.module }}
    run: go test -run='^$' -fuzz="^${{ matrix.target }}$" -fuzztime=120s .

  - name: Upload crash corpus on failure
    if: failure()
    uses: actions/upload-artifact@v4
    with:
      name: fuzz-crash-${{ matrix.module }}
      path: ${{ matrix.module }}/testdata/fuzz/
```

On failure the corpus is uploaded as a CI artifact so a developer can download
it and reproduce the crash locally without needing to re-run the fuzzer.

The regular `test` job (which runs on every push and PR) uses:

```yaml
run: go test -v -race -timeout 5m ./...
```

No `-fuzz` flag — but this still replays every file in `testdata/fuzz/`. The
committed seed is therefore tested on every CI run, not just on Mondays.

---

## Key takeaways

- A `FuzzXxx` function has two parts: `f.Add(seed...)` to register starting
  examples, and `f.Fuzz(func(t, data))` to define the invariant. The invariant
  should be "nothing panics" for a parser — call every downstream accessor.
- Seeds should cover the full range of the input space cheaply: valid input,
  empty input, truncated input, and any known dangerous patterns (e.g. zero
  channels for a divide-by-zero).
- When the fuzzer finds a crash, it writes a minimal reproducer to
  `testdata/fuzz/<Name>/`. Commit it with the fix in the same SHA.
- Committed seeds replay as ordinary unit tests on every `go test ./...` run —
  no special flags needed. This is free, permanent regression coverage.
- The `readDataChunk` OOM fix — capping `allocSize` to `r.Len()` — is the
  canonical example of why fuzzing finds what hand-written tests miss: no
  human thinks to write `make([]byte, 0xFFFFFFFF)` in a test case.
