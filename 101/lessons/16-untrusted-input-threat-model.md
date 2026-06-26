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

- **"Untrusted input"** means any bytes whose sender you cannot fully control:
  a file upload, a network response, a message from another service, a file
  sitting in a shared directory.
- **Parsing is high-risk work.** A parser reads structured bytes and turns them
  into in-memory objects. Every allocation, every recursion, every loop
  iteration is driven by values inside those bytes.
- **Go gives you memory safety for free** — you cannot scribble past an array
  bound or corrupt a pointer the way C can. Buffer overflows, use-after-free,
  and arbitrary-code-execution are largely off the table.
- **But DoS is still very much on the table.** An attacker who cannot run code
  can still crash your service or freeze it for minutes by crafting input that
  forces enormous allocations, infinite-looking loops, or a stack that fills up.
- **The four families to learn:** OOM (out-of-memory), CPU exhaustion, stack
  overflow, and bomb attacks (decompression bombs, decode bombs). Each family
  has a different defence.
- **Multiplying the problem:** parallel batch APIs amplify memory use linearly
  with the worker count. A 4-CPU machine processes four images at once, so a
  4 MB decode bomb becomes a 16 MB allocation storm.

**Why it matters:** a single malformed upload with no exploit code can take
down a production service; the fix is a single integer check at the right place.

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
PNG/JPEG, TTF, glTF.

---

## The four threat families

### 1. OOM — out-of-memory

A malformed header declares a huge size; the parser blindly allocates that
many bytes; the process runs out of memory or is killed by the OS.

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

The WAV format stores the data chunk's size as a 32-bit field — an attacker
can set that to `0xFFFFFFFF` (4 GiB) in a 100-byte file. Without the cap,
`make([]byte, 4GiB)` would crash most processes. With the cap, the allocator
only allocates as many bytes as are actually left in the reader.

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

**Pattern:** never call `make([]byte, n)` where `n` comes from untrusted input
without bounding `n` against the bytes actually present.

---

### 2. CPU exhaustion

A crafted input forces the parser into a very long (or infinite) loop. Memory
stays flat; your goroutine pegs a CPU core and never finishes.

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

**Pattern:** for any loop or recursion driven by input data, bound the
iteration count before you start, not after you notice it's too slow.

---

### 3. Stack overflow

A deeply recursive parser hits Go's goroutine stack limit and the runtime
terminates the whole program with a fatal exception — one that `recover()`
cannot catch. This is sometimes called a "billion laughs" attack when an
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

A TrueType "composite" glyph is composed of other glyphs, which may be
composed of yet more glyphs. A malicious font can make a glyph tree with K
children at each level over 8 levels — K^8 recursive calls from a tiny file.
The `glyphBudget` struct caps both the call-tree size (components) and the
total coordinate output (points), so the work is O(budget) regardless of the
glyph tree's shape.

!!! warning "Stack overflows are fatal"
    In Go, a stack overflow produces a `runtime: goroutine stack exceeds ...`
    message and kills the entire process — not just the goroutine. A `recover()`
    in a `defer` does **not** help. The only defence is to never let the stack
    get that deep in the first place. That is why the depth check in
    `parseElement` uses a `const`, not a config value that could be set to 0.

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

If you made the channel only `len(datas)` deep, a cancellation event (one send
per worker) could block goroutines that are trying to send — and `wg.Wait()`
would hang forever. This is the same deadlock shape described in
[Lesson 11](11-goroutines-channels-select.md); the fix is always to make the
channel deep enough for every possible send, not just the expected ones.

---

## The zero-dependency advantage

From SECURITY.md:

> SafeHeaders-Go has **zero external dependencies** (pure stdlib). This
> minimizes supply chain attack surface.

Every external dependency is a potential vector: a compromised package, a
malicious update, an indirect transitive dependency. By staying pure stdlib,
SafeHeaders-Go eliminates that entire class of risk. The trade-off is that
every parsing behaviour must be written from scratch — which is exactly what
these nine modules are.

---

!!! note "Try it"
    Verify that the OOM guard in dr-wav-go fires when the data chunk header
    claims more bytes than are actually present. The fuzz corpus already
    contains a seed that exercises this path:

    ```bash
    cd dr-wav-go
    go test -run TestParse -v ./...
    ```

    Expected outcome: all tests pass. No panics, no OOM. Then try the fuzzer
    for 10 seconds to confirm the guard holds against fresh mutations:

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
