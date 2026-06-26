# 19 - Recursion: stack overflow and billion-laughs

> **Objectives:** Understand why unbounded recursion causes a fatal stack overflow
> that `recover()` cannot catch, and how this repo defends against it with an
> absolute depth ceiling. Learn how a TrueType composite glyph can amplify a tiny
> input into billions of operations (billion-laughs), and how a shared budget counter
> stops the explosion before it starts.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **Recursion** is when a function calls itself. Parsing a tree naturally looks
  like this: "to parse a node, parse each of its children — which are also nodes."
- **The call stack** is a fixed-size chunk of memory the OS gives each goroutine.
  Every function call pushes a frame onto it. Nest deep enough and you fall off the
  edge.
- **A stack overflow is fatal.** Go prints a trace and kills the process. Unlike
  a panic, a `runtime.Stack` overflow cannot be caught with `recover()`. There is
  no way to handle it after the fact — you must prevent it.
- **Billion-laughs** is a name for a class of amplification attack first seen in
  XML entity expansion. In font land: a composite glyph that references K child
  glyphs, each of which references K more, 8 levels deep, produces K^8 work from
  a file that is only a few KB.
- **A budget counter is the right guard.** A depth ceiling stops linear blowup;
  a shared budget counter stops exponential fan-out even when the depth is legal.

**Why it matters:** both attacks are triggered by a single call to your parser with
a crafted input file. They require no authentication, no network, no memory
corruption — just a malformed file dropped on your API endpoint.

**See it — exponential fan-out.** Each composite glyph references K children, each
of which references K more. Depth `d` alone is legal, but the *work* is `K^d`. A
plain depth ceiling does not help once the depth is within bounds — only a shared
budget counter (`glyphBudget`), decremented across the whole expansion, stops it.

<svg viewBox="0 0 720 330" role="img" aria-labelledby="bl-t bl-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:680px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="bl-t">Billion-laughs composite glyph fan-out</title>
  <desc id="bl-d">One glyph references K children, each referencing K more, producing K to the power d work; a shared budget counter aborts the expansion.</desc>
  <defs><marker id="bl-ah" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto"><path d="M0,0 L6,3 L0,6 Z" fill="var(--md-default-fg-color--light)"/></marker></defs>
  <rect x="332" y="26" width="56" height="26" rx="6" fill="var(--md-accent-fg-color,#00897b)"/><text x="360" y="44" text-anchor="middle" font-size="12" fill="#fff">glyph</text>
  <text x="410" y="44" font-size="11" fill="var(--md-default-fg-color--light)">depth 0 — 1 glyph</text>
  <g stroke="var(--md-default-fg-color--light)" stroke-width="1.2" fill="none">
    <path d="M360,52 L175,84" marker-end="url(#bl-ah)"/><path d="M360,52 L360,84" marker-end="url(#bl-ah)"/><path d="M360,52 L545,84" marker-end="url(#bl-ah)"/>
  </g>
  <g fill="var(--md-accent-fg-color,#00897b)"><rect x="150" y="86" width="50" height="22" rx="5"/><rect x="335" y="86" width="50" height="22" rx="5"/><rect x="520" y="86" width="50" height="22" rx="5"/></g>
  <text x="600" y="102" font-size="11" fill="var(--md-default-fg-color--light)">depth 1 — K</text>
  <g stroke="var(--md-default-fg-color--lighter)" stroke-width="1" fill="none">
    <path d="M175,108 L120,150"/><path d="M175,108 L175,150"/><path d="M175,108 L230,150"/>
    <path d="M360,108 L305,150"/><path d="M360,108 L360,150"/><path d="M360,108 L415,150"/>
    <path d="M545,108 L490,150"/><path d="M545,108 L545,150"/><path d="M545,108 L600,150"/>
  </g>
  <g fill="var(--md-accent-fg-color,#00897b)">
    <circle cx="120" cy="158" r="8"/><circle cx="175" cy="158" r="8"/><circle cx="230" cy="158" r="8"/>
    <circle cx="305" cy="158" r="8"/><circle cx="360" cy="158" r="8"/><circle cx="415" cy="158" r="8"/>
    <circle cx="490" cy="158" r="8"/><circle cx="545" cy="158" r="8"/><circle cx="600" cy="158" r="8"/>
  </g>
  <text x="652" y="162" font-size="11" fill="var(--md-default-fg-color--light)">K²</text>
  <line x1="30" y1="196" x2="690" y2="196" stroke="#e5484d" stroke-width="2" stroke-dasharray="7 5"/>
  <text x="360" y="190" text-anchor="middle" font-size="11.5" fill="#e5484d" font-weight="600">glyphBudget exhausted → abort  (maxGlyphComponents = 4096, maxGlyphPoints = 1&lt;&lt;20)</text>
  <g fill="var(--md-default-fg-color--lightest)"><circle cx="110" cy="224" r="5"/><circle cx="150" cy="224" r="5"/><circle cx="190" cy="224" r="5"/><circle cx="230" cy="224" r="5"/><circle cx="290" cy="224" r="5"/><circle cx="330" cy="224" r="5"/><circle cx="370" cy="224" r="5"/><circle cx="410" cy="224" r="5"/><circle cx="470" cy="224" r="5"/><circle cx="510" cy="224" r="5"/><circle cx="550" cy="224" r="5"/><circle cx="600" cy="224" r="5"/></g>
  <text x="360" y="250" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">depth d (never reached) — Kᵈ nodes</text>
  <text x="360" y="288" text-anchor="middle" font-size="13" fill="currentColor">K = 8, depth = 8  →  8⁸ = 16,777,216 components from a few-KB file</text>
  <text x="360" y="310" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">a depth ceiling alone allows this; the shared budget counter is what stops it</text>
</svg>

---

## Part 1 — XML and the fatal stack overflow

### How the recursive parser works

[`tinyxml2-go/tinyxml2.go`](src/tinyxml2-go-tinyxml2-go.md) builds an XML DOM by calling `parseElement` once per
node. When it reads a child start-tag, it recurses:

```go
// from tinyxml2-go/tinyxml2.go
func parseElement(dec *xml.Decoder, se xml.StartElement, depth int) (*Node, error) {
    if depth > maxNestingDepth {
        return nil, fmt.Errorf("XML nesting exceeds maximum depth %d", maxNestingDepth)
    }
    // ...
    case xml.StartElement:
        child, err := parseElement(dec, v, depth+1)
        // ...
}
```

Each call to `parseElement` lives on the goroutine stack until its matching
`EndElement` is read. With deeply nested XML (`<a><b><c>...`), this grows linearly
with nesting depth. Go starts goroutines with a small stack (a few KB) and grows
it dynamically — but growth has a ceiling, and past that the runtime terminates
the program with no opportunity to recover.

### The absolute ceiling

The constant `maxNestingDepth` is the safety valve:

```go
// from tinyxml2-go/tinyxml2.go
const maxNestingDepth = 10000
```

It lives right above `parseElement` and is checked as the very first thing on
every recursive call. The comment in the source is explicit:

> Going far past it would overflow the goroutine stack — a fatal error
> `recover()` cannot catch — so the parser returns an error instead.

10 000 is far higher than any real XML document needs; it is well below the point
where Go would crash.

### Why `recover()` cannot save you

```go
// This does NOT work for a stack overflow:
func safeParseElement(...) (n *Node, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("caught: %v", r)
        }
    }()
    return parseElement(...)
}
```

`recover()` catches panics — explicit `panic(...)` calls or nil-pointer
dereferences. A stack overflow is handled by the Go runtime itself, before
`recover` ever runs. The program is already dead when it would fire.

!!! warning "recover() and stack overflows"
    `defer`/`recover` is not a substitute for a depth ceiling. If you remove
    `maxNestingDepth`, no amount of `recover` calls will save a process handed a
    10 001-deep XML file.

### The config-aware variant

`ParseWithConfig` uses `parseElementLimited`, which enforces both the user-visible
`MaxNestingDepth` from the config AND the hard `maxNestingDepth` ceiling:

```go
// from tinyxml2-go/tinyxml2.go
func parseElementLimited(
    dec *xml.Decoder,
    se xml.StartElement,
    config *Config,
    depth int,
    nodeCount *int,
) (*Node, error) {
    // Hard absolute ceiling — applies even to UnlimitedConfig.
    if depth > maxNestingDepth {
        return nil, fmt.Errorf("XML nesting exceeds maximum depth %d", maxNestingDepth)
    }
    // Soft user-configurable ceiling.
    if config.MaxNestingDepth > 0 && depth > config.MaxNestingDepth {
        return nil, ErrNestingTooDeep
    }
    // ...
}
```

The two-layer design matters: the soft limit gives callers control; the hard limit
is the last-resort guarantee even when `UnlimitedConfig` is passed.

### Iterative search avoids the same problem in traversal

Searching a deep tree with recursion has the same risk. `FindDeep` and
`FindAllDeep` use an explicit stack (a Go slice) instead of the call stack:

```go
// from tinyxml2-go/tinyxml2.go
func (n *Node) FindDeep(name string) *Node {
    stack := []*Node{n}
    for len(stack) > 0 {
        cur := stack[len(stack)-1]
        stack = stack[:len(stack)-1]
        if cur.Name == name {
            return cur
        }
        for i := len(cur.Children) - 1; i >= 0; i-- {
            stack = append(stack, cur.Children[i])
        }
    }
    return nil
}
```

A heap-allocated slice can grow as large as available memory; the goroutine call
stack cannot. Converting recursion to an explicit stack is the standard pattern
when depth is unbounded.

!!! note "Try it"
    From the repo root, run the tinyxml2 tests including the nesting limit:

    ```bash
    cd tinyxml2-go && go test -v -run 'Nesting|DepthCeiling' ./...
    ```

    Expected outcome: the test generates XML nested deeper than `MaxNestingDepth`
    and expects `ErrNestingTooDeep`. It also generates XML deeper than
    `maxNestingDepth` and expects the hard-ceiling error. Both should pass (PASS).
    The process should not crash.

---

## Part 2 — TrueType composite glyphs and billion-laughs

### What a composite glyph is

A TrueType font stores outlines in a table called `glyf`. Most glyphs (letters,
numbers) are *simple* — they store contour points directly. But some glyphs are
*composite*: instead of points, they store a list of (child glyph ID, transform)
pairs. The rasterizer resolves each reference, applies the transform, and stitches
the results together.

This is legitimate and useful: many fonts store accented characters as a base
letter plus a floating accent mark, reusing the same outline twice.

### The amplification

A malicious font can use this indirection as a multiplier. Suppose each composite
glyph references K children, and each child is itself composite, for 8 levels:

```
Level 0 (top glyph): 1 invocation
Level 1:             K invocations
Level 2:             K² invocations
...
Level 8:             K⁸ invocations
```

With K=10, that is 100 000 000 invocations triggered by requesting a single
character. The font file stays tiny because each glyph definition is short; the
work explodes only during rasterization. This is exactly the billion-laughs shape:
tiny storage, exponential work.

### How the rasterizer enters this path

In [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md), `rasterizeGlyph` calls `glyphContours` for the
top-level glyph. If `numContours < 0`, the glyph is composite and
`compositeContours` is called, which calls `glyphContours` recursively for each
component:

```go
// from stb-truetype-go/sfnt.go
func (f *Font) glyphContours(gid uint16, depth int, b *glyphBudget) ([][]glyphPoint, error) {
    if depth > 8 {
        return nil, errors.New("truetype: composite glyph nesting too deep")
    }
    if b.components--; b.components < 0 {
        return nil, errors.New("truetype: composite glyph component budget exceeded")
    }
    // ...
    if numContours < 0 {
        return f.compositeContours(g, depth, b)
    }
    // simple glyph path
    contours, err := parseSimpleGlyph(g, int(numContours))
    // ...
    for _, c := range contours {
        b.points -= len(c)
    }
    if b.points < 0 {
        return nil, errors.New("truetype: composite glyph point budget exceeded")
    }
    return contours, nil
}
```

### The glyphBudget type

The depth check alone stops linear depth (e.g. a chain 100 levels deep) but does
not stop exponential fan-out (a tree that is only 8 levels deep but has K=1000
children at each level). The `glyphBudget` struct is the solution:

```go
// from stb-truetype-go/sfnt.go
type glyphBudget struct {
    components int // remaining glyph invocations (call-tree nodes)
    points     int // remaining total contour points
}

const (
    maxGlyphComponents = 4096    // total component invocations per top-level glyph
    maxGlyphPoints     = 1 << 20 // total contour points per top-level glyph (≈ 1 M)
)
```

The comment on `glyphBudget` is the clearest statement of the threat:

> The depth cap alone does not stop a malicious composite with high fan-out
> (K children per level, 8 levels ≈ K^8 invocations from a tiny file — a
> billion-laughs amplification), so a shared counter caps total components
> visited and total points produced.

`components` counts every `glyphContours` call across the whole expansion tree.
`points` counts every contour point produced. Both counters are shared across the
entire recursion via a pointer, so the budget is global to the top-level glyph
request.

### The budget is initialized once per rasterize call

```go
// from stb-truetype-go/sfnt.go
func rasterizeGlyph(f *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error) {
    // ...
    budget := glyphBudget{components: maxGlyphComponents, points: maxGlyphPoints}
    contours, err := f.glyphContours(gid, 0, &budget)
    // ...
}
```

One pointer, threaded through the entire call tree, ensures every recursive
invocation decrements the same counters.

!!! tip "Two guards are better than one"
    Depth cap: blocks chains and guarantees the call stack cannot overflow.
    Budget counter: blocks exponential fan-out even within the legal depth limit.
    Neither alone is sufficient — use both.

!!! note "Try it"
    Run the stb-truetype tests with the race detector:

    ```bash
    cd stb-truetype-go && go test -race -v ./...
    ```

    Expected outcome: all tests pass; no data race reported. The budget counters
    live on the stack of `rasterizeGlyph` and are never shared across goroutines,
    so the race detector stays quiet.

---

## How the two attacks compare

| | XML deep nesting | TrueType billion-laughs |
|---|---|---|
| **Shape** | linear depth | exponential fan-out |
| **Crash mechanism** | fatal stack overflow | CPU/memory exhaustion |
| **`recover()` helps?** | no | n/a (not a crash) |
| **Primary guard** | depth ceiling | budget counter |
| **Secondary guard** | node count limit | depth ceiling |
| **Where in this repo** | `tinyxml2-go/tinyxml2.go` | `stb-truetype-go/sfnt.go` |

---

## Key takeaways

- A fatal goroutine stack overflow cannot be caught with `recover()`. The only
  defence is an explicit depth ceiling checked before each recursive call.
- `maxNestingDepth = 10000` in `tinyxml2-go` acts as an absolute hard ceiling
  even for the "unlimited" parse path, keeping the process alive.
- Iterative traversal (`FindDeep`, `FindAllDeep`) uses a heap-allocated slice
  as an explicit stack, sidestepping goroutine stack limits entirely for search.
- The billion-laughs attack exploits composite indirection: a small file produces
  exponential work. A depth ceiling stops linear chains; only a shared budget
  counter (components + points) stops exponential fan-out.
- `glyphBudget` in `stb-truetype-go/sfnt.go` is threaded as a pointer through
  the entire composite-glyph call tree so all recursive invocations decrement
  the same global counters — making the budget truly shared and impossible to
  circumvent by splitting work across branches.
