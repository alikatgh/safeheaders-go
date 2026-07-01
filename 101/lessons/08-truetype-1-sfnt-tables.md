# 08 · TrueType I - the sfnt table directory

> **Objectives:** Understand what a `.ttf` file actually contains at the binary level and how
> `stb-truetype-go` reads it safely. Learn why every offset must be validated before slicing,
> and trace the path from raw bytes to a usable `Font` struct.
> Estimated time: 20 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **sfnt container** = "a filing cabinet, not a single drawer." A `.ttf` file holds its data as a set of named tables, each located via an offset recorded in a front-of-file directory — exactly like a filing cabinet where a label on the front tells you which drawer each document lives in.
- **sfnt table directory** = "the building's room-number sign by the lobby." Each entry gives a four-byte name (e.g. `head`, `glyf`, `cmap`), a byte offset, and a length — so the parser knows exactly where every room starts and how far it extends.
- **bounds check** = "a bouncer who verifies you're not trying to walk into a wall." Before slicing `rawData`, `tableData` confirms that `offset + length` fits inside the actual file — without this check, a crafted entry pointing past the end of the file would cause a slice-out-of-range panic.
- **`head` / `maxp` / `loca`** = "the library card, the shelf count, and the index." `head` is the font's identity card (units per em, `loca` format); `maxp` records how many glyphs exist; `loca` is a flat array that maps each glyph ID to its byte range inside the `glyf` table.
- **`tableData` as single point of trust** = "one airport security gate instead of checks at every door." All four conditions — tag present, start non-negative, end &gt;= start, end within file — are enforced in this one function so every table parser can call it first and early-return without duplicating logic.

- **Why it matters:** skipping a single bounds check on an untrusted font file is a remote
  panic. Every lookup in this rasterizer goes through one gated helper — `tableData` — so the
  check is written once and used everywhere.

**See it — sfnt table directory layout and the tableData gate.**

<svg viewBox="0 0 700 340" role="img" aria-labelledby="t08 d08" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t08">sfnt table directory and tableData bounds-check gate</title>
  <desc id="d08">A block diagram showing how a .ttf file begins with a 12-byte header, followed by a table directory of 16-byte records. An arrow leads from the directory to the tableData function, which enforces four bounds-check conditions before returning a safe sub-slice of rawData.</desc>
  <defs>
    <marker id="l08-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-default-fg-color)"/>
    </marker>
  </defs>
  <rect x="20" y="20" width="200" height="300" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="120" y="14" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">
    <tspan font-weight="600">.ttf file</tspan>
  </text>
  <rect x="30" y="30" width="180" height="36" rx="4" ry="4" fill="var(--md-default-fg-color--lightest)" stroke="var(--md-default-fg-color--lighter)" stroke-width="1"/>
  <text x="120" y="52" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">12-byte header (version, numTables)</text>
  <rect x="30" y="76" width="180" height="34" rx="4" ry="4" fill="var(--md-default-fg-color--lightest)" stroke="var(--md-default-fg-color--lighter)" stroke-width="1"/>
  <text x="120" y="97" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">"head"  offset  length</text>
  <rect x="30" y="118" width="180" height="34" rx="4" ry="4" fill="var(--md-default-fg-color--lightest)" stroke="var(--md-default-fg-color--lighter)" stroke-width="1"/>
  <text x="120" y="139" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">"glyf"  offset  length</text>
  <rect x="30" y="160" width="180" height="34" rx="4" ry="4" fill="var(--md-default-fg-color--lightest)" stroke="var(--md-default-fg-color--lighter)" stroke-width="1"/>
  <text x="120" y="181" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">"loca"  offset  length</text>
  <text x="120" y="215" font-size="14" text-anchor="middle" fill="var(--md-default-fg-color--light)">· · ·</text>
  <rect x="30" y="228" width="180" height="78" rx="4" ry="4" fill="var(--md-default-fg-color--lightest)" stroke="var(--md-default-fg-color--lighter)" stroke-width="1" stroke-dasharray="4,3"/>
  <text x="120" y="265" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">rawData table bytes</text>
  <text x="120" y="282" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light)">(head, glyf, loca, cmap…)</text>
  <line x1="220" y1="140" x2="318" y2="140" stroke="var(--md-default-fg-color)" stroke-width="1.5" marker-end="url(#l08-arrow)"/>
  <text x="268" y="132" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light)">tag lookup</text>
  <rect x="320" y="80" width="200" height="180" rx="8" ry="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2"/>
  <text x="420" y="74" font-size="11" text-anchor="middle" font-weight="600" fill="var(--md-accent-fg-color,#00897b)">tableData(tag)</text>
  <text x="336" y="110" font-size="10" fill="var(--md-default-fg-color)">① tag in directory?</text>
  <text x="336" y="132" font-size="10" fill="var(--md-default-fg-color)">② start ≥ 0 ?</text>
  <text x="336" y="154" font-size="10" fill="var(--md-default-fg-color)">③ end ≥ start ?</text>
  <text x="336" y="176" font-size="10" fill="var(--md-default-fg-color)">④ end ≤ len(rawData) ?</text>
  <line x1="420" y1="197" x2="420" y2="220" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="3,3"/>
  <text x="420" y="240" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color)">all pass</text>
  <rect x="360" y="248" width="120" height="30" rx="4" ry="4" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="420" y="268" font-size="11" text-anchor="middle" fill="#fff">[]byte sub-slice, true</text>
  <line x1="520" y1="140" x2="580" y2="140" stroke="#e5484d" stroke-width="1.5" marker-end="url(#l08-arrow)"/>
  <text x="550" y="132" font-size="10" text-anchor="middle" fill="#e5484d">any fail</text>
  <rect x="582" y="118" width="96" height="44" rx="4" ry="4" fill="#e5484d" stroke="none"/>
  <text x="630" y="138" font-size="11" text-anchor="middle" fill="#fff">nil, false</text>
  <text x="630" y="154" font-size="10" text-anchor="middle" fill="#fff">(no panic)</text></svg>

---

## The sfnt container format

Every TrueType file starts with a 12-byte header (a "byte" is the smallest chunk of data a computer stores — one byte holds a number from 0 to 255; a 12-byte header is just 12 of these chunks in a row, read in a fixed order):

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

`parseSFNT` (a "function" is a named, reusable block of instructions — you can run it, i.e. "call" it, whenever you need that piece of work done, and it hands back a result) in [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md) reads this structure directly:

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

**In plain terms:** this function reads the 12-byte header, checks the file isn't
suspiciously short, then figures out how many tables the directory lists (`numTables`)
and walks through them one by one, recording each table's name, offset (how many bytes
into the file it starts), and length. Along the way, `error` is Go's built-in way of
saying "something went wrong" — instead of crashing, a function that hits a problem
can simply hand back a description of the problem instead of a real result. `return`
means the function stops running right there and hands its result (or its error) back
to whatever code asked it to run. `d[4:]` and similar bracket expressions are called
"indexing" or "slicing" — reaching into a row of bytes and pulling out the ones at
specific positions, the way you'd point to the 5th letter in a word. `f.tables` is a
"map", a lookup table that stores values under names (here, each four-letter table tag
like `"head"` is the name, and the offset/length pair is the value) — think of it as a
dictionary you can look words up in instantly rather than reading front to back.
`tableRec` is a "struct", a small bundle that groups a few related pieces of data (here,
an offset and a length) under one name, the way a form groups "name," "address," and
"phone" into one card. A "panic" is Go's word for an uncontrolled crash — the program
stops immediately instead of returning a tidy error, which is exactly what these length
checks are designed to prevent.

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

**In plain terms:** this function takes a table's four-letter name (like `"head"`) and
looks up where that table lives in the file. It first checks the name is actually in
the map (`ok` is a "boolean" — a value that is only ever true or false, used here to
mean "was it found?"). Then it computes the start and end byte positions and checks
they make sense before handing back that stretch of bytes (a "slice" — a view onto a
portion of a larger sequence of bytes, without copying them) plus a `true` meaning
"safe to use." If anything looks wrong, it hands back nothing (`nil`, meaning "no
value here") and `false`, meaning "don't trust this."

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

**In plain terms:** this builds a short list containing the five sub-parser functions
themselves (in Go, a function can be passed around like any other piece of data — this
is a list of "things to run," not their results yet), then goes through the list one
at a time (`for ... range` — "for each item in this list, do the following") and runs
(calls) each one. `err` is just the ordinary variable name programmers use for "the
error that came back, if any"; `err != nil` reads as "did something go wrong?" — if so,
this loop stops immediately and passes that same error further back up.

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

**In plain terms:** this function reads the `loca` table and turns it into a plain
list of numbers (one offset per glyph, plus one extra at the end). `make([]uint32, n)`
reserves a block of memory big enough to hold `n` numbers before filling it in — this
reserving step is what programmers call "allocating" memory, i.e. asking the computer
to set aside space to hold data. The result, `f.loca`, is what's called an "array" (or
in Go, a "slice" — a resizable list of same-type values, here unsigned 32-bit numbers)
that can be looked up by position (`f.loca[i]` means "the value at position `i`," where
positions start counting from 0 — this position is called an "index"). The two `for`
loops here just walk through each position filling in one number at a time, in one of
two possible byte formats.

The short format stores offsets divided by two (to fit in 16 bits), so the parser multiplies
back. Both formats are length-checked before the loop. Later, `glyphContours` reads
`f.loca[gid]` and `f.loca[gid+1]` to find the byte range of a glyph inside `glyf`; the
`+1` entry is what makes this fence-post indexing safe.

!!! note "Try it"
    Run the full test suite for stb-truetype-go (a "test" here is a small program that
    runs a piece of code and checks the result matches what's expected — a way of proving
    the code behaves correctly without a human re-checking it by hand each time):

    ```bash
    cd /path/to/safeheaders-go
    go test ./stb-truetype-go/...
    ```

    Expected outcome: all tests pass, including the table-parsing tests that verify that a
    truncated or zero-byte input returns an error and never panics. You can also run with
    the race detector (a checker that watches for two separate tasks running at the same
    time — see the concurrency note below — stepping on the same piece of data unsafely) to
    confirm the `Font` struct's concurrent-read safety:

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

**In plain terms:** `type Font struct { ... }` defines the shape of a "Font" bundle —
a struct is just a named group of related pieces of data (here called "fields," like
`rawData` or `numGlyphs`) that travel together as one unit, the way a form has several
labeled boxes on one page. This particular struct is the finished product of everything
`parseSFNT` figured out: the raw bytes, the table directory, the glyph offset list, and
a handful of small numbers.

All fields are unexported (in Go, a lowercase-starting name like `rawData` means "only
code inside this same package can see or touch this field directly" — outside code must
go through the package's own functions; a "package" is simply a named folder of related
Go source files bundled together). The struct is immutable after `parseSFNT` returns:
nothing modifies it at runtime (once created, none of its fields are ever changed again
— it's read-only for the rest of its life). This is what makes `Font` safe for concurrent
use from multiple goroutines (a "goroutine" is a lightweight independent task that Go can
run at the same time as other tasks; "concurrent" means several of these tasks may be
running simultaneously and touching the same data) — the `GlyphCache` takes a read lock
(a "lock," also called a mutex, is a mechanism that lets only one task at a time make
changes to shared data, so two goroutines can't corrupt it by writing at the same
moment) on the cache map but never on `Font` itself, because an object nothing ever
changes needs no such lock to read safely.

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

**In plain terms:** `LoadFontFromBytes` is the entry point a caller (whatever code
asked to run this function in the first place) uses to load a font: it makes its own
fresh copy of the incoming bytes, builds an empty `Font` bundle pointing at that copy,
then calls `parseSFNT` to fill in the rest. If parsing fails, it hands back `nil` (an
empty, "no font here" value) plus the error; if it succeeds, it hands back the
finished `Font` and no error (`nil` in the error slot means "nothing went wrong"). The
`*Font` in the function's signature means "a pointer to a Font" — instead of copying
the whole struct every time it's passed around, a pointer is a small note that says
"the real data lives over there," so multiple parts of the program can share the one
copy efficiently.

The copy is deliberate: the caller's slice might be a memory-mapped file or a slice into a
larger buffer. Owning our own copy means `f.rawData` cannot be mutated behind our back after
load. `tableData` slices into this copy, so every returned `[]byte` is a stable sub-slice of
`rawData`.

!!! tip "Fuzzing table parsing"
    The sfnt parsing path is a good fuzzing target (fuzzing means bombarding a function
    with huge amounts of random or semi-random input to see whether any input makes it
    crash — a way of hunting for edge cases no human would think to write by hand) because
    it processes untrusted binary data. Run a short fuzz session:

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
