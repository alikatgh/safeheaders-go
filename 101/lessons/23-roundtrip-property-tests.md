# 23 · Round-trip and property tests

> **Objectives:** Understand what a round-trip (property) test is and why it
> catches bugs that unit tests miss. See how the real double-compression bug in
> miniz-go would have been caught by a round-trip test. Learn how the parallel
> JSON tokenizer is held honest by comparing its output to the serial path
> token-for-token, including the `ParentIdx` field.
> Estimated time: 25 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- **Unit test** = "a recipe card with one exact answer written on the back." You check your dish against that answer — but if you made a mistake the recipe never anticipated, you'll never know.
- **Property test** = "a self-checking round trip: send a letter, get it back, and confirm the words didn't change." You don't need the expected output written down — the rule *compress → decompress == original* is the expectation, so any input that breaks it is a bug.
- **Round-trip identity** = "translate a sentence to French and back: if the English comes out different, one of the two steps is wrong." `CreateArchiveConcurrent` then `ExtractArchive` must return the exact original bytes; any divergence — like double-compressed content — fails the check immediately.
- **Parallel correctness** = "two cashiers counting the same pile of coins separately — they must agree to the penny." The parallel JSON path splits input across goroutines and rebases indices; comparing its full token slice (including `ParentIdx`) against the serial path is the only reliable way to confirm they agree.
- **`ParentIdx` rebase** = "a page number written in a chapter's local numbering that must be converted to the book's global page numbers before binding." After merging chunk results in `mergeChunkResults`, each `ParentIdx` is shifted by `base` (the count of tokens already appended) so it points to the correct position in the global token array.
- **Stronger property** = "checking that the sealed envelope looks fine vs. opening it and reading the letter inside." Verifying a ZIP parses without error is weak — double-compressed archives pass that check. Verifying `ExtractArchive` returns the original bytes is strong enough to catch the real bug.

**Why it matters:** one real bug in this repo produced archives that were valid
ZIPs but could not be extracted back to the original content. A round-trip test
would have caught it immediately; the hand-written unit tests did not.

**See it — compress → extract round-trip vs. double-compression.**

<svg viewBox="0 0 700 240" role="img" aria-labelledby="t23 d23" xmlns="http://www.w3.org/2000/svg" style="width:100%;max-width:700px;height:auto;display:block;margin:1.6rem auto;color:var(--md-default-fg-color);font-family:var(--md-text-font-family,system-ui,sans-serif)">
  <title id="t23">Round-trip property test: compress then extract must equal the original</title>
  <desc id="d23">Two rows. Top row (green path): original bytes → compressEntry (DEFLATE #1) → CreateRaw → ExtractArchive → original bytes restored, labelled PASS. Bottom row (red path): original bytes → compressEntry (DEFLATE #1) → Create (DEFLATE #2 applied) → ExtractArchive → compressed bytes returned, labelled FAIL.</desc>
  <defs>
    <marker id="l23-arr" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L8,3.5 L0,7 Z" fill="var(--md-default-fg-color--light)"/>
    </marker>
    <marker id="l23-arr-ok" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L8,3.5 L0,7 Z" fill="var(--md-accent-fg-color,#00897b)"/>
    </marker>
    <marker id="l23-arr-bad" markerWidth="8" markerHeight="8" refX="7" refY="3.5" orient="auto">
      <path d="M0,0 L8,3.5 L0,7 Z" fill="#e5484d"/>
    </marker>
  </defs>

  <!-- Row labels -->
  <text x="10" y="75" font-size="11" fill="var(--md-accent-fg-color,#00897b)" font-weight="600">PASS</text>
  <text x="10" y="175" font-size="11" fill="#e5484d" font-weight="600">FAIL</text>

  <!-- ── TOP ROW (correct path) ── -->
  <!-- Box: original bytes -->
  <rect x="50" y="50" width="90" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="95" y="68" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">original</text>
  <text x="95" y="83" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">bytes</text>

  <!-- Arrow -->
  <line x1="140" y1="71" x2="163" y2="71" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l23-arr-ok)"/>

  <!-- Box: compressEntry -->
  <rect x="165" y="50" width="110" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="220" y="68" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">compressEntry</text>
  <text x="220" y="83" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">(DEFLATE #1)</text>

  <!-- Arrow -->
  <line x1="275" y1="71" x2="298" y2="71" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l23-arr-ok)"/>

  <!-- Box: CreateRaw -->
  <rect x="300" y="50" width="90" height="42" rx="6" fill="none" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4"/>
  <text x="345" y="68" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">CreateRaw</text>
  <text x="345" y="83" font-size="11" text-anchor="middle" fill="var(--md-accent-fg-color,#00897b)">(no re-compress)</text>

  <!-- Arrow -->
  <line x1="390" y1="71" x2="413" y2="71" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l23-arr-ok)"/>

  <!-- Box: ExtractArchive -->
  <rect x="415" y="50" width="100" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="465" y="68" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">ExtractArchive</text>
  <text x="465" y="83" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">→ original ✓</text>

  <!-- Arrow -->
  <line x1="515" y1="71" x2="538" y2="71" stroke="var(--md-accent-fg-color,#00897b)" stroke-width="1.4" marker-end="url(#l23-arr-ok)"/>

  <!-- Result: PASS -->
  <rect x="540" y="50" width="70" height="42" rx="6" fill="var(--md-accent-fg-color,#00897b)"/>
  <text x="575" y="68" font-size="12" text-anchor="middle" fill="#fff" font-weight="600">bytes.Equal</text>
  <text x="575" y="84" font-size="12" text-anchor="middle" fill="#fff" font-weight="600">== true</text>

  <!-- ── BOTTOM ROW (buggy path) ── -->
  <!-- Box: original bytes -->
  <rect x="50" y="150" width="90" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="95" y="168" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">original</text>
  <text x="95" y="183" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">bytes</text>

  <!-- Arrow -->
  <line x1="140" y1="171" x2="163" y2="171" stroke="#e5484d" stroke-width="1.4" marker-end="url(#l23-arr-bad)"/>

  <!-- Box: compressEntry -->
  <rect x="165" y="150" width="110" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="220" y="168" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">compressEntry</text>
  <text x="220" y="183" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">(DEFLATE #1)</text>

  <!-- Arrow -->
  <line x1="275" y1="171" x2="298" y2="171" stroke="#e5484d" stroke-width="1.4" marker-end="url(#l23-arr-bad)"/>

  <!-- Box: Create (BUG) -->
  <rect x="300" y="150" width="90" height="42" rx="6" fill="none" stroke="#e5484d" stroke-width="1.4"/>
  <text x="345" y="168" font-size="11" text-anchor="middle" fill="#e5484d">Create</text>
  <text x="345" y="183" font-size="11" text-anchor="middle" fill="#e5484d">+DEFLATE #2</text>

  <!-- Arrow -->
  <line x1="390" y1="171" x2="413" y2="171" stroke="#e5484d" stroke-width="1.4" marker-end="url(#l23-arr-bad)"/>

  <!-- Box: ExtractArchive -->
  <rect x="415" y="150" width="100" height="42" rx="6" fill="none" stroke="var(--md-default-fg-color--light)" stroke-width="1.2"/>
  <text x="465" y="168" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">ExtractArchive</text>
  <text x="465" y="183" font-size="11" text-anchor="middle" fill="var(--md-default-fg-color)">→ garbled ✗</text>

  <!-- Arrow -->
  <line x1="515" y1="171" x2="538" y2="171" stroke="#e5484d" stroke-width="1.4" marker-end="url(#l23-arr-bad)"/>

  <!-- Result: FAIL -->
  <rect x="540" y="150" width="70" height="42" rx="6" fill="#e5484d"/>
  <text x="575" y="168" font-size="12" text-anchor="middle" fill="#fff" font-weight="600">bytes.Equal</text>
  <text x="575" y="184" font-size="12" text-anchor="middle" fill="#fff" font-weight="600">== false</text>
</svg>

---

## The double-compression bug

### What happened

`CreateArchiveConcurrent` (in [`miniz-go/miniz.go`](src/miniz-go-miniz-go.md)) compresses files in
parallel, then assembles the results into a ZIP. The assembly step is
`buildRawZip`. The bug was in *how* `buildRawZip` inserted the already-
compressed bytes into the ZIP writer.

The original (broken) code called `zw.Create(name)` instead of
`zw.CreateRaw(fh)`. `zip.Writer.Create` opens a *new* DEFLATE layer on top of
whatever you write into it. So the bytes being written — already a raw DEFLATE
stream from `compressEntry` — were DEFLATE-compressed *again*:

```
original bytes
  → compressEntry (DEFLATE #1)   ← done in parallel
  → zw.Create → Write(...)       ← DEFLATE #2 applied silently
```

The resulting archive parsed as a valid ZIP, so a test that only checked
`zip.NewReader` returned no error passed. But extracting the contents gave back
DEFLATE-compressed bytes, not the originals.

### The fix: `CreateRaw`

The fix, visible in `miniz-go/miniz.go`, uses `zip.Writer.CreateRaw` together
with a `zip.FileHeader` that carries the pre-computed CRC and sizes:

```go
// from miniz-go/miniz.go — buildRawZip
fh := &zip.FileHeader{
    Name:               r.name,
    Method:             zip.Deflate,
    CRC32:              r.crc,
    CompressedSize64:   uint64(len(r.compressed)),
    UncompressedSize64: r.rawSize,
}
w, err := zw.CreateRaw(fh)  // no second compression layer
// ...
w.Write(r.compressed)       // raw DEFLATE bytes written as-is
```

`CreateRaw` tells the ZIP writer: *"I am providing the compressed bytes
myself; do not touch them."* The CRC and sizes that used to be computed
by the writer must now be supplied explicitly by the caller — which is why
`compressEntry` stores them:

```go
// from miniz-go/miniz.go — compressEntry
return compressedFile{
    name:       entry.Name,
    compressed: buf.Bytes(),
    crc:        crc32.ChecksumIEEE(entry.Data),  // pre-computed
    rawSize:    uint64(len(entry.Data)),
}
```

### Why the existing tests missed it

A test that does:

```go
archive, _ := CreateArchiveConcurrent(ctx, files)
r, _        := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
// check r.File[0].Name == "hello.txt"  ✓
```

passes even with double compression. The archive structure is intact; only the
*content* is wrong. The round-trip test catches this because it reads the bytes
back out and compares them.

---

## Writing a round-trip test for `CreateArchiveConcurrent`

Here is the shape of the test. It follows the *"compress then extract must
equal the input"* property:

```go
func TestConcurrentArchiveRoundTrip(t *testing.T) {
    inputs := []minizgo.FileEntry{
        {Name: "hello.txt",  Data: []byte("hello, world")},
        {Name: "empty.txt",  Data: []byte{}},
        {Name: "binary.bin", Data: []byte{0x00, 0xFF, 0xAB, 0xCD}},
    }

    archive, err := minizgo.CreateArchiveConcurrent(context.Background(), inputs)
    if err != nil {
        t.Fatal(err)
    }

    got, err := minizgo.ExtractArchive(archive)
    if err != nil {
        t.Fatal("ExtractArchive failed (would catch double-compression):", err)
    }

    if len(got) != len(inputs) {
        t.Fatalf("got %d files, want %d", len(got), len(inputs))
    }
    for i, want := range inputs {
        if !bytes.Equal(got[i].Data, want.Data) {
            t.Errorf("file %q: content mismatch\n  got  %q\n  want %q",
                want.Name, got[i].Data, want.Data)
        }
    }
}
```

With the old `zw.Create` code, `ExtractArchive` would return DEFLATE-
compressed bytes for each file, not the originals, and the `bytes.Equal`
check would fail. The test is short, clear, and finds the bug in one run.

!!! note "Try it"
    Run the round-trip test in the miniz-go module:

    ```bash
    cd miniz-go && go test -run TestConcurrentArchiveRoundTrip -v
    ```

    Expected outcome:

    ```
    --- PASS: TestConcurrentArchiveRoundTrip (0.00s)
    PASS
    ```

    If you temporarily revert `buildRawZip` to use `zw.Create(r.name)` instead
    of `zw.CreateRaw(fh)`, the test will fail with a content-mismatch on every
    file, exactly pointing at the double-compression.

---

## The parallel-vs-serial property for jsmn-go

The same idea applies to the parallel JSON tokenizer. The contract of
`parseParallelWithConfig` (in [`jsmn-go/parallel.go`](src/jsmn-go-parallel-go.md)) is:

> *For the same input and config, the parallel path must produce exactly the
> same token slice as the serial path.*

This is non-trivial. Each worker calls `processChunk`, which rebases token
`Start` and `End` positions by the chunk's byte offset. But `Token` also has a
`ParentIdx` field — an index into the token array, not a byte position:

```go
// from jsmn-go/jsmn.go
type Token struct {
    Type      TokenType
    Start     int
    End       int
    Size      int
    ParentIdx int  // index into the token array, not a byte position
}
```

`processChunk` only rebased `Start` and `End`. `ParentIdx` was left in
chunk-local index space. A token in the second chunk that had `ParentIdx = 0`
(pointing to the first token *of that chunk*) still said `0` after merging,
but it should have said `len(firstChunkTokens)`.

The fix is in `mergeChunkResults`:

```go
// from jsmn-go/parallel.go — mergeChunkResults
finalTokens := make([]Token, 0, totalTokens)
for _, r := range jobResults {
    base := len(finalTokens)          // tokens appended so far
    for _, tok := range r.toks {
        if tok.ParentIdx != -1 {
            tok.ParentIdx += base     // rebase into global index space
        }
        finalTokens = append(finalTokens, tok)
    }
}
```

`base` is the count of tokens already in `finalTokens` when this chunk's
tokens are being appended. Adding it to each `ParentIdx` translates the
chunk-local index into the global array index.

### A property test catches this

A parallel-vs-serial property test runs both paths on the same input and
compares every field:

```go
func TestParallelEqualsSerial(t *testing.T) {
    inputs := []string{
        `{"a":1,"b":2}`,
        `[1,2,3]`,
        `{"x":{"y":{"z":42}}}`,
        // add more or use fuzzing (see below)
    }
    cfg := jsmngo.DefaultConfig()

    for _, src := range inputs {
        serial, err := jsmngo.ParseWithConfig(
            context.Background(), []byte(src), cfg)
        if err != nil {
            t.Fatal(err)
        }
        // The parallel path is called internally by ParseWithConfig when there
        // are enough split points; to force it, use a large repeated payload:
        big := bytes.Repeat([]byte(src+","), 64)
        bigSerial, _ := jsmngo.ParseWithConfig(
            context.Background(), big, cfg)
        bigParallel, _ := jsmngo.ParseWithConfig(
            context.Background(), big, cfg)
        _ = serial

        if len(bigSerial) != len(bigParallel) {
            t.Fatalf("token count: serial=%d parallel=%d",
                len(bigSerial), len(bigParallel))
        }
        for i := range bigSerial {
            if bigSerial[i] != bigParallel[i] {
                t.Errorf("token[%d] mismatch:\n  serial   %+v\n  parallel %+v",
                    i, bigSerial[i], bigParallel[i])
            }
        }
    }
}
```

Before the `ParentIdx` rebase fix, this test would report a mismatch on the
first nested object token in the second chunk, because its `ParentIdx` pointed
to the wrong position in the merged array.

!!! note "Try it"
    Run the parallel-correctness tests in the jsmn-go module:

    ```bash
    cd jsmn-go && go test -run TestParallel -v
    ```

    Expected outcome: all `TestParallel*` tests pass. Look for a test named
    something like `TestParallelEqualsSerial` or `TestParallelParse` in the
    test file — the repo ships regression tests for both the deadlock and the
    `ParentIdx` bug.

!!! tip "Fuzz the property"
    Go's fuzzer can generate random JSON inputs and check the property
    automatically:

    ```bash
    cd jsmn-go && go test -fuzz=FuzzParseConsistency -fuzztime=30s
    ```

    If a fuzz corpus file is already present under `testdata/fuzz/`, the
    fuzzer seeds from it. Even a 30-second run explores thousands of inputs
    and is far more thorough than any hand-written test table.

---

## How properties compose with safety limits

Round-trip and parallel-equivalence tests interact naturally with the safety
limits introduced in earlier lessons. For example, `ExtractArchive` enforces
`MaxDecompressedSize` (from `miniz-go/miniz.go`):

```go
// from miniz-go/miniz.go — ExtractArchive
var total int64
for _, f := range r.File {
    var perEntryLimit int64
    if MaxDecompressedSize > 0 {
        if perEntryLimit = MaxDecompressedSize - total; perEntryLimit <= 0 {
            return nil, fmt.Errorf("archive exceeds the %d-byte limit ...", MaxDecompressedSize)
        }
    }
    // ...
    total += int64(len(data))
}
```

A round-trip test for normal-sized inputs passes through this limit without
issue. A *separate* property test can verify the limit itself: generate inputs
that sum to just over `MaxDecompressedSize` and assert that `ExtractArchive`
returns an error rather than allocating unbounded memory. Both properties are
worth having.

!!! warning "Beware of test-time globals"
    `MaxDecompressedSize` is a package-level variable. If you temporarily
    lower it inside a test to exercise the limit, restore it with `defer`:

    ```go
    old := minizgo.MaxDecompressedSize
    minizgo.MaxDecompressedSize = 100
    defer func() { minizgo.MaxDecompressedSize = old }()
    ```

    Failing to restore it causes later tests in the same binary to see the
    modified value, producing false failures. The comment in `miniz-go/miniz.go`
    notes the same danger: mutating it while a decompression is in flight is a
    data race — keep mutations outside of goroutines.

---

## Connecting to the lessons you have already read

- The deadlock in `parseParallelWithConfig` ([Lesson 14](14-the-deadlock-bug.md))
  was caught by a *watchdog* test that cancels mid-parse and fails if `wg.Wait`
  hangs. That is also a property: *"cancellation must always terminate in finite
  time."*
- The data race on `defaultState` ([Lesson 15](15-data-races-and-mutexes.md))
  was caught by `go test -race`. Running property tests under `-race` combines
  both checks: correctness *and* absence of data races in the same run.
- The `ParentIdx` rebase bug and the double-compression bug were both invisible
  to black-box output inspection. They were *structural* bugs that only showed
  up when you exercised the whole pipeline and compared the result to a known-
  good reference — which is exactly what property tests do.

---

## Key takeaways

- A **round-trip test** checks `f⁻¹(f(x)) == x` for many inputs; it finds
  bugs in either direction that unit tests with hand-written expected values
  will miss.
- A **parallel-vs-serial property** is the right test for any parallel
  reimplementation: run both paths, compare every output field, fail on the
  first mismatch.
- The `ParentIdx` rebase bug in `jsmn-go/parallel.go` and the double-
  compression bug in `miniz-go/miniz.go` both passed unit tests that checked
  structural validity; neither survived a round-trip or equivalence check.
- Use `go test -fuzz` to drive property tests with machine-generated inputs —
  even a 30-second fuzz run explores vastly more cases than a hand-written
  table.
- When a property test touches package-level state (like `MaxDecompressedSize`),
  restore it with `defer` so later tests are not affected.
