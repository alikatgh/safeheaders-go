# 05 · Tokenizing text: the jsmn JSON tokenizer

> **Objectives:** Understand the difference between a tokenizer and a full parser, and
> see how jsmn-go represents a JSON document as a flat array of tokens linked by parent
> indices — with zero per-token heap allocation during the walk.
> Learn how to use the `Parser`, read token metadata, and choose the right `Config` for
> untrusted input.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **Tokenizer vs parser.** A *tokenizer* reads raw bytes and tells you *where* each
  piece is (start byte, end byte, type). It does not build objects, fill structs, or
  validate business rules. A *parser* does all of that on top of the tokenizer's work.
  jsmn-go is a tokenizer; `encoding/json.Unmarshal` is a parser built on top of a
  tokenizer.
- **Flat array as a tree.** Instead of allocating a node per JSON value (object → map,
  array → slice, string → `string`, …), jsmn-go fills one pre-allocated `[]Token`.
  The tree shape is encoded in each token's `ParentIdx` field — a plain integer index
  into that same slice.
- **Start/End are byte offsets, not copies.** The token does not hold the string `"hello"`;
  it holds `Start: 7, End: 12` so you can slice the original `[]byte` yourself.
  No extra allocation per value.
- **Size is child count, not byte count.** For an object or array token, `Size` is how
  many direct children it has. For a string or primitive, `Size` is 0.
- **`Config` is a safety fuse.** `DefaultConfig` caps input at 100 MB and tokens at
  1 000 000. `StrictConfig` tightens those to 10 MB / 100 000. You pick the fuse
  before parsing; the tokenizer returns an error if the input blows it.
- **Why it matters:** Tokenizing without allocating per-value is the foundation of
  fast, memory-bounded JSON processing. Every other module in this workspace (cjson-go,
  cgltf-go) sits on top of this idea.

**See it — a tree with no nodes.** The input below tokenizes into one flat
`[]Token`. The tree shape is *not* a graph of allocated objects — it lives entirely
in each token's `ParentIdx` (the curved arrows). `Start`/`End` are byte offsets into
the original input, so no value is ever copied.

<svg viewBox="0 0 720 330" role="img" aria-labelledby="tk-t tk-d" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="tk-t">A flat token array linked by parent index</title>
  <desc id="tk-d">The JSON object brace-quote-k-quote-colon-bracket-1-comma-2-bracket-brace tokenizes into five tokens whose ParentIdx fields encode the tree.</desc>
  <defs><marker id="tk-ah" markerWidth="9" markerHeight="9" refX="6" refY="3" orient="auto"><path d="M0,0 L7,3 L0,6 Z" fill="var(--md-accent-fg-color,#00897b)"/></marker></defs>
  <text x="40" y="40" font-size="12" fill="var(--md-default-fg-color--light)">input bytes (index above each):</text>
  <g font-family="ui-monospace,monospace">
    <g font-size="9" fill="var(--md-default-fg-color--light)" text-anchor="middle"><text x="159" y="58">0</text><text x="199" y="58">1</text><text x="239" y="58">2</text><text x="279" y="58">3</text><text x="319" y="58">4</text><text x="359" y="58">5</text><text x="399" y="58">6</text><text x="439" y="58">7</text><text x="479" y="58">8</text><text x="519" y="58">9</text><text x="559" y="58">10</text></g>
    <g font-size="15" fill="currentColor" text-anchor="middle"><text x="159" y="84">{</text><text x="199" y="84">"</text><text x="239" y="84">k</text><text x="279" y="84">"</text><text x="319" y="84">:</text><text x="359" y="84">[</text><text x="399" y="84">1</text><text x="439" y="84">,</text><text x="479" y="84">2</text><text x="519" y="84">]</text><text x="559" y="84">}</text></g>
  </g>
  <line x1="140" y1="94" x2="580" y2="94" stroke="var(--md-default-fg-color--lightest)"/>
  <path d="M228,206 C228,150 96,150 96,206" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.3" marker-end="url(#tk-ah)"/>
  <path d="M360,206 C360,128 96,128 96,206" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.3" marker-end="url(#tk-ah)"/>
  <path d="M492,206 C492,162 360,162 360,206" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.3" marker-end="url(#tk-ah)"/>
  <path d="M624,206 C624,140 360,140 360,206" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.3" marker-end="url(#tk-ah)"/>
  <g font-size="11">
    <rect x="40" y="206" width="112" height="86" rx="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="96" y="226" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">t0 Object</text><text x="96" y="246" text-anchor="middle" fill="var(--md-default-fg-color--light)">Start 0 · End 11</text><text x="96" y="263" text-anchor="middle" fill="var(--md-default-fg-color--light)">Size 2</text><text x="96" y="282" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">Parent -1</text>
    <rect x="172" y="206" width="112" height="86" rx="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="228" y="226" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">t1 String</text><text x="228" y="246" text-anchor="middle" fill="var(--md-default-fg-color--light)">Start 2 · End 3</text><text x="228" y="263" text-anchor="middle" fill="var(--md-default-fg-color--light)">"k" (key)</text><text x="228" y="282" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">Parent 0</text>
    <rect x="304" y="206" width="112" height="86" rx="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="360" y="226" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">t2 Array</text><text x="360" y="246" text-anchor="middle" fill="var(--md-default-fg-color--light)">Start 5 · End 10</text><text x="360" y="263" text-anchor="middle" fill="var(--md-default-fg-color--light)">Size 2</text><text x="360" y="282" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">Parent 0</text>
    <rect x="436" y="206" width="112" height="86" rx="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="492" y="226" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">t3 Primitive</text><text x="492" y="246" text-anchor="middle" fill="var(--md-default-fg-color--light)">Start 6 · End 7</text><text x="492" y="263" text-anchor="middle" fill="var(--md-default-fg-color--light)">1</text><text x="492" y="282" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">Parent 2</text>
    <rect x="568" y="206" width="112" height="86" rx="5" fill="none" stroke="currentColor" stroke-width="1.5"/><text x="624" y="226" text-anchor="middle" font-size="12" font-weight="600" fill="currentColor">t4 Primitive</text><text x="624" y="246" text-anchor="middle" fill="var(--md-default-fg-color--light)">Start 8 · End 9</text><text x="624" y="263" text-anchor="middle" fill="var(--md-default-fg-color--light)">2</text><text x="624" y="282" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">Parent 2</text>
  </g>
  <text x="360" y="314" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">arrows = ParentIdx · one pre-allocated []Token · zero per-value heap allocation</text>
</svg>

---

## The Token struct

All the information jsmn-go records about one JSON value lives in four fields.
From `jsmn-go/jsmn.go`:

```go
type Token struct {
    Type      TokenType // Object | Array | String | Primitive
    Start     int       // byte offset of the first character (inclusive)
    End       int       // byte offset after the last character (exclusive)
    Size      int       // number of direct children (0 for strings/primitives)
    ParentIdx int       // index of the enclosing token in the flat slice (-1 = top-level)
}
```

`Start` and `End` follow the usual Go half-open convention: `json[tok.Start:tok.End]`
gives you the raw bytes of that value, including any surrounding quotes for strings.

The four `TokenType` values match the four structural elements of JSON:

| Constant    | Matches                        |
|-------------|-------------------------------|
| `Object`    | `{ … }`                       |
| `Array`     | `[ … ]`                       |
| `String`    | `"…"` (keys and string values)|
| `Primitive` | numbers, `true`, `false`, `null` |

---

## How the flat array forms a tree

Consider the tiny document:

```json
{"name":"Alice","age":30}
```

After parsing, the token slice looks like this (indices 0–4):

```
idx  Type       Start  End  Size  ParentIdx
 0   Object       0    25    2      -1
 1   String       2     6    0       0     ← key "name"
 2   String       9    14    0       0     ← value "Alice"
 3   String      17    20    0       0     ← key "age"
 4   Primitive   23    25    0       0     ← value 30
```

No heap node was allocated for the object or any of its values. The whole tree
is read from a single `[]Token` slice that was filled in one pass over the
input bytes.

To extract the raw bytes of a token you just slice the original input:

```go
// "Alice" — includes the surrounding quotes
raw := json[tokens[2].Start:tokens[2].End]

// strip quotes for a string token
content := json[tokens[2].Start:tokens[2].End] // e.g. `"Alice"`
unquoted := json[tokens[2].Start+1 : tokens[2].End-1] // e.g. `Alice`
```

---

## Creating a Parser and walking tokens

The simplest path: `NewParser` + `Parse`. From `jsmn-go/jsmn.go`:

```go
// NewParser creates a new parser with space for numTokens.
func NewParser(numTokens int) *Parser {
    return &Parser{
        tokens: make([]Token, numTokens),
    }
}

// Parse tokenizes the JSON input, returning the number of tokens or an error.
func (p *Parser) Parse(json []byte) (int, error) { … }

// Tokens returns the parsed tokens (slice up to toknext).
func (p *Parser) Tokens() []Token {
    return p.tokens[:p.toknext]
}
```

A minimal usage pattern:

```go
package main

import (
    "fmt"
    jsmngo "github.com/yourorg/safeheaders-go/jsmn-go"
)

func main() {
    input := []byte(`{"lang":"Go","year":2009}`)

    p := jsmngo.NewParser(32)
    n, err := p.Parse(input)
    if err != nil {
        panic(err)
    }

    for i, tok := range p.Tokens()[:n] {
        raw := input[tok.Start:tok.End]
        fmt.Printf("[%d] type=%-10s size=%d parent=%d  %s\n",
            i, tok.Type, tok.Size, tok.ParentIdx, raw)
    }
}
```

Running this prints one line per token, making the flat-tree structure visible.

---

## Walking the tree by parent index

Because every token records its parent's index, you can filter children of any
node with a plain loop — no recursion, no stack:

```go
// direct children of token at parentIdx
func directChildren(tokens []jsmngo.Token, parentIdx int) []int {
    var children []int
    for i, tok := range tokens {
        if tok.ParentIdx == parentIdx {
            children = append(children, i)
        }
    }
    return children
}
```

For a deeply nested document this linear scan can be replaced by a single pass
that builds an adjacency list — still no extra allocations on the token side.

!!! tip "Key/value pairing in objects"
    In jsmn-go, object keys and their values appear consecutively in the token
    slice: key at index *i*, value at *i + 1*. Both have the same `ParentIdx`
    (the object). You can iterate object fields with a step of 2:

    ```go
    for i := 1; i < len(tokens); i += 2 {
        key   := input[tokens[i].Start+1 : tokens[i].End-1]   // strip quotes
        value := input[tokens[i+1].Start : tokens[i+1].End]
        fmt.Printf("%s => %s\n", key, value)
    }
    ```

    This works because the parser emits tokens in document order.

---

## Choosing a Config

For production code that accepts user-supplied JSON, use `ParseWithConfig` from
`jsmn-go/config.go` instead of the bare `Parse` method.

```go
// DefaultConfig: 100 MB input, 1 000 000 tokens
func DefaultConfig() *Config { … }

// StrictConfig: 10 MB input, 100 000 tokens — for untrusted input
func StrictConfig() *Config { … }

// UnlimitedConfig: no caps — use only in benchmarks or controlled pipelines
func UnlimitedConfig() *Config { … }
```

Usage:

```go
import (
    "context"
    jsmngo "github.com/yourorg/safeheaders-go/jsmn-go"
)

tokens, err := jsmngo.ParseWithConfig(context.Background(), input, jsmngo.StrictConfig())
if err != nil {
    // could be ErrInputTooLarge, ErrTooManyTokens, ErrEmptyInput, or a parse error
    log.Fatalf("tokenize: %v", err)
}
```

The three sentinel errors let callers distinguish limit violations from malformed JSON:

```go
switch {
case errors.Is(err, jsmngo.ErrInputTooLarge):
    http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
case errors.Is(err, jsmngo.ErrTooManyTokens):
    http.Error(w, "document too complex", http.StatusBadRequest)
case err != nil:
    http.Error(w, "invalid JSON", http.StatusBadRequest)
}
```

!!! warning "UnlimitedConfig in production"
    `UnlimitedConfig` exists for benchmarks and for `ParseParallel`'s internal
    fast-path. Passing it to a public HTTP handler means a 2 GB JSON blob will
    attempt to fill a token slice of that size — or exhaust heap and OOM.
    Always prefer `DefaultConfig` or `StrictConfig` on the boundary.

---

## How `allocToken` avoids a hard capacity limit

Early versions required you to pre-size the token slice exactly. The current
implementation in `jsmn-go/jsmn.go` grows automatically:

```go
func (p *Parser) allocToken(tok Token) error {
    if p.toknext >= len(p.tokens) {
        // grow instead of returning an error
        p.tokens = append(p.tokens, Token{})
    }
    p.tokens[p.toknext] = tok
    if p.toksuper != -1 {
        p.tokens[p.toksuper].Size++
    }
    p.toknext++
    return nil
}
```

`append` doubles capacity when it needs to grow (standard Go amortisation), so
the total number of allocations is O(log n), not O(n). The `Config.MaxTokens`
cap in `ParseWithConfig` is enforced *after* `Parse` returns, not inside
`allocToken` — so the growth is still bounded by the config when you use the
safe entry point.

---

## Try it

!!! note "Try it"
    From the repo root, run the jsmn-go test suite:

    ```bash
    cd jsmn-go && go test ./... -v -count=1
    ```

    Expected outcome: all tests pass; output includes lines like
    `--- PASS: TestParse`, `--- PASS: TestParseWithConfig`, and
    `--- PASS: TestParallelVsSerial` confirming that parallel and serial
    tokenization produce identical token slices for the same input.

!!! note "Try it — race detector"
    The parallel tokenizer uses goroutines. Verify no data races:

    ```bash
    cd jsmn-go && go test ./... -race -count=1
    ```

    Expected outcome: `PASS` with no `DATA RACE` reports. The parallel path
    in `jsmn-go/parallel.go` was designed to share no mutable state between
    workers; each chunk gets its own `Parser` instance.

---

## Under the hood: how `Parse` handles closing brackets

When the parser sees `}` or `]`, it closes the current container by writing
the current position into the *open* container token's `End` field, then
climbs up via `ParentIdx`. From `jsmn-go/jsmn.go`:

```go
case '}', ']':
    if p.toksuper != -1 {
        p.tokens[p.toksuper].End = p.pos + 1
        p.toksuper = p.tokens[p.toksuper].ParentIdx
    }
    p.pos++
```

`p.toksuper` is effectively a cursor pointing at the innermost open container.
Opening a `{` or `[` pushes a new token and updates `toksuper` to point at it.
Closing `}` or `]` seals that token and pops `toksuper` back to the parent —
without any explicit stack allocation, because the parent chain is already
encoded in `ParentIdx`.

At the end of `Parse`, any token whose `End` is still `-1` (a top-level
primitive that extends to the end of input) gets `End = len(json)`, and the
parser returns an error if any container is still open:

```go
if p.toksuper != -1 {
    return 0, errors.New("unclosed object or array")
}
```

---

## Related lessons

- [Lesson 06](14-the-deadlock-bug.md) — how `ParseParallel` splits the input at
  top-level commas, runs per-chunk parsers concurrently, and then merges the token
  slices (including the `ParentIdx` rebasing fix that was a real production bug).
- [Lesson 07](14-the-deadlock-bug.md) — the channel-buffer deadlock that lurked
  inside the parallel worker pool, and the watchdog test that catches it.

---

## Key takeaways

- A **tokenizer** records *where* values are (byte offsets + type); it does not
  copy or interpret them. That is intentionally all jsmn-go does.
- The **flat `[]Token` array** encodes a full tree through `ParentIdx` integer
  links — zero per-node heap allocation during traversal.
- **`Start`/`End` are half-open byte offsets** into the original input slice;
  slice them yourself, pay nothing extra.
- **`Size`** on an Object or Array token is its direct child count, not its byte
  size — useful for pre-allocating a map or slice before you walk the children.
- Always use **`ParseWithConfig` with `StrictConfig`** (or at least `DefaultConfig`)
  on any input you did not generate yourself; the sentinel errors
  (`ErrInputTooLarge`, `ErrTooManyTokens`) give you clean HTTP status codes.
