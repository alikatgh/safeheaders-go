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

- **Font units vs pixels.** Glyphs are drawn in an abstract grid — maybe 2048
  units tall. Your screen thinks in pixels. "Rendering at 16 px" means: multiply
  every coordinate by `16 / 2048 ≈ 0.0078`. That fraction is called `scale`.
- **On-curve and off-curve points.** A TrueType contour is not a list of line
  endpoints; most points are *off-curve* control handles that pull the outline
  into a curve (quadratic Bézier). "Flattening" means sampling enough points
  along each curve to approximate it with a thin polygon.
- **Scanline fill.** Once you have a polygon, find where each horizontal row
  of pixels crosses the polygon's edges. Everything between a matching pair of
  crossings is "inside" and gets painted.
- **Nonzero winding.** When two contours overlap (like the counters of a letter
  "P"), you need a rule to decide which regions are solid. The nonzero winding
  rule tracks a running counter: crossing a left-to-right edge increments it,
  right-to-left decrements it. Any pixel where the counter is nonzero is inside.
- **Supersampling (4×).** Instead of asking "is the center of this pixel inside
  the glyph?" we ask the question 16 times per pixel (4 sub-rows × 4 sub-columns)
  and average the answers. That averaging is the anti-aliasing — diagonal edges
  come out grey rather than jagged.

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
everywhere:

```go
// from stb-truetype-go/sfnt.go — rasterizeGlyph
scale := size / float64(f.unitsPerEm)
metrics := GlyphMetrics{
    AdvanceWidth: int(math.Round(float64(f.advanceWidth(gid)) * scale)),
    Scale:        scale,
}
```

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

The `4096` ceiling is a safety guard: a pathological or malicious font file
might claim a glyph that spans a million font units; without this check, the
coverage array allocation would be enormous.

---

## Step 1: Flatten quadratic Béziers into polygons

TrueType outlines mix two kinds of points: *on-curve* (solid corners or line
endpoints) and *off-curve* (control handles). Two consecutive off-curve points
imply a hidden on-curve midpoint between them — the spec calls these "implied"
points.

`withImpliedPoints` inserts those midpoints, then `flattenContour` walks the
expanded sequence and emits straight-line segments from each Bézier:

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

Each supersampled sub-pixel that lands inside a nonzero-winding span increments
the `coverage` counter for its parent output pixel (`sx/ssaa`). After all 4 × 4
sub-rows, the maximum possible count is `ssaa * ssaa = 16`.

---

## Step 5: Coverage → pixel intensity

Back in `rasterizeGlyph`, the coverage counts are converted to 8-bit grey values:

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

A pixel fully inside the glyph (all 16 sub-samples hit) gets `255`. One on the
edge with 8 sub-samples inside gets `127` — perceptually half-covered grey.
That's anti-aliasing, mechanically.

---

## The glyphBudget: stopping the billion-laughs amplification

Composite glyphs (like accented characters) reference other glyphs recursively.
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

Both counters are decremented on every recursive call to `glyphContours`; if
either goes negative the call returns an error immediately. The depth cap (8
levels) alone is not enough — a tree 8 levels deep with 4 children each still
visits 65 536 nodes. The budget stops the expansion regardless of shape.

See [Lesson 9](09-truetype-2-glyph-outlines.md) for how composite glyphs are
assembled before rasterization.

!!! warning "Why `recover` can't save you here"
    Unbounded recursion causes a Go stack overflow — a *fatal* crash, not a
    panic. `defer/recover` does not catch fatal crashes. The `glyphBudget` + depth
    ceiling are the only lines of defence. This same class of bug (billion-laughs
    amplification) also appeared in tinyxml2-go's XML parser; see
    [Lesson 6](19-recursion-and-billion-laughs.md).

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
    Run the full rasterizer test suite from the module root:

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
    The parser pipeline is a natural fuzzing target — random bytes as a "font
    file" should never crash the process, only return an error.

    ```bash
    cd stb-truetype-go && go test -fuzz=FuzzLoadFont -fuzztime=30s .
    ```

    Any `fatal` exit (stack overflow, nil-pointer) is a bug. A returned `error`
    is fine and expected.

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
