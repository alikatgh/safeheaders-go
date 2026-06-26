# 08 · TrueType I - the sfnt table directory

> **Objectives:** Understand what a `.ttf` file actually contains at the binary level and how
> `stb-truetype-go` reads it safely. Learn why every offset must be validated before slicing,
> and trace the path from raw bytes to a usable `Font` struct.
> Estimated time: 20 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- A `.ttf` file is not a single blob — it is a **named-table container** (called "sfnt"). Think of
  it like a ZIP archive: there is a table of contents at the front, and each entry says where
  to find a named chunk of data.
- The table of contents is called the **sfnt table directory**. Each entry has a four-byte name
  (e.g. `head`, `glyf`, `cmap`), a file offset, and a byte length.
- A **bounds check** is the guard that makes sure an offset + length does not point outside the
  file. Without it, a malicious or truncated file could cause a panic (slice-out-of-range) or
  silent memory corruption.
- `head` is the font's "ID card": units per em, loca format. `maxp` says how many glyphs exist.
  `loca` is an index — it maps each glyph ID to its byte range inside the `glyf` table.
- **Why it matters:** skipping a single bounds check on an untrusted font file is a remote
  panic. Every lookup in this rasterizer goes through one gated helper — `tableData` — so the
  check is written once and used everywhere.

---

## The sfnt container format

Every TrueType file starts with a 12-byte header:

| Bytes | Field | Meaning |
|-------|-------|---------|
| 0–3 | version tag | `0x00010000` or `"true"` = TrueType; `"OTTO"` = CFF/OpenType |
| 4–5 | numTables | how many table records follow |
| 6–11 | search hint fields | used for binary search; we skip them |

Then come `numTables` records of 16 bytes each:

| Bytes (within record) | Field | Meaning |
|-----------------------|-------|---------|
| 0–3 | tag | four-byte ASCII name, e.g. `"head"` |
| 4–7 | checksum | integrity; we do not verify it |
| 8–11 | offset | byte offset from file start |
| 12–15 | length | byte length of table data |

`parseSFNT` in [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md) reads this structure directly:

```go
// from stb-truetype-go/sfnt.go
func (f *Font) parseSFNT() error {
    d := f.rawData
    if len(d) < 12 {
        return errors.New("truetype: data too short for sfnt header")
    }
    switch binary.BigEndian.Uint32(d) {
    case 0x00010000, 0x74727565: // TrueType
    case 0x4F54544F: // "OTTO"
        return errors.New("truetype: OpenType/CFF fonts are not supported (no glyf table)")
    default:
        return fmt.Errorf("truetype: unrecognized sfnt version 0x%08x", binary.BigEndian.Uint32(d))
    }

    numTables := int(binary.BigEndian.Uint16(d[4:]))
    f.tables = make(map[string]tableRec, numTables)
    for i := 0; i < numTables; i++ {
        off := 12 + i*16
        if off+16 > len(d) {
            return errors.New("truetype: truncated table directory")
        }
        f.tables[string(d[off:off+4])] = tableRec{
            offset: binary.BigEndian.Uint32(d[off+8:]),
            length: binary.BigEndian.Uint32(d[off+12:]),
        }
    }
    // ...
}
```

Notice the early `len(d) < 12` check before any indexing, and the `off+16 > len(d)` guard
inside the loop. These two lines together mean a truncated file returns a clean error rather
than a panic.

---

## tableData: the single point of trust

Once the directory is in memory, every caller that wants table bytes goes through one function:

```go
// from stb-truetype-go/sfnt.go
func (f *Font) tableData(tag string) ([]byte, bool) {
    rec, ok := f.tables[tag]
    if !ok {
        return nil, false
    }
    start, end := int(rec.offset), int(rec.offset)+int(rec.length)
    if start < 0 || end < start || end > len(f.rawData) {
        return nil, false
    }
    return f.rawData[start:end], true
}
```

Four things are checked in one place:

1. The tag exists in the directory.
2. `start` is non-negative (overflow guard — `uint32` fits in `int` but a crafted value near
   max-uint32 would wrap negative on 32-bit builds).
3. `end >= start` (length is not zero-or-wrapping-negative).
4. `end` does not exceed the actual file length.

All table parsers call `tableData` first and early-return on the `bool`. No parser ever indexes
`f.rawData` directly. This is the "write the check once, use it everywhere" pattern.

!!! warning "What happens without this guard"
    Without the `end > len(f.rawData)` check, a crafted font could list a table with
    `offset=0` and `length=math.MaxUint32`. The slice expression `d[0:math.MaxUint32]`
    panics immediately. The guard turns that into a silent `nil, false`.

---

## Reading the tables that matter

After building the directory, `parseSFNT` calls five sub-parsers in order:

```go
// from stb-truetype-go/sfnt.go
for _, step := range []func() error{
    f.parseHead, f.parseMaxp, f.parseHhea, f.parseLoca, f.parseCmap,
} {
    if err := step(); err != nil {
        return err
    }
}
```

### head — the font's identity card

```go
// from stb-truetype-go/sfnt.go
func (f *Font) parseHead() error {
    d, ok := f.tableData("head")
    if !ok || len(d) < 54 {
        return errors.New("truetype: bad head table")
    }
    f.unitsPerEm = binary.BigEndian.Uint16(d[18:])
    if f.unitsPerEm == 0 {
        f.unitsPerEm = 1000
    }
    f.indexToLoc = i16(d[50:])
    return nil
}
```

`unitsPerEm` is the coordinate grid size — typically 1000 or 2048. Every glyph coordinate is
expressed in these units; the rasterizer divides by `unitsPerEm` to get the pixel scale.
`indexToLoc` says whether `loca` entries are 16-bit (short) or 32-bit (long).

### maxp — glyph count

```go
// from stb-truetype-go/sfnt.go
func (f *Font) parseMaxp() error {
    d, ok := f.tableData("maxp")
    if !ok || len(d) < 6 {
        return errors.New("truetype: bad maxp table")
    }
    f.numGlyphs = binary.BigEndian.Uint16(d[4:])
    return nil
}
```

`numGlyphs` is used later when building the `loca` array — we need to allocate exactly
`numGlyphs + 1` entries.

### loca — the glyph offset index

```go
// from stb-truetype-go/sfnt.go
func (f *Font) parseLoca() error {
    d, ok := f.tableData("loca")
    if !ok {
        return errors.New("truetype: bad loca table")
    }
    n := int(f.numGlyphs) + 1
    f.loca = make([]uint32, n)
    if f.indexToLoc == 0 { // short format: uint16 offsets, doubled
        if len(d) < n*2 {
            return errors.New("truetype: loca too short (short format)")
        }
        for i := 0; i < n; i++ {
            f.loca[i] = uint32(binary.BigEndian.Uint16(d[i*2:])) * 2
        }
        return nil
    }
    if len(d) < n*4 { // long format: uint32 offsets
        return errors.New("truetype: loca too short (long format)")
    }
    for i := 0; i < n; i++ {
        f.loca[i] = binary.BigEndian.Uint32(d[i*4:])
    }
    return nil
}
```

The short format stores offsets divided by two (to fit in 16 bits), so the parser multiplies
back. Both formats are length-checked before the loop. Later, `glyphContours` reads
`f.loca[gid]` and `f.loca[gid+1]` to find the byte range of a glyph inside `glyf`; the
`+1` entry is what makes this fence-post indexing safe.

!!! note "Try it"
    Run the full test suite for stb-truetype-go:

    ```bash
    cd /path/to/safeheaders-go
    go test ./stb-truetype-go/...
    ```

    Expected outcome: all tests pass, including the table-parsing tests that verify that a
    truncated or zero-byte input returns an error and never panics. You can also run with
    the race detector to confirm the `Font` struct's concurrent-read safety:

    ```bash
    go test -race ./stb-truetype-go/...
    ```

---

## The Font struct: what parseSFNT produces

After all five sub-parsers succeed, the `Font` struct in [`stb-truetype-go/stb_truetype.go`](src/stb-truetype-go-stb-truetype-go.md)
holds everything the rasterizer needs:

```go
// from stb-truetype-go/stb_truetype.go
type Font struct {
    rawData     []byte
    tables      map[string]tableRec // sfnt table directory
    loca        []uint32            // glyph offsets into glyf (numGlyphs+1)
    cmapData    []byte              // selected cmap subtable
    unitsPerEm  uint16
    indexToLoc  int16
    numGlyphs   uint16
    numHMetrics uint16
}
```

All fields are unexported. The struct is immutable after `parseSFNT` returns: nothing modifies
it at runtime. This is what makes `Font` safe for concurrent use from multiple goroutines —
the `GlyphCache` takes a read lock on the cache map but never on `Font` itself.

### Loading a font

```go
// from stb-truetype-go/stb_truetype.go
func LoadFontFromBytes(data []byte) (*Font, error) {
    copiedData := make([]byte, len(data))
    copy(copiedData, data)
    font := &Font{rawData: copiedData}
    if err := font.parseSFNT(); err != nil {
        return nil, err
    }
    return font, nil
}
```

The copy is deliberate: the caller's slice might be a memory-mapped file or a slice into a
larger buffer. Owning our own copy means `f.rawData` cannot be mutated behind our back after
load. `tableData` slices into this copy, so every returned `[]byte` is a stable sub-slice of
`rawData`.

!!! tip "Fuzzing table parsing"
    The sfnt parsing path is a good fuzzing target because it processes untrusted binary data.
    Run a short fuzz session:

    ```bash
    go test -fuzz=FuzzParse -fuzztime=30s ./stb-truetype-go/...
    ```

    If the project has `testdata/fuzz/` seeds, the fuzzer starts from those; otherwise it
    generates random inputs. Any panic is a bug — the expected outcome is only clean errors.

---

## How tableData feeds the rest of the pipeline

After `parseSFNT`, every downstream function calls `tableData` by name. For example,
`glyphContours` in [`sfnt.go`](src/stb-truetype-go-sfnt-go.md) retrieves the `glyf` table:

```go
// from stb-truetype-go/sfnt.go (inside glyphContours)
glyf, ok := f.tableData("glyf")
if !ok || int(end) > len(glyf) || start > end {
    return nil, errors.New("truetype: glyph data out of range")
}
g := glyf[start:end]
```

Notice the second layer of bounds checking: even after `tableData` validates the table's
outer bounds, the individual glyph's `start`/`end` offsets (from `loca`) are checked against
the `glyf` slice. Defense in depth: the outer check catches bad table records; the inner
check catches bad `loca` entries.

This pattern — validate once at the gate, then again at the point of use — is the repeating
motif across the whole rasterizer. The next lesson ([Lesson 09](09-truetype-2-glyph-outlines.md))
follows the same bytes one level deeper, into `parseSimpleGlyph` and the composite glyph
expander with its billion-laughs budget.

---

## Key takeaways

- A `.ttf` file is an sfnt container: a 12-byte header, a table directory, and named data
  chunks. Understanding this structure is the prerequisite for everything else the rasterizer does.
- `parseSFNT` validates the version tag and checks `off+16 > len(d)` on every directory entry
  before slicing — truncated files get a clean error, never a panic.
- `tableData` is the single place where offset + length are bounds-checked against `f.rawData`.
  Every table parser calls it first; no parser indexes `rawData` directly.
- The `Font` struct is immutable after load and is safe for concurrent reads without locking.
- Defense in depth: bounds are checked at the directory level (in `tableData`) AND again at
  point of use (e.g. `loca` entry vs. `glyf` length). One check failing is a bug; two
  failing is a catastrophe the second check still blocks.
