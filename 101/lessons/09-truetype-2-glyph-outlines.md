# 09 · TrueType II - decoding glyph outlines

> **Objectives:** Understand how a TrueType font maps a character to a glyph
> ID (cmap), locates the raw outline bytes (loca/glyf), and encodes that outline
> as contours of on- and off-curve points with delta-compressed coordinates.
> See how composite glyphs reuse simpler ones, and why an unchecked composite
> tree is a security hazard.
> Estimated time: 25 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **cmap** = "a phone book that has four different editions." You hand it a character (say `'A'`) and it returns a small integer called a *glyph ID*; the rasterizer scores all available editions (formats 0, 4, 6, 12) and uses the best one.
- **loca / glyf** = "a table of contents paired with the actual chapters." `loca[gid]` and `loca[gid+1]` are byte offsets that bracket the exact raw outline bytes of one glyph inside the `glyf` table.
- **contour** = "a rubber-band loop stretched over a peg-board." Each loop is a list of points; on-curve points sit on the outline, off-curve points are quadratic Bézier control handles, and two consecutive off-curve points imply a hidden on-curve midpoint exactly halfway between them.
- **delta-encoded coordinates** = "a road-trip odometer that only records how far you drove each leg, not where you are." Every x and y value is the signed difference from the previous point, so the decoder must accumulate a running sum to recover absolute positions.
- **composite glyph** = "a cut-and-paste collage of simpler shapes." It lists component glyph IDs with per-component offsets and transforms, letting accented letters reuse base glyphs — but composites can reference other composites, making the call tree grow exponentially if unchecked.
- **glyphBudget** = "a shared fuel tank for the whole recursive trip." A single struct with two counters — `components` (max 4096 invocations) and `points` (max ~1 M) — is decremented at every level of the recursion and returns an error the moment either counter goes negative. See [Lesson 19](19-recursion-and-billion-laughs.md) for the full threat model.

**Why it matters:** without these bounds, a single maliciously crafted `.ttf`
file can freeze or crash any program that renders text - including your own.

**See it — on-curve, off-curve, and the implied midpoint.** A contour is a loop
of points. *On-curve* points sit on the outline; *off-curve* points are quadratic
Bézier control handles. Two off-curve points in a row imply a hidden on-curve
midpoint between them. Coordinates arrive as deltas, so the decoder keeps a running
sum.

<svg viewBox="0 0 720 340" role="img" aria-labelledby="tt-t tt-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="tt-t">Glyph contour points and quadratic Bézier curves</title>
  <desc id="tt-d">On-curve points sit on the outline, off-curve points are Bézier control handles, and two consecutive off-curve points imply an on-curve midpoint.</desc>
  <text x="195" y="40" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">one quadratic segment</text>
  <path d="M70,165 Q190,55 320,150" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2"/>
  <line x1="70" y1="165" x2="190" y2="55" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="4 3"/>
  <line x1="190" y1="55" x2="320" y2="150" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="4 3"/>
  <circle cx="70" cy="165" r="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <circle cx="320" cy="150" r="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <circle cx="190" cy="55" r="5" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="70" y="186" text-anchor="middle" font-size="10" fill="currentColor">on-curve</text>
  <text x="320" y="171" text-anchor="middle" font-size="10" fill="currentColor">on-curve</text>
  <text x="190" y="46" text-anchor="middle" font-size="10" fill="currentColor">off-curve control</text>
  <path d="M60,300 Q150,238 220,238 Q290,238 380,300" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2"/>
  <line x1="150" y1="238" x2="290" y2="238" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="4 3"/>
  <circle cx="60" cy="300" r="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <circle cx="380" cy="300" r="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <circle cx="150" cy="238" r="5" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <circle cx="290" cy="238" r="5" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <circle cx="220" cy="238" r="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/><circle cx="220" cy="238" r="1.8" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="220" y="324" text-anchor="middle" font-size="10" fill="currentColor">two off-curve in a row ⇒ implied on-curve midpoint (M)</text>
  <g font-size="11">
    <circle cx="456" cy="66" r="5" fill="var(--md-accent-fg-color,#00897b)"/><text x="472" y="70" fill="currentColor">on-curve point — on the outline</text>
    <circle cx="456" cy="96" r="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="472" y="100" fill="currentColor">off-curve — quadratic control handle</text>
    <circle cx="456" cy="126" r="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/><circle cx="456" cy="126" r="1.8" fill="var(--md-accent-fg-color,#00897b)"/><text x="472" y="130" fill="currentColor">implied on-curve midpoint</text>
    <line x1="448" y1="154" x2="466" y2="154" stroke="var(--md-default-fg-color--light)" stroke-width="1" stroke-dasharray="4 3"/><text x="472" y="158" fill="currentColor">control handle</text>
  </g>
  <rect x="440" y="190" width="262" height="116" rx="6" fill="none" stroke="var(--md-default-fg-color--lightest)"/>
  <text x="454" y="214" font-size="11" fill="currentColor">Coordinates are stored as deltas:</text>
  <text x="454" y="236" font-size="12" fill="var(--md-accent-fg-color,#00897b)" font-family="ui-monospace,monospace">x += dx ;  y += dy</text>
  <text x="454" y="266" font-size="11" fill="currentColor">glyphBudget caps total points:</text>
  <text x="454" y="288" font-size="12" fill="var(--md-accent-fg-color,#00897b)" font-family="ui-monospace,monospace">maxGlyphPoints = 1&lt;&lt;20</text>
</svg>

---

## Step 1 - the table directory and tableData

Before anything else, `parseSFNT` (from [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md)) reads the
file header and builds a map of every table's byte range:

```go
// stb-truetype-go/sfnt.go
func (f *Font) parseSFNT() error {
    d := f.rawData
    switch binary.BigEndian.Uint32(d) {
    case 0x00010000, 0x74727565: // TrueType magic
    case 0x4F54544F: // "OTTO" = OpenType/CFF - not supported
        return errors.New("truetype: OpenType/CFF fonts are not supported (no glyf table)")
    default:
        return fmt.Errorf("truetype: unrecognized sfnt version 0x%08x", ...)
    }
    numTables := int(binary.BigEndian.Uint16(d[4:]))
    f.tables = make(map[string]tableRec, numTables)
    for i := 0; i < numTables; i++ {
        off := 12 + i*16
        f.tables[string(d[off:off+4])] = tableRec{
            offset: binary.BigEndian.Uint32(d[off+8:]),
            length: binary.BigEndian.Uint32(d[off+12:]),
        }
    }
    // then parse head, maxp, hhea, loca, cmap in order
}
```

Every subsequent lookup goes through `tableData`, which always bounds-checks the
slice before returning it:

```go
// stb-truetype-go/sfnt.go
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

!!! warning "Why bounds-check every table?"
    Font files are untrusted. A corrupt or adversarial file can store a table
    offset that points outside the file. Without this guard, the next line that
    slices `f.rawData` panics. The `ok` return lets every caller fail cleanly.

---

## Step 2 - cmap: four formats, one goal

`parseCmap` scores each subtable and keeps the best one. The priority (from
`cmapScore` in [`sfnt.go`](src/stb-truetype-go-sfnt-go.md)) is:

| Score | Platform / Encoding |
|-------|---------------------|
| 5 | Windows, UCS-4 (encoding 10) or Unicode, full BMP (encodings 4/6) |
| 4 | Windows, BMP (encoding 1) |
| 3 | Unicode platform, any |
| 1 | anything else |

Then `glyphIndex` dispatches to the right decoder:

```go
// stb-truetype-go/sfnt.go
func (f *Font) glyphIndex(r rune) uint16 {
    switch binary.BigEndian.Uint16(f.cmapData) { // format field at byte 0
    case 0:  return cmapFormat0(f.cmapData, r)   // 256-entry byte array
    case 4:  return cmapFormat4(f.cmapData, r)   // BMP, sorted segment ranges
    case 6:  return cmapFormat6(f.cmapData, r)   // dense range of codepoints
    case 12: return cmapFormat12(f.cmapData, r)  // full Unicode, 32-bit groups
    }
    return 0
}
```

**Format 0** is trivial - just index a 256-byte array with the rune value.
**Format 4** is the most common for Latin fonts. It stores sorted *segments*:
each segment covers a range [start, end] and either adds a constant delta or
indexes into a glyph-ID array. Here is the core loop, condensed from `sfnt.go`:

```go
// stb-truetype-go/sfnt.go - cmapFormat4 (condensed)
func cmapFormat4(d []byte, r rune) uint16 {
    c := uint16(r)
    segX2 := int(binary.BigEndian.Uint16(d[6:]))   // 2 * segCount
    endOff    := 14
    startOff  := endOff + segX2 + 2                // +2 = reservedPad
    deltaOff  := startOff + segX2
    rangeOff  := deltaOff + segX2
    for i := 0; i < segX2/2; i++ {
        if c > binary.BigEndian.Uint16(d[endOff+i*2:]) {
            continue
        }
        start   := binary.BigEndian.Uint16(d[startOff+i*2:])
        idDelta := binary.BigEndian.Uint16(d[deltaOff+i*2:])
        idRange := binary.BigEndian.Uint16(d[rangeOff+i*2:])
        if idRange == 0 {
            return c + idDelta          // simple arithmetic mapping
        }
        gidx := rangeOff + i*2 + int(idRange) + int(c-start)*2
        g := binary.BigEndian.Uint16(d[gidx:])
        if g != 0 {
            return g + idDelta
        }
        return 0
    }
    return 0
}
```

**Format 12** handles full Unicode (including emoji, CJK extensions) using
groups of `(startChar, endChar, startGlyphID)` triples:

```go
// stb-truetype-go/sfnt.go - cmapFormat12 (condensed)
func cmapFormat12(d []byte, r rune) uint16 {
    nGroups := binary.BigEndian.Uint32(d[12:])
    for i := uint32(0); i < nGroups; i++ {
        g := 16 + int(i)*12
        startChar := binary.BigEndian.Uint32(d[g:])
        endChar   := binary.BigEndian.Uint32(d[g+4:])
        if uint32(r) >= startChar && uint32(r) <= endChar {
            gid := binary.BigEndian.Uint32(d[g+8:]) + (uint32(r) - startChar)
            return uint16(gid)
        }
    }
    return 0
}
```

---

## Step 3 - loca and glyf: finding the outline bytes

`parseLoca` (from `sfnt.go`) fills `f.loca[]`, a slice of byte offsets into the
`glyf` table. There are two sub-formats:

```go
// stb-truetype-go/sfnt.go
func (f *Font) parseLoca() error {
    n := int(f.numGlyphs) + 1
    f.loca = make([]uint32, n)
    if f.indexToLoc == 0 {  // short format: uint16 values, each doubled
        for i := 0; i < n; i++ {
            f.loca[i] = uint32(binary.BigEndian.Uint16(d[i*2:])) * 2
        }
        return nil
    }
    // long format: uint32 values, used as-is
    for i := 0; i < n; i++ {
        f.loca[i] = binary.BigEndian.Uint32(d[i*4:])
    }
    return nil
}
```

The head table's `indexToLocFormat` field (stored in `f.indexToLoc`) says which
sub-format applies. When it is 0 the values are halved to fit in a uint16, so
the parser multiplies by 2.

With those offsets in hand, `glyphContours` (from `sfnt.go`) slices out the raw
glyph bytes:

```go
// stb-truetype-go/sfnt.go
start, end := f.loca[gid], f.loca[gid+1]
if start >= end {
    return nil, nil // empty glyph (e.g. space)
}
g := glyf[start:end]
numContours := i16(g)   // first 2 bytes; negative means composite
if numContours < 0 {
    return f.compositeContours(g, depth, b)
}
return parseSimpleGlyph(g, int(numContours))
```

A negative `numContours` is the TrueType spec's signal for a composite glyph.

---

## Step 4 - simple glyph decoding

`parseSimpleGlyph` (from `sfnt.go`) decodes a glyph with `numContours >= 0`.
The layout of the glyph bytes is:

```
[numContours 2B][bBox 8B][endPtsOfContours numContours×2B]
[instructionLength 2B][instructions ...B]
[flags ...B][xCoordinates ...B][yCoordinates ...B]
```

Hinting instructions are skipped entirely (the parser just advances past them).
The interesting parts are flags and coordinates.

### Flags

Each point gets one flag byte. `readGlyphFlags` expands run-length-encoded
repeats:

```go
// stb-truetype-go/sfnt.go
fl := g[*pos]; *pos++
flags[i] = fl; i++
if fl&0x08 != 0 {        // REPEAT_FLAG set
    rep := int(g[*pos]); *pos++
    for j := 0; j < rep && i < numPts; j++ {
        flags[i] = fl; i++
    }
}
```

The important bit in each flag byte is bit 0: `1` = on-curve point,
`0` = off-curve (Bezier control handle).

### Delta-encoded coordinates

Both axes use the same scheme, handled by `readGlyphCoords`:

```go
// stb-truetype-go/sfnt.go
v := 0
for i, fl := range flags {
    switch {
    case fl&shortMask != 0:          // 1-byte magnitude; sign from sameMask
        d := int(g[*pos]); *pos++
        if fl&sameMask == 0 { d = -d }
        v += d
    case fl&sameMask == 0:           // 2-byte signed delta
        v += int(i16(g[*pos:])); *pos += 2
    // else: coordinate same as previous (delta == 0)
    }
    coords[i] = v
}
```

Three cases: 1-byte delta (with a sign bit), 2-byte delta, or repeat the
previous value (both flags clear means delta is zero - implicit in the spec).

### Implied on-curve midpoints

TrueType allows two consecutive off-curve points with no on-curve point between
them. The spec says there is an *implied* on-curve midpoint at the average of
the two. `withImpliedPoints` inserts it:

```go
// stb-truetype-go/sfnt.go
if !p.onCurve && !nxt.onCurve {
    seq = append(seq, glyphPoint{
        x: (p.x + nxt.x) / 2,
        y: (p.y + nxt.y) / 2,
        onCurve: true,
    })
}
```

Without this step the Bezier flattening would treat two control handles as a
segment, producing garbage output.

---

## Step 5 - composite glyphs and glyphBudget

A composite glyph is a list of `(flags, componentGID, dx, dy, optional-transform)`
records. `compositeContours` (from `sfnt.go`) loops over them:

```go
// stb-truetype-go/sfnt.go
for pos+4 <= len(g) {
    flags  := binary.BigEndian.Uint16(g[pos:])
    compGID := binary.BigEndian.Uint16(g[pos+2:])
    pos += 4
    tf, ok := readComponentTransform(g, &pos, flags)
    if !ok { break }
    all, err = f.appendComponent(all, compGID, depth, tf, b)
    if flags&0x0020 == 0 { break }  // no MORE_COMPONENTS
}
```

`appendComponent` calls `glyphContours` recursively, applies the 2x2 matrix
plus translation to every point, and appends the transformed contours:

```go
// stb-truetype-go/sfnt.go
for _, pt := range ct {
    np[i] = glyphPoint{
        x: tf.a*pt.x + tf.c*pt.y + tf.dx,
        y: tf.b*pt.x + tf.d*pt.y + tf.dy,
        onCurve: pt.onCurve,
    }
}
```

### The billion-laughs danger

A composite can reference other composites. Without a limit, a font with K
children per level and 8 levels of nesting triggers K^8 invocations from a
single top-level glyph request - exponential work from a tiny file.

`glyphBudget` stops it with two counters:

```go
// stb-truetype-go/sfnt.go
type glyphBudget struct {
    components int // remaining glyph invocations (call-tree nodes)
    points     int // remaining total contour points
}

const (
    maxGlyphComponents = 4096
    maxGlyphPoints     = 1 << 20 // ~1 million
)
```

Every recursive call to `glyphContours` decrements `b.components` first:

```go
// stb-truetype-go/sfnt.go
if b.components--; b.components < 0 {
    return nil, errors.New("truetype: composite glyph component budget exceeded")
}
```

And after decoding a simple glyph the point count is charged:

```go
// stb-truetype-go/sfnt.go
for _, c := range contours {
    b.points -= len(c)
}
if b.points < 0 {
    return nil, errors.New("truetype: composite glyph point budget exceeded")
}
```

A fresh budget is created per top-level render call in `rasterizeGlyph`:

```go
// stb-truetype-go/sfnt.go
budget := glyphBudget{components: maxGlyphComponents, points: maxGlyphPoints}
contours, err := f.glyphContours(gid, 0, &budget)
```

!!! note "Try it"
    Run the TrueType tests (including the security-oriented budget tests) from
    the repo root:

    ```bash
    cd stb-truetype-go && go test -v -run TestCompositeBudgetAborts ./...
    ```

    Expected outcome: `TestCompositeBudgetAborts` passes, confirming that
    composite budget violations are caught and returned as errors rather than
    hanging or panicking. `go test ./...` should also pass cleanly with no
    panics on the bundled font fixtures.

!!! tip "Also try the race detector"
    The GlyphCache in [`stb_truetype.go`](src/stb-truetype-go-stb-truetype-go.md) uses a sync.Mutex-guarded LRU. Run:

    ```bash
    cd stb-truetype-go && go test -race ./...
    ```

    No data-race reports should appear.

---

## Step 6 - from contours to pixels (a brief look ahead)

Once `glyphContours` returns the point lists, `rasterizeGlyph` (in `sfnt.go`)
takes over:

1. `flattenContour` converts quadratic Bezier arcs to straight-line polygons
   (10 steps per arc via `flattenQuad`).
2. `buildEdges` transforms the polygon to supersampled device space (`ssaa = 4`,
   so 4x supersampling per axis).
3. `fillCoverage` scan-fills with the nonzero winding rule, accumulating a
   coverage count per output pixel.
4. Coverage is scaled to 0-255 and stored in an `image.Gray`.

The rasterization pipeline is the subject of [Lesson 10](10-truetype-3-rasterizing.md).

---

## Key takeaways

- **cmap is not one format; it is four.** The rasterizer scores all available
  subtables and picks the best. Format 4 covers BMP; format 12 covers full
  Unicode including supplementary planes.
- **loca is an indirection table.** `loca[gid]` and `loca[gid+1]` bracket the
  exact bytes of a glyph in `glyf`. An empty glyph (like space) has
  `loca[gid] == loca[gid+1]` and produces no contours.
- **Coordinates are delta-encoded with three sub-cases:** 1-byte magnitude
  (with sign bit), 2-byte signed delta, or implicit zero. You must accumulate a
  running sum to get absolute coordinates.
- **Two consecutive off-curve points imply a hidden midpoint.** Missing this
  insertion produces incorrect Bezier shapes. `withImpliedPoints` inserts it
  before flattening.
- **Composite glyphs must be budget-capped.** Recursive expansion without limits
  is a billion-laughs attack vector. `glyphBudget` caps both total component
  invocations (4096) and total points (1M), and is checked at every level of the
  recursion.
