# Lab C · Add a decode-bomb guard

> **Objectives:** Understand what a decode bomb is and why the header check happens
> before the full decode. Explore the two real guards already in this repository
> (`stb-image-go` pixels, `miniz-go` decompressed bytes), then write a small guard
> and a test that proves it rejects hostile input while passing normal input.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Decode bomb** = "a shipping container labelled 'small parcel' that expands into a warehouse when opened." The compressed or encoded file is tiny, but it tricks the decoder into allocating gigabytes of RAM; the attacker pays almost nothing to send while your server pays everything to receive.
- **Image header** = "the nutrition label on the outside of the tin — you read it before you open the tin." A PNG stores `width` and `height` in a cheap header read in microseconds; a crafted file sets `width = 32768, height = 32768` so the allocator tries to reserve ~4 GB before a single pixel is decoded.
- **Aggregate ZIP budget** = "a per-person buffet limit that also caps the whole table, not just each individual plate." A handful of compressed entries can each expand 1000x; without a running `total` across all entries, many small ones still blow the limit together.
- **Header-first guard** = "check the passport at the door, not after you've seated the guest." Both guards in this repo follow that pattern: `image.DecodeConfig` or `io.LimitReader` runs cheaply before the expensive `image.Decode` or `io.ReadAll` ever allocates.
- **Sentinel limit variable** = "a thermostat dial you can turn in tests without rewiring the house." `MaxImagePixels` and `MaxDecompressedSize` are package-level `var` values — a test saves the original, overrides it with a low cap, and restores it with `defer`, no recompile needed.

**Why it matters:** a single unguarded `image.Decode` call on untrusted input
is a one-line DoS. The guard costs one extra header read; skipping it costs
your server's memory.

**See it — cheap header check gates the expensive full decode.**

<svg viewBox="0 0 700 310" role="img" aria-labelledby="txlabcde dxlabcde" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="txlabcde">Decode-bomb guard flow: header check gates full decode</title>
  <desc id="dxlabcde">A block-and-arrow diagram showing untrusted input flowing into a cheap header check (image.DecodeConfig / io.LimitReader). If the header exceeds the limit the request is rejected with an error. If it is within budget the flow continues to the expensive full decode (image.Decode / io.ReadAll) which produces safe output.</desc>
  <defs>
    <marker id="xlabcde-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="currentColor"/>
    </marker>
  </defs>
  <rect x="20" y="120" width="130" height="54" rx="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="85" y="142" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Untrusted input</text>
  <text x="85" y="160" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">(image / archive)</text>
  <line x1="150" y1="147" x2="198" y2="147" stroke="currentColor" stroke-width="1.5" marker-end="url(#xlabcde-arrow)"/>
  <rect x="200" y="110" width="160" height="74" rx="8" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="280" y="133" text-anchor="middle" font-size="12" fill="#fff" font-weight="700">Cheap header check</text>
  <text x="280" y="151" text-anchor="middle" font-size="10" fill="#fff">image.DecodeConfig</text>
  <text x="280" y="167" text-anchor="middle" font-size="10" fill="#fff">io.LimitReader</text>
  <line x1="280" y1="110" x2="280" y2="62" stroke="#e5484d" stroke-width="1.5" marker-end="url(#xlabcde-arrow)"/>
  <rect x="200" y="18" width="160" height="44" rx="8" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="280" y="38" text-anchor="middle" font-size="12" fill="#e5484d" font-weight="600">✗ Rejected</text>
  <text x="280" y="54" text-anchor="middle" font-size="10" fill="#e5484d">exceeds limit → error</text>
  <text x="294" y="92" font-size="10" fill="#e5484d">over limit</text>
  <line x1="360" y1="147" x2="408" y2="147" stroke="currentColor" stroke-width="1.5" marker-end="url(#xlabcde-arrow)"/>
  <text x="362" y="140" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">within budget</text>
  <rect x="410" y="110" width="160" height="74" rx="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="490" y="133" text-anchor="middle" font-size="12" fill="currentColor" font-weight="600">Full decode</text>
  <text x="490" y="151" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">image.Decode</text>
  <text x="490" y="167" text-anchor="middle" font-size="10" fill="var(--md-default-fg-color--light,currentColor)">io.ReadAll</text>
  <line x1="570" y1="147" x2="618" y2="147" stroke="currentColor" stroke-width="1.5" marker-end="url(#xlabcde-arrow)"/>
  <rect x="620" y="120" width="60" height="54" rx="8" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.5"/>
  <text x="650" y="142" text-anchor="middle" font-size="12" fill="var(--md-accent-fg-color,#00897b)" font-weight="600">✓ Safe</text>
  <text x="650" y="158" text-anchor="middle" font-size="10" fill="var(--md-accent-fg-color,#00897b)">output</text>
  <rect x="200" y="220" width="14" height="14" rx="3" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="220" y="232" font-size="11" fill="currentColor">header check (cheap — no pixel alloc)</text>
  <rect x="200" y="244" width="14" height="14" rx="3" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="220" y="256" font-size="11" fill="currentColor">reject path — error returned before any allocation</text>
  <rect x="200" y="268" width="14" height="14" rx="3" fill="none" stroke="currentColor" stroke-width="1.5"/>
  <text x="220" y="280" font-size="11" fill="currentColor">full decode — only reached when dimensions are safe</text></svg>

---

## The image pixel guard (`stb-image-go`)

[`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md) exports a single tunable (in plain terms: this file is one "package" — a bundled unit of code that other code can pull in and reuse — and it makes one setting adjustable from outside):

```go
// from stb-image-go/stb_image.go
var MaxImagePixels = 64 << 20   // 64 megapixels (e.g. 8192 × 8192)
```

**In plain terms:** this line creates a named setting, `MaxImagePixels`, and gives it a starting value of about 64 million. Because it's declared with `var` at the package level (outside of any function), any code in the package can read or change it later — it isn't locked away inside one piece of code.

The internal helper `checkPixelLimit` reads only the image header — a cheap call
(in plain terms: "call" a function just means "run" it, and "cheap" here means it takes very little time or memory)
that does not allocate pixel memory (in plain terms: "allocate" means reserve a chunk of the computer's memory to hold something — here, it means the check does *not* reserve any memory for the actual picture) — and rejects anything too large:

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

**In plain terms:** this is a function — a named, reusable block of instructions (in plain terms: think of it as a small recipe you can run by name whenever you need it). `checkPixelLimit` takes the raw file bytes (in plain terms: a "byte" is one small unit of digital data — files and memory are made of many bytes strung together; `[]byte` means "a list of bytes"), peeks only at the header to find the width and height, multiplies them, and compares that to the limit. If the picture would be too big, it hands back (in plain terms: "return" means the function finishes and passes a result back to whoever ran it — the "caller") an error describing why; otherwise it returns `nil`, meaning "no error, all clear."

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

**In plain terms:** `Load` is the function callers actually use. It first invokes (in plain terms: "invoke" is just another word for "call" — run the function) `checkPixelLimit`; if that comes back with an error, `Load` immediately hands that error back to whoever called it and stops — it never reaches the expensive `image.Decode` line that would actually reserve memory for every pixel.

`LoadStream` does the same for an `io.Reader` (in plain terms: a value representing "a stream of bytes coming from somewhere" — a file, a network connection, anything you can read data out of piece by piece, rather than all the data already sitting in memory) by using `io.TeeReader` to peek
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
limit was actually hit (in plain terms: "wraps" means it takes an existing tool and adds a bit of extra behavior around it, without changing the original):

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

**In plain terms:** this function reads a stream of bytes but refuses to read more than `limit + 1` of them, using `io.LimitReader` as a safety valve. It then checks: did the amount actually read come out bigger than the allowed `limit`? If so, that is the tell-tale sign of a decode bomb, and the function returns an error instead of the data.

!!! warning "The +1 trick"
    `io.LimitReader(r, limit)` reads *up to* `limit` bytes without error.
    Without the `+1` you cannot tell whether the stream ended exactly at the
    limit (fine) or was cut off (bomb detected). By requesting one extra byte,
    you can test `len(data) > limit` to distinguish the two cases.

`ExtractArchive` applies the budget *across all entries*, not per entry. A ZIP
bomb with 1000 small entries is still caught (the code below loops over each file in the archive one at a time, running the same steps for each — this repeating-steps pattern is called a "loop"):

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

**In plain terms:** before opening each file inside the archive, the code checks how much of the total budget is left (`MaxDecompressedSize - total`); if nothing is left, it stops and errors out immediately. Otherwise it reads that one file (capped at whatever remains) and adds its size to the running `total`, so the very next entry in the loop sees a smaller remaining allowance. That running tally is what stops many small "innocent-looking" entries from adding up to a bomb.

---

## Lab: write your own guard and test it

You will write a tiny `SafeDecoder` type that wraps the two real package-level
guards, then add a test that proves:

1. A crafted "big" input (low limit, large declared size) is rejected.
2. A real small image passes.

### Step 1 — create the test file

Create `stb-image-go/guard_lab_test.go` with the content below. A "test" here is simply a small piece of code, written for the machine rather than a person, whose only job is to run some other code and check that the result is what you expected — instead of you checking by hand every time. The test file
uses the `_test` package suffix so it can set `MaxImagePixels` without affecting
other tests that run in parallel (in plain terms: "in parallel" means several tests running at the same time rather than one after another — so a change one test makes to a shared setting could interfere with another test running alongside it).

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

**In plain terms:** `encodePNG` is a helper function that builds a small fake picture of a given width and height, entirely in memory, and turns it into real PNG file bytes — so the tests below have something realistic to feed into `Load` without needing an actual image file on disk. `TestDecodeBombRejected` then temporarily lowers `MaxImagePixels` to a tiny number, builds an oversized picture, and checks that `Load` refuses it (`defer` here means "run this cleanup line automatically once the surrounding function is about to finish," which is how the original limit gets restored no matter what happens in between). `TestNormalImagePasses` does the same setup but with a picture that is exactly at the limit, and checks that `Load` accepts it instead.

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
