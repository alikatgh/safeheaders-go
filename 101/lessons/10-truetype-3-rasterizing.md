# 10 - TrueType III · Rasterizing the Outline

> **Objectives:** Understand how a vector glyph outline — a list of curve
> control points — becomes a pixel bitmap. Follow the pipeline from quadratic
> Bézier flattening, through coordinate scaling, to supersampled scanline fill.
> See exactly where anti-aliasing comes from and how the nonzero winding rule
> decides what is "inside" a glyph.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Font units vs pixels** = "a blueprint drawn in centimetres, then shrunk to fit a stamp." Glyph coordinates live in an abstract grid (e.g. 2048 units tall); `scale = size / unitsPerEm` is the single multiplier that converts every coordinate into real screen pixels before anything is drawn.
- **On-curve and off-curve points** = "a kite string held by anchor pegs, pulled sideways by invisible handles." On-curve points are the solid corners; off-curve points are the control handles that bow the outline into a quadratic Bézier curve. Flattening samples each Bézier at 10 evenly-spaced `t` values and replaces the curve with a chain of straight segments.
- **Scanline fill** = "dragging a horizontal ruler across the shape and painting between every pair of edge-hits." For each supersampled row `sy`, `scanlineCrossings` finds where the polygon edges cross `y = sy + 0.5`, sorts those x positions, and `accumulateSpans` increments the coverage counter for every sub-pixel between a matched pair.
- **Nonzero winding** = "counting which way the fences cross the road: right-going adds one, left-going subtracts one — you're inside if the total isn't zero." The `winding` counter in `accumulateSpans` handles overlapping contours (like the counter of "P") correctly: only spans where `winding ≠ 0` are filled.
- **Supersampling (4×)** = "asking the question sixteen times per tile and averaging the votes." Each output pixel maps to a 4 × 4 block of sub-pixels (`ssaa = 4`); the coverage count — how many of the 16 sub-pixels land inside a nonzero-winding span — is divided by 16 to produce a natural anti-aliasing weight with no extra math.

**Why it matters:** Every `<canvas>` rendering call, every PDF word, every
terminal glyph goes through exactly these steps. Knowing them helps you predict
quality, debug blurry rendering, and understand why a 4096-pixel size limit is a
safety constraint, not an arbitrary number.

**See it — scanline fill, then supersample.** Each horizontal scanline finds where
it crosses the outline's edges; the span between a matching pair is "inside". To
anti-alias, each pixel is sampled on a 4×4 grid and the inside-fraction becomes its
grey level.

<svg viewBox="0 0 720 340" role="img" aria-labelledby="ras-t ras-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="ras-t">Scanline fill and 4x supersampling</title>
  <desc id="ras-d">A scanline crosses the glyph outline at two edges; the span between is filled. Each pixel is sampled 16 times and the inside-fraction sets its grey level.</desc>
  <g stroke="var(--md-default-fg-color--lightest)" stroke-width="1">
    <line x1="60" y1="60" x2="60" y2="280"/><line x1="100" y1="60" x2="100" y2="280"/><line x1="140" y1="60" x2="140" y2="280"/><line x1="180" y1="60" x2="180" y2="280"/><line x1="220" y1="60" x2="220" y2="280"/><line x1="260" y1="60" x2="260" y2="280"/><line x1="300" y1="60" x2="300" y2="280"/><line x1="340" y1="60" x2="340" y2="280"/>
    <line x1="60" y1="60" x2="340" y2="60"/><line x1="60" y1="100" x2="340" y2="100"/><line x1="60" y1="140" x2="340" y2="140"/><line x1="60" y1="180" x2="340" y2="180"/><line x1="60" y1="220" x2="340" y2="220"/><line x1="60" y1="260" x2="340" y2="260"/>
  </g>
  <path d="M120,260 L240,80 L330,260 Z" fill="none" stroke="currentColor" stroke-width="1.6"/>
  <line x1="70" y1="190" x2="350" y2="190" stroke="var(--md-default-fg-color--light)" stroke-width="1.2" stroke-dasharray="5 4"/>
  <line x1="167" y1="190" x2="295" y2="190" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="7" opacity="0.4"/>
  <circle cx="167" cy="190" r="4" fill="var(--md-accent-fg-color,#00897b)"/><circle cx="295" cy="190" r="4" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="160" y="178" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">+1</text>
  <text x="302" y="178" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">−1</text>
  <text x="231" y="300" text-anchor="middle" font-size="10.5" fill="var(--md-default-fg-color--light)">scanline → edge crossings → fill the span</text>
  <text x="545" y="52" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">1 pixel = 4×4 = 16 samples</text>
  <g fill="var(--md-accent-fg-color,#00897b)" opacity="0.4">
    <rect x="545" y="107.5" width="37.5" height="37.5"/><rect x="582.5" y="107.5" width="37.5" height="37.5"/>
    <rect x="470" y="145" width="37.5" height="37.5"/><rect x="507.5" y="145" width="37.5" height="37.5"/><rect x="545" y="145" width="37.5" height="37.5"/><rect x="582.5" y="145" width="37.5" height="37.5"/>
    <rect x="470" y="182.5" width="37.5" height="37.5"/><rect x="507.5" y="182.5" width="37.5" height="37.5"/><rect x="545" y="182.5" width="37.5" height="37.5"/><rect x="582.5" y="182.5" width="37.5" height="37.5"/>
  </g>
  <g stroke="var(--md-default-fg-color--lighter)" stroke-width="1">
    <line x1="507.5" y1="70" x2="507.5" y2="220"/><line x1="545" y1="70" x2="545" y2="220"/><line x1="582.5" y1="70" x2="582.5" y2="220"/>
    <line x1="470" y1="107.5" x2="620" y2="107.5"/><line x1="470" y1="145" x2="620" y2="145"/><line x1="470" y1="182.5" x2="620" y2="182.5"/>
  </g>
  <rect x="470" y="70" width="150" height="150" fill="none" stroke="currentColor" stroke-width="1.6"/>
  <line x1="470" y1="150" x2="620" y2="90" stroke="currentColor" stroke-width="1.8"/>
  <text x="545" y="246" text-anchor="middle" font-size="11.5" fill="currentColor">10 of 16 inside → 62% coverage</text>
  <text x="500" y="270" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">grey level →</text>
  <rect x="546" y="258" width="40" height="16" fill="currentColor" opacity="0.62" stroke="var(--md-default-fg-color--lightest)"/>
  <text x="360" y="324" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">nonzero winding: +1 crossing a left→right edge, −1 right→left — inside where the counter ≠ 0</text>
</svg>

---

## The scale factor: font-units → pixels

The code in [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md) computes `scale` once and reuses it
everywhere (this code lives in a **package** — in plain terms, a named folder of Go source files that can be reused elsewhere by **importing** it, i.e. declaring "I want to use the code from that folder"):

```go
// from stb-truetype-go/sfnt.go — rasterizeGlyph
scale := size / float64(f.unitsPerEm)
metrics := GlyphMetrics{
    AdvanceWidth: int(math.Round(float64(f.advanceWidth(gid)) * scale)),
    Scale:        scale,
}
```

**In plain terms:** this computes one multiplier (`scale`) from the font's size and its internal unit grid, then packages up the glyph's measurements — how wide it is, and that same multiplier — into a labeled bundle called `GlyphMetrics` (in plain terms: a **struct**, a bundle of named values grouped under one name; each named value inside, like `AdvanceWidth`, is called a **field**).

`f.unitsPerEm` was read from the `head` table during parsing (see [Lesson 8](08-truetype-1-sfnt-tables.md)).
A font with `unitsPerEm = 1000` rendered at 20 px uses `scale = 0.02`.
Every glyph coordinate is then multiplied by `scale` before it touches a pixel.

The bounding box (the pixel rectangle that will hold the glyph) is derived from
the outline's extremes, also through `scale`:

```go
// from stb-truetype-go/sfnt.go — rasterizeGlyph (continued)
px0 := int(math.Floor(minX * scale))
px1 := int(math.Ceil(maxX * scale))
py0 := int(math.Floor(-maxY * scale)) // y is up in font space, down in device space
py1 := int(math.Ceil(-minY * scale))
w, h := px1-px0, py1-py0
if w > 4096 || h > 4096 {
    return nil, metrics, errors.New("truetype: rasterized glyph too large")
}
```

**In plain terms:** this rounds the glyph's scaled corner coordinates outward to whole pixels to get the box's edges, computes its width and height, and if either is bigger than 4096 pixels it stops and hands back an error instead of the glyph (`return` here means: the function immediately finishes and sends these values back to whoever called it, rather than continuing on to draw anything).

The `4096` ceiling is a safety guard: a pathological or malicious font file
might claim a glyph that spans a million font units; without this check, the
coverage array allocation would be enormous (in plain terms: the program would try to **reserve** — set aside — a huge chunk of the computer's working memory just to hold one glyph's coverage data, which could slow or crash the program).

---

## Step 1: Flatten quadratic Béziers into polygons

TrueType outlines mix two kinds of points: *on-curve* (solid corners or line
endpoints) and *off-curve* (control handles). Two consecutive off-curve points
imply a hidden on-curve midpoint between them — the spec calls these "implied"
points.

`withImpliedPoints` inserts those midpoints, then `flattenContour` walks the
expanded sequence and emits straight-line segments from each Bézier. (`withImpliedPoints` and `flattenContour` are **functions** — named, reusable chunks of code that do one job; writing their name followed by parentheses, like `withImpliedPoints(pts)`, is called **calling** or **invoking** the function, which simply means: run it, with the values in the parentheses handed to it as input.)

```go
// from stb-truetype-go/sfnt.go
func withImpliedPoints(pts []glyphPoint) []glyphPoint {
    n := len(pts)
    seq := make([]glyphPoint, 0, n*2)
    for i := 0; i < n; i++ {
        p := pts[i]
        seq = append(seq, p)
        nxt := pts[(i+1)%n]
        if !p.onCurve && !nxt.onCurve {
            seq = append(seq, glyphPoint{
                x: (p.x + nxt.x) / 2,
                y: (p.y + nxt.y) / 2,
                onCurve: true,
            })
        }
    }
    return seq
}
```

**In plain terms:** `pts` is a **slice** — an ordinable, resizable list of items, here a list of glyph points — and the function walks through it one point at a time (`for i := 0; i < n; i++` is a loop: it repeats the same steps, once per point, until it's covered them all). Each original point is kept; and whenever two off-curve points sit next to each other, a new on-curve point is inserted exactly halfway between them (`glyphPoint{...}` builds one point value with its `x`, `y`, and `onCurve` fields filled in; `append` adds it onto the growing list `seq`). Once every point has been checked, the function returns — finishes and hands back — the new, longer list.

Each on-curve-to-off-curve-to-on-curve triplet becomes a quadratic Bézier.
`flattenQuad` samples it at 10 evenly-spaced values of `t`:

```go
// from stb-truetype-go/sfnt.go
func flattenQuad(out *[]fpoint, p0, p1, p2 fpoint) {
    const steps = 10
    for s := 1; s <= steps; s++ {
        t := float64(s) / steps
        mt := 1 - t
        *out = append(*out, fpoint{
            x: mt*mt*p0.x + 2*mt*t*p1.x + t*t*p2.x,
            y: mt*mt*p0.y + 2*mt*t*p1.y + t*t*p2.y,
        })
    }
}
```

**In plain terms:** this walks the curve in 10 small steps from one end to the other, and at each step computes a point that lies on the curve, adding it onto the output list `out` so the smooth curve becomes a chain of straight-line points. (`out` is a **pointer** — instead of the function receiving a plain copy of the list, it receives a note saying "here is where the real list lives in memory," so any changes it makes are visible back where the function was called from, without needing to separately return the list.)

The quadratic Bézier formula `B(t) = (1-t)²P0 + 2(1-t)tP1 + t²P2` in code form,
evaluated once per step. The result is a `[]fpoint` polygon — the glyph contour
is now ordinary geometry.

---

## Step 2: Build edges in supersampled device space

Once all contours are flattened, `buildEdges` converts them from font-unit
coordinates to *supersampled device pixels*. The supersampling factor is `ssaa = 4`:

```go
// from stb-truetype-go/sfnt.go
const ssaa = 4

func buildEdges(polys [][]fpoint, scale float64, px0, py0 int) []gEdge {
    toDev := func(p fpoint) (float64, float64) {
        return (p.x*scale - float64(px0)) * ssaa,
               (-p.y*scale - float64(py0)) * ssaa
    }
    var edges []gEdge
    for _, poly := range polys {
        for i := 0; i < len(poly); i++ {
            ax, ay := toDev(poly[i])
            bx, by := toDev(poly[(i+1)%len(poly)])
            edges = append(edges, gEdge{ax, ay, bx, by})
        }
    }
    return edges
}
```

**In plain terms:** `polys` is a list of polygons (each itself a list of points — a "slice of slices"). `toDev` is a small, throwaway function defined right here that converts one font-unit point into a supersampled screen-pixel point. The outer loop goes polygon by polygon; the inner loop walks each polygon's points two at a time (a point and the one right after it) to describe one edge of the shape, and appends that edge (`gEdge`, a struct holding both endpoints) onto the growing `edges` list, which is returned once every polygon has been processed.

The minus sign on `y` flips from font space (y-up) to screen space (y-down).
Multiplying by `ssaa` expands the coordinate space: a 16 × 20 pixel bitmap
becomes a 64 × 80 supersampled grid. Every polygon edge gets one `gEdge` entry.

---

## Step 3: Scanline crossings

`fillCoverage` is the outer loop. For each supersampled scanline `sy` it calls
`scanlineCrossings` to find where the edges pierce the horizontal ray at
`y = sy + 0.5` (the centre of the sub-row):

```go
// from stb-truetype-go/sfnt.go
func scanlineCrossings(edges []gEdge, yc float64, xs []xCrossing) []xCrossing {
    for _, e := range edges {
        lo, hi, dir := e.y0, e.y1, 1
        if lo > hi {
            lo, hi, dir = hi, lo, -1
        }
        if yc < lo || yc >= hi {
            continue
        }
        t := (yc - e.y0) / (e.y1 - e.y0)
        xs = append(xs, xCrossing{e.x0 + t*(e.x1-e.x0), dir})
    }
    return xs
}
```

**In plain terms:** for every edge of the glyph's outline, this checks whether the current horizontal line (`yc`) actually passes through that edge's vertical span; if it doesn't, `continue` skips straight to the next edge without doing anything else. If it does pass through, the code works out exactly where (`t` is "how far along the edge, as a fraction, the line crosses it") and records that x position plus a direction, onto the `xs` list, which is handed back once every edge has been checked.

Each crossing records the x position and a `dir` (+1 or −1) based on whether
the edge travels downward or upward. This direction is the raw material for the
nonzero winding rule.

---

## Step 4: Nonzero winding fill

After sorting crossings by x, `accumulateSpans` walks them left to right,
maintaining a running `winding` counter:

```go
// from stb-truetype-go/sfnt.go
func accumulateSpans(coverage []uint32, xs []xCrossing, py, w, sw int) {
    winding := 0
    rowBase := py * w
    for i := 0; i+1 < len(xs); i++ {
        winding += xs[i].dir
        if winding == 0 {
            continue
        }
        lo := int(math.Ceil(xs[i].x - 0.5))
        hi := int(math.Floor(xs[i+1].x - 0.5))
        // clamp to supersampled row width
        if lo < 0 { lo = 0 }
        if hi >= sw { hi = sw - 1 }
        for sx := lo; sx <= hi; sx++ {
            coverage[rowBase+sx/ssaa]++
        }
    }
}
```

**In plain terms:** `coverage` is a slice — a resizable list — of counters, one per output pixel, each holding an unsigned (never-negative) whole number (`uint32`). `rowBase` finds where the current row's counters begin within that flat list, since `coverage` stores every row of the image back-to-back rather than as a grid (`py` — one form of what's often called an **index**: a position/count used to locate something within a list). The loop walks each pair of neighboring crossings; it adds the current crossing's direction into `winding`, and if the running total is exactly zero, `continue` skips ahead — nothing between here and the next crossing is "inside" the glyph. Otherwise, it works out the range of sub-pixel columns (`lo` to `hi`) between this crossing and the next, clamps that range so it can't run off either edge of the row, and increments the coverage counter for each sub-pixel's parent pixel (`sx/ssaa` maps a fine sub-pixel column back to its coarser output-pixel column).

Each supersampled sub-pixel that lands inside a nonzero-winding span increments
the `coverage` counter for its parent output pixel (`sx/ssaa`). After all 4 × 4
sub-rows, the maximum possible count is `ssaa * ssaa = 16`.

---

## Step 5: Coverage → pixel intensity

Back in `rasterizeGlyph`, the coverage counts are converted to 8-bit grey values
(an "8-bit value" is a number built from 8 **bits** — the smallest unit of computer information, each either a 0 or a 1 — which together can represent whole numbers from 0 to 255; a **byte** is exactly 8 bits, so this is one byte per pixel):

```go
// from stb-truetype-go/sfnt.go
const maxCov = ssaa * ssaa  // = 16
for i, c := range coverage {
    if v := c * 255 / maxCov; v >= 255 {
        img.Pix[i] = 255
    } else {
        img.Pix[i] = uint8(v)
    }
}
```

**In plain terms:** this walks every pixel's coverage count and rescales it from the 0–16 range onto the 0–255 grey-level range that an image actually stores, writing the result into `img.Pix` (the image's raw pixel bytes) at the matching position `i`.

A pixel fully inside the glyph (all 16 sub-samples hit) gets `255`. One on the
edge with 8 sub-samples inside gets `127` — perceptually half-covered grey.
That's anti-aliasing, mechanically.

---

## The glyphBudget: stopping the billion-laughs amplification

Composite glyphs (like accented characters) reference other glyphs
**recursively** — in plain terms, a function that (directly or indirectly)
calls itself again to handle a smaller piece of the same problem, the way a
composite glyph can be built from other glyphs that are themselves composite.
A malicious font could nest K components per level, 8 levels deep — K⁸
recursive calls from a small file. The rasterizer guards this with
`glyphBudget`:

```go
// from stb-truetype-go/sfnt.go
const (
    maxGlyphComponents = 4096
    maxGlyphPoints     = 1 << 20 // 1 048 576
)

type glyphBudget struct {
    components int
    points     int
}
```

**In plain terms:** this defines two fixed limits (constants, values that never change while the program runs) and a small struct, `glyphBudget`, that carries two running counters — how many components and how many points are still "allowed" — down through each recursive call.

Both counters are decremented on every recursive call to `glyphContours`; if
either goes negative the call returns an error immediately. The depth cap (8
levels) alone is not enough — a tree 8 levels deep with 4 children each still
visits 65 536 nodes. The budget stops the expansion regardless of shape.

See [Lesson 9](09-truetype-2-glyph-outlines.md) for how composite glyphs are
assembled before rasterization.

!!! warning "Why `recover` can't save you here"
    Unbounded recursion (recursion with no limit stopping it) causes a Go
    stack overflow — a *fatal* crash, not a panic. (The **stack** is the
    region of the computer's memory that keeps track of which function called
    which, and where to resume when each one finishes; if recursion never
    stops, that record keeps growing until it runs out of room.) `defer/recover`
    is a mechanism Go code can use to catch certain errors and keep running
    instead of crashing, but it does not catch fatal crashes like a stack
    overflow. The `glyphBudget` + depth ceiling are the only lines of defence.
    This same class of bug (billion-laughs amplification) also appeared in
    tinyxml2-go's XML parser; see [Lesson 6](19-recursion-and-billion-laughs.md).

---

## Putting it all together: the pipeline in one view

```
rune  ──► glyphIndex (cmap)
           │
           ▼
      glyphContours (loca → glyf → parseSimpleGlyph / compositeContours)
           │  font units, on/off-curve points
           ▼
      flattenContour  ──► withImpliedPoints ──► flattenQuad × N
           │  font-unit polygons
           ▼
      buildEdges  (apply scale, flip y, multiply by ssaa=4)
           │  supersampled device-space edges
           ▼
      fillCoverage  (per supersampled scanline)
        ├─ scanlineCrossings  (x intercepts + winding dir)
        └─ accumulateSpans    (nonzero winding → coverage++)
           │
           ▼
      coverage[i] * 255 / 16  →  image.Gray pixel
```

---

!!! note "Try it"
    Run the full rasterizer test suite from the module root (a **test** is a small
    piece of code written specifically to check that another piece of code behaves
    correctly; running the suite executes all of them and reports pass/fail):

    ```bash
    cd stb-truetype-go && go test -v -run TestRasterize ./...
    ```

    Expected outcome: all `TestRasterize*` subtests pass, each printing the
    pixel dimensions of the rendered glyph. A zero-size result for the space
    character (`' '`) is expected — space has no contours.

    To confirm the composite-glyph budget kicks in on a crafted deep nesting:

    ```bash
    go test -v -run TestCompositeBudgetAborts ./...
    ```

    You should see a test that feeds a synthetically deep composite chain and
    expects an error containing `"budget exceeded"`.

!!! tip "Fuzz the rasterizer"
    The parser pipeline is a natural fuzzing target — **fuzzing** means
    automatically feeding a program huge amounts of random or malformed input to
    see if anything makes it crash — random bytes (a **byte** is 8 bits, the
    basic unit raw file data is measured in) as a "font file" should never crash
    the process, only return an error.

    ```bash
    cd stb-truetype-go && go test -fuzz=FuzzLoadFont -fuzztime=30s .
    ```

    Any `fatal` exit (stack overflow, nil-pointer) is a bug. A returned `error`
    is fine and expected. (A **nil pointer** is a pointer — a note saying where a
    value lives in memory — that points nowhere; trying to use it crashes the
    program.)

---

## Key takeaways

- **`scale = size / unitsPerEm`** is the single conversion factor between the
  font's abstract grid and your screen. Everything downstream multiplies by it.
- **Flattening** turns quadratic Béziers into straight-line polygons using the
  formula `B(t) = (1-t)²P0 + 2(1-t)tP1 + t²P2`, sampled at fixed steps.
- **Supersampling at `ssaa = 4`** means 16 sub-pixels per output pixel; the
  coverage count divided by 16 gives a natural anti-aliasing weight with no
  extra math.
- **Nonzero winding** is a simple counter: increment when crossing a downward
  edge, decrement upward — any nonzero result means "inside". It correctly
  handles overlapping contours like counters and compound characters.
- **`glyphBudget`** (4096 components, 1 M points) is the safety net against
  billion-laughs amplification via deeply nested composite glyphs; a depth cap
  alone is insufficient.
