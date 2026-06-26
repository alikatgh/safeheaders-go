# 20 · The configurable-limits pattern

> **Objectives:** Understand how a `Config` struct with named preset constructors
> gives callers safe defaults while still allowing tuning without recompiling.
> See how `validateInput` and sentinel errors turn silent failures into explicit,
> testable contracts. Learn when to reach for `StrictConfig` vs `UnlimitedConfig`.
> Estimated time: 15 minutes.

---

## What this actually means (plain English)

- **A parser is like a gate at a venue.** Without a Config, the gate lets everyone
  in regardless of crowd size. With a Config, you set a fire-code limit up front,
  and the gate refuses entry once the limit is hit.
- **`DefaultConfig` is the sensible house rule.** It works for most callers
  without them having to think about limits at all.
- **`StrictConfig` is the bouncer at a high-security event.** Lower caps,
  same shape — drop it in when input comes from the internet or an untrusted file.
- **`UnlimitedConfig` is the override switch.** It exists so you can opt out
  explicitly in code, not by accident. The comment "use with caution" is
  intentional.
- **Sentinel errors (`ErrInputTooLarge`, `ErrTooManyTokens`) are named
  smoke alarms.** They let callers `errors.Is()` on the exact failure reason
  instead of parsing an error string.

**Why it matters:** A library that is unsafe by default will eventually be called
with default settings on untrusted data — and that is how OOM crashes and
denial-of-service bugs happen in production.

---

## The Config struct

Both `jsmn-go` and `tinyxml2-go` follow the same pattern. Here is the JSON
tokenizer's version, from `jsmn-go/config.go`:

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

And the XML parser's version, from `tinyxml2-go/config.go`:

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
