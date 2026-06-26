# 07 · Binary parsing: dr-wav RIFF/PCM

> **Objectives:** Understand how a WAV file is laid out as a sequence of binary
> chunks, learn how `encoding/binary` reads little-endian integers directly into
> Go structs, and see why every size field from an untrusted file is a potential
> OOM vector that must be capped before allocation.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **RIFF/WAV chunk layout** = "a set of shipping boxes stacked inside a bigger shipping box — each box has a label tag on the outside and a size stamp, so you know exactly how far to reach before the next box starts." The outermost RIFF box holds a WAVE stamp, a `fmt ` metadata box, and a `data` box; the parser walks them left to right by reading the 4-byte tag and the 4-byte size before touching the payload.
- **Little-endian byte order** = "reading a price tag that lists cents before dollars — the least-significant digit comes first." WAV files store every multi-byte integer with the least-significant byte at the lowest address; `encoding/binary.LittleEndian` reverses the bytes into the normal Go value so you never do that arithmetic by hand.
- **`binary.Read`** = "a tape measure that advances itself — each time you call it, it reads exactly as many bytes as the target type needs and moves the cursor forward by that amount." Pass it a `bytes.Reader`, `binary.LittleEndian`, and a pointer to a Go integer or struct field; it fills the value and leaves the reader positioned right after those bytes, with no manual offset tracking.
- **Untrusted size field as OOM vector** = "a forged hotel booking that claims 4 000 rooms are reserved — blindly honouring it collapses the building." A crafted WAV can set the `data` chunk size to 4 GB in four bytes; calling `make([]byte, subchunkSize)` on that number crashes the process, so `readDataChunk` caps `allocSize` to `r.Len()`, the bytes actually present.
- **Seek to skip** = "fast-forwarding a tape instead of listening to the part you don't need." `r.Seek(int64(subchunk1Size-16), io.SeekCurrent)` jumps over extra `fmt ` bytes or unknown subchunks without allocating a buffer, so an oversized size field wastes no memory even before the cap check fires.
- **Separate parse from validate (`ValidateWAV`)** = "a customs officer who first checks whether a package is sealed correctly, then hands it to an inspector who checks whether the contents are legal." `Parse` accepts any byte layout that is structurally sound; `ValidateWAV` enforces semantic rules like `NumChannels != 0`, `AudioFormat == 1`, and a supported `BitsPerSample`, keeping each concern in one place.

**Why it matters:** audio processing pipelines ingest user-supplied WAV files.
A single malformed header field can turn "decode this file" into "allocate all
available RAM and crash."

**See it — the byte layout.** A WAV file is a flat strip of labelled chunks. Each
chunk is a 4-byte ASCII tag, a 4-byte little-endian size, then that many bytes. The
parser walks it left to right with `binary.Read`. The one field you can never trust
is the highlighted `data` size — cap it before any `make([]byte, n)`.

<svg viewBox="0 0 720 230" role="img" aria-labelledby="riff-t riff-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="riff-t">RIFF/WAV byte layout</title>
  <desc id="riff-d">A WAV file is a strip of chunks: RIFF tag, size, WAVE, fmt chunk, then a data chunk whose size field is the untrusted OOM vector.</desc>
  <g font-size="11.5" text-anchor="middle">
    <rect x="20" y="70" width="66" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="53" y="100" fill="currentColor" font-family="ui-monospace,monospace">"RIFF"</text>
    <rect x="86" y="70" width="66" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="119" y="96" fill="currentColor">file</text><text x="119" y="110" fill="currentColor">size</text>
    <rect x="152" y="70" width="66" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="185" y="100" fill="currentColor" font-family="ui-monospace,monospace">"WAVE"</text>
    <rect x="218" y="70" width="62" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="249" y="100" fill="currentColor" font-family="ui-monospace,monospace">"fmt "</text>
    <rect x="280" y="70" width="50" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="305" y="100" fill="currentColor">16</text>
    <rect x="330" y="70" width="118" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="389" y="92" fill="currentColor">fmt: channels,</text><text x="389" y="108" fill="var(--md-default-fg-color--light)">rate, bits…</text>
    <rect x="448" y="70" width="60" height="50" fill="none" stroke="currentColor" stroke-width="1.4"/><text x="478" y="100" fill="currentColor" font-family="ui-monospace,monospace">"data"</text>
    <rect x="508" y="70" width="74" height="50" fill="none" stroke="#e5484d" stroke-width="2"/><text x="545" y="92" fill="#e5484d" font-weight="600">data</text><text x="545" y="108" fill="#e5484d" font-weight="600">size</text>
    <rect x="582" y="70" width="118" height="50" fill="none" stroke="var(--md-default-fg-color--lighter)" stroke-width="1.4" stroke-dasharray="4 3"/><text x="641" y="100" fill="var(--md-default-fg-color--light)">samples…</text>
  </g>
  <g font-size="9" fill="var(--md-default-fg-color--light)" text-anchor="middle">
    <text x="53" y="136">0</text><text x="119" y="136">4</text><text x="185" y="136">8</text><text x="249" y="136">12</text><text x="305" y="136">16</text><text x="389" y="136">20</text><text x="478" y="136">36</text><text x="545" y="136">40</text><text x="641" y="136">44</text>
  </g>
  <text x="360" y="40" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">each chunk = 4-byte tag · 4-byte little-endian size · payload — read left to right</text>
  <path d="M545,150 L545,178" fill="none" stroke="#e5484d" stroke-width="1.4"/>
  <text x="360" y="196" text-anchor="middle" font-size="12" fill="#e5484d" font-weight="600">untrusted — a crafted file can claim 4 GB here</text>
  <text x="360" y="214" text-anchor="middle" font-size="11.5" fill="currentColor" font-family="ui-monospace,monospace">cap it, then make([]byte, n) — never trust the number</text>
</svg>

---

## The RIFF/WAV byte layout

A canonical 44-byte WAV header looks like this:

```
Offset  Size  Field
------  ----  -----
0       4     "RIFF"            (ASCII magic)
4       4     fileSize - 8      (uint32 LE)
8       4     "WAVE"            (format tag)
12      4     "fmt "            (subchunk ID)
16      4     16                (subchunk1 size, uint32 LE)
20      2     audioFormat       (1 = PCM, uint16 LE)
22      2     numChannels       (uint16 LE)
24      4     sampleRate        (uint32 LE)
28      4     byteRate          (uint32 LE)
32      2     blockAlign        (uint16 LE)
34      2     bitsPerSample     (uint16 LE)
36      4     "data"            (subchunk ID)
40      4     dataSize          (uint32 LE)
44      …     PCM samples
```

After the mandatory header, any number of extra subchunks may appear before
`data`. The parser must scan forward until it finds the `"data"` tag.

---

## Reading fixed fields with `encoding/binary`

From [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md), the `WAVHeader` struct maps directly onto the fixed
fields in the `fmt ` subchunk:

```go
// from dr-wav-go/dr_wav.go
type WAVHeader struct {
    AudioFormat   uint16 // 1 = PCM
    NumChannels   uint16
    SampleRate    uint32
    ByteRate      uint32
    BlockAlign    uint16
    BitsPerSample uint16
}
```

`Parse` wraps the input slice in a `bytes.Reader` and reads each field in order:

```go
// from dr-wav-go/dr_wav.go
r := bytes.NewReader(data)

var riff [4]byte
binary.Read(r, binary.LittleEndian, &riff)   // advances 4 bytes
// … validate "RIFF" …

var chunkSize uint32
binary.Read(r, binary.LittleEndian, &chunkSize) // advances 4 bytes

// … read "WAVE", "fmt ", subchunk1Size …

var header WAVHeader
binary.Read(r, binary.LittleEndian, &header.AudioFormat)
binary.Read(r, binary.LittleEndian, &header.NumChannels)
binary.Read(r, binary.LittleEndian, &header.SampleRate)
// … and so on for ByteRate, BlockAlign, BitsPerSample
```

Each `binary.Read` call consumes exactly the number of bytes that the target
type requires. There is no manual offset arithmetic — `bytes.Reader` tracks the
position internally.

!!! note "Why not read the whole struct at once?"
    `binary.Read` *can* decode a whole struct in one call if every field is a
    fixed-size type. The code reads field-by-field to make each error message
    name the failing field explicitly, which is more useful when debugging
    corrupted files. Both styles are correct.

---

## Skipping unknown `fmt ` bytes safely

The `fmt ` subchunk is 16 bytes for PCM but can be larger for compressed formats.
The size is declared by `subchunk1Size`, a `uint32` from the file.

```go
// from dr-wav-go/dr_wav.go
// Skip any extra format bytes. Seek rather than allocate: subchunk1Size is an
// untrusted uint32, so make([]byte, subchunk1Size-16) is an OOM vector. If the
// declared size runs past EOF, the next chunk read fails cleanly.
if subchunk1Size > 16 {
    if _, err := r.Seek(int64(subchunk1Size-16), io.SeekCurrent); err != nil {
        return nil, fmt.Errorf("failed to skip extra format bytes: %w", err)
    }
}
```

`Seek` with `io.SeekCurrent` moves the internal cursor by `n` bytes without
reading or allocating. If the seek lands past EOF, the *next* read operation
returns an error — a clean, safe failure rather than an OOM crash.

---

## Finding the data chunk

WAV files can have subchunks between `fmt ` and `data` (e.g. `LIST`, `cue `).
`readDataChunk` loops over them, skipping unknown ones and stopping when it sees
`"data"`:

```go
// from dr-wav-go/dr_wav.go
func readDataChunk(r *bytes.Reader) ([]byte, error) {
    for {
        var subchunkID [4]byte
        binary.Read(r, binary.LittleEndian, &subchunkID)

        var subchunkSize uint32
        binary.Read(r, binary.LittleEndian, &subchunkSize)

        if string(subchunkID[:]) == "data" {
            allocSize := int(subchunkSize)
            if allocSize > r.Len() {
                allocSize = r.Len() // never trust the declared size past EOF
            }
            pcmData := make([]byte, allocSize)
            io.ReadFull(r, pcmData)
            return pcmData, nil
        }

        // Skip this subchunk.
        r.Seek(int64(subchunkSize), io.SeekCurrent)
    }
}
```

The critical line is the cap:

```go
if allocSize > r.Len() {
    allocSize = r.Len()
}
```

`r.Len()` returns the number of bytes *actually remaining* in the reader. A
file claiming a 4 GB data chunk but containing only 100 bytes will allocate
100 bytes, not 4 GB. This is the fix for the fuzz-discovered OOM bug (see
[Lesson 17](17-unbounded-allocation-oom.md) for how `go test -fuzz` found it).

!!! warning "Size fields are always untrusted"
    Any integer read from a file, network packet, or user input that controls
    an allocation is an OOM vector. The rule is: **cap to bytes present, not
    bytes declared**. The same principle applies in `miniz-go` for ZIP entry
    sizes and in `tinyxml2-go` for nesting depth.

---

## Division-by-zero guards in derived calculations

After parse, `GetSampleCount` computes how many samples are in the data:

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

`Parse` does not reject a zero-channel header — that is left to `ValidateWAV`.
So `GetSampleCount` must guard the division independently. The same guard
applies to `bytesPerSample`: if `BitsPerSample` is 0, dividing by 8 gives 0,
and dividing by that would panic.

---

## Serialization: the 4 GB guard

`Serialize` writes a WAV file back to bytes. RIFF size fields are `uint32`, so
a PCM payload larger than `2^32 - 1 - 44` bytes cannot be represented:

```go
// from dr-wav-go/dr_wav.go
const maxWAVDataSize = math.MaxUint32 - 44

func Serialize(wav *WAV) ([]byte, error) {
    if len(wav.Data) > maxWAVDataSize {
        return nil, fmt.Errorf("WAV data too large to serialize: %d bytes", len(wav.Data))
    }
    // … write fields …
}
```

The check happens before any allocation. Failing fast with a clear error is
preferable to writing a truncated file that silently corrupts audio data.

---

## Validation as a second pass

`ValidateWAV` enforces semantic constraints that `Parse` intentionally skips:

```go
// from dr-wav-go/dr_wav.go
func ValidateWAV(wav *WAV) error {
    if wav.Header.AudioFormat != 1 {
        return fmt.Errorf("unsupported audio format: %d (only PCM supported)", wav.Header.AudioFormat)
    }
    if wav.Header.NumChannels == 0 {
        return errors.New("invalid number of channels: 0")
    }
    if wav.Header.SampleRate == 0 {
        return errors.New("invalid sample rate: 0")
    }
    if wav.Header.BitsPerSample != 8 && wav.Header.BitsPerSample != 16 &&
        wav.Header.BitsPerSample != 24 && wav.Header.BitsPerSample != 32 {
        return fmt.Errorf("unsupported bits per sample: %d", wav.Header.BitsPerSample)
    }
    return nil
}
```

This separation keeps `Parse` focused on structure (does the byte layout make
sense?) and `ValidateWAV` focused on semantics (do the values make sense?). A
library that silently rejects odd-but-parseable headers surprises callers; one
that parses everything and surfaces validation as a separate step is more
composable.

---

!!! note "Try it"
    Run the dr-wav tests, including the fuzz regression corpus:

    ```bash
    cd dr-wav-go
    go test ./... -v -count=1
    ```

    Expected outcome: all tests pass in a few milliseconds. You should see test
    names like `TestParse`, `TestValidateWAV`, `TestWAV_GetSampleCount`, and
    `TestSerialize`. None should report a FAIL line.

    To replay just the fuzz regression seeds (no new mutation, fast):

    ```bash
    go test ./... -run=FuzzParse -v
    ```

    If fuzz seeds exist under `testdata/fuzz/FuzzParse/`, each one is run as a
    named sub-test and should pass cleanly — these are the exact byte sequences
    that previously caused OOM crashes.

---

!!! tip "Reading deeper"
    `ParseBatch` in `dr-wav-go/dr_wav.go` uses a worker pool (one goroutine per
    CPU) to parse multiple WAV files concurrently. The channel buffer sizes
    match `len(dataList)` exactly so no goroutine blocks on a send —
    compare this with the deadlock bug in `jsmn-go` described in
    [Lesson 14](14-the-deadlock-bug.md), where a buffer that was one slot too
    small caused `wg.Wait` to hang forever.

---

## Key takeaways

- **`encoding/binary.LittleEndian` + `bytes.Reader`** gives you a streaming,
  stateful view of raw bytes; each `binary.Read` advances the cursor by exactly
  the size of its target type — no offset math needed.
- **Every size field from an untrusted source is an OOM vector.** Cap allocations
  to `r.Len()` (bytes actually present), never to the declared size.
- **Seek to skip, never allocate-and-discard.** `r.Seek(n, io.SeekCurrent)` is
  free; `make([]byte, n)` followed by a read is proportional to `n`.
- **Separate parse from validate.** `Parse` checks byte-level structure;
  `ValidateWAV` checks semantic invariants. Callers that only need to transcode
  a file can skip validation; callers that need clean data run both.
- **Guard every division by a field that could be zero.** `BitsPerSample` and
  `NumChannels` come from the file and can both be zero; `GetSampleCount`
  returns 0 rather than panicking when either is absent.
