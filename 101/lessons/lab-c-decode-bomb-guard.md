# Lab C · Add a decode-bomb guard

> **Objectives:** Understand what a decode bomb is and why the header check happens
> before the full decode. Explore the two real guards already in this repository
> (`stb-image-go` pixels, `miniz-go` decompressed bytes), then write a small guard
> and a test that proves it rejects hostile input while passing normal input.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **A decode bomb is a magic trick.** The compressed or encoded file is tiny (a few
  hundred bytes), but it tricks the decoder into allocating gigabytes of RAM. The
  attacker pays almost nothing to send; your server pays everything to receive.
- **Image headers lie.** A PNG file stores `width` and `height` in a small header
  that is read in microseconds. A crafted file can set `width = 32768, height =
  32768` (one billion pixels) so that the allocator tries to reserve ~4 GB before
  a single pixel is decoded.
- **ZIP bombs stack the same trick.** A handful of compressed entries can each
  expand 1000x. The per-entry check stops one entry; without an *aggregate* budget,
  many small entries still blow the limit together.
- **Read the header first, pay for the full decode only if safe.** Both guards in
  this repo follow that pattern: cheap metadata check first, expensive allocation
  only when the dimensions are within budget.
- **Sentinel variables make the limit easy to tune.** `MaxImagePixels` and
  `MaxDecompressedSize` are package-level `var` values — tests can override them
  for a single case without recompiling.
- **Why it matters:** a single unguarded `image.Decode` call on untrusted input
  is a one-line DoS. The guard costs one extra header read; skipping it costs
  your server's memory.

---

## The image pixel guard (`stb-image-go`)

[`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md) exports a single tunable:

```go
// from stb-image-go/stb_image.go
var MaxImagePixels = 64 << 20   // 64 megapixels (e.g. 8192 × 8192)
```

The internal helper `checkPixelLimit` reads only the image header — a cheap call
that does not allocate pixel memory — and rejects anything too large:

```go
// from stb-image-go/stb_image.go
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
        return fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit"+
            " (adjust MaxImagePixels)", cfg.Width, cfg.Height, MaxImagePixels)
    }
    return nil
}
```

`Load` calls this before ever touching `image.Decode`:

```go
// from stb-image-go/stb_image.go
func Load(data []byte) (image.Image, error) {
    if len(data) == 0 {
        return nil, errors.New("empty image data")
    }
    if err := checkPixelLimit(data); err != nil {
        return nil, err           // rejected before any pixel allocation
    }
    img, format, err := image.Decode(bytes.NewReader(data))
    // ...
}
```

`LoadStream` does the same for an `io.Reader` by using `io.TeeReader` to peek
at the header bytes without consuming them:

```go
// from stb-image-go/stb_image.go
var header bytes.Buffer
cfg, _, cfgErr := image.DecodeConfig(io.TeeReader(r, &header))
// ... reject if too large ...
r = io.MultiReader(&header, r)   // replay header + rest of stream
img, _, err := image.Decode(r)
```

!!! tip "Why `io.TeeReader` + `io.MultiReader`?"
    `image.DecodeConfig` consumes bytes from the reader (it reads the header).
    `TeeReader` copies every consumed byte into `header`. After the check,
    `MultiReader` replays `header` followed by the original reader, so the full
    decode sees the complete file. Without this trick the first few header bytes
    would be silently skipped and the full decode would fail or corrupt.

---

## The decompression-size guard (`miniz-go`)

[`miniz-go/miniz.go`](src/miniz-go-miniz-go.md) uses a different shape: a byte budget instead of a pixel
count. The package variable is:

```go
// from miniz-go/miniz.go
var MaxDecompressedSize int64 = 256 << 20   // 256 MiB aggregate
```

The helper `readAllLimited` wraps `io.LimitReader` and then checks whether the
limit was actually hit:

```go
// from miniz-go/miniz.go
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
    src := r
    if limit > 0 {
        src = io.LimitReader(r, limit+1)  // +1 to distinguish "at limit" from "over"
    }
    data, err := io.ReadAll(src)
    if err != nil {
        return nil, fmt.Errorf("read: %w", err)
    }
    if limit > 0 && int64(len(data)) > limit {
        return nil, fmt.Errorf("decompressed size exceeds %d-byte limit"+
            " (adjust MaxDecompressedSize)", limit)
    }
    return data, nil
}
```

!!! warning "The +1 trick"
    `io.LimitReader(r, limit)` reads *up to* `limit` bytes without error.
    Without the `+1` you cannot tell whether the stream ended exactly at the
    limit (fine) or was cut off (bomb detected). By requesting one extra byte,
    you can test `len(data) > limit` to distinguish the two cases.

`ExtractArchive` applies the budget *across all entries*, not per entry. A ZIP
bomb with 1000 small entries is still caught:

```go
// from miniz-go/miniz.go
var total int64
for _, f := range r.File {
    var perEntryLimit int64
    if MaxDecompressedSize > 0 {
        if perEntryLimit = MaxDecompressedSize - total; perEntryLimit <= 0 {
            return nil, fmt.Errorf("archive exceeds the %d-byte limit"+
                " (adjust MaxDecompressedSize)", MaxDecompressedSize)
        }
    }
    rc, err := f.Open()
    if err != nil {
        return nil, err
    }
    data, err := readAllLimited(rc, perEntryLimit)
    rc.Close()
    total += int64(len(data))
    // ...
}
```

---

## Lab: write your own guard and test it

You will write a tiny `SafeDecoder` type that wraps the two real package-level
guards, then add a test that proves:

1. A crafted "big" input (low limit, large declared size) is rejected.
2. A real small image passes.

### Step 1 — create the test file

Create `stb-image-go/guard_lab_test.go` with the content below. The test file
uses the `_test` package suffix so it can set `MaxImagePixels` without affecting
other tests that run in parallel.

```go
// stb-image-go/guard_lab_test.go  (new file — your lab work)
package stbimagego_test

import (
    "image"
    "image/color"
    "image/png"
    "bytes"
    "testing"

    stbimage "github.com/your-org/safeheaders-go/stb-image-go"
)

// encodePNG builds an in-memory PNG with the given width and height,
// filled with a solid colour. Real pixel data so DecodeConfig parses correctly.
func encodePNG(t *testing.T, w, h int) []byte {
    t.Helper()
    img := image.NewRGBA(image.Rect(0, 0, w, h))
    for y := 0; y < h; y++ {
        for x := 0; x < w; x++ {
            img.Set(x, y, color.RGBA{R: 0x42, G: 0x42, B: 0x42, A: 0xff})
        }
    }
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        t.Fatalf("png.Encode: %v", err)
    }
    return buf.Bytes()
}

func TestDecodeBombRejected(t *testing.T) {
    // Save and restore the global so parallel tests aren't affected.
    orig := stbimage.MaxImagePixels
    defer func() { stbimage.MaxImagePixels = orig }()

    // Lower the cap to 100 pixels (10×10).
    stbimage.MaxImagePixels = 100

    // Build a PNG that is 200×200 = 40 000 pixels — clearly over the cap.
    bomb := encodePNG(t, 200, 200)

    _, err := stbimage.Load(bomb)
    if err == nil {
        t.Fatal("expected an error for oversized image, got nil")
    }
    t.Logf("correctly rejected: %v", err)
}

func TestNormalImagePasses(t *testing.T) {
    orig := stbimage.MaxImagePixels
    defer func() { stbimage.MaxImagePixels = orig }()

    stbimage.MaxImagePixels = 100

    // A 10×10 PNG = 100 pixels, exactly at the limit — must pass.
    ok := encodePNG(t, 10, 10)

    img, err := stbimage.Load(ok)
    if err != nil {
        t.Fatalf("expected success for small image, got: %v", err)
    }
    if img == nil {
        t.Fatal("got nil image")
    }
    t.Logf("correctly accepted: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
}
```

!!! note "Try it"
    Run the two new tests from the `stb-image-go` directory:

    ```bash
    cd stb-image-go
    go test -v -run 'TestDecodeBomb|TestNormalImage' ./...
    ```

    Expected output (approximate):

    ```
    --- PASS: TestDecodeBombRejected (0.00s)
        guard_lab_test.go:49: correctly rejected: image 200x200 exceeds the 100-pixel decode limit (adjust MaxImagePixels)
    --- PASS: TestNormalImagePasses (0.00s)
        guard_lab_test.go:67: correctly accepted: 10x10
    PASS
    ```

    The key line is the error message from `checkPixelLimit` in
    `stb-image-go/stb_image.go` — you never paid for a single pixel allocation.

### Step 2 — run with the race detector

```bash
go test -race -run 'TestDecodeBomb|TestNormalImage' ./...
```

Because both tests mutate `MaxImagePixels` and you use `t.Parallel()` is *not*
called here, the sequential run is clean. If you add `t.Parallel()` you must
protect the global with a mutex or use `-count=1` to serialize. The existing
codebase documents this in the `MaxDecompressedSize` comment:

> "It must be set before any concurrent decompression begins; it is read without
> synchronization, so mutating it while a decompress is in flight is a data race."
> — `miniz-go/miniz.go`

!!! warning "Global limits and parallel tests"
    `MaxImagePixels` and `MaxDecompressedSize` are package-level variables.
    Mutating them in parallel tests causes a race. Always save and restore with
    `defer`, and never call `t.Parallel()` in tests that mutate them unless
    you protect access with a mutex.

### Step 3 — try the miniz aggregate guard

```bash
cd miniz-go
go test -v -run TestExtract ./...
```

Look for any test named `TestExtract` that exercises `ExtractArchive`. The
aggregate budget in `ExtractArchive` (tracked via the `total` variable) is what
distinguishes this repo from a naive per-entry limit.

---

## Solution — what is happening under the hood

The guard has four moving parts:

| Part | Where | Role |
|------|-------|------|
| `MaxImagePixels` | [`stb_image.go`](src/stb-image-go-stb-image-go.md) var | Tunable ceiling; 0 = disabled |
| `image.DecodeConfig` | `checkPixelLimit` | Reads header only — no pixel alloc |
| pixel count check | `checkPixelLimit` | `Width * Height > limit` → error |
| `Load` gate | `Load()` | Calls `checkPixelLimit` before `image.Decode` |

And for the ZIP side:

| Part | Where | Role |
|------|-------|------|
| `MaxDecompressedSize` | [`miniz.go`](src/miniz-go-miniz-go.md) var | Aggregate byte budget |
| `io.LimitReader(r, limit+1)` | `readAllLimited` | Cuts the stream early |
| `len(data) > limit` | `readAllLimited` | Detects the bomb trigger |
| `total` accumulator | `ExtractArchive` | Tracks aggregate across all entries |

The pattern generalizes to any decoder: **read metadata cheaply → validate →
pay for the full allocation only when safe.**

---

## Key takeaways

- A decode bomb exploits the gap between the cost of sending a file and the cost of
  decoding it. Guard at the header, not after allocation.
- `image.DecodeConfig` is the cheap pre-check; `image.Decode` is the expensive
  full decode. Never skip to the expensive one on untrusted input.
- For archive formats, per-entry limits are not enough. Track aggregate output
  across all entries (`total` in `miniz-go/miniz.go`).
- The `+1` trick in `readAllLimited` is the idiomatic Go way to distinguish "ended
  exactly at limit" from "was cut off because it exceeded the limit."
- Package-level limit variables are easy to tune but dangerous to mutate in
  parallel tests. Always save, override, and restore with `defer`.
