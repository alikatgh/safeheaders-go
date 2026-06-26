# 18 - Decode and decompression bombs

> **Objectives:** Understand how a tiny input file can trick a decoder into
> allocating gigabytes of RAM, and learn the two concrete patterns this repo
> uses to prevent it — a cheap header-only pre-check for images and an
> aggregate byte budget for ZIP entries.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

- **A bomb is a ratio trick.** A 42-byte ZIP file can expand to 4.5 GB of
  zeros. A 50 KB PNG can declare dimensions of 65535 × 65535 = 4.3 billion
  pixels. The file is small; the allocation is enormous.
- **Headers lie cheaply.** Most binary formats have a short header that says
  "I am this big." Reading the header costs almost nothing. Trusting it
  without a sanity check costs you the machine.
- **Decoding is the expensive step.** `image.Decode` (or `zip.Open`) actually
  allocates the memory the header claims. The guard has to fire *before* that
  call, not after.
- **Per-entry limits are not enough for ZIP.** A ZIP bomb with 1 000 entries,
  each just under your per-entry cap, still exhausts RAM. You need an
  *aggregate* budget across all entries.
- **`io.LimitReader` is the one-liner fix.** Wrap any reader in
  `io.LimitReader(r, budget+1)` and the Go stdlib does the rest — it returns
  `io.EOF` at the limit instead of reading forever.

**Why it matters:** a single malicious upload can take down a service that
processes files without these guards; with them, the worst outcome is a
rejected request and a logged error.

---

## Decode bombs: the image case

An image decode bomb works because image formats store width and height in a
tiny header. A decoder that trusts them blindly allocates
`width × height × bytes_per_pixel` bytes before it has read more than a
kilobyte of input.

Go's `image.DecodeConfig` reads *only* the header and returns `image.Config`
(width, height, colour model) without allocating the pixel buffer. That makes
it the perfect, cheap pre-check.

### The guard: `checkPixelLimit` in stb-image-go/stb_image.go

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
        return fmt.Errorf(
            "image %dx%d exceeds the %d-pixel decode limit (adjust MaxImagePixels)",
            cfg.Width, cfg.Height, MaxImagePixels)
    }
    return nil
}
```

Two details are worth noting:

1. **`int64` multiplication.** `cfg.Width` and `cfg.Height` are plain `int`
   (32-bit on 32-bit platforms). Multiplying two large `int32` values overflows
   silently, so the code casts to `int64` first.
2. **Returning `nil` on `DecodeConfig` error.** If the header is corrupt,
   `image.DecodeConfig` will fail. The function does not reject the input here
   — it lets `image.Decode` run and produce a proper error message. This avoids
   the trap of hiding real decode errors behind a spurious "pixel limit
   exceeded" message.

### How `Load` uses the guard

```go
func Load(data []byte) (image.Image, error) {
    if len(data) == 0 {
        return nil, errors.New("empty image data")
    }
    if err := checkPixelLimit(data); err != nil {
        return nil, err          // ← rejected before any pixel allocation
    }
    img, format, err := image.Decode(bytes.NewReader(data))
    // ...
    return img, nil
}
```

The guard is always called first. `image.Decode` is only reached if the
declared dimensions are within budget.

### Streaming variant: `LoadStream`

When you have an `io.Reader` instead of a byte slice, you cannot rewind. The
trick is to use `io.TeeReader` to record the bytes that `DecodeConfig` consumes,
then replay them with `io.MultiReader`:

```go
func LoadStream(r io.Reader) (image.Image, error) {
    if MaxImagePixels > 0 {
        var header bytes.Buffer
        cfg, _, cfgErr := image.DecodeConfig(io.TeeReader(r, &header))
        if cfgErr == nil && cfg.Width > 0 && cfg.Height > 0 &&
            int64(cfg.Width)*int64(cfg.Height) > int64(MaxImagePixels) {
            return nil, fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit ...",
                cfg.Width, cfg.Height, MaxImagePixels)
        }
        // Replay the consumed header, then the remainder of the stream.
        r = io.MultiReader(&header, r)
    }
    img, _, err := image.Decode(r)
    // ...
}
```

`TeeReader` + `MultiReader` is a standard Go idiom for "peek at a reader
without consuming it". You will see similar patterns in HTTP middleware that
needs to inspect `r.Body` before passing it on.

!!! note "Try it"
    Run the stb-image tests, which include a bomb-rejection case:

    ```bash
    cd stb-image-go && go test -v -run TestLoad ./...
    ```

    Expected: all tests pass. Look for a test case that checks an image whose
    declared dimensions exceed `MaxImagePixels` — it should return an error
    containing the words `"exceeds the"` without allocating any pixel memory.

---

## Decompression bombs: the ZIP case

ZIP bombs work differently. The ZIP format stores `UncompressedSize` in each
entry header, but that field is metadata — the real expansion happens during
decompression. A classic zip bomb puts highly compressible data (e.g. a file
of all-zero bytes) in each entry and nests or repeats it. The classic
`42.zip` is 42 bytes compressed; uncompressed it reaches 4.5 GB.

### The per-stream guard: `readAllLimited` in miniz-go/miniz.go

```go
// readAllLimited reads all of r, but errors instead of allocating without bound
// once the output would exceed limit. A limit <= 0 means unlimited.
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
    src := r
    if limit > 0 {
        // +1 so we can distinguish "exactly at the limit" from "over it".
        src = io.LimitReader(r, limit+1)
    }
    data, err := io.ReadAll(src)
    if err != nil {
        return nil, fmt.Errorf("read: %w", err)
    }
    if limit > 0 && int64(len(data)) > limit {
        return nil, fmt.Errorf(
            "decompressed size exceeds %d-byte limit (adjust MaxDecompressedSize)", limit)
    }
    return data, nil
}
```

The `limit+1` trick is subtle: `io.LimitReader` returns `io.EOF` once the
limit is reached, so if we passed `limit` we could not distinguish "exactly
at the limit (fine)" from "one byte over (bad)". Passing `limit+1` lets the
underlying reader deliver up to one extra byte; if `io.ReadAll` comes back
with more than `limit` bytes, we know the stream was truncated and we reject
it.

### The aggregate budget in `ExtractArchive`

A per-entry limit is necessary but not sufficient. Consider an archive with
1 000 entries, each one byte under the per-entry cap. Each entry passes
individually, but together they exhaust RAM.

The fix is to track a running `total` across all entries and shrink the
per-entry limit by the amount already consumed:

```go
var MaxDecompressedSize int64 = 256 << 20  // 256 MiB default

var total int64  // aggregate decompressed bytes across all entries
for _, f := range r.File {
    var perEntryLimit int64
    if MaxDecompressedSize > 0 {
        if perEntryLimit = MaxDecompressedSize - total; perEntryLimit <= 0 {
            return nil, fmt.Errorf(
                "archive exceeds the %d-byte limit (adjust MaxDecompressedSize)",
                MaxDecompressedSize)
        }
    }

    rc, err := f.Open()
    // ...
    data, err := readAllLimited(rc, perEntryLimit)
    rc.Close()
    // ...
    total += int64(len(data))
}
```

Each iteration passes `MaxDecompressedSize - total` as the limit for that
entry. Once the running total reaches the cap, `perEntryLimit` becomes zero
or negative, and the loop rejects the next entry before even opening it.

!!! warning "The aggregate check was the bug"
    The original code capped each entry with a fixed `MaxDecompressedSize`
    limit. A multi-entry archive could silently exceed the budget because no
    one was tracking the running total. The audit (documented in
    `docs/audits/2026-06-23-code-review-security-audit.md`) flagged this as
    finding M5. The fix shown above — a `total` accumulator passed as the
    narrowing per-entry limit — was added alongside a regression test.

### The global cap

```go
// MaxDecompressedSize caps how many bytes ExtractArchive and DecompressData will
// produce, guarding against decompression bombs. For ExtractArchive the cap is
// on the ARCHIVE TOTAL across all entries, not per entry. The default is 256 MiB.
// Set it to 0 to disable.
//
// It must be set before any concurrent decompression begins; it is read without
// synchronization, so mutating it while a decompress is in flight is a data race.
var MaxDecompressedSize int64 = 256 << 20
```

The doc comment is explicit: this is an aggregate cap, not a per-entry cap, and
mutating it mid-flight is a data race. If you need a different limit per call
site, you would need to pass it as a parameter — the global is a convenience
for the common single-service case.

### Stream decompression also checks the limit

`DecompressStream` uses the same variable for streaming output:

```go
func DecompressStream(dst io.Writer, src io.Reader) error {
    r := flate.NewReader(src)
    defer r.Close()

    var reader io.Reader = r
    if MaxDecompressedSize > 0 {
        reader = io.LimitReader(r, MaxDecompressedSize+1)
    }
    n, err := io.Copy(dst, reader)
    if err != nil {
        return fmt.Errorf("decompress stream: %w", err)
    }
    if MaxDecompressedSize > 0 && n > MaxDecompressedSize {
        return fmt.Errorf("decompressed size exceeds %d-byte limit ...", MaxDecompressedSize)
    }
    return nil
}
```

Same `+1` trick, same post-copy check.

!!! note "Try it"
    ```bash
    cd miniz-go && go test -v -run TestExtractArchive ./...
    ```

    Expected: all tests pass, including one that crafts an archive whose total
    uncompressed size exceeds `MaxDecompressedSize` and expects an error
    containing `"exceeds the"`.

---

## Comparing the two patterns

| Concern | Image decode bomb | ZIP decompression bomb |
|---|---|---|
| File format | PNG / JPEG / GIF | ZIP / DEFLATE |
| Attack vector | Lies in the image header | Highly compressible data in entries |
| Cheap pre-check | `image.DecodeConfig` (header only) | ZIP entry `UncompressedSize` field (unreliable; use after decompression) |
| Main guard | `checkPixelLimit` before `image.Decode` | `readAllLimited` with a shrinking per-entry budget |
| Global cap | `MaxImagePixels` (pixel count) | `MaxDecompressedSize` (byte count) |
| Aggregate tracking | Not needed (single image) | Essential (`total` accumulator) |

!!! tip "Rule of thumb"
    For any format that has a cheap way to read declared size before doing
    real work: read the declared size first, compare it to a sane limit,
    reject early. `io.LimitReader` is always available as a belt-and-suspenders
    fallback during the actual read.

---

## Putting it together: what happens on a real bomb

Here is the execution path when an attacker sends a 50 KB PNG claiming
65535 × 65535 pixels (about 4 billion pixels, ~12 GB at 3 bytes/pixel):

1. `Load(data)` is called.
2. `checkPixelLimit(data)` calls `image.DecodeConfig` — reads ~20 bytes of header.
3. `cfg.Width = 65535`, `cfg.Height = 65535`.
4. `int64(65535) * int64(65535) = 4_294_836_225 > 64<<20` — check fails.
5. Error returned: `"image 65535x65535 exceeds the 67108864-pixel decode limit"`.
6. `image.Decode` is **never called**. Zero bytes allocated for pixels.

And for a 1 000-entry ZIP bomb where each entry decompresses to 300 MiB:

1. `ExtractArchive` loops over entries.
2. Entry 0: `perEntryLimit = 256 MiB - 0 = 256 MiB`. Entry decompresses to
   300 MiB → `readAllLimited` truncates at 256 MiB + 1 byte and returns an
   error on the `int64(len(data)) > limit` check.
3. Extraction stops. Total allocation: at most ~256 MiB.

!!! note "Try it — race detector"
    The image batch loader is concurrent. Verify the pixel-limit check is safe
    under concurrent load:

    ```bash
    cd stb-image-go && go test -race ./...
    ```

    Expected: no data race reported. `MaxImagePixels` is read (not written)
    inside `checkPixelLimit`, which is safe for concurrent reads. `LoadStream`
    and `Load` share no mutable state.

---

## Key takeaways

- **Read the header cheap; reject before decoding.** `image.DecodeConfig`
  costs almost nothing and reveals the declared dimensions before any pixel
  memory is allocated.
- **`io.LimitReader(r, budget+1)` is the canonical Go decompression guard.**
  The `+1` distinguishes "exactly at the limit" from "over it"; the
  post-read length check converts that into a clean error.
- **Per-entry limits alone do not stop multi-entry bombs.** You need a running
  aggregate (`total`) that shrinks each entry's budget as bytes accumulate.
- **Both caps are package-level globals with documented data-race caveats.**
  Set them once at startup; do not mutate them concurrently.
- **Zero allocation on rejection.** When the guard fires, the expensive
  allocation (`image.Decode`, `io.ReadAll` on a decompressed stream) is never
  reached. That is the goal: fail fast and cheap, not after the damage is done.
