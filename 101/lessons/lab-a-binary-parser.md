# Lab A - Write a bounds-checked binary parser

!!! note "This is a lab, not a lecture."
    You will write code, not just read it. Follow the guided steps in order.
    Each step has an expected outcome — run the command and check it before moving on.

> **Objectives:** Learn to read structured binary data with `encoding/binary`, defend every
> length field with a bounds check before slicing, and understand how dr-wav-go applies
> exactly these techniques to a real-world RIFF/WAV file.
> Estimated time: 40 minutes.

---

## What this actually means (plain English)

- **A binary format is a sequence of fields with exact byte positions.** Unlike JSON, there
  are no `{` or `"` separators — you have to know the schema ahead of time and read each
  field at the right offset.
- **Every "length" field is a claim, not a fact.** A file can say its data chunk is 4 GB
  long while the actual file is 100 bytes. If you trust it blindly and call `make([]byte, n)`,
  your program crashes with an out-of-memory panic.
- **Bounds-checking means: read what remains, not what is claimed.** Cap any allocation to
  `reader.Len()` (bytes remaining in the buffer) before you `make` the slice.
- **`encoding/binary.Read` is your friend.** It reads fixed-size values in a specified byte
  order and returns an error instead of silently misaligning everything.
- **A chunk-based format lets you skip unknown sections.** RIFF, IFF, PNG, and many others
  use a repeating `[4-byte tag][4-byte length][data…]` pattern. Unknown tags get `Seek`-ed
  past, not crashed on.

**Why it matters:** a single missing bounds check in a binary parser is a denial-of-service
waiting to happen — pass in a crafted file, exhaust the server's memory, game over. The
fix is three lines; skipping it costs you the production incident.

---

## The format you will parse: LP-Chunk

You will build a parser for **LP-Chunk**, a tiny invented binary format:

```
File layout
───────────────────────────────────────
Offset  Size  Field
0       4     Magic: "LPCK" (ASCII)
4       2     Version (uint16, little-endian)
6       2     ChunkCount (uint16, little-endian)
── repeated ChunkCount times ──────────
  0     4     ChunkID  (4 ASCII bytes, e.g. "NAME", "DATA", "CSUM")
  4     4     ChunkLen (uint32, little-endian) — length of Payload
  8     N     Payload  (ChunkLen bytes)
───────────────────────────────────────
```

Every number is **little-endian**. This is the same layout RIFF/WAV uses: a file-level
header, a count of sub-chunks, then repeating `[tag][length][data]` records. dr-wav-go
(`dr-wav-go/dr_wav.go`) does exactly this for real WAV files.

---

## Step 0 — create the module

```bash
mkdir lpchunk && cd lpchunk
go mod init example.com/lpchunk
touch parser.go parser_test.go
```

---

## Step 1 — define the types

Open `parser.go` and paste the skeleton:

```go
// Package lpchunk parses the LP-Chunk binary format.
// This is a lab companion to the SafeHeaders-Go course.
package lpchunk

import (
    "bytes"
    "encoding/binary"
    "errors"
    "fmt"
    "io"
)

// Header is the 6-byte file header.
type Header struct {
    Version    uint16
    ChunkCount uint16
}

// Chunk is one [tag][len][payload] record.
type Chunk struct {
    ID      [4]byte
    Payload []byte
}

// IDString returns the tag as a printable string.
func (c Chunk) IDString() string { return string(c.ID[:]) }

// File is the fully parsed result.
type File struct {
    Header Header
    Chunks []Chunk
}
```

---

## Step 2 — read the magic bytes

Still in `parser.go`, add the `Parse` function:

```go
// Parse reads and validates an LP-Chunk file from data.
func Parse(data []byte) (*File, error) {
    if len(data) < 8 {
        return nil, errors.New("lpchunk: data too short for header")
    }

    r := bytes.NewReader(data)

    // ── Magic ──────────────────────────────────────────────────────────────
    var magic [4]byte
    if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
        return nil, fmt.Errorf("lpchunk: read magic: %w", err)
    }
    if string(magic[:]) != "LPCK" {
        return nil, fmt.Errorf("lpchunk: bad magic %q", magic)
    }

    // ── Header ─────────────────────────────────────────────────────────────
    var hdr Header
    if err := binary.Read(r, binary.LittleEndian, &hdr.Version); err != nil {
        return nil, fmt.Errorf("lpchunk: read version: %w", err)
    }
    if err := binary.Read(r, binary.LittleEndian, &hdr.ChunkCount); err != nil {
        return nil, fmt.Errorf("lpchunk: read chunk count: %w", err)
    }

    // TODO: read chunks (Step 3)
    _ = hdr
    return nil, errors.New("lpchunk: chunk reading not yet implemented")
}
```

!!! note "Try it"
    ```bash
    go build ./...
    ```
    Expected outcome: **no compiler errors**. The `_ = hdr` line silences the "declared
    and not used" error while you stub the next step. You should see nothing printed —
    silence is success.

---

## Step 3 — read the chunks (with bounds checking)

Replace the `// TODO` block and the stub return with the real loop.  
This is where bounds-checking matters most.

```go
    // ── Chunks ─────────────────────────────────────────────────────────────
    chunks := make([]Chunk, 0, hdr.ChunkCount)

    for i := 0; i < int(hdr.ChunkCount); i++ {
        var ch Chunk

        // 4-byte tag
        if err := binary.Read(r, binary.LittleEndian, &ch.ID); err != nil {
            return nil, fmt.Errorf("lpchunk: chunk %d: read id: %w", i, err)
        }

        // declared payload length
        var declaredLen uint32
        if err := binary.Read(r, binary.LittleEndian, &declaredLen); err != nil {
            return nil, fmt.Errorf("lpchunk: chunk %d: read length: %w", i, err)
        }

        // ── THE CRITICAL CHECK ──────────────────────────────────────────────
        // Trust the bytes that are actually present, not the number in the file.
        // A crafted file can claim declaredLen = 4 GB; r.Len() tells us the truth.
        allocLen := int(declaredLen)
        if allocLen > r.Len() {
            allocLen = r.Len() // cap to bytes present — never OOM on a lie
        }
        // ───────────────────────────────────────────────────────────────────

        ch.Payload = make([]byte, allocLen)
        if _, err := io.ReadFull(r, ch.Payload); err != nil && !errors.Is(err, io.EOF) {
            return nil, fmt.Errorf("lpchunk: chunk %d: read payload: %w", i, err)
        }

        chunks = append(chunks, ch)
    }

    return &File{Header: hdr, Chunks: chunks}, nil
```

The three lines around "THE CRITICAL CHECK" are directly modeled on `readDataChunk` in
`dr-wav-go/dr_wav.go`:

```go
// from dr-wav-go/dr_wav.go — readDataChunk
allocSize := int(subchunkSize)
if allocSize > r.Len() {
    allocSize = r.Len() // never trust the declared size past EOF
}
pcmData := make([]byte, allocSize)
```

Same shape, same reason: `subchunkSize` is a `uint32` from an untrusted file; `r.Len()`
is the ground truth.

!!! warning "What happens without the check"
    If you delete those three lines and a malicious file claims `declaredLen = 0xFFFFFFFF`
    (about 4 GB), `make([]byte, 4294967295)` panics with `runtime: out of memory` — or,
    on a machine with enough swap, it actually allocates 4 GB and stalls the process.
    This is a real denial-of-service class: the fuzzer found exactly this shape in
    dr-wav-go before the fix landed.

---

## Step 4 — write a helper to build test files

Add `builder.go` in the same package so your tests can construct valid (and malicious)
LP-Chunk files without hand-encoding bytes:

```go
package lpchunk

import (
    "bytes"
    "encoding/binary"
)

// BuildFile serializes an LP-Chunk file. Use in tests only.
func BuildFile(version uint16, chunks []Chunk) []byte {
    var buf bytes.Buffer
    buf.WriteString("LPCK")
    binary.Write(&buf, binary.LittleEndian, version)
    binary.Write(&buf, binary.LittleEndian, uint16(len(chunks)))
    for _, ch := range chunks {
        buf.Write(ch.ID[:])
        binary.Write(&buf, binary.LittleEndian, uint32(len(ch.Payload)))
        buf.Write(ch.Payload)
    }
    return buf.Bytes()
}
```

`binary.Write` to a `bytes.Buffer` cannot fail (fixed-size values, in-memory buffer), so
ignoring the error here is safe — but only in a test helper. In production parsers,
always check it. `dr-wav-go/dr_wav.go`'s `Serialize` function uses a tiny `put` closure
to capture and propagate write errors:

```go
// from dr-wav-go/dr_wav.go — Serialize
var werr error
put := func(v any) {
    if werr == nil {
        werr = binary.Write(&buf, binary.LittleEndian, v)
    }
}
```

---

## Step 5 — write the tests

Paste this into `parser_test.go`:

```go
package lpchunk_test

import (
    "testing"

    "example.com/lpchunk"
)

func TestRoundTrip(t *testing.T) {
    original := []lpchunk.Chunk{
        {ID: [4]byte{'N', 'A', 'M', 'E'}, Payload: []byte("hello")},
        {ID: [4]byte{'D', 'A', 'T', 'A'}, Payload: []byte{0x01, 0x02, 0x03}},
    }
    data := lpchunk.BuildFile(1, original)

    f, err := lpchunk.Parse(data)
    if err != nil {
        t.Fatalf("Parse: %v", err)
    }
    if len(f.Chunks) != len(original) {
        t.Fatalf("got %d chunks, want %d", len(f.Chunks), len(original))
    }
    for i, ch := range f.Chunks {
        if ch.IDString() != original[i].IDString() {
            t.Errorf("chunk %d: id %q, want %q", i, ch.IDString(), original[i].IDString())
        }
        if string(ch.Payload) != string(original[i].Payload) {
            t.Errorf("chunk %d: payload mismatch", i)
        }
    }
}

func TestBadMagic(t *testing.T) {
    data := lpchunk.BuildFile(1, nil)
    data[0] = 'X' // corrupt the magic
    if _, err := lpchunk.Parse(data); err == nil {
        t.Fatal("expected error for bad magic, got nil")
    }
}

func TestTruncatedData(t *testing.T) {
    // Data too short for the header at all
    if _, err := lpchunk.Parse([]byte{1, 2, 3}); err == nil {
        t.Fatal("expected error for truncated input")
    }
}

func TestBoundsCheck_OversizedChunk(t *testing.T) {
    // Build a valid file with one chunk, then overwrite its declared length
    // with a giant number. The parser must NOT panic or OOM.
    chunk := lpchunk.Chunk{
        ID:      [4]byte{'D', 'A', 'T', 'A'},
        Payload: []byte{0xAA, 0xBB},
    }
    data := lpchunk.BuildFile(1, []lpchunk.Chunk{chunk})

    // Offset of the chunk length field:
    //   4 (magic) + 2 (version) + 2 (count) + 4 (chunk id) = 12
    const lenOffset = 12
    // Write 0xFFFFFFFF (4 GB) as little-endian uint32
    data[lenOffset+0] = 0xFF
    data[lenOffset+1] = 0xFF
    data[lenOffset+2] = 0xFF
    data[lenOffset+3] = 0xFF

    // Must return without panicking. The payload will be truncated to
    // whatever bytes are actually remaining — that's the correct behavior.
    f, err := lpchunk.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(f.Chunks[0].Payload) > 2 {
        t.Errorf("payload longer than bytes present: %d bytes", len(f.Chunks[0].Payload))
    }
    t.Logf("claimed 4 GB, got %d bytes (correct)", len(f.Chunks[0].Payload))
}
```

!!! note "Try it"
    ```bash
    go test -v ./...
    ```
    Expected outcome:
    ```
    --- PASS: TestRoundTrip (0.00s)
    --- PASS: TestBadMagic (0.00s)
    --- PASS: TestTruncatedData (0.00s)
    --- PASS: TestBoundsCheck_OversizedChunk (0.00s)
        parser_test.go:XX: claimed 4 GB, got 2 bytes (correct)
    PASS
    ok  	example.com/lpchunk
    ```
    If `TestBoundsCheck_OversizedChunk` **panics** instead, your bounds check is missing
    from Step 3.

---

## Step 6 — prove there is no data race

```bash
go test -race -v ./...
```

!!! note "Try it"
    Expected outcome: all four tests pass, no `DATA RACE` output.  
    If you see a race warning, look for any shared variable written from two goroutines
    without a mutex. (This lab is single-threaded, so you should be clean.)

---

## Step 7 — add a fuzz target

The dr-wav OOM bug was found by `go test -fuzz`. Add the same tool to your parser:

```go
// In parser_test.go, add:

func FuzzParse(f *testing.F) {
    // Seed corpus: a valid file and an empty input
    f.Add(lpchunk.BuildFile(1, []lpchunk.Chunk{
        {ID: [4]byte{'D', 'A', 'T', 'A'}, Payload: []byte("seed")},
    }))
    f.Add([]byte{})

    f.Fuzz(func(t *testing.T, data []byte) {
        // Must not panic, must not OOM, must not hang.
        lpchunk.Parse(data) //nolint:errcheck
    })
}
```

!!! note "Try it"
    ```bash
    go test -fuzz=FuzzParse -fuzztime=30s ./...
    ```
    Expected outcome: the fuzzer runs for 30 seconds and reports no crashes. Any failure
    written to `testdata/fuzz/FuzzParse/` is a reproducible regression seed — commit it
    alongside a fix, exactly as dr-wav-go does under `testdata/fuzz/`.

---

## How dr-wav-go applies these same patterns

| LP-Chunk lab pattern | dr-wav-go equivalent (dr-wav-go/dr_wav.go) |
|---|---|
| Check `len(data) < 8` before reading the header | `if len(data) < 44 { return nil, errors.New("data too short…") }` |
| `binary.Read(r, binary.LittleEndian, &magic)` | Same for RIFF/WAVE/fmt tags |
| Cap `allocLen` to `r.Len()` | `if allocSize > r.Len() { allocSize = r.Len() }` in `readDataChunk` |
| `io.ReadFull(r, ch.Payload)` | `io.ReadFull(r, pcmData)` |
| Skip unknown chunks with `r.Seek` | `r.Seek(int64(subchunkSize), io.SeekCurrent)` for non-"data" subchunks |
| `Serialize` with a `binary.Write` error wrapper | `put` closure capturing `werr` |
| Fuzz with `go test -fuzz` | Fuzz regression seeds in `dr-wav-go/testdata/fuzz/` |

The guard in `readDataChunk` was added after the OOM bug was found by fuzzing — a crafted
WAV declared a 4 GB data chunk in a 50-byte file. The same three-line pattern now lives in
your parser too.

!!! tip "Read the real source"
    Open `dr-wav-go/dr_wav.go` and read `readDataChunk` (around line 121) and `Serialize`
    (around line 329). Every technique in this lab appears there verbatim. Seeing a pattern
    in a toy parser and then recognising it in a production file is how you build the instinct.

---

## Key takeaways

- **Every length field from a file is a claim.** Validate it against the bytes actually
  present (`reader.Len()`) before allocating. Three lines prevent a class of OOM panics.
- **`encoding/binary.Read` gives you typed, endian-aware reads with proper error
  propagation.** Prefer it over manual slice indexing for fixed-size fields.
- **Chunk-based formats let you skip unknown sections safely.** Use `Seek` rather than
  allocating a throw-away buffer — especially for untrusted size fields.
- **`go test -fuzz` finds the bugs you didn't think to write tests for.** The OOM in
  dr-wav-go was found this way. A 30-second fuzz run is cheap insurance.
- **Wrap every `binary.Write` error in production serializers.** Writing to a
  `bytes.Buffer` looks infallible but isn't; dr-wav-go's `put` closure is the right
  pattern.
