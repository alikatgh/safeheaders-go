# 17 · Unbounded allocation = OOM

> **Objectives:** Understand why trusting a size field from untrusted input can crash your
> process with an out-of-memory error, see how the dr-wav-go parser was fixed by capping
> allocation to bytes actually present, and learn how fuzzing surfaces this class of bug
> before attackers do.
> Estimated time: 20 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **WAV file header** = "a shipping label stapled to an empty box." The file contains a 32-bit integer near the top claiming how many bytes of audio payload follow — and the parser reads that number to decide how large a slice to allocate.
- **`subchunkSize` from untrusted input** = "a stranger handing you a tape measure that reads '4 kilometres' for a doorframe." A 44-byte file can legally declare `subchunkSize = 0xFFFFFFFF`; if the parser believes it, `make([]byte, 4294967295)` asks the OS for ~4 GB and the process is killed.
- **`r.Len()` cap** = "only pour as much as the jug actually holds, no matter what the label says." `r.Len()` returns the bytes physically remaining in the reader; the fix clamps `allocSize` to this real bound so no declared header value can exceed what is actually present.
- **`r.Seek(...)` instead of `make` for `subchunk1Size`** = "stepping over a puddle instead of mopping it up." Seeking past the extra format bytes moves the reader position without touching the heap, so the untrusted `uint32` in `subchunk1Size-16` never controls an allocation.
- **`maxWAVDataSize` serialization guard** = "refusing to fill a bottle that's too big for the cap." A `uint32` RIFF size field can only hold ~4 GB; payloads above `math.MaxUint32 - 44` would silently truncate on write, so `Serialize` returns an explicit error instead of writing a corrupt file.
- **Fuzz corpus seed** = "a tripwire permanently wired into the test suite." Once `go test -fuzz` crashed the parser with a giant declared size, that input was saved under `testdata/fuzz/`; `go test ./...` replays it on every CI run so the bug cannot silently return.

**Why it matters:** one crafted upload, one allocation, process dead — no exploit code
required.

**See it — declared size vs. actual bytes: safe vs. unsafe allocation.**

<svg viewBox="0 0 700 310" role="img" aria-labelledby="t17 d17" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t17">Unbounded vs. capped allocation from an untrusted WAV header size field</title>
  <desc id="d17">Two side-by-side flows. Left (unsafe): attacker file declares subchunkSize=4 GB, parser calls make with that value directly, OS kills the process with OOM. Right (safe): attacker file declares subchunkSize=4 GB, parser compares with r.Len() which is 44 bytes, allocSize is clamped to 44, make succeeds, parse continues.</desc>
  <defs>
    <marker id="l17-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 Z" fill="currentColor"/>
    </marker>
  </defs>
  <text x="175" y="28" text-anchor="middle" font-size="13" font-weight="700" fill="#e5484d">BEFORE (unsafe)</text>
  <text x="525" y="28" text-anchor="middle" font-size="13" font-weight="700" fill="var(--md-accent-fg-color,#00897b)">AFTER (safe)</text>
  <line x1="350" y1="10" x2="350" y2="300" stroke="var(--md-default-fg-color--lightest,#ccc)" stroke-width="1" stroke-dasharray="4,4"/>
  <rect x="60" y="44" width="230" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="175" y="61" text-anchor="middle" font-size="11" fill="currentColor">Attacker-crafted WAV file</text>
  <text x="175" y="77" text-anchor="middle" font-size="11" fill="currentColor">subchunkSize = 0xFFFFFFFF (~4 GB)</text>
  <line x1="175" y1="86" x2="175" y2="114" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="60" y="114" width="230" height="42" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="175" y="131" text-anchor="middle" font-size="11" fill="currentColor">Parser calls directly:</text>
  <text x="175" y="149" text-anchor="middle" font-size="11" font-style="italic" fill="currentColor">make([]byte, subchunkSize)</text>
  <line x1="175" y1="156" x2="175" y2="184" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="60" y="184" width="230" height="42" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="175" y="201" text-anchor="middle" font-size="11" fill="currentColor">OS asked for ~4 GB of RAM</text>
  <text x="175" y="219" text-anchor="middle" font-size="11" fill="currentColor">on a 44-byte file</text>
  <line x1="175" y1="226" x2="175" y2="254" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="60" y="254" width="230" height="38" rx="6" fill="#e5484d" stroke="#e5484d" stroke-width="1.5"/>
  <text x="175" y="270" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">OOM — process killed</text>
  <text x="175" y="285" text-anchor="middle" font-size="10" fill="#fff">denial of service, no exploit needed</text>
  <rect x="410" y="44" width="230" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light,#666)" stroke-width="1.5"/>
  <text x="525" y="61" text-anchor="middle" font-size="11" fill="currentColor">Attacker-crafted WAV file</text>
  <text x="525" y="77" text-anchor="middle" font-size="11" fill="currentColor">subchunkSize = 0xFFFFFFFF (~4 GB)</text>
  <line x1="525" y1="86" x2="525" y2="114" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="410" y="114" width="230" height="56" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="525" y="131" text-anchor="middle" font-size="11" fill="currentColor">Cap check:</text>
  <text x="525" y="147" text-anchor="middle" font-size="11" font-style="italic" fill="currentColor">if allocSize &gt; r.Len() {</text>
  <text x="525" y="162" text-anchor="middle" font-size="11" font-style="italic" fill="currentColor">  allocSize = r.Len() // 44 bytes</text>
  <line x1="525" y1="170" x2="525" y2="198" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="410" y="198" width="230" height="42" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="525" y="215" text-anchor="middle" font-size="11" fill="currentColor">make([]byte, 44)</text>
  <text x="525" y="233" text-anchor="middle" font-size="11" fill="currentColor">bounded by bytes physically present</text>
  <line x1="525" y1="240" x2="525" y2="258" stroke="currentColor" stroke-width="1.5" marker-end="url(#l17-arrow)"/>
  <rect x="410" y="258" width="230" height="34" rx="6" fill="var(--md-accent-fg-color,#00897b)" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="525" y="279" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">Parse continues safely</text></svg>

---

## The vulnerability: trusting the header

RIFF/WAV binary layout (simplified). "Binary" here just means the file is a raw sequence
of bytes (a byte is one small chunk of data, eight on/off switches worth, that a computer
reads and writes as a unit) meant for a program to interpret, not a text file meant for
a human to read directly:

```
"RIFF" [4-byte total size] "WAVE"
"fmt " [subchunk1Size]  <format fields>
"data" [subchunkSize]   <PCM bytes …>
```

**In plain terms:** this is the skeleton of a WAV audio file — a few short text labels
("RIFF", "fmt ", "data") each followed by a number that announces how many bytes of
content come next, followed by that content itself.

Both `subchunk1Size` and `subchunkSize` are `uint32` values (in plain terms: an unsigned
32-bit whole number — "unsigned" meaning it can never be negative, and 32-bit meaning it's
built from 32 of those on/off switches, so its largest possible value is about 4.29
billion) decoded directly from the file. Before the fix, the data-chunk reader looked
roughly like this (a "reader" here is just the piece of code responsible for pulling
bytes out of the file one step at a time):

```go
// BEFORE — do NOT copy this pattern
var subchunkSize uint32
binary.Read(r, binary.LittleEndian, &subchunkSize)
pcmData := make([]byte, subchunkSize) // ← attacker controls this number
io.ReadFull(r, pcmData)
```

**In plain terms:** the code reads the "how many bytes follow" number straight out of the
file and immediately uses it, unmodified, to reserve that much memory — like taking a
stranger's word for how big a container needs to be, no matter how absurd the number.
`make([]byte, subchunkSize)` is Go's instruction to allocate (in plain terms: reserve a
chunk of the computer's memory of a given size) a new byte slice (a slice is a resizable,
in-memory list of values — here, a list of raw bytes) exactly `subchunkSize` bytes long.

A file 44 bytes long with `subchunkSize = 0xFFFFFFFF` (≈ 4 GB) causes
`make([]byte, 4294967295)` — the allocator asks the OS for 4 GB. On most machines the
process is killed immediately.

The header skip had the same problem. `subchunk1Size` is untrusted, so:

```go
// BEFORE — also unsafe
extra := make([]byte, subchunk1Size-16) // subchunk1Size also attacker-controlled
io.ReadFull(r, extra)
```

---

## The fix: cap to bytes present

From [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md), the current `readDataChunk`
function (a function is a named, reusable block of code that does one job; calling or
invoking it means running it, usually by writing its name followed by parentheses; when
it finishes, it returns — hands its result back to whoever called it):

```go
// readDataChunk scans subchunks until it finds the "data" chunk and returns its
// PCM payload. The allocation is capped at the bytes actually remaining in the
// reader so a malformed or malicious header that declares a huge data size
// cannot trigger an out-of-memory allocation.
func readDataChunk(r *bytes.Reader) ([]byte, error) {
    for {
        var subchunkID [4]byte
        if err := binary.Read(r, binary.LittleEndian, &subchunkID); err != nil {
            return nil, fmt.Errorf("failed to find data subchunk: %w", err)
        }

        var subchunkSize uint32
        if err := binary.Read(r, binary.LittleEndian, &subchunkSize); err != nil {
            return nil, fmt.Errorf("failed to read subchunk size: %w", err)
        }

        if string(subchunkID[:]) == "data" {
            allocSize := int(subchunkSize)
            if allocSize > r.Len() {
                allocSize = r.Len() // never trust the declared size past EOF
            }
            pcmData := make([]byte, allocSize)
            if _, err := io.ReadFull(r, pcmData); err != nil && err != io.EOF {
                return nil, fmt.Errorf("failed to read PCM data: %w", err)
            }
            return pcmData, nil
        }

        // Skip this subchunk.
        if _, err := r.Seek(int64(subchunkSize), io.SeekCurrent); err != nil {
            return nil, fmt.Errorf("failed to skip subchunk: %w", err)
        }
    }
}
```

**In plain terms:** the function loops through the file's labeled sections one at a time.
When it finds the section named "data", it now first checks how many bytes are actually
still left in the file and shrinks its allocation request to that real amount if the
declared size is bigger — then reserves memory only for that safe amount and reads the
audio bytes into it. For every other section, it just steps over ("seeks past") that many
bytes without reserving any memory at all.

The key pair of lines:

```go
allocSize := int(subchunkSize)
if allocSize > r.Len() {
    allocSize = r.Len() // never trust the declared size past EOF
}
```

`r.Len()` is `bytes.Reader.Len()` — the count of bytes not yet read. (A "method" — the
`.Len()` written after `r` — is simply a function that belongs to a particular value, here
the reader `r`, and acts on it.) Because the reader wraps the original `[]byte` the caller
provided (the "caller" is whatever code invoked this function — the one waiting for its
return value), this is a hard physical bound. No matter what the header claims, the
allocation is at most as large as the remaining input.

---

## The header-skip fix: seek instead of allocate

For the format subchunk, the fix avoids allocating entirely — from `dr-wav-go/dr_wav.go`:

```go
// Skip any extra format bytes. Seek rather than allocate: subchunk1Size is an
// untrusted uint32, so make([]byte, subchunk1Size-16) is an OOM vector. If the
// declared size runs past EOF, the next chunk read fails cleanly.
if subchunk1Size > 16 {
    if _, err := r.Seek(int64(subchunk1Size-16), io.SeekCurrent); err != nil {
        return nil, fmt.Errorf("failed to skip extra format bytes: %w", err)
    }
}
```

**In plain terms:** instead of reserving memory for the extra format bytes and reading
them in, the code just moves its current reading position forward past them — like
flipping past pages you don't need to read instead of photocopying them first.

`bytes.Reader.Seek` does not allocate memory for the skipped region. If the seek target
is past EOF (EOF = "end of file," the point where there is no more data left to read), the
subsequent `binary.Read` call on the next chunk returns an error — graceful failure, no
OOM (OOM = "out of memory": the moment the computer refuses or fails a request for memory
because it doesn't have enough free, usually causing whatever asked for it to be shut
down).

!!! tip "Prefer Seek over make when discarding bytes"
    Whenever you need to skip N bytes from an untrusted source, `Seek` (or `io.Discard`
    with a length-capped copy) avoids allocating a throwaway buffer. Reserve `make` for
    bytes you actually intend to use.

---

## The serialization guard: mirror risk on the write side

Parsing is not the only direction. From `dr-wav-go/dr_wav.go`:

```go
// maxWAVDataSize is the largest PCM payload that fits in the 32-bit RIFF size
// fields (total file size must also fit, hence the 44-byte header allowance).
const maxWAVDataSize = math.MaxUint32 - 44

func Serialize(wav *WAV) ([]byte, error) {
    if wav == nil {
        return nil, errors.New("nil WAV")
    }
    if len(wav.Data) > maxWAVDataSize {
        return nil, fmt.Errorf("WAV data too large to serialize: %d bytes", len(wav.Data))
    }
    // …
}
```

**In plain terms:** before writing a WAV file out, this function checks whether the audio
data is too big to ever have its size correctly recorded in the file's 32-bit size field;
if it is, it refuses to write the file and reports an error instead of producing a broken
one. (`Serialize` here means "turn an in-memory representation of the WAV data into the
actual sequence of bytes that gets saved to disk.")

Without this guard, `uint32(len(wav.Data))` silently truncates (in plain terms: the number
gets cut down to fit, losing its high-order information, with no warning) for payloads
above 4 GB, writing a RIFF size field that points to the wrong amount of data. Any player or
downstream parser would misread the file. The guard turns silent corruption into an
explicit error.

!!! warning "Silent integer truncation is a data-corruption bug"
    In Go, `uint32(x)` for a large `int` does NOT panic — it wraps. Always guard
    conversions from `int`/`int64` to narrower unsigned types with an explicit range
    check, especially when the result ends up in a serialized binary field.

---

## How fuzzing found this

The `go test -fuzz` harness (a "harness" is just a scaffold of surrounding test code that
drives the thing being tested; "fuzzing" means automatically generating huge numbers of
random or semi-random inputs to see if any of them crash the program) generates random byte
sequences and feeds them to the parser as if they were valid WAV files. A sequence with a
plausible RIFF/WAVE/fmt header but a `subchunkSize` of `0xFFFFFFFF` in the data chunk
would:

1. Pass all structural checks (it looks like a valid header).
2. Trigger `make([]byte, 4294967295)` — OOM, process killed.
3. Produce a fuzz corpus entry (saved under `testdata/fuzz/`) that permanently guards
   against regression.

The fuzzer also found a second instance — the `make([]byte, subchunk1Size-16)` skip
allocation — because once one crash is fixed, the engine keeps mutating the input and
finds the next reachable crash.

!!! note "Try it"
    Run the fuzz target (the specific function the fuzzer drives with generated inputs)
    to verify the guard holds. From the repo root:

    ```bash
    cd dr-wav-go
    go test -fuzz=FuzzParse -fuzztime=15s ./...
    ```

    Expected outcome: the fuzzer runs for 15 seconds and exits cleanly with
    `no interesting inputs were found`. If you temporarily remove the `allocSize = r.Len()`
    cap and re-run, the fuzzer will almost immediately find the OOM crash and print a
    reproducer path.

    To run the existing regression seeds without fuzzing:

    ```bash
    go test ./...
    ```

    Expected: `ok  	drwavgo` — all tests pass including the seeds.

---

## The general pattern

This class of bug appears whenever:

1. A binary format stores a **size field** for a following payload.
2. The parser reads the field as an integer and uses it **directly** in `make`.
3. The input is **not fully trusted** — a file from disk, a network packet, a user upload.

The fix is always the same shape:

```go
allocSize := int(declaredSize)
if allocSize > available {
    allocSize = available
}
buf := make([]byte, allocSize)
```

Where `available` is derived from something the attacker cannot control: `r.Len()`,
`len(input) - offset` (an "offset" is a position counted in bytes from the start of some
data — "byte 16" rather than a name), or a hard-coded maximum capacity (a number written
directly into the source code rather than computed or read from input).

!!! note "This is not Go-specific"
    The same bug appears in C, Rust, Python, and every other language that has binary
    parsers. Go's advantage is that `make` panics (in plain terms: the program stops what
    it's doing right there and unwinds with an error, rather than continuing on with bad
    data) for negative sizes and the allocator will return an error for truly enormous
    allocations rather than silently succeeding like some C `malloc` implementations — but
    a panicking process is still a denial of service (a "denial of service," often
    shortened DoS, is any attack whose goal is simply to make a system stop working or
    become unavailable, rather than to steal data).

---

## Key takeaways

- **Never pass an untrusted integer directly to `make`.** Cap it first against the bytes
  you actually have (`r.Len()`, `len(input)-offset`, or a hard limit).
- **Seek instead of allocate when skipping.** `bytes.Reader.Seek` and `io.Discard` skip
  bytes without touching the heap (the heap is the pool of memory a program reserves
  chunks from at runtime, as opposed to memory that's part of the program's fixed,
  automatically-managed local workspace).
- **Serialization has the mirror risk.** A 32-bit size field cannot represent payloads
  larger than ~4 GB; guard the conversion explicitly or you get silent data corruption.
- **Fuzzing surfaces this class of bug reliably.** Size-field OOM crashes are found in
  seconds once a fuzzer generates plausible-looking headers with extreme size values.
- **Regression seeds prevent recurrence.** Every fuzzer-found crash becomes a
  `testdata/fuzz/` seed; `go test ./...` replays it on every CI run.
