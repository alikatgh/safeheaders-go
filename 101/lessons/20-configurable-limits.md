# 20 · The configurable-limits pattern

> **Objectives:** Understand how a `Config` struct with named preset constructors
> gives callers safe defaults while still allowing tuning without recompiling.
> See how `validateInput` and sentinel errors turn silent failures into explicit,
> testable contracts. Learn when to reach for `StrictConfig` vs `UnlimitedConfig`.
> Estimated time: 15 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **`Config` struct** = "a thermostat panel installed before the boiler ever turns on." It is a plain Go struct whose exported `int` fields declare exactly how much work the parser is allowed to do — set once, enforced on every call.
- **`DefaultConfig`** = "the factory setting that ships with the appliance." It works for most callers (100 MB, 1 M tokens) without requiring them to think about limits at all; they just call the function and get safe behaviour.
- **`StrictConfig`** = "the tamper-proof lock a landlord installs on the thermostat." Lower caps (10 MB, 100 K tokens) in the same shape — drop it in whenever input arrives from the internet or an untrusted file and you don't want a caller accidentally raising the ceiling.
- **`UnlimitedConfig`** = "the master-key override that lives in the facilities closet." It sets every limit to `0` so the `MaxInputSize > 0` guard is skipped entirely; opting out of limits is a conscious, named act in source code, never an accident.
- **`Validate()` vs `validateInput()`** = "the pre-flight checklist vs the boarding-gate scanner." `Validate()` runs at construction and rejects bad Config values (negative limits are programming errors); `validateInput()` runs per call and rejects oversized payloads at runtime.
- **Sentinel errors (`ErrInputTooLarge`, `ErrTooManyTokens`)** = "a labelled circuit breaker instead of a melted fuse." Because they are package-level `errors.New` values, callers use `errors.Is()` to match the exact failure reason and map it to an HTTP status — no string parsing required.

**Why it matters:** A library that is unsafe by default will eventually be called
with default settings on untrusted data — and that is how OOM crashes and
denial-of-service bugs happen in production.

**See it — the three preset constructors and the two-stage validation gate.**

<svg viewBox="0 0 700 370" role="img" aria-labelledby="t20 d20" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t20">Configurable-limits pattern: three preset constructors feed into ParseWithConfig, which runs Validate then validateInput before doing any parsing work.</title>
  <desc id="d20">Three boxes on the left represent DefaultConfig, StrictConfig, and UnlimitedConfig. Arrows lead right into a ParseWithConfig box. Inside that box two sequential steps are shown: Validate() checks the Config, then validateInput() checks the payload. A final arrow exits right labelled "parse / return tokens".</desc>
  <defs>
    <marker id="l20-arrow" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L0,7 L8,3.5 Z" fill="var(--md-default-fg-color--light)"/>
    </marker>
    <marker id="l20-arrow-accent" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L0,7 L8,3.5 Z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
  </defs>

  <!-- Preset constructor boxes -->
  <rect x="20" y="40" width="150" height="44" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="95" y="58" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">DefaultConfig()</text>
  <text x="95" y="75" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">100 MB · 1 M tokens</text>

  <rect x="20" y="158" width="150" height="44" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="95" y="176" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">StrictConfig()</text>
  <text x="95" y="193" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">10 MB · 100 K tokens</text>

  <rect x="20" y="278" width="150" height="44" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.4"/>
  <text x="95" y="296" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">UnlimitedConfig()</text>
  <text x="95" y="313" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light)">0 · 0 (skip checks)</text>

  <!-- Arrows from presets to ParseWithConfig -->
  <line x1="170" y1="62" x2="248" y2="155" stroke="var(--md-default-fg-color--light)" stroke-width="1.3" marker-end="url(#l20-arrow)"/>
  <line x1="170" y1="180" x2="248" y2="180" stroke="var(--md-default-fg-color--light)" stroke-width="1.3" marker-end="url(#l20-arrow)"/>
  <line x1="170" y1="300" x2="248" y2="205" stroke="var(--md-default-fg-color--light)" stroke-width="1.3" marker-end="url(#l20-arrow)"/>

  <!-- ParseWithConfig outer box -->
  <rect x="252" y="100" width="310" height="160" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.8"/>
  <text x="407" y="122" text-anchor="middle" font-size="13" fill="var(--md-accent-fg-color,#00897b)" font-weight="700">ParseWithConfig</text>

  <!-- Step 1: Validate() -->
  <rect x="272" y="134" width="130" height="38" rx="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="337" y="149" text-anchor="middle" font-size="11" fill="#fff" font-weight="600">1 · Validate()</text>
  <text x="337" y="164" text-anchor="middle" font-size="9" fill="#fff">Config fields ≥ 0?</text>

  <!-- Arrow between steps -->
  <line x1="402" y1="153" x2="422" y2="153" stroke="var(--md-default-fg-color--light)" stroke-width="1.2" marker-end="url(#l20-arrow)"/>

  <!-- Step 2: validateInput() -->
  <rect x="424" y="134" width="122" height="38" rx="5" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="485" y="149" text-anchor="middle" font-size="11" fill="#fff" font-weight="600">2 · validateInput()</text>
  <text x="485" y="164" text-anchor="middle" font-size="9" fill="#fff">payload within limit?</text>

  <!-- Error exit downward from step 2 -->
  <line x1="485" y1="172" x2="485" y2="222" stroke="#e5484d" stroke-width="1.3" marker-end="url(#l20-arrow)"/>
  <rect x="430" y="224" width="130" height="32" rx="5" fill="none" stroke="#e5484d" stroke-width="1.3"/>
  <text x="495" y="238" text-anchor="middle" font-size="10" fill="#e5484d" font-weight="600">ErrInputTooLarge</text>
  <text x="495" y="250" text-anchor="middle" font-size="9" fill="#e5484d">ErrTooManyTokens …</text>

  <!-- Arrow out to parse work -->
  <line x1="562" y1="153" x2="630" y2="153" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l20-arrow-accent)"/>
  <text x="638" y="148" text-anchor="start" font-size="11" fill="currentColor">parse</text>
  <text x="638" y="161" text-anchor="start" font-size="11" fill="currentColor">tokens</text>
</svg>

---

## The Config struct

Both `jsmn-go` and `tinyxml2-go` follow the same pattern. Here is the JSON
tokenizer's version, from [`jsmn-go/config.go`](src/jsmn-go-config-go.md):

```go
// Config holds parsing configuration and limits.
type Config struct {
    // MaxInputSize limits the maximum JSON input size in bytes.
    // Default: 100MB. Set to 0 for unlimited (not recommended).
    MaxInputSize int

    // MaxTokens limits the maximum number of tokens that can be parsed.
    // Default: 1,000,000. Set to 0 for unlimited (not recommended).
    MaxTokens int

    // InitialTokenCapacity is the initial capacity for the token slice.
    // Default: inputSize / 4. The slice will grow automatically if needed.
    InitialTokenCapacity int

    // ParallelThreshold is the minimum input size (in bytes) to enable parallel parsing.
    // Default: 4KB.
    ParallelThreshold int
}
```

And the XML parser's version, from [`tinyxml2-go/config.go`](src/tinyxml2-go-config-go.md):

```go
type Config struct {
    MaxInputSize    int  // bytes
    MaxNodeCount    int  // total element nodes
    MaxNestingDepth int  // how many levels deep elements may nest
}
```

Notice the shape is the same — a plain struct, all exported fields, all `int`.
No interface magic, no build tags. A caller can read the field names and
understand the contract without looking at documentation.

`MaxNestingDepth` is the field that prevents **billion-laughs / stack-exhaustion
attacks**: deeply nested XML causes the parser to recurse. Without a ceiling the
runtime panics with a stack overflow that `recover` cannot catch (see
[Lesson 15](19-recursion-and-billion-laughs.md)). The tinyxml2-go parser enforces a hard
ceiling of `10 000` regardless of this field — Config just sets the
caller-visible soft limit.

---

## Three preset constructors

Each module ships three constructor functions. From `jsmn-go/config.go`:

```go
func DefaultConfig() *Config {
    return &Config{
        MaxInputSize:      100 * 1024 * 1024, // 100MB
        MaxTokens:         1_000_000,
        InitialTokenCapacity: 0,              // auto-calculated
        ParallelThreshold: 4 * 1024,          // 4KB
    }
}

func StrictConfig() *Config {
    return &Config{
        MaxInputSize:      10 * 1024 * 1024,  // 10MB
        MaxTokens:         100_000,
        InitialTokenCapacity: 0,
        ParallelThreshold: 4 * 1024,
    }
}

func UnlimitedConfig() *Config {
    return &Config{
        MaxInputSize:      0, // unlimited
        MaxTokens:         0, // unlimited
        InitialTokenCapacity: 0,
        ParallelThreshold: 4 * 1024,
    }
}
```

The same trio appears in `tinyxml2-go/config.go` with matching field names and
comments. Consistent naming across modules matters: once a developer learns this
pattern in `jsmn-go`, they can navigate `tinyxml2-go` without reading a new
README.

!!! tip "Pick the right preset"
    - **Internal tooling, trusted files** → `DefaultConfig()` (100 MB, 1M tokens).
    - **API endpoint, user-uploaded data** → `StrictConfig()` (10 MB, 100K tokens/nodes).
    - **Offline batch pipeline where you own the data** → `UnlimitedConfig()` — but
      name it explicitly so the next reader knows the choice was deliberate.

---

## Validate: catching bad Config at construction time

`Config.Validate()` runs before any parsing work. From `jsmn-go/config.go`:

```go
func (c *Config) Validate() error {
    if c.MaxInputSize < 0 {
        return errors.New("MaxInputSize cannot be negative")
    }
    if c.MaxTokens < 0 {
        return errors.New("MaxTokens cannot be negative")
    }
    if c.InitialTokenCapacity < 0 {
        return errors.New("InitialTokenCapacity cannot be negative")
    }
    if c.ParallelThreshold < 0 {
        return errors.New("ParallelThreshold cannot be negative")
    }
    return nil
}
```

`tinyxml2-go/config.go` mirrors this for its three fields. The rule is: a `0`
means "unlimited" (the caller opted out intentionally), but a negative number
is a programming error — fail loudly at construction, not silently at parse
time.

---

## validateInput: enforcing limits per call

`Validate()` checks the Config itself. `validateInput()` checks the actual bytes
handed in. From `jsmn-go/config.go`:

```go
func (c *Config) validateInput(data []byte) error {
    if len(data) == 0 {
        return ErrEmptyInput
    }
    if c.MaxInputSize > 0 && len(data) > c.MaxInputSize {
        return ErrInputTooLarge
    }
    return nil
}
```

`tinyxml2-go/config.go` is identical in structure:

```go
func (c *Config) validateInput(data []byte) error {
    if len(data) == 0 {
        return ErrEmptyInput
    }
    if c.MaxInputSize > 0 && len(data) > c.MaxInputSize {
        return ErrInputTooLarge
    }
    return nil
}
```

The `c.MaxInputSize > 0` guard is the "unlimited" opt-out path: when the caller
passes `UnlimitedConfig()`, `MaxInputSize` is `0`, so the size check is skipped
entirely. This avoids a footgun where `0` accidentally means "reject everything".

---

## Sentinel errors

Both modules declare their errors as package-level variables. From
`jsmn-go/config.go`:

```go
var (
    ErrInputTooLarge = errors.New("input size exceeds maximum allowed")
    ErrTooManyTokens = errors.New("token count exceeds maximum allowed")
    ErrEmptyInput    = errors.New("empty input")
)
```

From `tinyxml2-go/config.go`:

```go
var (
    ErrInputTooLarge = errors.New("input size exceeds maximum allowed")
    ErrTooManyNodes  = errors.New("node count exceeds maximum allowed")
    ErrNestingTooDeep = errors.New("nesting depth exceeds maximum allowed")
    ErrEmptyInput    = errors.New("empty input")
)
```

Because these are plain `errors.New` values (not custom types), callers check
them with `errors.Is`:

```go
tokens, err := jsmngo.ParseWithConfig(ctx, data, jsmngo.StrictConfig())
if errors.Is(err, jsmngo.ErrInputTooLarge) {
    http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
    return
}
```

This lets you write tests that pin the exact error path, not just "an error
occurred".

!!! warning "Token count checked after parsing"
    In `jsmn-go`, `MaxTokens` is enforced *after* `Parse` runs — the tokenizer
    does its work and then the count is compared. This means the allocator still
    touches memory up to `MaxTokens`. If you need a hard pre-parse cap, set
    `MaxInputSize` tightly; the token count is a second line of defense against
    inputs that are small in bytes but generate many tokens (e.g. `[1,2,3,...,999999]`).

---

## The ParseWithConfig entry point

`ParseWithConfig` is the public function that wires everything together. From
`jsmn-go/config.go` (condensed):

```go
func ParseWithConfig(ctx context.Context, data []byte, config *Config) ([]Token, error) {
    if config == nil {
        config = DefaultConfig()  // nil-safe: always have a config
    }

    if err := config.Validate(); err != nil {
        return nil, err
    }

    if err := config.validateInput(data); err != nil {
        return nil, err
    }

    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    if config.shouldUseParallel(len(data)) {
        return parseParallelWithConfig(ctx, data, config)
    }

    // ... serial path
}
```

The call order is always: validate Config → validate input → check context →
do work. This sequence ensures that any limit violation is caught before a
single byte of the input is processed, and that context cancellation is
respected before any goroutines are launched.

!!! note "Try it"
    Run the config-related tests for both modules:

    ```bash
    cd /path/to/safeheaders-go
    go test ./jsmn-go/... -run TestConfig -v
    go test ./tinyxml2-go/... -run TestConfig -v
    ```

    Expected outcome: all `TestConfig*` cases pass. Look for cases named
    `TestConfigDefaultConfig`, `TestConfigStrictConfig`, and
    `TestConfigUnlimitedConfig` — they verify that `DefaultConfig()` rejects a
    200 MB input, `StrictConfig()` rejects 20 MB, and `UnlimitedConfig()` accepts
    the same payload without error.

    To verify the sentinel error surfaces correctly:

    ```bash
    go test ./jsmn-go/... -run TestParseWithConfig -v
    go test ./tinyxml2-go/... -run TestParseWithConfig -v
    ```

---

## Why not just use constants?

You might wonder: why a struct and constructors instead of package-level `const`
values? Three reasons:

1. **Per-call tuning.** A single binary may parse trusted internal data with
   `DefaultConfig` and untrusted user uploads with `StrictConfig` — in the same
   process, on the same code path.
2. **Testability.** A test can construct a `Config{MaxInputSize: 100}` and verify
   limit enforcement on tiny inputs without manufacturing a 10 MB fixture.
3. **Future extensibility.** Adding a new field (say, `MaxAttributeCount`) is a
   backwards-compatible change — existing callers using `DefaultConfig()` get the
   new field's default automatically.

---

## Key takeaways

- A `Config` struct with `DefaultConfig` / `StrictConfig` / `UnlimitedConfig`
  constructors gives callers a safe default, a hardened preset, and an explicit
  escape hatch — without exposing internal constants.
- `Validate()` catches programming errors (negative limits) at construction;
  `validateInput()` catches runtime violations (oversized payloads) per call.
- `0` means "unlimited" for every limit field; callers must opt in to `0`
  explicitly via `UnlimitedConfig()` rather than getting it by accident.
- Sentinel errors (`ErrInputTooLarge`, `ErrTooManyTokens`, `ErrNestingTooDeep`)
  make limit violations testable and HTTP-mappable without parsing error strings.
- The `ParseWithConfig` entry point follows a strict call order: validate Config,
  validate input, check context, do work — ensuring no CPU or memory is spent on
  inputs that violate a limit.
