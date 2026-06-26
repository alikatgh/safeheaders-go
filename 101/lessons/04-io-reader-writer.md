# 04 - The interfaces that matter: io.Reader/Writer

> **Objectives:** Understand what `io.Reader` and `io.Writer` are and why the Go standard library is built around them. See how safeheaders-go uses these interfaces for streaming compression, JSON parsing, and image loading. Learn why `io.LimitReader` is a first-class safety tool, not an afterthought.
> Estimated time: 20 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **`io.Reader` is a pipe tap.** Anything that produces a sequence of bytes — a file, a network socket, a compressed stream, an in-memory `[]byte` — can wear the `io.Reader` face. Your code doesn't care which.
- **`io.Writer` is a drain.** Anything that can absorb bytes — a file, a network connection, an in-memory buffer — implements `io.Writer`. Write to the drain without knowing what's at the other end.
- **`bytes.Reader` turns a `[]byte` into a `Reader`; `bytes.Buffer` is both at once.** They are the glue between the "I already have bytes in memory" world and any function that expects a stream.
- **`io.Copy` moves data from a Reader to a Writer without allocating a full intermediate buffer.** It's 32 KB at a time, not "slurp everything, then write everything."
- **`io.LimitReader` wraps a Reader and cuts it off at N bytes.** It's a one-liner safety fence: the downstream code never even sees byte N+1, so it cannot allocate for it.
- **Small interfaces compose.** `io.Reader` is two methods (actually one: `Read`). Because the interface is tiny, every concrete type in the standard library already satisfies it, and you can stack wrappers — `LimitReader(TeeReader(r, log), max)` — without writing a single new type.

**Why it matters:** the moment untrusted input enters your program, you need to decide how much memory you're willing to spend on it. The `io` interfaces let you make that decision once, at the boundary, using `io.LimitReader` — rather than allocating a huge slice and hoping for the best.

---

## The two-method contract

```go
// from the Go standard library — shown here for reference
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}
```

That's the whole contract. `Read` fills `p` and returns how many bytes it put there, plus any error. `Write` drains `p` and returns how many bytes it consumed. Everything else in the `io` package — `Copy`, `ReadAll`, `LimitReader`, `TeeReader`, `MultiReader` — is built on these two methods.

---

## bytes.Reader and bytes.Buffer — the in-memory bridges

Most functions in safeheaders-go receive raw `[]byte` from the caller (because the caller already loaded or received a blob). They bridge to the streaming world with `bytes.NewReader`:

From [`miniz-go/miniz.go`](src/miniz-go-miniz-go.md):

```go
func ExtractArchive(data []byte) ([]ZipFile, error) {
    r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
    // ...
}
```

`zip.NewReader` needs an `io.ReaderAt` (a positioned reader). `bytes.NewReader` provides it from a plain `[]byte`. No copy, no temp file.

The write side uses `bytes.Buffer` as the accumulator:

```go
func CreateArchive(files []FileEntry) ([]byte, error) {
    var buf bytes.Buffer
    w := zip.NewWriter(&buf)
    // ... write entries ...
    return buf.Bytes(), nil
}
```

`bytes.Buffer` satisfies `io.Writer`, so `zip.NewWriter` can write directly into it. When you're done, `buf.Bytes()` hands back the accumulated slice.

---

## CompressStream — when you don't want to buffer at all

`CompressData` (in `miniz-go/miniz.go`) reads a `[]byte`, compresses it, returns a `[]byte`. Simple. But if your file is 2 GB, you don't want two copies of it in memory at once. That's what `CompressStream` is for:

```go
// from miniz-go/miniz.go
func CompressStream(dst io.Writer, src io.Reader) error {
    if dst == nil || src == nil {
        return errors.New("nil reader or writer")
    }
    w, err := flate.NewWriter(dst, flate.BestCompression)
    if err != nil {
        return fmt.Errorf("create compressor: %w", err)
    }
    if _, err := io.Copy(w, src); err != nil {
        _ = w.Close()
        return fmt.Errorf("compress stream: %w", err)
    }
    if err := w.Close(); err != nil {
        return fmt.Errorf("finalize compression: %w", err)
    }
    return nil
}
```

`io.Copy(w, src)` is the key line. It loops internally, reading up to 32 KB from `src` and writing it to the compressor `w`, until `src` returns `io.EOF`. The total memory in flight at any moment is just those 32 KB — not the whole input.

The caller can pass an `os.File` as `src` and a network connection as `dst`, and the function works unchanged. That's the composition payoff.

---

## io.LimitReader — the safety fence

A decompression bomb is a compressed file that expands to gigabytes. Without a fence, `io.ReadAll` on the decompressed stream will happily allocate until the process is killed.

Here is the helper that protects `DecompressData` and `ExtractArchive` in `miniz-go/miniz.go`:

```go
// from miniz-go/miniz.go
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
        return nil, fmt.Errorf("decompressed size exceeds %d-byte limit (adjust MaxDecompressedSize)", limit)
    }
    return data, nil
}
```

The `+1` trick is intentional: `io.LimitReader` stops at exactly `limit+1` bytes. If `io.ReadAll` returns `limit+1` bytes, you know the real stream was larger and you return an error. If it returns `<= limit` bytes, you're within budget.

`DecompressStream` uses the same pattern on the fly, without a helper:

```go
// from miniz-go/miniz.go
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
        return fmt.Errorf("decompressed size exceeds %d-byte limit (adjust MaxDecompressedSize)", MaxDecompressedSize)
    }
    return nil
}
```

The aggregate cap across all ZIP entries is enforced in `ExtractArchive` by tracking `total` and computing a `perEntryLimit = MaxDecompressedSize - total` for each entry — see the full code in `miniz-go/miniz.go`. A multi-entry bomb that stays under the per-entry threshold still hits the aggregate ceiling.

!!! warning "The global cap is a shared variable"
    `MaxDecompressedSize` is a package-level `var`, readable by all goroutines without a lock. The doc comment in the source says: *"It must be set before any concurrent decompression begins; it is read without synchronization, so mutating it while a decompress is in flight is a data race."* Set it once at startup and leave it alone.

---

## UnmarshalStream and MarshalStream — JSON over a Reader/Writer

[`cjson-go/cjson.go`](src/cjson-go-cjson-go.md) shows the same pattern for JSON. The all-in-memory variants take `[]byte`:

```go
// from cjson-go/cjson.go
func Unmarshal(data []byte, v interface{}) error {
    if len(data) == 0 {
        return errors.New("empty JSON data")
    }
    if err := json.Unmarshal(data, v); err != nil {
        return fmt.Errorf("unmarshal error: %w", err)
    }
    return nil
}
```

The streaming variants take `io.Reader` / `io.Writer`:

```go
// from cjson-go/cjson.go
func UnmarshalStream(r io.Reader, v interface{}) error {
    decoder := json.NewDecoder(r)
    if err := decoder.Decode(v); err != nil {
        return fmt.Errorf("stream unmarshal error: %w", err)
    }
    return nil
}

func MarshalStream(w io.Writer, v interface{}) error {
    encoder := json.NewEncoder(w)
    if err := encoder.Encode(v); err != nil {
        return fmt.Errorf("stream marshal error: %w", err)
    }
    return nil
}
```

`json.NewDecoder` reads from the `io.Reader` incrementally. You can pass it an HTTP response body directly, and the JSON parser reads from the network as it goes — no need to slurp the whole body first.

!!! warning "UnmarshalStream does not impose a size limit"
    The doc comment in the source is explicit: *"It does not impose a size limit, so for untrusted input callers MUST wrap r in an io.LimitReader (or http.MaxBytesReader) to bound memory."* Always wrap before passing an untrusted reader:

    ```go
    limited := io.LimitReader(resp.Body, 4<<20) // 4 MiB
    err := cjsongo.UnmarshalStream(limited, &result)
    ```

---

## LoadStream — streaming image decode with a header peek

[`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md) shows a slightly more involved pattern: it needs to read the image header (to check dimensions) and then re-read the full stream for decoding — but it has only one `io.Reader`, and reading consumes bytes.

```go
// from stb-image-go/stb_image.go
func LoadStream(r io.Reader) (image.Image, error) {
    if r == nil {
        return nil, errors.New("nil reader")
    }
    if MaxImagePixels > 0 {
        // Tee the header bytes consumed by DecodeConfig into a buffer so the full
        // image can still be decoded (DecodeConfig reads only the header).
        var header bytes.Buffer
        cfg, _, cfgErr := image.DecodeConfig(io.TeeReader(r, &header))
        if cfgErr == nil && cfg.Width > 0 && cfg.Height > 0 &&
            int64(cfg.Width)*int64(cfg.Height) > int64(MaxImagePixels) {
            return nil, fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit (adjust MaxImagePixels)",
                cfg.Width, cfg.Height, MaxImagePixels)
        }
        // Replay the consumed header, then the remainder of the stream.
        r = io.MultiReader(&header, r)
    }
    img, _, err := image.Decode(r)
    // ...
}
```

Three `io` helpers work together here:

| Helper | Role |
|--------|------|
| `io.TeeReader(r, &header)` | Reads from `r`; every byte also goes into `header` |
| `bytes.Buffer` (`header`) | Captures the bytes `DecodeConfig` consumed |
| `io.MultiReader(&header, r)` | Presents the captured header, then the remaining stream, as one `Reader` |

The result: `image.DecodeConfig` gets the header; the full `image.Decode` sees the complete stream from the beginning, with no seeking and no extra allocation of the full body.

---

## When to use streaming vs. all-in-memory

| Situation | Prefer |
|-----------|--------|
| Small, known-size input (< a few MB) | `[]byte` all-in-memory — simpler code |
| Large or variable-size input | `io.Reader` streaming — constant memory |
| Must check limits before allocating | `io.LimitReader` or header-peek pattern |
| Output destination varies (file, network, buffer) | `io.Writer` — caller picks the destination |
| Processing each JSON item independently | `UnmarshalArrayParallel` with `MaxArrayItems` cap |

---

!!! note "Try it"
    Run the miniz streaming round-trip test to see `CompressStream` / `DecompressStream` in action:

    ```bash
    cd miniz-go && go test -v -run TestStream
    ```

    Expected outcome: the test compresses a known string, decompresses it back through `DecompressStream`, and asserts the result equals the original. You should see `PASS` with no allocation complaints. If you want to confirm the limit fires, also run:

    ```bash
    go test -v -run TestDecompress
    ```

    which exercises `DecompressData` with `readAllLimited` and checks that oversized output is rejected.

!!! tip "Compose for free"
    Because all of these functions speak `io.Reader`/`io.Writer`, you can stack them at the call site without touching library code:

    ```go
    // Read from an HTTP response, cap at 8 MiB, decompress on the fly,
    // write into a file — no intermediate allocation.
    limited := io.LimitReader(resp.Body, 8<<20)
    if err := minizgo.DecompressStream(outFile, limited); err != nil {
        log.Fatal(err)
    }
    ```

---

## Key takeaways

- `io.Reader` and `io.Writer` are the universal language of byte streams in Go; any concrete source or sink can satisfy them with one method each.
- `bytes.NewReader` and `bytes.Buffer` bridge the `[]byte` world to the streaming world — they appear in almost every file in this repo.
- `io.Copy` moves data in fixed-size chunks; it never loads the full stream into memory and is the right default for large transfers.
- `io.LimitReader` is a one-line safety check at the trust boundary; wrap any untrusted reader with it before passing it downstream — `UnmarshalStream` in `cjson-go/cjson.go` explicitly documents this requirement.
- When you need to peek at a stream header without consuming it, combine `io.TeeReader` (to capture bytes as they're read) with `io.MultiReader` (to replay them) — the pattern used by `LoadStream` in `stb-image-go/stb_image.go`.
