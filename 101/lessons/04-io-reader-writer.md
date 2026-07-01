# 04 - The interfaces that matter: io.Reader/Writer

> **Objectives:** Understand what `io.Reader` and `io.Writer` are and why the Go standard library is built around them. See how safeheaders-go uses these interfaces for streaming compression, JSON parsing, and image loading. Learn why `io.LimitReader` is a first-class safety tool, not an afterthought.
> Estimated time: 20 minutes.

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **`io.Reader`** = "a garden hose you can plug into any tap." Anything that produces bytes — a file, a network socket, a compressed stream, a `[]byte` already in memory — can satisfy the `io.Reader` contract, so your code works with all of them identically.
- **`io.Writer`** = "a drain that doesn't care what pipe feeds it." Anything that absorbs bytes — a file, a network connection, an in-memory buffer — implements `io.Writer`, so the same `CompressStream` function can write to a disk file or a network connection without any changes.
- **`bytes.Reader` / `bytes.Buffer`** = "a travel adapter between the plug-in-the-wall world and the streaming world." `bytes.NewReader` wraps a plain `[]byte` so it satisfies `io.Reader`; `bytes.Buffer` does the reverse — it collects streaming writes and hands back a `[]byte` when you call `buf.Bytes()`.
- **`io.Copy`** = "a bucket brigade, not a dump truck." It moves data from a `Reader` to a `Writer` 32 KB at a time, so a 2 GB file never fully materialises in RAM — only the current bucket is ever in flight.
- **`io.LimitReader`** = "a turnstile at the door of your memory." Wrapping any untrusted reader with `io.LimitReader(r, N)` means downstream code is physically incapable of reading byte N+1, making decompression-bomb protection a one-liner rather than a convention.
- **Small interfaces compose** = "LEGO bricks that snap together without custom connectors." Because `io.Reader` has only one method (`Read`), every concrete type in the standard library already satisfies it, and you can stack wrappers — `LimitReader(TeeReader(r, log), max)` — without writing any new type, as `LoadStream` demonstrates with `TeeReader` + `MultiReader`.

**Why it matters:** the moment untrusted input enters your program, you need to decide how much memory you're willing to spend on it. The `io` interfaces let you make that decision once, at the boundary, using `io.LimitReader` — rather than allocating a huge slice and hoping for the best.

**See it — how `io.LimitReader` guards a streaming decompress.**

<svg viewBox="0 0 700 300" role="img" aria-labelledby="t04 d04" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t04">io.LimitReader guarding a streaming decompress pipeline</title>
  <desc id="d04">A block-and-arrow diagram showing bytes flowing from an untrusted source through a flate.Reader decompressor, then through an io.LimitReader that cuts the stream at MaxDecompressedSize+1, and finally into io.Copy which writes to the dst io.Writer. A rejected overflow path exits the LimitReader to a red error box labelled "size exceeds limit".</desc>
  <defs>
    <marker id="l04-arrow" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto">
      <path d="M0,0 L0,6 L8,3 z" fill="currentColor"/>
    </marker>
  </defs>
  <!-- Untrusted source -->
  <rect id="l04-src" x="20" y="110" width="130" height="52" rx="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="85" y="132" text-anchor="middle" font-size="12" fill="currentColor">untrusted</text>
  <text x="85" y="150" text-anchor="middle" font-size="12" fill="currentColor">src io.Reader</text>
  <!-- Arrow src -> flate -->
  <line x1="150" y1="136" x2="198" y2="136" stroke="currentColor" stroke-width="1.5" marker-end="url(#l04-arrow)"/>
  <!-- flate.Reader -->
  <rect id="l04-flate" x="200" y="110" width="130" height="52" rx="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="265" y="132" text-anchor="middle" font-size="12" fill="currentColor">flate.Reader</text>
  <text x="265" y="150" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">(decompressor)</text>
  <!-- Arrow flate -> limit -->
  <line x1="330" y1="136" x2="378" y2="136" stroke="currentColor" stroke-width="1.5" marker-end="url(#l04-arrow)"/>
  <!-- io.LimitReader -->
  <rect id="l04-limit" x="380" y="100" width="140" height="72" rx="8" fill="var(--md-accent-fg-color,#00897b)" stroke="none"/>
  <text x="450" y="126" text-anchor="middle" font-size="12" fill="#fff" font-weight="600">io.LimitReader</text>
  <text x="450" y="143" text-anchor="middle" font-size="11" fill="#fff">cap: Max+1 bytes</text>
  <text x="450" y="160" text-anchor="middle" font-size="10" fill="#fff">(turnstile)</text>
  <!-- Overflow arrow downward to error -->
  <line x1="450" y1="172" x2="450" y2="218" stroke="#e5484d" stroke-width="1.5" stroke-dasharray="4,3" marker-end="url(#l04-arrow)"/>
  <rect id="l04-err" x="365" y="220" width="170" height="44" rx="8" fill="none" stroke="#e5484d" stroke-width="1.5"/>
  <text x="450" y="239" text-anchor="middle" font-size="11" fill="#e5484d">size exceeds limit</text>
  <text x="450" y="256" text-anchor="middle" font-size="10" fill="#e5484d">→ error returned</text>
  <!-- Arrow limit -> io.Copy -->
  <line x1="520" y1="136" x2="568" y2="136" stroke="currentColor" stroke-width="1.5" marker-end="url(#l04-arrow)"/>
  <!-- io.Copy + dst -->
  <rect id="l04-copy" x="570" y="110" width="110" height="52" rx="8" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.5"/>
  <text x="625" y="132" text-anchor="middle" font-size="12" fill="currentColor">io.Copy</text>
  <text x="625" y="150" text-anchor="middle" font-size="11" fill="var(--md-default-fg-color--light)">→ dst Writer</text>
  <!-- 32 KB label on arrow -->
  <text x="340" y="128" text-anchor="middle" font-size="9" fill="var(--md-default-fg-color--light)">32 KB chunks</text>
</svg>

---

## The two-method contract

An **interface** in Go is a kind of contract: it names one or more actions (called **methods** — think of a method as a function, i.e. a named, reusable chunk of instructions, that happens to belong to a particular kind of value) and says "anything that can perform these actions counts as satisfying this contract," no matter what it actually is underneath. A **byte** is the basic unit computers store data in — roughly one letter or number's worth of raw data — and `[]byte` means "a list of bytes," which Go calls a **slice** (a resizable, ordered sequence of values, similar in spirit to a numbered list you can grow or shrink).

```go
// from the Go standard library — shown here for reference
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}
```

**In plain terms:** this defines two contracts. Anything that has a `Read` method — that can fill a `[]byte` handed to it with the next chunk of data and report back how much it filled — counts as an `io.Reader`. Anything with a `Write` method — that can accept a `[]byte` and absorb (consume) it — counts as an `io.Writer`. That's the whole contract. `Read` fills `p` and returns (hands back to whoever called it) how many bytes it put there, plus any error — a special value that signals something went wrong, or a specific "end of data" signal called `io.EOF`. `Write` drains `p` and returns how many bytes it consumed. Everything else in the `io` package (a **package** is a named, reusable bundle of pre-written Go code — here, the built-in "io" bundle that ships with the language, part of what's called the **standard library**, the set of ready-made packages that come with Go itself) — `Copy`, `ReadAll`, `LimitReader`, `TeeReader`, `MultiReader` — is built on these two methods.

---

## bytes.Reader and bytes.Buffer — the in-memory bridges

Most functions in safeheaders-go receive raw `[]byte` from the **caller** (the piece of code that runs — "calls" or "invokes" — a function, here meaning the caller already loaded or received a blob of bytes). They bridge to the streaming world with `bytes.NewReader`:

From [`miniz-go/miniz.go`](src/miniz-go-miniz-go.md):

```go
func ExtractArchive(data []byte) ([]ZipFile, error) {
    r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
    // ...
}
```

**In plain terms:** this function takes a plain `[]byte` (data already sitting in memory) and wraps it so it can be handed to `zip.NewReader`, which expects the streaming-style interface rather than a raw byte list. `zip.NewReader` needs an `io.ReaderAt` (a positioned reader — one that can jump to a specific spot, or **offset**, in the data rather than only reading forward). `bytes.NewReader` provides it from a plain `[]byte`. No copy, no temp file.

The write side uses `bytes.Buffer` as the accumulator:

```go
func CreateArchive(files []FileEntry) ([]byte, error) {
    var buf bytes.Buffer
    w := zip.NewWriter(&buf)
    // ... write entries ...
    return buf.Bytes(), nil
}
```

**In plain terms:** `buf` is an empty holding area (a **buffer** — a chunk of memory used to temporarily collect data as it arrives); the zip writer sends its output into that buffer piece by piece, and at the end `buf.Bytes()` hands back everything collected as one plain `[]byte`. `bytes.Buffer` satisfies `io.Writer` (in plain terms: it has a `Write` method, so it counts as one), so `zip.NewWriter` can write directly into it. When you're done, `buf.Bytes()` hands back the accumulated slice.

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

**In plain terms:** this function reads from whatever `src` is (a file, a network connection, memory — it doesn't care) and writes the compressed result into whatever `dst` is, checking at each step whether something went wrong and, if so, reporting that back immediately instead of continuing. `io.Copy(w, src)` is the key line. It loops internally (repeats the same small step over and over on its own, without the calling code writing the loop itself), reading up to 32 KB from `src` and writing it to the compressor `w`, until `src` returns `io.EOF` (the built-in signal meaning "there is no more data to read"). The total memory in flight at any moment is just those 32 KB — not the whole input.

The caller can pass an `os.File` as `src` and a network connection as `dst`, and the function works unchanged. That's the composition payoff.

---

## io.LimitReader — the safety fence

A decompression bomb is a compressed file that expands to gigabytes. Without a fence, `io.ReadAll` on the decompressed stream will happily **allocate** (reserve a chunk of the computer's memory to hold data — every time a program needs space to store something, it has to allocate that space first) until the process is killed.

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

**In plain terms:** this function reads everything from `r`, but refuses to read more than `limit+1` bytes no matter how much data is actually available; if it hits that ceiling, it treats the input as too big and hands back an error instead of the data. The `+1` trick is intentional: `io.LimitReader` stops at exactly `limit+1` bytes. If `io.ReadAll` returns `limit+1` bytes, you know the real stream was larger and you return an error. If it returns `<= limit` bytes, you're within budget.

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
    `MaxDecompressedSize` is a package-level `var` (a variable declared once at the package level, so every part of the program can see and use the same one, rather than each function having its own private copy), readable by all **goroutines** (Go's lightweight units of work that can run at the same time as each other — this "running multiple things at once" is called **concurrency**) without a **lock** (a mechanism, often called a mutex, that lets only one goroutine touch a piece of shared data at a time, so two of them don't collide). The doc comment in the source says: *"It must be set before any concurrent decompression begins; it is read without synchronization, so mutating it while a decompress is in flight is a data race."* (A **data race** is what happens when two goroutines read and change the same piece of memory at the same time with no coordination — the result becomes unpredictable.) Set it once at startup and leave it alone.

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

**In plain terms:** this function takes a whole block of JSON text already sitting in memory as `data`, and fills in the value `v` with what it parsed from that text, reporting an error if the text is empty or malformed. The streaming variants take `io.Reader` / `io.Writer`:

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

**In plain terms:** `UnmarshalStream` reads JSON text directly from a stream (rather than requiring it all in memory first) and fills in `v`; `MarshalStream` does the reverse, writing `v` back out as JSON text directly into a stream. `json.NewDecoder` reads from the `io.Reader` incrementally (a little at a time, as it goes, rather than all at once). You can pass it an HTTP response body directly, and the JSON parser reads from the network as it goes — no need to slurp the whole body first.

!!! warning "UnmarshalStream does not impose a size limit"
    The doc comment in the source is explicit: *"It does not impose a size limit, so for untrusted input callers MUST wrap r in an io.LimitReader (or http.MaxBytesReader) to bound memory."* Always wrap before passing an untrusted reader:

    ```go
    limited := io.LimitReader(resp.Body, 4<<20) // 4 MiB
    err := cjsongo.UnmarshalStream(limited, &result)
    ```

---

## LoadStream — streaming image decode with a header peek

[`stb-image-go/stb_image.go`](src/stb-image-go-stb-image-go.md) shows a slightly more involved pattern: it needs to read the image header (to check dimensions) and then re-read the full stream for decoding — but it has only one `io.Reader`, and reading consumes bytes (once you've read a byte from a stream, it's gone from that stream — there's no rewinding without special help, which is the problem this pattern solves).

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

**In plain terms:** the function first peeks at just the image's header (a small chunk at the start that describes its width and height) to reject anything absurdly large, while simultaneously saving a copy of those header bytes; then it glues that saved copy back onto the front of the stream so the real decoder can read the file from the very beginning, as if nothing had been peeked at all. Three `io` helpers work together here:

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
