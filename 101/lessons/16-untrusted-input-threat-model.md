# 16 · The untrusted-input threat model

> **Objectives:** Understand who controls the bytes that reach a parser and why
> that question determines your security posture. Learn the four families of
> denial-of-service attack that threaten Go parsers — OOM, CPU exhaustion, stack
> overflow, and decompression/decode bombs — and see where each one appears in
> this repository. Understand the layered defence strategy that guards every
> module in SafeHeaders-Go.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Untrusted input** = "a letter from a stranger — you open it, but you don't let it set your house on fire." Any bytes whose sender you cannot fully control (file uploads, network responses, messages from other services) must be treated as potentially hostile before your parser touches them.
- **Parsing is high-risk work** = "the teller who counts the cash hands the thief exactly as much money as the thief wrote on the slip." A parser executes allocations, recursions, and loops driven by values inside those bytes — the attacker writes the slip.
- **Go gives you memory safety for free** = "the vault has unbreakable walls, but the door is still open." Buffer overflows, use-after-free, and arbitrary-code-execution are largely off the table in Go — the runtime enforces bounds — but nothing stops a parser from obeying an attacker-controlled size field.
- **DoS is still very much on the table** = "you can't pick the lock, but you can clog the keyhole with cement." An attacker who cannot run code can still crash a service or freeze it for minutes by crafting input that forces enormous allocations, infinite-looking loops, or a stack that fills up.
- **The four families** = "four ways to exhaust a machine without breaking in." OOM (out-of-memory), CPU exhaustion, stack overflow, and bomb attacks (decompression bombs, decode bombs) each attack a different resource and each requires its own specific defence in this codebase.
- **Multiplying the problem** = "if one fire hose floods the basement, eight fire hoses flood it eight times faster." Parallel batch APIs amplify memory use linearly with the worker count — a 4 MB decode bomb processed on 4 CPUs simultaneously becomes a 16 MB allocation storm.

**Why it matters:** a single malformed upload with no exploit code can take
down a production service; the fix is a single integer check at the right place.

**See it — four DoS families and the three-layer defence in SafeHeaders-Go.**

<svg viewBox="0 0 700 370" role="img" aria-labelledby="t16 d16" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t16">Four DoS threat families and three-layer defence</title>
  <desc id="d16">Left column: four threat boxes (OOM, CPU exhaustion, stack overflow, bomb attacks) each pointing right. Centre column: three defence layers (pre-parse gate, during-parse budget, aggregate cap). Right column: context timeout as the last line of defence.</desc>
  <defs>
    <marker id="l16-arr" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
  </defs>
  <!-- Threat family boxes -->
  <rect x="20" y="30" width="160" height="44" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="100" y="48" text-anchor="middle" font-size="12" font-weight="600" fill="#e5484d">OOM</text>
  <text x="100" y="64" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Inflated size field</text>

  <rect x="20" y="94" width="160" height="44" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="100" y="112" text-anchor="middle" font-size="12" font-weight="600" fill="#e5484d">CPU exhaustion</text>
  <text x="100" y="128" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Unbounded loop / tokens</text>

  <rect x="20" y="158" width="160" height="44" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="100" y="176" text-anchor="middle" font-size="12" font-weight="600" fill="#e5484d">Stack overflow</text>
  <text x="100" y="192" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Deep nesting / composites</text>

  <rect x="20" y="222" width="160" height="44" rx="6" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="100" y="240" text-anchor="middle" font-size="12" font-weight="600" fill="#e5484d">Bomb attacks</text>
  <text x="100" y="256" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Decomp / decode bombs</text>

  <!-- Arrows threat -> defence -->
  <line x1="180" y1="52" x2="258" y2="100" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l16-arr)"/>
  <line x1="180" y1="116" x2="258" y2="148" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l16-arr)"/>
  <line x1="180" y1="180" x2="258" y2="180" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l16-arr)"/>
  <line x1="180" y1="244" x2="258" y2="214" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l16-arr)"/>

  <!-- Defence layer boxes -->
  <rect x="262" y="78" width="176" height="44" rx="6" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="350" y="96" text-anchor="middle" font-size="12" font-weight="600" fill="#fff">Pre-parse gate</text>
  <text x="350" y="112" text-anchor="middle" font-size="10" fill="#fff">Check size before any work</text>

  <rect x="262" y="156" width="176" height="44" rx="6" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="350" y="174" text-anchor="middle" font-size="12" font-weight="600" fill="#fff">During-parse budget</text>
  <text x="350" y="190" text-anchor="middle" font-size="10" fill="#fff">Tokens, depth, pixels, nodes</text>

  <rect x="262" y="234" width="176" height="44" rx="6" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="350" y="252" text-anchor="middle" font-size="12" font-weight="600" fill="#fff">Aggregate cap</text>
  <text x="350" y="268" text-anchor="middle" font-size="10" fill="#fff">Archive total, not per-entry</text>

  <!-- Arrow defence -> timeout -->
  <line x1="438" y1="174" x2="510" y2="174" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l16-arr)"/>

  <!-- Last-resort box -->
  <rect x="514" y="130" width="166" height="88" rx="6" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5" stroke-dasharray="5 3"/>
  <text x="597" y="158" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">Context timeout</text>
  <text x="597" y="176" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Last line of defence —</text>
  <text x="597" y="192" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">application layer</text>

  <!-- Section labels -->
  <text x="100" y="16" text-anchor="middle" font-size="11" font-weight="700" fill="currentColor">Threats</text>
  <text x="350" y="64" text-anchor="middle" font-size="11" font-weight="700" fill="currentColor">Defence layers</text>
  <text x="597" y="118" text-anchor="middle" font-size="11" font-weight="700" fill="currentColor">Safety net</text>

  <!-- Parallel amplification note -->
  <text x="350" y="316" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">Parallel workers multiply memory pressure — budget per item × worker count.</text>
</svg>

---

## Who controls the bytes?

Before writing any parsing code, ask yourself one question:

> "Could an untrusted party choose or influence the bytes I am about to parse?"

The answer is almost always "yes" for:

- **File uploads** — images, audio, archives, fonts, documents.
- **API responses** — even from "trusted" partners (their data can be poisoned upstream).
- **Config/asset files** that users can edit.
- **Any network stream** — even if you authenticate the sender, a compromised
  sender can still craft malicious payloads.

The answer is "no" (and you can relax somewhat) only when the bytes are
compiled into your binary or come from hardware you exclusively control.

SafeHeaders-Go exists precisely in the first category: all nine modules parse
formats that routinely arrive from untrusted sources — JSON, XML, WAV, ZIP,
PNG/JPEG, TTF, glTF. (A "module" here is one self-contained piece of the
project — a folder of related source code, usually built around one file
format — and "parse"/"parser" means the code that reads raw bytes and makes
sense of their structure, the way you'd decode a sentence written in a
private shorthand.)

---

## The four threat families

### 1. OOM — out-of-memory

A malformed header declares a huge size; the parser blindly allocates that
many bytes (in plain terms: "allocate" means the program reserves a chunk of
the computer's memory to hold data — like claiming a table at a restaurant
before you know how many guests will show up); the process runs out of
memory or is killed by the OS (the operating system — the software, like
Windows or macOS or Linux, that manages the machine's memory and can kill a
program that asks for too much of it).

**Example from dr-wav-go** (from [`dr-wav-go/dr_wav.go`](src/dr-wav-go-dr-wav-go.md)):

```go
// readDataChunk scans subchunks until it finds the "data" chunk and returns its
// PCM payload. The allocation is capped at the bytes actually remaining in the
// reader so a malformed or malicious header that declares a huge data size
// cannot trigger an out-of-memory allocation.
func readDataChunk(r *bytes.Reader) ([]byte, error) {
    // ...
    if string(subchunkID[:]) == "data" {
        allocSize := int(subchunkSize)
        if allocSize > r.Len() {
            allocSize = r.Len() // never trust the declared size past EOF
        }
        pcmData := make([]byte, allocSize)
```

**In plain terms:** this code (a "function" — named `readDataChunk` here —
is simply a named, reusable block of instructions; "calling" or "invoking"
it just means running it) reads through a WAV audio file looking for the
chunk that holds the actual sound data. Before it reserves memory for that
data, it checks how many bytes are actually left in the file (`r.Len()`) and
shrinks the request to fit, so a lying header can't make it grab an
impossible amount of memory. (A "byte" is one small unit of digital data —
roughly enough to store one letter of text; files and memory are measured in
byte counts.)

The WAV format stores the data chunk's size as a 32-bit field — an attacker
can set that to `0xFFFFFFFF` (4 GiB) in a 100-byte file. Without the cap,
`make([]byte, 4GiB)` would crash most processes. With the cap, the allocator
only allocates as many bytes as are actually left in the reader (a "reader"
here is just an object that hands out the file's bytes a piece at a time).

Similarly, the `fmt` sub-chunk has an extra-bytes field. The original code
allocated a `[]byte` of that size; the fixed version uses `r.Seek` instead —
seek is free, allocation is not (from `dr-wav-go/dr_wav.go`):

```go
// Skip any extra format bytes. Seek rather than allocate: subchunk1Size is an
// untrusted uint32, so make([]byte, subchunk1Size-16) is an OOM vector.
if subchunk1Size > 16 {
    if _, err := r.Seek(int64(subchunk1Size-16), io.SeekCurrent); err != nil {
        return nil, fmt.Errorf("failed to skip extra format bytes: %w", err)
    }
}
```

**In plain terms:** instead of reserving memory to hold bytes it doesn't
actually need, this code just moves a "read position" marker forward past
them (`Seek` — like skipping ahead in an audiobook without downloading the
skipped part) — so an attacker-controlled size can't force a memory
reservation at all. Note also `return nil, fmt.Errorf(...)`: a function
"returns" by finishing its work and handing a result back to whoever called
it — here it hands back "no chunk, plus an error describing what went
wrong."

**Pattern:** never call `make([]byte, n)` where `n` comes from untrusted input
without bounding `n` against the bytes actually present.

---

### 2. CPU exhaustion

A crafted input forces the parser into a very long (or infinite) loop. Memory
stays flat; your goroutine (in plain terms: a lightweight, independently-
running unit of work inside the program — think of it as one worker among
possibly many working at once) pegs a CPU core and never finishes.

**Example from jsmn-go** — token-count limit (from [`jsmn-go/config.go`](src/jsmn-go-config-go.md)):

```go
// StrictConfig returns a stricter configuration suitable for untrusted input.
func StrictConfig() *Config {
    return &Config{
        MaxInputSize: 10 * 1024 * 1024, // 10MB
        MaxTokens:   100_000,           // 100k tokens
        // ...
    }
}
```

**In plain terms:** `StrictConfig` is a function that returns (hands back) a
ready-made bundle of safety settings — here, a limit on how many bytes of
input to accept and how many "tokens" to allow. A tokenizer is the part of a
parser that chops raw text into small meaningful pieces ("tokens") — for
JSON, things like `{`, `"name"`, `:`, `42`, `}` each count as one token.

JSON like `[[[[[...10 million nesting levels...]]]]]` is tiny on disk but
causes the tokenizer to emit millions of tokens. `StrictConfig` caps both the
input bytes and the token count so the work is bounded before it starts.

The same pattern appears in tinyxml2-go, which limits input size, total node
count, and nesting depth (from [`tinyxml2-go/tinyxml2.go`](src/tinyxml2-go-tinyxml2-go.md)):

```go
// ParseWithConfig builds a full DOM tree while enforcing the limits in config
// (input size, total node count, and nesting depth) to guard against
// denial-of-service from oversized or maliciously nested XML.
func ParseWithConfig(data []byte, config *Config) (*XMLDocument, error) {
    // ...
    root, err := parseElementLimited(dec, v, config, 1, &nodeCount)
```

**In plain terms:** parsing one XML tag calls a function that, for every
tag nested inside it, calls that same function again on the nested tag —
this "a function calling itself" pattern is called recursion. Nested tags
inside nested tags inside nested tags produce deep recursion. This code
threads a counter (`nodeCount`) and the current nesting depth through every
call so it can refuse to go further once either limit is hit.

**Pattern:** for any loop or recursion driven by input data, bound the
iteration count before you start, not after you notice it's too slow.

---

### 3. Stack overflow

A deeply recursive parser hits Go's goroutine stack limit (in plain terms:
the "stack" is a fixed-size scratchpad of memory the program uses to keep
track of which function called which — every nested call takes a bit more
of that space, and Go caps how big it can grow per goroutine) and the
runtime (the background system that manages running Go programs — starting
goroutines, cleaning up memory, and so on) terminates the whole program
with a fatal exception — one that `recover()` cannot catch (`recover()` is
Go's normal tool for catching a crash inside one goroutine so the rest of
the program survives; a stack overflow is severe enough that even this
safety net fails). This is sometimes called a "billion laughs" attack when an
entity references entities that reference entities.

**Example from tinyxml2-go** — hard depth ceiling (from `tinyxml2-go/tinyxml2.go`):

```go
// maxNestingDepth is an absolute hard ceiling on recursion depth that applies
// even to Parse (which has no user-facing config).
const maxNestingDepth = 10000

func parseElement(dec *xml.Decoder, se xml.StartElement, depth int) (*Node, error) {
    if depth > maxNestingDepth {
        return nil, fmt.Errorf("XML nesting exceeds maximum depth %d", maxNestingDepth)
    }
```

**In plain terms:** every time `parseElement` calls itself for a nested XML
tag, it passes in `depth` one higher than before. If that number ever
climbs above 10,000, the function refuses to go deeper and hands back an
error instead of recursing again — capping how deep the "function calling
itself" chain can go, no matter how deeply nested the attacker's XML is.

This constant is enforced at every level of recursion — even in the
`Parse` function that has no config object — so there is no path an attacker
can take to skip it.

**Example from stb-truetype-go** — composite glyph budget (from [`stb-truetype-go/sfnt.go`](src/stb-truetype-go-sfnt-go.md)):

```go
// glyphBudget bounds total work for one top-level glyph. The depth cap alone
// does not stop a malicious composite with high fan-out (K children per level,
// 8 levels ≈ K^8 invocations from a tiny file — a billion-laughs amplification),
// so a shared counter caps total components visited and total points produced.
type glyphBudget struct {
    components int // remaining glyph invocations (call-tree nodes)
    points     int // remaining total contour points
}

const (
    maxGlyphComponents = 4096    // total component invocations per top-level glyph
    maxGlyphPoints     = 1 << 20 // total contour points per top-level glyph
)

func (f *Font) glyphContours(gid uint16, depth int, b *glyphBudget) ([][]glyphPoint, error) {
    if depth > 8 {
        return nil, errors.New("truetype: composite glyph nesting too deep")
    }
    if b.components--; b.components < 0 {
        return nil, errors.New("truetype: composite glyph component budget exceeded")
    }
```

**In plain terms:** `glyphBudget` here is a "struct" — a small custom data
shape that bundles a few related values together under one name (like a
form with labeled boxes: one box called `components`, one called `points`).
Each named box inside a struct is called a "field." This particular struct
travels along with every recursive call and gets decremented (`b.components--`)
each time, so the code can refuse to continue once either the component
count or the point count runs out — even if the depth alone hasn't hit its
limit yet.

A TrueType "composite" glyph is composed of other glyphs, which may be
composed of yet more glyphs. A malicious font can make a glyph tree with K
children at each level over 8 levels — K^8 recursive calls from a tiny file.
The `glyphBudget` struct caps both the call-tree size (components) and the
total coordinate output (points), so the work is O(budget) regardless of the
glyph tree's shape.

!!! warning "Stack overflows are fatal"
    In Go, a stack overflow produces a `runtime: goroutine stack exceeds ...`
    message and kills the entire process — not just the goroutine. A `recover()`
    in a `defer` does **not** help — `defer` is a Go feature that schedules a
    bit of cleanup code to run automatically when a function finishes, and
    `recover()` is normally called inside one to catch a crash before it takes
    down more than the current goroutine. The only defence is to never let the
    stack get that deep in the first place. That is why the depth check in
    `parseElement` uses a `const` (a fixed value baked in when the program is
    built, which nothing at runtime can change), not a config value that could
    be set to 0.

---

### 4. Decompression bombs and decode bombs

A compressed or encoded file stores a tiny payload that expands to gigabytes
when decoded. The attacker does no work; your CPU and memory do all of it.

**Decompression bomb — miniz-go** (from [`miniz-go/miniz.go`](src/miniz-go-miniz-go.md)):

```go
// MaxDecompressedSize caps how many bytes ExtractArchive and DecompressData will
// produce, guarding against decompression bombs (a small input that inflates to
// gigabytes). For ExtractArchive the cap is on the ARCHIVE TOTAL across all
// entries, not per entry. The default is 256 MiB. Set it to 0 to disable.
var MaxDecompressedSize int64 = 256 << 20
```

**In plain terms:** ZIP files store their contents squeezed smaller
("compressed"); unpacking them ("decompressing") restores the full size,
which can be far bigger than the compressed file itself. This setting is a
running total — a ceiling on how many bytes may come out of *all* the files
inside one archive combined, not a separate allowance for each individual
file.

The critical detail: the cap is on the **aggregate** output across all ZIP
entries, not per entry. A naive per-entry cap can be bypassed with 1000
entries each "under the limit."

**Decode bomb — stb-image-go** (from [`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md)):

```go
// MaxImagePixels caps the number of pixels Load will decode, guarding against
// decode bombs — a tiny file whose header declares enormous dimensions can drive
// image.Decode to allocate gigabytes. The default is 64 megapixels (e.g.
// 8192x8192). Set it to 0 to disable the guard.
var MaxImagePixels = 64 << 20

func checkPixelLimit(data []byte) error {
    if MaxImagePixels <= 0 {
        return nil
    }
    cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
    if err != nil {
        return nil // let the full decode surface the real error
    }
    if cfg.Width > 0 && cfg.Height > 0 &&
        int64(cfg.Width)*int64(cfg.Height) > int64(MaxImagePixels) {
        return fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit (adjust MaxImagePixels)",
            cfg.Width, cfg.Height, MaxImagePixels)
    }
    return nil
}
```

**In plain terms:** before actually decoding an image's full pixel data,
this code peeks at just the header to read the claimed width and height,
multiplies them, and refuses to continue if that number of pixels is
unreasonably large — the same "check the label before opening the box"
idea as the WAV example earlier, applied to images instead of audio.

`image.DecodeConfig` reads only the header — it is cheap. If the declared
dimensions exceed the pixel cap, `Load` returns an error before calling
`image.Decode`, which is the expensive (and potentially enormous) operation.

**Pattern:** read cheap metadata first, reject before doing expensive work.

---

## The defence-in-depth model

No single guard is enough. SafeHeaders-Go uses three layers, and SECURITY.md
recommends a fourth at the application level:

| Layer | Mechanism | Where |
|-------|-----------|-------|
| **Pre-parse gate** | Check input size before any work | `jsmn-go/config.go` `validateInput`, tinyxml2-go `validateInput` |
| **During-parse budget** | Cap tokens, nodes, depth, pixels, bytes | all eight modules |
| **Parallel amplification** | Context cancellation + correctly-sized error channels | `stb-image-go` `LoadBatchConcurrent`, `jsmn-go` `parseParallelWithConfig` |
| **Application layer** | Size limits, timeouts, GOMAXPROCS caps | SECURITY.md "Security Best Practices" |

The application-layer advice from SECURITY.md is worth repeating verbatim:

```go
// Use Context for Timeouts — prevent long-running ops from blocking:
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
images, err := stbimagego.LoadBatchConcurrent(ctx, imageData)
```

**In plain terms:** a "context" in Go is a small object you pass into a
long-running operation to say "give up after this much time, or if I tell
you to." Here it's set to 30 seconds; "blocking" means a piece of code has
stopped and is just waiting — for example, waiting for a slow decode to
finish — and doing nothing else until it's unblocked. This line arranges
for the operation to be forcibly stopped rather than block forever.

A context timeout is the ultimate safety net: even if a bug lets a parser run
longer than its budgets intend, the context kills it.

---

## The parallel amplification trap

Batch APIs spin up `runtime.NumCPU()` workers. On an 8-core machine, eight
images are decoded simultaneously. If each image is a decode bomb, your
protection is 8× more important — not less.

The `errs` channel in `stb-image-go/stb_image.go` shows one subtle
amplification risk that is already fixed:

```go
// Buffer the worst case so no worker blocks on send: up to len(datas) decode
// failures plus up to numWorkers cancellation sends. An under-sized buffer
// deadlocks wg.Wait when cancellation coincides with decode failures.
errs := make(chan error, len(datas)+numWorkers)
```

**In plain terms:** a "channel" is a pipe that goroutines use to hand
values (like error messages) to each other safely. This one is "buffered" —
sized to hold a certain number of messages waiting in the pipe before
anyone has to read them out — big enough here to hold every possible error
message any worker could ever try to send, so no worker ever gets stuck
waiting for space in the pipe.

If you made the channel only `len(datas)` deep, a cancellation event (one send
per worker) could block goroutines that are trying to send — and `wg.Wait()`
would hang forever (a "deadlock": everything is stuck waiting on everything
else, permanently, with nothing able to move — like a four-way stop where
every car is waiting for another to go first). This is the same deadlock shape
described in [Lesson 11](11-goroutines-channels-select.md); the fix is always
to make the channel deep enough for every possible send, not just the expected
ones.

---

## The zero-dependency advantage

From SECURITY.md:

> SafeHeaders-Go has **zero external dependencies** (pure stdlib). This
> minimizes supply chain attack surface.

(A "dependency" is someone else's pre-written code that a project relies on
instead of writing that logic itself — installed as a "package." The
"stdlib," short for standard library, is the large set of ready-made
packages that ship with Go itself, so using only it means relying on
nobody's code but Go's own.)

Every external dependency is a potential vector: a compromised package, a
malicious update, an indirect transitive dependency (one you didn't choose
directly — it came along because a package you *did* choose depends on it
in turn). By staying pure stdlib, SafeHeaders-Go eliminates that entire class
of risk. The trade-off is that every parsing behaviour must be written from
scratch — which is exactly what these nine modules are.

---

!!! note "Try it"
    Verify that the OOM guard in dr-wav-go fires when the data chunk header
    claims more bytes than are actually present. (A "test" here is a small
    piece of code, separate from the program itself, written specifically to
    check that some behaviour works as expected — `go test` is the command
    that runs all such checks and reports pass/fail.) The fuzz corpus already
    contains a seed that exercises this path — "fuzzing" means automatically
    generating huge numbers of random or semi-random inputs to try to break
    the code, and the "corpus" is the growing collection of inputs it has
    tried so far, some of which are saved as starting "seeds":

    ```bash
    cd dr-wav-go
    go test -run TestParse -v ./...
    ```

    Expected outcome: all tests pass. No panics (a "panic" is Go's term for a
    program crashing abruptly instead of returning a normal error), no OOM.
    Then try the fuzzer for 10 seconds to confirm the guard holds against
    fresh mutations:

    ```bash
    go test -fuzz=FuzzParse -fuzztime=10s ./...
    ```

    Expected outcome: the fuzzer runs 10 s and stops cleanly; any corpus entry
    that would trigger an OOM is caught by the `r.Len()` cap and returns an
    error instead of crashing.

!!! tip "Predict before you run"
    Before running the fuzz command, predict: will `FuzzParse` find a crash?
    Almost certainly not — the guard converts the crash into an error return.
    That is precisely the point. A fuzzer finding "no crash" on hardened code
    is a green result, not a boring one.

---

## Key takeaways

- **In Go the real parser threat is DoS, not memory corruption.** Buffer
  overflows and pointer corruption are gone; OOM, CPU exhaustion, stack
  overflow, and bombs remain.
- **Never trust a size field from untrusted input.** Cap all allocations to the
  bytes actually present (e.g. `r.Len()`) before calling `make`.
- **Bound every input-driven loop and recursion** with a budget checked at
  entry, not at the point of failure — by then it is too late.
- **Aggregate budgets beat per-item budgets.** A decompression bomb spread
  across 1000 entries defeats a per-entry cap; miniz-go caps the archive total.
- **Context timeouts are the last line of defence.** They catch bugs that slip
  past all the other guards. Always wire them in at the application layer when
  processing untrusted input.
