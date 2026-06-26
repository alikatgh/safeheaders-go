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

  <!-- Input label -->
  <text x="18" y="160" font-size="12" text-anchor="middle" dominant-baseline="middle" fill="currentColor" transform="rotate(-90,18,160)">raw bytes</text>

  <!-- Arrow: input → stdlib -->
  <line x1="34" y1="155" x2="88" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>

  <!-- Box 1: stdlib parser -->
  <rect x="90" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="165" y="135" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">stdlib parser</text>
  <text x="165" y="153" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">encoding/json</text>
  <text x="165" y="167" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">encoding/xml</text>
  <text x="165" y="185" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">grammar only</text>

  <!-- Arrow: stdlib → limits -->
  <line x1="241" y1="155" x2="275" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>

  <!-- Box 2: Go limit checks -->
  <rect x="277" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="352" y="130" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">Go limit checks</text>
  <text x="352" y="149" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">MaxArrayItems</text>
  <text x="352" y="163" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">MaxInputSize</text>
  <text x="352" y="177" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">maxNestingDepth</text>
  <text x="352" y="195" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">reject early</text>

  <!-- Arrow: limits → validation -->
  <line x1="428" y1="155" x2="462" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>

  <!-- Box 3: Go validation -->
  <rect x="464" y="100" width="150" height="110" rx="8" ry="8" fill="none" stroke="var(--md-default-fg-color--light,currentColor)" stroke-width="1.5"/>
  <text x="539" y="130" font-size="12" font-weight="bold" text-anchor="middle" fill="currentColor">Go validation</text>
  <text x="539" y="149" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">ValidateGLTF</text>
  <text x="539" y="163" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">cross-ref checks</text>
  <text x="539" y="177" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--light,currentColor)">domain rules</text>
  <text x="539" y="195" font-size="10" text-anchor="middle" fill="var(--md-default-fg-color--lightest,currentColor)">semantics</text>

  <!-- Arrow: validation → output -->
  <line x1="615" y1="155" x2="665" y2="155" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="2" marker-end="url(#l06-arrow)"/>
  <text x="683" y="150" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">safe</text>
  <text x="683" y="163" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">struct</text>

  <!-- Rejection arrow down from limits box -->
  <line x1="352" y1="211" x2="352" y2="265" stroke="#e5484d" stroke-width="2" marker-end="url(#l06-arrow-err)"/>
  <rect x="295" y="268" width="115" height="28" rx="5" ry="5" fill="#e5484d" opacity="0.12" stroke="#e5484d" stroke-width="1"/>
  <text x="352" y="284" font-size="10" text-anchor="middle" fill="#e5484d">error returned</text>

  <!-- Rejection arrow down from validation box -->
  <line x1="539" y1="211" x2="539" y2="265" stroke="#e5484d" stroke-width="2" marker-end="url(#l06-arrow-err)"/>
  <rect x="482" y="268" width="115" height="28" rx="5" ry="5" fill="#e5484d" opacity="0.12" stroke="#e5484d" stroke-width="1"/>
  <text x="539" y="284" font-size="10" text-anchor="middle" fill="#e5484d">error returned</text>
</svg>

---

## cjson-go: JSON with an array-size brake

[`cjson-go/cjson.go`](src/cjson-go-cjson-go.md) is a thin layer over `encoding/json`.
The interesting addition is `UnmarshalArrayParallel`.

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

The first `json.Unmarshal` into `[]json.RawMessage` is cheap: it records offsets, it does
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

Each worker picks an index off `jobs`, decodes one element, and writes to its own slot in
`results` (no lock needed — slots never overlap). The `errs` channel is buffered to
`numWorkers` so a failing worker can always send without blocking.

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

`Parse` only checks that the JSON is well-formed and that the mandatory `asset.version`
field is present. Integer indices scattered throughout the file — scene index, node
indices, mesh indices — are not checked here.

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

`ParseBatch` respects a `context.Context` — cancel it and workers stop picking up new work
within one item. Each result carries the original index so the output slice preserves
input order regardless of which worker finishes first.

!!! tip "Always validate after batch parse"
    `ParseBatch` calls `Parse` internally; it does **not** call `ValidateGLTF`. After the
    batch returns, loop over results and call `ValidateGLTF` on each.

---

## tinyxml2-go: depth limits against the billion-laughs attack

[`tinyxml2-go/tinyxml2.go`](src/tinyxml2-go-tinyxml2-go.md) wraps `encoding/xml` into a DOM, which means it builds a
recursive tree in memory. Recursive trees have a specific failure mode: deeply nested
input can overflow the goroutine stack — and unlike a panic, a stack overflow is a
**fatal** error that `recover()` cannot catch.

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

The explicit stack replaces recursion. Children are pushed in reverse so they pop in
document order (pre-order DFS). `FindAllDeep` has the same shape and collects all
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
    Run the test suite for all three modules from the workspace root:

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

    To run with the race detector (catches concurrent map writes and slice races):

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
