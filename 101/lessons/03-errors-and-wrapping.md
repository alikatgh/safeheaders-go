# 03 · Errors, wrapping and sentinels

> **Objectives:** Understand how Go treats errors as plain values, how `fmt.Errorf("%w", err)` wraps
> them into a chain, and how `errors.Is` / `errors.As` let callers inspect that chain without
> string-matching. See all three patterns in action inside safeheaders-go.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

- **Error as a value** — in Go an error is just an interface with one method: `Error() string`.
  A function that can fail returns `(result, error)` as its last two values. There are no
  exceptions, no try/catch.
- **Sentinel error** — a package-level variable (`var ErrFoo = errors.New("...")`) that acts
  as a named signal. Callers compare against it with `errors.Is` rather than reading its text.
  Think of it like an HTTP status code: `404` means "not found" regardless of the body text.
- **Wrapping** — `fmt.Errorf("context: %w", err)` attaches a parent message while keeping the
  original error reachable. It is like a chain of "cause" links in a log entry.
- **`errors.Is`** — walks the whole chain looking for an exact match. Use it to test "did
  *any* step in the call fail with ErrFoo?".
- **`errors.As`** — same walk, but extracts a value whose concrete type matches. Use it when
  you need fields from a custom error struct.
- **Bare return vs wrap** — return bare errors from low-level helpers; wrap at the layer that
  adds context a caller would actually want to read.

**Why it matters:** precise, composable errors are how a library signals "you sent too much
data" vs "the file is corrupt" vs "the OS refused the read" — all without panicking or losing
the original cause.

---

## Sentinel errors in practice

A *sentinel* is an error value that lives in the package's public API surface.
Callers never need to read its text; they compare with `==` (or `errors.Is`).

Both `jsmn-go` and `tinyxml2-go` declare their sentinels the same way.
From `jsmn-go/config.go`:

```go
// Common errors.
var (
    // ErrInputTooLarge is returned when input exceeds MaxInputSize.
    ErrInputTooLarge = errors.New("input size exceeds maximum allowed")

    // ErrTooManyTokens is returned when token count exceeds MaxTokens.
    ErrTooManyTokens = errors.New("token count exceeds maximum allowed")

    // ErrEmptyInput is returned when input is empty.
    ErrEmptyInput = errors.New("empty input")
)
```

`errors.New` allocates a unique value; two separate calls with the same string
produce two *different* errors. That uniqueness is what makes sentinel comparison
reliable — there is no accidental collision.

`tinyxml2-go/config.go` follows the exact same pattern with its own set:
`ErrInputTooLarge`, `ErrTooManyNodes`, `ErrNestingTooDeep`, `ErrEmptyInput`.

### How the guard code uses them

From `jsmn-go/config.go`, `validateInput`:

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

`nil` is the idiomatic "no error" value. The function returns a sentinel directly —
no wrapping — because this is the *defining* site of that error. Adding a wrapper
here would make `errors.Is` harder, not easier (though `%w` still works; see below).

### Checking for a sentinel as a caller

```go
tokens, err := jsmngo.ParseWithConfig(ctx, data, jsmngo.StrictConfig())
if errors.Is(err, jsmngo.ErrInputTooLarge) {
    // reject at the HTTP layer, don't log as a server error
    http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
    return
}
if err != nil {
    log.Printf("parse failed: %v", err)
}
```

`errors.Is(err, target)` returns `true` if `err == target` **or** if any error in
the chain wraps `target`. That makes it safe even when the error has been wrapped
on the way up.

!!! warning "Don't compare with `==` directly"
    `err == jsmngo.ErrInputTooLarge` works today but breaks the moment any layer
    wraps the error. `errors.Is` is always correct; `==` is a trap.

---

## Wrapping: adding context without losing the cause

### `fmt.Errorf` with `%w`

The `%w` verb is the only Go formatting verb that *wraps* rather than *stringifies*.
It stores the original error inside the new one so `errors.Is` / `errors.As` can
unwrap it later.

`dr-wav-go/dr_wav.go` is a binary parser and wraps liberally at every read step.
Here is the RIFF header section of `Parse`:

```go
// dr-wav-go/dr_wav.go
var riff [4]byte
if err := binary.Read(r, binary.LittleEndian, &riff); err != nil {
    return nil, fmt.Errorf("failed to read RIFF: %w", err)
}
if string(riff[:]) != "RIFF" {
    return nil, errors.New("invalid RIFF header")
}
```

Two patterns side-by-side:

| Line | Pattern | When to use |
|------|---------|-------------|
| `fmt.Errorf("... %w", err)` | Wrapping | The underlying `err` might be `io.ErrUnexpectedEOF`, `io.EOF`, etc. — keep it reachable. |
| `errors.New("invalid RIFF header")` | New bare error | There is no underlying cause; the problem *is* the data itself. |

### Wrapping propagates all the way up

`readDataChunk` (still in `dr-wav-go/dr_wav.go`) is a private helper that wraps
each of its own read errors:

```go
// dr-wav-go/dr_wav.go  — readDataChunk
var subchunkSize uint32
if err := binary.Read(r, binary.LittleEndian, &subchunkSize); err != nil {
    return nil, fmt.Errorf("failed to read subchunk size: %w", err)
}
```

`Parse` calls `readDataChunk` and returns its error *bare*:

```go
pcmData, err := readDataChunk(r)
if err != nil {
    return nil, err   // already wrapped inside readDataChunk — no double-wrap
}
```

!!! tip "Double-wrapping is not wrong, just noisy"
    Wrapping the same error twice (`fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", err))`)
    produces a longer message but doesn't break `errors.Is`. Prefer to wrap once at
    the layer that adds genuine context; pass through at layers that add none.

### Wrapping in ParseBatch: adding the index

`ParseBatch` in `dr-wav-go/dr_wav.go` processes multiple WAV files concurrently.
When one fails it wraps the error with the index so the caller knows *which* file:

```go
// dr-wav-go/dr_wav.go  — ParseBatch result collector
if res.err != nil {
    return nil, fmt.Errorf("failed to parse WAV at index %d: %w", res.index, res.err)
}
```

This is the canonical use of `%w`: the outer message is human-readable context;
the wrapped `res.err` carries the machine-testable cause.

---

## Wrapping vs bare: the decision rule

`tinyxml2-go/tinyxml2.go` shows both patterns next to each other in `parseElement`:

```go
// tinyxml2-go/tinyxml2.go  — parseElement
tok, err := dec.Token()
if err == io.EOF {
    return nil, errors.New("unexpected EOF")   // no underlying err to preserve
}
if err != nil {
    return nil, fmt.Errorf("parse XML token: %w", err)  // preserve decoder error
}
```

**Decision rule:**

- Got an error from *another* function (stdlib, another package)? → wrap with `%w`.
  You are adding context; the original cause might be inspected by the caller.
- Detecting a problem yourself (wrong magic bytes, empty input, bad length)? →
  `errors.New(...)` or a sentinel. There is no underlying cause to preserve.

---

## Sentinels vs `errors.As`: structured errors

`errors.Is` tests identity. `errors.As` extracts type. Use `errors.As` when
your error struct carries data the caller needs.

`ValidateWAV` in `dr-wav-go/dr_wav.go` returns formatted errors that embed the
bad value:

```go
// dr-wav-go/dr_wav.go  — ValidateWAV
if wav.Header.AudioFormat != 1 {
    return fmt.Errorf("unsupported audio format: %d (only PCM supported)",
        wav.Header.AudioFormat)
}
if wav.Header.BitsPerSample != 8 && wav.Header.BitsPerSample != 16 &&
    wav.Header.BitsPerSample != 24 && wav.Header.BitsPerSample != 32 {
    return fmt.Errorf("unsupported bits per sample: %d", wav.Header.BitsPerSample)
}
```

These are *not* sentinels (each call produces a unique value) and not wrapped
(no `%w`). They are purely informational: the caller logs the message and moves
on. There is no need for `errors.Is` or `errors.As` here.

If `ValidateWAV` had instead returned a custom `*ValidationError{Field, Got}`
struct, a caller would extract it with `errors.As`:

```go
// hypothetical — not in the repo, just illustrating errors.As
var ve *ValidationError
if errors.As(err, &ve) {
    log.Printf("field %s rejected value %v", ve.Field, ve.Got)
}
```

The rule: reach for a sentinel when "did X happen?" is enough; reach for a
custom error type when the caller needs the *values* inside the error.

---

## The depth-ceiling pattern in tinyxml2-go

`parseElementLimited` in `tinyxml2-go/tinyxml2.go` shows error-return as a
safety mechanism that replaces what would otherwise be a fatal stack overflow:

```go
// tinyxml2-go/tinyxml2.go  — parseElementLimited
const maxNestingDepth = 10000

if depth > maxNestingDepth {
    return nil, fmt.Errorf("XML nesting exceeds maximum depth %d", maxNestingDepth)
}
if config.MaxNestingDepth > 0 && depth > config.MaxNestingDepth {
    return nil, ErrNestingTooDeep
}
```

Two levels of guard:

1. **Absolute hard ceiling** (`maxNestingDepth = 10000`) — returned as a formatted
   error with the actual limit in the message. Not a sentinel because callers
   should never normally hit it; the message is diagnostic.
2. **Config-driven limit** (`MaxNestingDepth`) — returned as `ErrNestingTooDeep`,
   a sentinel, because a caller might legitimately want to catch and handle it
   (e.g. reject the request with 422 rather than 500).

The comment in the source explains *why* a return-error is necessary here rather
than `recover()`: a goroutine stack overflow is a `runtime` fatal — it bypasses
`recover`. Returning an error is the only safe option.

---

!!! note "Try it"
    Run the sentinel-error tests for jsmn-go to see both `ErrInputTooLarge` and
    `ErrEmptyInput` fire:

    ```bash
    cd jsmn-go && go test -v -run TestParseWithConfig ./...
    ```

    Expected: lines like
    ```
    --- PASS: TestParseWithConfig/empty_input (0.00s)
    --- PASS: TestParseWithConfig/input_too_large (0.00s)
    ```
    Each test calls `errors.Is(err, ErrEmptyInput)` (or `ErrInputTooLarge`) on
    the returned error and asserts it matches. Try predicting which sub-test will
    fail if you comment out the `return ErrInputTooLarge` line in `config.go`,
    then run again to confirm.

---

!!! note "Try it — wrapping round-trip"
    Check that `%w` preserves the standard-library cause through the dr-wav chain:

    ```bash
    cd dr-wav-go && go test -v -run TestParse ./...
    ```

    Then add a quick one-off program (temporary, local):

    ```go
    // main.go (scratch)
    package main

    import (
        "errors"
        "fmt"
        "io"

        drwavgo "github.com/example/safeheaders-go/dr-wav-go"
    )

    func main() {
        _, err := drwavgo.Parse([]byte("RIFF"))   // too short to be valid
        fmt.Println(err)
        fmt.Println("is EOF?", errors.Is(err, io.EOF))
        fmt.Println("is ErrUnexpectedEOF?", errors.Is(err, io.ErrUnexpectedEOF))
    }
    ```

    Expected output shows the human-readable chain and that `errors.Is` can find
    the standard-library sentinel even through the `fmt.Errorf` wrappers.

---

## Quick reference: which tool for which job

| Situation | Tool |
|-----------|------|
| Named, stable signal ("input too large") | `var ErrFoo = errors.New(...)` sentinel |
| Caller needs to branch on "did X fail?" | `errors.Is(err, ErrFoo)` |
| Caller needs values from inside the error | custom `error` struct + `errors.As` |
| Adding context to someone else's error | `fmt.Errorf("context: %w", err)` |
| Detecting a problem yourself (no cause) | `errors.New("description")` |
| Passing through without adding context | bare `return ..., err` |

---

## Key takeaways

- Go errors are values; `nil` means success. There are no exceptions.
- **Sentinel errors** (`var ErrFoo = errors.New(...)`) give callers a stable identity
  to test with `errors.Is` — more robust than string-matching the error message.
- **`fmt.Errorf("%w", err)`** wraps an error into a chain so context accumulates
  across call layers without losing the original cause.
- The **decision rule** is simple: if you received an error from another function,
  wrap it; if you detected the problem yourself, use `errors.New` or a sentinel.
- **Returning an error** is sometimes the only safe choice — as the `tinyxml2-go`
  depth ceiling shows, a fatal stack overflow cannot be caught with `recover`, but
  an `error` return can be caught by any caller.
