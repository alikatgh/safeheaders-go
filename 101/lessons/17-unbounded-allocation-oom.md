# 17 · Unbounded allocation = OOM

> **Objectives:** Understand why trusting a size field from untrusted input can crash your
> process with an out-of-memory error, see how the dr-wav-go parser was fixed by capping
> allocation to bytes actually present, and learn how fuzzing surfaces this class of bug
> before attackers do.
> Estimated time: 20 minutes.

## What this actually means (plain English)

- **A WAV file is just a structured binary envelope.** Near the top it has a 32-bit integer
  that says "my audio payload is N bytes long." The parser reads that number and allocates
  a slice to receive the data.
- **That number comes from the file — which can be crafted by anyone.** A file that is 44
  bytes on disk can legally claim its audio chunk is 4 GB. If the parser believes it, the
  OS is asked for 4 GB of RAM and the process dies.
- **The fix is one comparison: never allocate more than the bytes you actually have.**
  `r.Len()` returns how many bytes remain in the reader. If the declared size is larger, use
  `r.Len()` instead.
- **The same shape appears in the format header.** Before the fix, skipping "extra format
  bytes" used `make([]byte, subchunk1Size-16)` — another untrusted `uint32` controlling an
  allocation. The fix there is different: `r.Seek(...)` instead of allocating at all.
- **Serialization has the mirror risk.** Writing a WAV file uses 32-bit RIFF size fields.
  If the audio payload is larger than a `uint32` can express (roughly 4 GB), the size field
  silently wraps — producing a corrupt file. A guard rejects the write early.
- **Fuzzing found this.** The `go test -fuzz` harness generated inputs with giant declared
  sizes and crashed the parser. The crash became a regression seed; the fix made it
  permanent.

**Why it matters:** one crafted upload, one allocation, process dead — no exploit code
required.

---

## The vulnerability: trusting the header

RIFF/WAV binary layout (simplified):

```
"RIFF" [4-byte total size] "WAVE"
"fmt " [subchunk1Size]  <format fields>
"data" [subchunkSize]   <PCM bytes …>
```

Both `subchunk1Size` and `subchunkSize` are `uint32` values decoded directly from the
file. Before the fix, the data-chunk reader looked roughly like:

```go
// BEFORE — do NOT copy this pattern
var subchunkSize uint32
binary.Read(r, binary.LittleEndian, &subchunkSize)
pcmData := make([]byte, subchunkSize) // ← attacker controls this number
io.ReadFull(r, pcmData)
```

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

From `dr-wav-go/dr_wav.go`, the current `readDataChunk` function:

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

The key pair of lines:

```go
allocSize := int(subchunkSize)
if allocSize > r.Len() {
    allocSize = r.Len() // never trust the declared size past EOF
}
```

`r.Len()` is `bytes.Reader.Len()` — the count of bytes not yet read. Because the reader
wraps the original `[]byte` the caller provided, this is a hard physical bound. No matter
what the header claims, the allocation is at most as large as the remaining input.

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

`bytes.Reader.Seek` does not allocate memory for the skipped region. If the seek target
is past EOF, the subsequent `binary.Read` call on the next chunk returns an error —
graceful failure, no OOM.

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

Without this guard, `uint32(len(wav.Data))` silently truncates for payloads above 4 GB,
writing a RIFF size field that points to the wrong amount of data. Any player or downstream
parser would misread the file. The guard turns silent corruption into an explicit error.

!!! warning "Silent integer truncation is a data-corruption bug"
    In Go, `uint32(x)` for a large `int` does NOT panic — it wraps. Always guard
    conversions from `int`/`int64` to narrower unsigned types with an explicit range
    check, especially when the result ends up in a serialized binary field.

---

## How fuzzing found this

The `go test -fuzz` harness generates random byte sequences and feeds them to the parser
as if they were valid WAV files. A sequence with a plausible RIFF/WAVE/fmt header but a
`subchunkSize` of `0xFFFFFFFF` in the data chunk would:

1. Pass all structural checks (it looks like a valid header).
2. Trigger `make([]byte, 4294967295)` — OOM, process killed.
3. Produce a fuzz corpus entry (saved under `testdata/fuzz/`) that permanently guards
   against regression.

The fuzzer also found a second instance — the `make([]byte, subchunk1Size-16)` skip
allocation — because once one crash is fixed, the engine keeps mutating the input and
finds the next reachable crash.

!!! note "Try it"
    Run the fuzz target to verify the guard holds. From the repo root:

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
`len(input) - offset`, or a hard-coded maximum capacity.

!!! note "This is not Go-specific"
    The same bug appears in C, Rust, Python, and every other language that has binary
    parsers. Go's advantage is that `make` panics for negative sizes and the allocator
    will return an error for truly enormous allocations rather than silently succeeding
    like some C `malloc` implementations — but a panicking process is still a denial of
    service.

---

## Key takeaways

- **Never pass an untrusted integer directly to `make`.** Cap it first against the bytes
  you actually have (`r.Len()`, `len(input)-offset`, or a hard limit).
- **Seek instead of allocate when skipping.** `bytes.Reader.Seek` and `io.Discard` skip
  bytes without touching the heap.
- **Serialization has the mirror risk.** A 32-bit size field cannot represent payloads
  larger than ~4 GB; guard the conversion explicitly or you get silent data corruption.
- **Fuzzing surfaces this class of bug reliably.** Size-field OOM crashes are found in
  seconds once a fuzzer generates plausible-looking headers with extreme size values.
- **Regression seeds prevent recurrence.** Every fuzzer-found crash becomes a
  `testdata/fuzz/` seed; `go test ./...` replays it on every CI run.
