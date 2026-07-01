# 06 · Standing on the stdlib: cjson, cgltf, tinyxml2

> **Objectives:** Understand why wrapping `encoding/json` and `encoding/xml` is a legitimate
> porting strategy, see what the wrapper still has to own (validation, limits, concurrency), and
> know when to call `ParseWithConfig` instead of bare `Parse`.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Wrapping, not reimplementing** = "renting a professional kitchen instead of building one from scratch." The C originals (cJSON, cgltf, tinyxml2) parse bytes byte-by-byte in hand-written C; these Go modules plug in `encoding/json` and `encoding/xml` instead, then add the things the stdlib leaves to you.
- **The stdlib is fuzzed and CVE-tracked upstream** = "inheriting a security guard who never sleeps." `encoding/json` and `encoding/xml` receive constant fuzzing from the Go team, so every bug fix and hardening pass flows to your code for free.
- **"Parsing" is only the first 20%** = "reading a blueprint is not the same as inspecting the building." The grammar pass tells you the bytes are well-formed JSON or XML; the remaining 80% is rejecting absurd input sizes before they land in RAM, validating cross-references, and scaling to multiple documents at once.
- **Limits are not optional for untrusted input** = "a menu that lets a customer order ten million plates before you check whether the kitchen has food." A 3-byte JSON body can describe an array of ten million items; without `MaxArrayItems`, your process allocates ten million slots before it reads a single element value.
- **Validation is domain knowledge, not parser knowledge** = "a spell-checker that passes 'the cat ate the cloud' because every word is spelled correctly." `encoding/json` cannot know that glTF scene 5 referencing node 99 is invalid when the model only defines 3 nodes — `ValidateGLTF` owns that check.
- **Concurrency is composable on top** = "a single recipe that ten cooks can follow in parallel once it is written down." The stdlib parsers are not concurrent, but once parsing is a pure function (bytes in, struct out) you can fan multiple calls across a worker pool as `UnmarshalArrayParallel` and `ParseBatch` both do.

**Why it matters:** choosing the right porting strategy halves the attack surface you have to
audit — use the stdlib for the grammar, write Go for everything else.

**See it — three-layer wrapper: stdlib grammar, Go limits, Go validation.**

<svg viewBox="0 0 700 310" role="img" aria-labelledby="t06 d06" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t06">Three-layer wrapper architecture for cjson-go, cgltf-go, and tinyxml2-go</title>
  <desc id="d06">A block-and-arrow diagram showing raw bytes flowing right through three layers: stdlib parser (encoding/json or encoding/xml), then Go limit checks (size cap, array cap, depth ceiling), then Go validation (cross-reference checks, domain rules), producing a safe Go struct on the right. A rejection arrow exits downward from the limit and validation layers.</desc>
  <defs>
    <marker id="l06-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 Z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
    <marker id="l06-arrow-err" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 Z" fill="#e5484d"/>
    </marker>
  </defs>
  <text x="18" y="160" font-size="12" text-anchor="middle" dominant-baseline="middle" fill="currentColor" transform="rotate(-90,18,160)">raw bytes</text>
  <line x1="34" y1="155" x2="88" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>
  <rect x="90" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="165" y="135" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">stdlib parser</text>
  <text x="165" y="153" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">encoding/json</text>
  <text x="165" y="167" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">encoding/xml</text>
  <text x="165" y="185" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">grammar only</text>
  <line x1="241" y1="155" x2="275" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>
  <rect x="277" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="352" y="130" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">Go limit checks</text>
  <text x="352" y="149" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">MaxArrayItems</text>
  <text x="352" y="163" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">MaxInputSize</text>
  <text x="352" y="177" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">maxNestingDepth</text>
  <text x="352" y="195" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">reject early</text>
  <line x1="428" y1="155" x2="462" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>
  <rect x="464" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="539" y="130" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">Go validation</text>
  <text x="539" y="149" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">ValidateGLTF</text>
  <text x="539" y="163" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">cross-ref checks</text>
  <text x="539" y="177" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">domain rules</text>
  <text x="539" y="195" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">semantics</text>
  <line x1="615" y1="155" x2="665" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>
  <text x="683" y="150" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">safe</text>
  <text x="683" y="163" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">struct</text>
  <line x1="352" y1="211" x2="352" y2="265" stroke="#e5484d" stroke-width="2" marker-end="url(#l06-arrow-err)"/>
  <rect x="295" y="268" width="115" height="28" rx="5" ry="5" fill="#e5484d" opacity="0.12" stroke="#e5484d" stroke-width="1"/>
  <text x="352" y="284" font-size="10" text-anchor="middle" fill="#e5484d">error returned</text>
  <line x1="539" y1="211" x2="539" y2="265" stroke="#e5484d" stroke-width="2" marker-end="url(#l06-arrow-err)"/>
  <rect x="482" y="268" width="115" height="28" rx="5" ry="5" fill="#e5484d" opacity="0.12" stroke="#e5484d" stroke-width="1"/>
  <text x="539" y="284" font-size="10" text-anchor="middle" fill="#e5484d">error returned</text></svg>

---

## cjson-go: JSON with an array-size brake

[`cjson-go/cjson.go`](src/cjson-go-cjson-go.md) is a thin layer over `encoding/json` (a
"package" is a bundle of pre-written code that solves one problem — here, reading and
writing JSON text — that you can pull into your own program instead of writing that logic
yourself; `encoding/json` ships as part of Go's own "standard library," the large set of
trustworthy, built-in packages every Go program can use for free).
The interesting addition is `UnmarshalArrayParallel` — a "function" is a named, reusable
chunk of instructions; you "call" (or "invoke") a function by writing its name to make the
computer run those instructions right now, optionally handing it some input and later
getting a result back.

### The cap

```go
// from cjson-go/cjson.go
var MaxArrayItems = 1 << 20  // 1,048,576

func UnmarshalArrayParallel(data []byte) ([]map[string]interface{}, error) {
    var rawArray []json.RawMessage
    if err := json.Unmarshal(data, &rawArray); err != nil {
        return nil, fmt.Errorf("failed to parse array: %w", err)
    }

    if MaxArrayItems > 0 && len(rawArray) > MaxArrayItems {
        return nil, fmt.Errorf("array has %d items, exceeding the %d-item limit ...",
            len(rawArray), MaxArrayItems)
    }
    ...
}
```

**In plain terms:** this function takes in raw JSON text (`data`, a sequence of bytes — a
byte is the basic unit computers store information in, roughly one character's worth) and
tries to read it as a list of items. `[]byte` means "a list of bytes," and a "list" here is
what programmers call a "slice" — a resizable, ordered collection of values, similar to a
numbered row of boxes. It first checks whether the list is longer than the allowed maximum
and, if so, hands back ("returns" — the function stops running and gives its result back to
whoever called it) an error instead of proceeding.

The first `json.Unmarshal` into `[]json.RawMessage` is cheap: it records offsets (an offset
is just "how far into the data this piece starts," a position counted in bytes from the
beginning), it does
**not** decode each element. The cap fires before any per-item work begins. Adjust
`MaxArrayItems` to 0 to disable it — but only do that if you control the input.

### The worker pool

```go
// from cjson-go/cjson.go
numWorkers := runtime.NumCPU()
if len(rawArray) < numWorkers {
    numWorkers = len(rawArray)
}

results := make([]map[string]interface{}, len(rawArray))
errs    := make(chan error, numWorkers)
jobs    := make(chan int,   len(rawArray))

for i := range rawArray { jobs <- i }
close(jobs)

var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for idx := range jobs {
            var item map[string]interface{}
            if err := json.Unmarshal(rawArray[idx], &item); err != nil {
                errs <- fmt.Errorf("failed to unmarshal item %d: %w", idx, err)
                return
            }
            results[idx] = item
        }
    }()
}
wg.Wait()
close(errs)
```

**In plain terms:** rather than decoding every array element one after another, this code
splits the work across several "workers" that run at the same time (this is called
"concurrency" — the ability to have multiple pieces of work in flight together, each one a
"goroutine," which is Go's lightweight name for an independent, concurrently-running unit of
work). It creates one job per array item, lets `numWorkers` goroutines pick jobs off a shared
queue (a "channel" — a typed pipe that one goroutine can send values into and another can
receive them from, safely, even when many goroutines use it at once), and waits for all of
them to finish before moving on.

Each worker picks an index off `jobs`, decodes one element, and writes to its own slot in
`results` (no lock needed — slots never overlap; a "lock," also called a "mutex," is a tool
that lets only one goroutine touch a shared piece of data at a time, preventing two workers
from corrupting it by writing simultaneously — it isn't needed here because each worker only
ever writes to its own reserved slot). The `errs` channel is buffered to
`numWorkers` so a failing worker can always send without blocking (a channel operation
"blocks" when it simply waits — the goroutine pauses right there and does nothing else until
someone is ready to receive what it sent; "buffered" means the channel can hold a few values
before anyone receives them, so the sender doesn't have to wait every time).

### Stream path: the caller must set the limit

```go
// from cjson-go/cjson.go
// UnmarshalStream parses JSON from an io.Reader. It does not impose a size
// limit, so for untrusted input callers MUST wrap r in an io.LimitReader (or
// http.MaxBytesReader) to bound memory.
func UnmarshalStream(r io.Reader, v interface{}) error {
    decoder := json.NewDecoder(r)
    if err := decoder.Decode(v); err != nil {
        return fmt.Errorf("stream unmarshal error: %w", err)
    }
    return nil
}
```

**In plain terms:** instead of handing this function a complete chunk of bytes already sitting
in memory, you hand it an `io.Reader` — a stand-in for "something that produces bytes over
time," like a network connection still receiving data. The function reads from it piece by
piece and decodes it into `v` (an "interface" like `interface{}` is Go's way of saying "a
value of any type"; here it means "whatever variable you pass in to receive the result").

The comment is load-bearing. Streaming hides the total size, so the library cannot
safely enforce a cap itself. You wrap the reader:

```go
limited := io.LimitReader(r, 64<<20) // 64 MB ceiling
if err := cjsongo.UnmarshalStream(limited, &v); err != nil { ... }
```

!!! warning "Streaming + limits = caller's job"
    `UnmarshalStream` and `MarshalStream` do not impose size limits. Always wrap the
    reader with `io.LimitReader` or `http.MaxBytesReader` before passing it in.

---

## cgltf-go: parse once, validate separately

[`cgltf-go/cgltf.go`](src/cgltf-go-cgltf-go.md) is structured around a deliberate two-step: **parse** (grammar),
then **validate** (semantics). This mirrors how the original C library works.

### Parse: grammar only

```go
// from cgltf-go/cgltf.go
func Parse(data []byte) (*GLTF, error) {
    if len(data) == 0 {
        return nil, errors.New("empty glTF data")
    }
    var gltf GLTF
    if err := json.Unmarshal(data, &gltf); err != nil {
        return nil, fmt.Errorf("failed to parse glTF: %w", err)
    }
    if gltf.Asset.Version == "" {
        return nil, errors.New("missing required field: asset.version")
    }
    return &gltf, nil
}
```

**In plain terms:** this function reads the raw file bytes, turns them into a `GLTF` value —
a "struct," Go's word for a bundle of related named pieces of data grouped under one type,
similar to a form with labeled fields (a "field" is one of those named slots, like
`Asset.Version` here) — and hands that struct back if the bytes were valid JSON and the
required version field was present.

`Parse` only checks that the JSON is well-formed and that the mandatory `asset.version`
field is present. Integer indices scattered throughout the file — scene index, node
indices, mesh indices — are not checked here (an "index" is a number giving the position of
an item in a list, counting from zero — so index `2` means "the third item").

### ValidateGLTF: cross-reference checks

```go
// from cgltf-go/cgltf.go
func ValidateGLTF(gltf *GLTF) error {
    if gltf.Asset.Version != "2.0" {
        return fmt.Errorf("unsupported glTF version: %s (only 2.0 supported)", gltf.Asset.Version)
    }

    if len(gltf.Scenes) > 0 {
        if gltf.Scene < 0 || gltf.Scene >= len(gltf.Scenes) {
            return fmt.Errorf("invalid scene index: %d", gltf.Scene)
        }
    }

    for si, scene := range gltf.Scenes {
        for _, n := range scene.Nodes {
            if n < 0 || n >= len(gltf.Nodes) {
                return fmt.Errorf("scene %d references invalid node: %d", si, n)
            }
        }
    }

    for i, node := range gltf.Nodes {
        for _, c := range node.Children {
            if c < 0 || c >= len(gltf.Nodes) {
                return fmt.Errorf("node %d references invalid child: %d", i, c)
            }
        }
    }
    return nil
}
```

**In plain terms:** this function walks through the parsed struct and checks that every index
it contains actually points at something that exists — for example, that a scene's node
number is not bigger than the list of nodes actually has entries for. If any reference points
outside the bounds of its list, it hands back an error describing exactly which one is bad.

These range-checks are exactly what `encoding/json` cannot do: it has no idea that the
integer `99` in `"scene": 99` is supposed to be a valid index into the `scenes` array.

!!! warning "ValidateGLTF does not cover everything"
    The docstring is explicit: accessor → bufferView → buffer referential integrity is
    **not** checked. If your code indexes into those arrays, bounds-check yourself.

### Concurrent batch loading

```go
// from cgltf-go/cgltf.go
func ParseBatch(ctx context.Context, dataList [][]byte) ([]*GLTF, error) {
    numWorkers := runtime.NumCPU()
    if numWorkers > len(dataList) {
        numWorkers = len(dataList)
    }
    // workers drain dataChan, send to resultChan
    // resultChan is buffered to len(dataList) so workers never block
    resultChan := make(chan result, len(dataList))
    ...
}
```

**In plain terms:** this function parses many glTF files at once by spreading the work over
several goroutines (the same "run things at the same time" idea used in cjson-go above), and
collects all the finished results into `resultChan` for the caller to read back.

`ParseBatch` respects a `context.Context` — cancel it and workers stop picking up new work
within one item (a `Context` is a small, standard Go value you pass around that carries a
"please stop soon" signal and optional deadlines; goroutines that check it can bail out early
instead of running to completion once nobody needs their answer). Each result carries the
original index so the output slice preserves
input order regardless of which worker finishes first.

!!! tip "Always validate after batch parse"
    `ParseBatch` calls `Parse` internally; it does **not** call `ValidateGLTF`. After the
    batch returns, loop over results and call `ValidateGLTF` on each.

---

## tinyxml2-go: depth limits against the billion-laughs attack

[`tinyxml2-go/tinyxml2.go`](src/tinyxml2-go-tinyxml2-go.md) wraps `encoding/xml` into a DOM
(a "DOM," or Document Object Model, is a tree-shaped representation of a document in memory —
each element becomes a "node" that can contain child nodes, mirroring how the tags are nested
in the original file), which means it builds a
recursive tree in memory ("recursive" describes a function that calls itself to handle
smaller and smaller pieces of the same problem — here, parsing one XML element calls the same
parsing logic again for each element nested inside it; "in memory" means the tree is
"allocated" — reserved as a chunk of the computer's working memory, RAM — rather than kept on
disk). Recursive trees have a specific failure mode: deeply nested
input can overflow the goroutine stack (every goroutine gets a limited region of memory,
its "stack," to keep track of function calls in progress; each nested recursive call uses a
bit more of it, and sufficiently deep nesting can exhaust that region entirely) — and unlike
a panic (Go's term for a program erroring out abruptly mid-execution), a stack overflow is a
**fatal** error that `recover()` cannot catch (`recover()` is the normal tool for regaining
control after a panic and continuing to run; a stack overflow bypasses it entirely and
crashes the whole program).

### The hard ceiling

```go
// from tinyxml2-go/tinyxml2.go
const maxNestingDepth = 10000

func parseElement(dec *xml.Decoder, se xml.StartElement, depth int) (*Node, error) {
    if depth > maxNestingDepth {
        return nil, fmt.Errorf("XML nesting exceeds maximum depth %d", maxNestingDepth)
    }
    ...
    case xml.StartElement:
        child, err := parseElement(dec, v, depth+1)
    ...
}
```

**In plain terms:** every time the parser steps one level deeper into nested XML tags, it
calls itself again with `depth` increased by one; if that count ever passes the hard-coded
ceiling of 10,000, it stops immediately and reports an error instead of continuing to recurse.

This ceiling applies even to the bare `Parse` path and to `UnlimitedConfig`. There is no
way to configure it away — that is intentional.

### Three config presets

The config lives in [`tinyxml2-go/config.go`](src/tinyxml2-go-config-go.md):

```go
// DefaultConfig — sensible production values
&Config{
    MaxInputSize:    100 * 1024 * 1024, // 100 MB
    MaxNodeCount:    1_000_000,
    MaxNestingDepth: 1_000,
}

// StrictConfig — for user-supplied or third-party XML
&Config{
    MaxInputSize:    10 * 1024 * 1024,  // 10 MB
    MaxNodeCount:    100_000,
    MaxNestingDepth: 100,
}

// UnlimitedConfig — no soft limits; hard ceiling still applies
&Config{
    MaxInputSize:    0,
    MaxNodeCount:    0,
    MaxNestingDepth: 0,
}
```

**In plain terms:** these are three ready-made bundles of settings (each one a `Config`
struct — the bundle-of-named-fields idea from cgltf-go above) that trade off strictness
against flexibility: `DefaultConfig` is reasonable for everyday use, `StrictConfig` clamps
things down hard for input you don't trust, and `UnlimitedConfig` turns off every adjustable
limit while the un-adjustable 10,000-deep ceiling from above still applies no matter what.

### Using ParseWithConfig

```go
// from tinyxml2-go/tinyxml2.go
func ParseWithConfig(data []byte, config *Config) (*XMLDocument, error) {
    if config == nil {
        config = DefaultConfig()
    }
    if err := config.Validate(); err != nil {
        return nil, err
    }
    if err := config.validateInput(data); err != nil {
        return nil, err
    }
    // then parse, enforcing MaxNestingDepth and MaxNodeCount per element
    ...
}
```

`parseElementLimited` — called internally by `ParseWithConfig` — enforces both the
config depth limit and the hard `maxNestingDepth` ceiling. The config limit fires first,
so `ErrNestingTooDeep` reaches the caller before the hard ceiling is ever approached.

### Iterative search methods

The `Find` / `FindAll` methods search only direct children. For deep trees, use the
iterative variants:

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

**In plain terms:** instead of having the function call itself for each child node (which
would grow the goroutine stack the same way `parseElement` above does), it keeps its own
manual to-do list — the `stack` slice — of nodes still waiting to be checked, adding and
removing items from that list with an ordinary loop. `*Node` means "a pointer to a `Node`" —
a pointer is a value that stores the location of some other piece of data in memory rather
than a copy of the data itself, so passing one around is cheap and lets multiple places refer
to the exact same node.

The explicit stack replaces recursion. Children are pushed in reverse so they pop in
document order (pre-order DFS — "depth-first search," a strategy for walking a tree that
dives all the way down one branch before backing up to try the next). `FindAllDeep` has the
same shape and collects all
matches. Neither can overflow the goroutine stack no matter how deep the tree is.

---

## Choosing between Parse and ParseWithConfig

| Situation | Recommended call |
|---|---|
| Trusted, internal data (config files you wrote) | `Parse` / `cgltf.Parse` |
| User uploads, API payloads, anything from the network | `ParseWithConfig(data, StrictConfig())` |
| Known-large internal documents | `ParseWithConfig(data, DefaultConfig())` |
| Performance benchmarks / generated test data | `ParseWithConfig(data, UnlimitedConfig())` — hard ceiling still applies |

---

!!! note "Try it"
    Run the test suite for all three modules from the workspace root (a "test suite" is a
    collection of small programs, each one called a "test," that runs a piece of code and
    checks the result matches what's expected — `go test` is the command that finds and runs
    them, printing PASS or FAIL for each one):

    ```bash
    cd /path/to/safeheaders-go
    go test ./cjson-go/... ./cgltf-go/... ./tinyxml2-go/... -v -count=1
    ```

    Expected outcome: all tests pass. Watch for lines like
    `--- PASS: TestUnmarshalArrayParallel` and `--- PASS: TestParseWithConfig`.
    If a limit test is included you will see inputs rejected with `ErrNestingTooDeep`,
    `ErrTooManyNodes`, or the `MaxArrayItems` error — those rejections are the feature,
    not a failure.

    To probe the depth limit specifically:

    ```bash
    go test ./tinyxml2-go/... -run 'Nesting|DepthCeiling' -v
    ```

    To run with the race detector (a "race" happens when two goroutines read and write the
    same piece of memory at the same time without coordination, producing unpredictable
    results; this extra check instruments the test run to catch that class of bug — it
    catches concurrent map writes and slice races):

    ```bash
    go test -race ./cjson-go/... ./cgltf-go/... ./tinyxml2-go/...
    ```

---

## Key takeaways

- **Delegate grammar to the stdlib; own everything above it.** `encoding/json` and
  `encoding/xml` handle byte-level parsing — you own size limits, cross-reference
  validation, and concurrency.
- **Caps must fire before work begins.** `UnmarshalArrayParallel` checks `MaxArrayItems`
  after the cheap token scan, before allocating results or spawning workers.
- **Validate semantics separately from parsing.** `cgltf.ValidateGLTF` is a distinct call
  you must make; `Parse` alone does not verify index references.
- **Hard depth ceilings are not configurable by design.** `maxNestingDepth = 10000` in
  tinyxml2-go exists precisely because stack overflows are fatal and `recover()` cannot
  help you.
- **Streaming callers own the size limit.** `UnmarshalStream` documents this explicitly —
  wrap the reader with `io.LimitReader` for untrusted network input.
