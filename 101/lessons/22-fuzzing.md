# 22 - Fuzzing: how go test -fuzz found real bugs

> **Objectives:** Understand what Go's built-in fuzzer does and how to write a
> `FuzzXxx` function. See exactly how fuzzing the `dr-wav-go` parser discovered
> a divide-by-zero and an out-of-memory crash that human-written tests missed.
> Learn why regression seeds under `testdata/fuzz/` matter and how to replay
> them.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Fuzzing** = "a monkey with infinite patience, randomly smashing keys on your keyboard to see if your program breaks." The fuzzer mutates bytes — flipping bits, swapping numbers, inserting NULs — and calls `FuzzParse` millions of times, watching for any panic or crash.
- **Seeds** = "the first picture in a game of telephone." You hand the fuzzer one real, valid WAV file; it generates every plausible distortion of that file, finding corner cases that live just one field-flip away from legitimate input.
- **Crash files under `testdata/fuzz/`** = "a mugshot kept on file forever." Once the fuzzer captures a bad input in `testdata/fuzz/FuzzParse/be78194ecdd4d533`, `go test` replays it on every CI run — the bug that once crashed the process is now a permanent regression guard.
- **Fuzzing parsers AND accessors** = "checking that both the front door lock and the safe inside are secure." `Parse` accepting a malformed WAV without error is not enough; `GetSampleCount` and `ExtractChannels` can still divide by zero on the successfully-parsed but structurally weird struct, so `FuzzParse` calls all of them.
- **OOM from an untrusted length field** = "trusting a stranger's claim that they packed a whale into a carry-on bag." The pre-fix code did `make([]byte, subchunkSize)` directly from the WAV header — a header claiming 4 GB triggered a 4 GB allocation. Capping to `r.Len()` means you only allocate as much as is actually present.
- **Fuzzing vs. unit tests** = "a crash-test dummy vs. a checklist." The fuzzer discovers inputs that destroy the car; unit tests verify the seatbelt works as designed. Both are needed — fuzzing covers "must not crash," unit tests cover "must behave correctly."

**Why it matters:** the two real bugs fuzzing found in this repo — an OOM and
a divide-by-zero — would never have been written into a hand-crafted test
suite, because no developer thinks to produce a WAV header that claims 4 GB of
audio data backed by 4 bytes of actual bytes.

**See it — fuzzing lifecycle: seed → mutate → crash → regression seed.**

<svg viewBox="0 0 700 260" role="img" aria-labelledby="t22 d22" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t22">Fuzzing lifecycle diagram</title>
  <desc id="d22">Five stages: a valid WAV seed enters the fuzzer engine, which mutates bytes and calls FuzzParse. If Parse returns an error the fuzzer loops back. If Parse succeeds all accessors are called. A panic or crash saves a crash file to testdata/fuzz/ which then replays as a permanent CI regression test.</desc>
  <defs>
    <marker id="l22-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-default-fg-color,#333)"/>
    </marker>
    <marker id="l22-arrow-acc" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
    <marker id="l22-arrow-red" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="#e5484d"/>
    </marker>
  </defs>

  <!-- Box 1: Valid WAV seed -->
  <rect x="20" y="100" width="110" height="52" rx="7" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="75" y="121" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">Valid WAV seed</text>
  <text x="75" y="139" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--lighter,#888)">f.Add(valid)</text>

  <!-- Arrow 1→2 -->
  <line x1="131" y1="126" x2="168" y2="126" stroke="var(--md-default-fg-color,#333)" stroke-width="1.5" marker-end="url(#l22-arrow)"/>

  <!-- Box 2: Fuzzer engine -->
  <rect x="170" y="100" width="120" height="52" rx="7" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="230" y="121" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">Fuzzer engine</text>
  <text x="230" y="139" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--lighter,#888)">mutates bytes</text>

  <!-- Arrow 2→3 -->
  <line x1="291" y1="126" x2="328" y2="126" stroke="var(--md-default-fg-color,#333)" stroke-width="1.5" marker-end="url(#l22-arrow)"/>

  <!-- Box 3: FuzzParse / Parse -->
  <rect x="330" y="100" width="120" height="52" rx="7" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="390" y="121" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">FuzzParse</text>
  <text x="390" y="139" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--lighter,#888)">Parse(data)</text>

  <!-- "err != nil → loop back" label -->
  <path d="M390,100 Q390,55 230,55 Q170,55 170,99" fill="none" stroke="var(--md-default-fg-color--lighter,#888)" stroke-width="1.2" stroke-dasharray="4,3" marker-end="url(#l22-arrow)"/>
  <text x="305" y="46" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--lighter,#888)">err != nil → loop</text>

  <!-- Arrow 3→4 (success path) -->
  <line x1="451" y1="126" x2="488" y2="126" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5" marker-end="url(#l22-arrow-acc)"/>
  <text x="469" y="119" text-anchor="middle" font-size="9" fill="var(--md-accent-fg-color,#00897b)">ok</text>

  <!-- Box 4: Accessors -->
  <rect x="490" y="100" width="120" height="52" rx="7" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="550" y="118" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">Accessors</text>
  <text x="550" y="133" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--lighter,#888)">GetSampleCount</text>
  <text x="550" y="146" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--lighter,#888)">ExtractChannels…</text>

  <!-- Arrow 4→5 (panic path) -->
  <line x1="550" y1="153" x2="550" y2="188" stroke="#e5484d" stroke-width="1.5" marker-end="url(#l22-arrow-red)"/>
  <text x="560" y="175" font-size="9" fill="#e5484d">panic!</text>

  <!-- Box 5: crash file saved -->
  <rect x="430" y="190" width="180" height="52" rx="7" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="520" y="210" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">Crash file saved</text>
  <text x="520" y="228" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--lighter,#888)">testdata/fuzz/FuzzParse/&lt;hash&gt;</text>

  <!-- Arrow 5→CI label -->
  <line x1="429" y1="216" x2="310" y2="216" stroke="var(--md-default-fg-color,#333)" stroke-width="1.5" marker-end="url(#l22-arrow)"/>

  <!-- Box 6: CI regression -->
  <rect x="140" y="190" width="168" height="52" rx="7" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="224" y="210" text-anchor="middle" font-size="11" fill="currentColor" font-weight="600">CI regression test</text>
  <text x="224" y="228" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--lighter,#888)">go test replays forever</text>
</svg>

---

## How Go fuzzing works

Go 1.18 added native fuzzing. The rules are simple:

- The function must be named `FuzzXxx` and live in a `_test.go` file (in Go, any file whose name ends in `_test.go` is a special file that holds tests instead of program logic — the compiler, the tool that turns human-written source text into a runnable program, treats it differently from regular code).
- Its first argument is `*testing.F`, not `*testing.T`. (A function's "arguments" are the values you hand it to work with when you run it. `*testing.F` and `*testing.T` are two different kinds of helper objects Go's testing tools pass in — `F` is for fuzz tests, `T` is for ordinary ones.)
- You call `f.Add(...)` to register seed values (one call per seed). ("Calling" a function just means running it, right here, with specific values plugged in.)
- You call `f.Fuzz(func(t *testing.T, data []byte) { ... })` with your test
  body. (`func(...) { ... }` is Go's way of writing a small nameless function on the spot, right where it's needed, instead of defining it elsewhere and giving it a name. `[]byte` means "a stretchy list of bytes" — a byte is one small chunk of computer memory, usually enough to hold one letter or a small number.)

During `go test` (no flags) the engine replays every file in
`testdata/fuzz/FuzzXxx/` as a unit test (a unit test is a small, automated check that runs a specific piece of code and confirms it behaves as expected — it's the programmer's version of double-checking your work). During `go test -fuzz=FuzzXxx` it
runs indefinitely, mutating inputs and looking for panics (a "panic" is Go's term for the program hitting a fatal error it can't recover from at that moment — the running program stops what it's doing and unwinds, often crashing unless something is watching for it).

```bash
# Run the fuzzer for 30 seconds against FuzzParse:
go test -fuzz=FuzzParse -fuzztime=30s ./dr-wav-go/

# Replay only the known crash seeds (runs in CI like a normal test):
go test ./dr-wav-go/
```

**In plain terms:** the first command tells Go to hammer the `FuzzParse` function with randomly-mutated input for 30 seconds, hunting for crashes. The second command just replays the inputs that have already been saved as known trouble-makers, as a quick sanity check.

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

**In plain terms:** this code hands the fuzzer five starting examples (one valid WAV file, an empty one, a truncated one, a blank 44-byte one, and one with zero audio channels), then tells it: for every random variation you try, parse the bytes, and if that succeeds, run every other function that reads the result — none of them should ever crash.

Three things worth noticing:

**1. Early-exit on parse failure.** The body returns immediately if `Parse`
returns an error (in plain terms: as soon as the function reaches this `return`, it stops running right there and hands control back to whoever called it — nothing after it in the function runs for this attempt). The fuzzer is allowed to generate garbage — the important
invariant is not "all bytes must parse" but "if parsing succeeds, no accessor
may panic." (An "accessor" is just a small function whose only job is to read and hand back one piece of information from a larger bundle of data — here, things like "how many samples does this WAV have?")

**2. Seeds cover interesting edge cases.** The last `f.Add` builds a WAV whose
`NumChannels` is 0 (the default zero value — in Go, a variable that's never explicitly set starts out at a predictable "empty" value, `0` for numbers). A hand-written test suite probably
never constructs that, but it is a one-mutation step away from valid input.

**3. Every accessor is exercised.** Not just `Parse` — also `ValidateWAV`,
`GetDuration`, `GetSampleCount`, `ExtractChannels`, and `Serialize`. Any of
those could have a hidden divide-by-zero or nil-deref on edge-case headers. ("Nil" is Go's word for "no value here" — a nil-deref is trying to read details out of something that turned out to be empty, which crashes the program.)

---

## Bug 1 — the OOM crash

The WAV format stores the size of each data chunk as a `uint32` field inside
the file (a `uint32` is a number stored in a fixed-size 4-byte slot that can only hold non-negative whole numbers — it cannot represent anything larger than about 4.3 billion, and it cannot go negative). Before the fix, the code did:

```go
// BEFORE (unsafe — trusts untrusted input)
pcmData := make([]byte, subchunkSize)  // subchunkSize comes from the file
```

**In plain terms:** `make([]byte, subchunkSize)` tells the program "reserve a chunk of memory big enough to hold `subchunkSize` bytes" (this reserving is called allocating memory) — and it does this using a number read straight from the file, with no sanity check on how large that number is.

The fuzzer produced a header where `subchunkSize` was 4,294,967,295
(`0xFFFFFFFF`, the maximum `uint32`) while only 4 bytes of actual data followed
it. The `make` call tried to allocate 4 GB on the heap (the "heap" is the large pool of memory a running program can request chunks from) and the process was
killed by the OOM killer (OOM = "out of memory" — when a program tries to grab more memory than the computer can give it, the operating system forcibly kills it rather than let it keep trying).

The fix is in [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md), inside `readDataChunk`:

```go
// AFTER — cap to bytes actually present (from dr-wav-go/dr_wav.go)
allocSize := int(subchunkSize)
if allocSize > r.Len() {
    allocSize = r.Len() // never trust the declared size past EOF
}
pcmData := make([]byte, allocSize)
```

**In plain terms:** before reserving memory, the fixed code first checks how many bytes are actually left to read, and if the file's claimed size is bigger than that, it uses the smaller, real number instead — so a lying header can no longer trigger a multi-gigabyte reservation.

`r.Len()` returns how many bytes are left in the `bytes.Reader` (a "Reader" here is a small helper object that hands out bytes from the file one piece at a time and keeps track of how much is left — the function "returns" a value, meaning it finishes its work and hands that value back to whoever asked). By capping the
allocation to what is actually present, the function can never allocate more
than the input itself, no matter what the header claims.

!!! note "The key lesson"
    A binary parser must **never** use an untrusted length field directly as an
    allocation size. Always cap it against the bytes available in the reader.
    See [Lesson 17](17-unbounded-allocation-oom.md) for the full treatment.

---

## Bug 2 — the divide-by-zero crash

After parsing succeeds, the caller can ask for the sample count (the "caller" is whatever code runs — calls — this function; here it means the fuzz test itself, invoking `GetSampleCount` on the parsed result):

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

**In plain terms:** this function calculates how many audio samples are in the file by dividing the total data length by the size of one sample. Before it does that division, it checks whether either number it's about to divide by is zero — dividing by zero is not just "wrong," it makes the program crash outright, so the check simply hands back `0` instead of ever attempting it.

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
    use. (A "struct" is Go's way of bundling several related pieces of data — here, all the WAV header details — into one named package; each named piece inside it, like `NumChannels`, is called a "field.") Fields derived from untrusted binary data may be zero, negative, or
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
normal CI — "CI" stands for Continuous Integration, the automated system that runs a project's tests every time someone proposes a change, before that change is allowed in) automatically replays every file in that directory. If the bug
comes back — for example, someone removes the `NumChannels == 0` guard (a "guard" is just a check, like the `if` condition above, placed before risky code to stop it from running when conditions are unsafe) — the
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

The same pattern applies across every module in this workspace (a "module" here is one self-contained chunk of the codebase — like `dr-wav-go`, the WAV-file part — that can be built and tested on its own). A good fuzz
corpus (the "corpus" is simply the whole collection of seed and crash files a fuzzer has accumulated for a given function) seeds from a real (valid) file, then exercises every public function
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
