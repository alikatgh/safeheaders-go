# SafeHeaders-Go 101

**From zero to a production-grade, secure Go library — taught entirely from this repository's real code.**

This is a hands-on course built around [`safeheaders-go`](https://github.com/alikatgh/safeheaders-go):
nine pure-Go ports of popular C single-header libraries — JSON, XML, and glTF parsers, a WAV
decoder, a ZIP/DEFLATE codec, an image loader, a CLI line editor, and a from-scratch
**TrueType font rasterizer**. Together they are a working museum of the things a real library
must get right: parsing untrusted input without crashing, fanning work across goroutines
without deadlocking, resisting denial-of-service, and proving all of it with tests, fuzzing,
linting, CI, and a security audit.

## Who this is for

You can read a little code (any language) and want to learn how *production* Go is actually
written and hardened. You do **not** need to know Go yet — Module 1 starts from packages and
error handling. By the end you will be able to read every line of this repo and explain why
it is the way it is.

## What makes it different

!!! quote "No toy examples"
    When this course teaches a deadlock, it is a bug this project *shipped*: a results channel
    sized for `numJobs` when workers could send `numJobs + numWorkers` messages, so cancelling
    a parse mid-flight wedged the whole pool. You will see the break, the goroutine dump, the
    one-line fix, and the watchdog test that now guards it. Same for an out-of-memory
    allocation, an image decode bomb, a zip bomb, and a "billion-laughs" font.

- **Real code, with citations.** Every snippet names its source file. Keep the repo open.
- **Foundations, then depth.** Go and error-handling first; then a six-lesson arc that builds
  the TrueType rasterizer from the byte layout of a font file up to anti-aliased pixels.
- **The bug classes that matter.** Two whole modules — concurrency and security — are built
  around defects that actually occurred here and how fuzzing and a 10-agent code audit caught
  them.

## The map

| Module | You'll learn to… |
|--------|------------------|
| **1 · Go & the workspace** | Read a Go module, wrap errors the idiomatic way, and use `io.Reader`/`io.Writer`. |
| **2 · Parsing untrusted input** | Tokenize text, lean on the stdlib safely, and decode binary formats byte by byte — culminating in a real TrueType rasterizer. |
| **3 · Concurrency** | Build worker pools, cancel them with `context`, and avoid the deadlocks and data races this repo had to fix. |
| **4 · Security & DoS** | Stop OOM allocations, decode/decompression bombs, and unbounded recursion with a clean configurable-limits pattern. |
| **5 · Testing & fuzzing** | Write table tests, run `-race`, and fuzz parsers — the way real bugs here were found. |
| **6 · Tooling, CI & the audit** | Wire up `golangci-lint`, GitHub Actions, `govulncheck`/`gosec`, and learn how a multi-agent audit reviews a codebase. |

## How to work through it

1. **Read the real file.** Each lesson points at code in the repo — open it.
2. **Run the "Try it."** Short commands you paste into a terminal; predict the output first.
3. **Do the lab.** At the end of each module you build or break something yourself.
4. **Explain it back.** If you can't, re-read — that's the signal, not a failure.

> **Prerequisites:** Go 1.23+ (`go version`), a terminal, and this repo cloned. That's it.

Start with **[Lesson 01 — Why pure-Go ports of C libraries](01-why-pure-go-ports.md)**.
