# 22 - Fuzzing: how go test -fuzz found real bugs

> **Objectives:** Understand what Go's built-in fuzzer does and how to write a
> `FuzzXxx` function. See exactly how fuzzing the `dr-wav-go` parser discovered
> a divide-by-zero and an out-of-memory crash that human-written tests missed.
> Learn why regression seeds under `testdata/fuzz/` matter and how to replay
> them.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

- **Fuzzing is a robot test-case writer.** You give it a valid example and a
  function to call. It then generates millions of random mutations — flipping
  bytes, swapping numbers, inserting NULs — and calls your function with each
  one, watching for panics or crashes.
- **Seeds are your starting examples.** Without seeds, the fuzzer generates
  random bytes that rarely look like WAV files. With a real WAV seed, it
  quickly finds the interesting corner cases that live just one or two
  mutations away from valid input.
- **Crash files are permanent regression tests.** When the fuzzer finds a bad
  input, it saves the bytes to `testdata/fuzz/<FuzzName>/<hash>`. After that,
  `go test` (no `-fuzz` flag) always replays that input, so the bug can never
  come back silently.
- **Fuzz parsers AND accessors.** A parser that returns an error for malformed
  input is fine — but what about the functions that consume its output?
  A successfully-parsed but structurally weird file can still cause a
  divide-by-zero inside an accessor like `GetSampleCount`.
- **Fuzzing is not a replacement for unit tests.** It finds crashes and panics
  fast; it does not verify correct behavior. Unit tests cover the "should
  work" cases; fuzzing covers "must not crash."

**Why it matters:** the two real bugs fuzzing found in this repo — an OOM and
a divide-by-zero — would never have been written into a hand-crafted test
suite, because no developer thinks to produce a WAV header that claims 4 GB of
audio data backed by 4 bytes of actual bytes.

---

## How Go fuzzing works

Go 1.18 added native fuzzing. The rules are simple:

- The function must be named `FuzzXxx` and live in a `_test.go` file.
- Its first argument is `*testing.F`, not `*testing.T`.
- You call `f.Add(...)` to register seed values (one call per seed).
- You call `f.Fuzz(func(t *testing.T, data []byte) { ... })` with your test
  body.

During `go test` (no flags) the engine replays every file in
`testdata/fuzz/FuzzXxx/` as a unit test. During `go test -fuzz=FuzzXxx` it
runs indefinitely, mutating inputs and looking for panics.

```bash
# Run the fuzzer for 30 seconds against FuzzParse:
go test -fuzz=FuzzParse -fuzztime=30s ./dr-wav-go/

# Replay only the known crash seeds (runs in CI like a normal test):
go test ./dr-wav-go/
```

---

## The FuzzParse function — reading the real code

Here is the complete fuzz test from `dr-wav-go/dr_wav_fuzz_test.go`:

```go
// FuzzParse ensures Parse and every accessor survive arbitrary input without
// panicking (e.g. divide-by-zero on a zero NumChannels / bit depth).
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
    zeroCh, _ := Serialize(&WAV{
        Header: WAVHeader{AudioFormat: 1, BitsPerSample: 16},
        Data:   []byte{1, 2, 3, 4},
    })
    f.Add(zeroCh)

    f.Fuzz(func(_ *testing.T, data []byte) {
        wav, err := Parse(data)
        if err != nil || wav == nil {
            return
        }
        // None of these may panic on a successfully-parsed (but possibly
        // malformed) WAV.
        _ = ValidateWAV(wav)
        _ = wav.GetDuration()
        _ = wav.GetSampleCount()
        _, _ = wav.ExtractChannels()
        _, _ = Serialize(wav)
    })
}
```

Three things worth noticing:

**1. Early-exit on parse failure.** The body returns immediately if `Parse`
returns an error. The fuzzer is allowed to generate garbage — the important
invariant is not "all bytes must parse" but "if parsing succeeds, no accessor
may panic."

**2. Seeds cover interesting edge cases.** The last `f.Add` builds a WAV whose
`NumChannels` is 0 (the default zero value). A hand-written test suite probably
never constructs that, but it is a one-mutation step away from valid input.

**3. Every accessor is exercised.** Not just `Parse` — also `ValidateWAV`,
`GetDuration`, `GetSampleCount`, `ExtractChannels`, and `Serialize`. Any of
those could have a hidden divide-by-zero or nil-deref on edge-case headers.

---

## Bug 1 — the OOM crash

The WAV format stores the size of each data chunk as a `uint32` field inside
the file. Before the fix, the code did:

```go
// BEFORE (unsafe — trusts untrusted input)
pcmData := make([]byte, subchunkSize)  // subchunkSize comes from the file
```

The fuzzer produced a header where `subchunkSize` was 4,294,967,295
(`0xFFFFFFFF`, the maximum `uint32`) while only 4 bytes of actual data followed
it. The `make` call tried to allocate 4 GB on the heap and the process was
killed by the OOM killer.

The fix is in [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md), inside `readDataChunk`:

```go
// AFTER — cap to bytes actually present (from dr-wav-go/dr_wav.go)
allocSize := int(subchunkSize)
if allocSize > r.Len() {
    allocSize = r.Len() // never trust the declared size past EOF
}
pcmData := make([]byte, allocSize)
```

`r.Len()` returns how many bytes are left in the `bytes.Reader`. By capping the
allocation to what is actually present, the function can never allocate more
than the input itself, no matter what the header claims.

!!! note "The key lesson"
    A binary parser must **never** use an untrusted length field directly as an
    allocation size. Always cap it against the bytes available in the reader.
    See [Lesson 17](17-unbounded-allocation-oom.md) for the full treatment.

---

## Bug 2 — the divide-by-zero crash

After parsing succeeds, the caller can ask for the sample count:

```go
// from dr-wav-go/dr_wav.go
func (w *WAV) GetSampleCount() int {
    bytesPerSample := int(w.Header.BitsPerSample) / 8
    if bytesPerSample == 0 || w.Header.NumChannels == 0 {
        return 0
    }
    return len(w.Data) / bytesPerSample / int(w.Header.NumChannels)
}
```

The current code guards `NumChannels == 0` and `BitsPerSample == 0` with an
early return. Before this guard existed, `GetSampleCount` divided by
`w.Header.NumChannels` unconditionally, so a fuzzer-generated WAV with
`NumChannels = 0` caused an integer divide-by-zero panic (not an OOM — the
runtime kills the process with `panic: runtime error: integer divide by zero`).

The seed the fuzzer used to find this is preserved verbatim in
`testdata/fuzz/FuzzParse/be78194ecdd4d533`:

```
go test fuzz v1
[]byte("RIFF000\x00WAVEfmt \x10\xde\x16\xc4\xd6\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x10\x00data")
```

The byte sequence is a mostly-valid RIFF/WAV header but with `NumChannels = 0`
and `BitsPerSample = 0` in the fmt chunk. `Parse` accepts it (it does not
validate those fields); `GetSampleCount` then divided by zero.

!!! warning "Accessors can panic even on "successful" parses"
    `Parse` returning `nil` error does not mean the parsed struct is safe to
    use. Fields derived from untrusted binary data may be zero, negative, or
    otherwise surprising. Every accessor that does arithmetic on header fields
    must guard those fields explicitly.

---

## The corpus and regression seeds

Once the fuzzer finds a crash it writes the offending bytes to a file under
`testdata/fuzz/<FuzzFunctionName>/`. In this repo:

```
dr-wav-go/
  testdata/
    fuzz/
      FuzzParse/
        be78194ecdd4d533   ← the zero-channels crash seed
```

From that point on, running `go test ./dr-wav-go/` (no `-fuzz` flag, as in
normal CI) automatically replays every file in that directory. If the bug
comes back — for example, someone removes the `NumChannels == 0` guard — the
test fails immediately, on every run.

This is why fuzz-found seeds belong in the repository alongside the fix, not in
a personal scratch directory.

!!! note "Try it"
    From the repository root, replay the known crash seed as a unit test:

    ```bash
    go test ./dr-wav-go/ -v -run FuzzParse
    ```

    Expected output: the test passes, printing something like
    `--- PASS: FuzzParse (0.00s)`. The seed that once crashed the process now
    runs cleanly and confirms the fix is in place.

    To run the fuzzer live (stop it with Ctrl-C when you have seen enough):

    ```bash
    go test -fuzz=FuzzParse -fuzztime=20s ./dr-wav-go/
    ```

    Expected output: a line like `fuzz: elapsed: 20s, execs: 1234567, ...`
    followed by `ok` — no new crashes found after the fixes landed.

---

## Fuzz parsers AND accessors — the pattern

The structure of `FuzzParse` is the right model for any binary parser:

```
1. Build one or more valid seeds with f.Add.
2. Include seeds that target known edge cases (zero fields, minimum sizes).
3. In f.Fuzz: call Parse; if it returns an error, return early.
4. If Parse succeeds, call EVERY downstream accessor on the result.
```

Step 4 is the part that catches the divide-by-zero class of bugs. The fuzzer
found `GetSampleCount` crashing not by breaking the parser, but by feeding a
header that the parser accepted and that exposed a missing guard further down
the call chain.

The same pattern applies across every module in this workspace. A good fuzz
corpus seeds from a real (valid) file, then exercises every public function
that consumes the parsed result.

---

## What the audit confirmed about fuzzing

The 10-agent security audit (`docs/audits/2026-06-23-code-review-security-audit.md`)
noted that the recently-landed anti-OOM and divide-by-zero fixes in `dr-wav-go`
were "present and correct." It also confirmed the broader finding: **no
memory-corruption, slice-out-of-range, nil-deref, or integer-overflow bug was
found in any module** after the fuzz-driven fixes landed.

The audit did flag that `stb-image-go`'s `LoadStream` had no pixel-count guard
(M6) — a gap that fuzzing would likely have found too, had a `FuzzLoadStream`
function existed. Writing fuzz targets for every public parser entry point, not
just the primary one, is the lesson.

!!! tip "CI fuzzing on a schedule"
    The repository runs a weekly scheduled fuzzing job in
    [`.github/workflows/go-ci.yaml`](src/github-workflows-go-ci-yaml.md). Even after all known seeds pass, the
    fuzzer keeps exploring. A regression that slips through code review may
    still be caught by the next scheduled run before it reaches a release.

---

## Key takeaways

- **`func FuzzXxx(f *testing.F)`** is all you need to start. Seed with
  `f.Add`, exercise with `f.Fuzz`. The fuzzer handles the rest.
- **Seeds matter.** A zero-channel WAV is one field-flip away from a valid
  file. Including that seed directly pointed the fuzzer at the divide-by-zero
  in seconds.
- **Cap allocations to bytes present.** `make([]byte, untrustedSize)` is an
  OOM waiting to happen. `make([]byte, min(untrustedSize, reader.Len()))` is
  safe. The `readDataChunk` fix in `dr-wav-go/dr_wav.go` is the canonical
  example.
- **Fuzz accessors, not just parsers.** A successfully-parsed struct can still
  kill your process if an accessor divides by a zero field. `FuzzParse`
  exercises `GetSampleCount`, `GetDuration`, `ExtractChannels`, and `Serialize`
  in the same fuzz body.
- **Crash files under `testdata/fuzz/` are permanent CI tests.** They replay
  automatically on every `go test` run. A bug found by the fuzzer is a bug
  that can never silently return.
