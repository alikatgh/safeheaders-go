# 06 · Standing on the stdlib: cjson, cgltf, tinyxml2

> **Objectives:** Understand why wrapping `encoding/json` and `encoding/xml` is a legitimate
> porting strategy, see what the wrapper still has to own (validation, limits, concurrency), and
> know when to call `ParseWithConfig` instead of bare `Parse`.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **Wrapping, not reimplementing.** The C originals (cJSON, cgltf, tinyxml2) parse bytes
  byte-by-byte in hand-written C. Go already ships battle-tested parsers in the standard library.
  These modules plug those parsers in and add the things the stdlib leaves to you.
- **The stdlib is fuzzed and CVE-tracked upstream.** `encoding/json` and `encoding/xml` receive
  constant fuzzing from the Go team. You inherit that work for free.
- **"Parsing" is only the first 20%.** The rest is: reject absurd input sizes before they land
  in RAM, validate that cross-references inside the document actually point somewhere real, and
  scale to multiple documents at once.
- **Limits are not optional for untrusted input.** A 3-byte JSON body can describe an array of
  ten million items. Without a cap, your process allocates ten million slots before it reads a
  single element value.
- **Validation is domain knowledge, not parser knowledge.** `encoding/json` cannot know that
  glTF scene 5 referencing node 99 is invalid when the model only defines 3 nodes. You own that
  check.
- **Concurrency is composable on top.** The stdlib parsers are not concurrent, but once parsing
  is a pure function (bytes in, struct out) you can fan multiple calls across a worker pool.

**Why it matters:** choosing the right porting strategy halves the attack surface you have to
audit — use the stdlib for the grammar, write Go for everything else.

---

## cjson-go: JSON with an array-size brake

`cjson-go/cjson.go` is a thin layer over `encoding/json`.
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

`cgltf-go/cgltf.go` is structured around a deliberate two-step: **parse** (grammar),
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

`tinyxml2-go/tinyxml2.go` wraps `encoding/xml` into a DOM, which means it builds a
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

The config lives in `tinyxml2-go/config.go`:

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
    go test ./tinyxml2-go/... -run TestNesting -v
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
