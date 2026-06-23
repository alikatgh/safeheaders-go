# SafeHeaders-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/alikatgh/safeheaders-go/jsmn-go)](https://goreportcard.com/report/github.com/alikatgh/safeheaders-go/jsmn-go)
[![Tests](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml/badge.svg)](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml)
[![Coverage](https://codecov.io/gh/alikatgh/safeheaders-go/branch/main/graph/badge.svg)](https://codecov.io/gh/alikatgh/safeheaders-go)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Security](https://img.shields.io/badge/Security-Scanning-brightgreen)](SECURITY.md)
[![GoDoc](https://pkg.go.dev/badge/github.com/alikatgh/safeheaders-go)](https://pkg.go.dev/github.com/alikatgh/safeheaders-go)

**Production-ready, pure Go implementations of popular single-header C libraries with built-in concurrency support and zero CGO dependencies.**

> **Status:** 8 modules are production-ready; `stb-truetype-go` is Beta (font
> parsing + glyph cache are solid, but the rasterizer is a placeholder). Every
> module is lint-clean, race-tested, and above the 70% coverage gate.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Available Modules](#available-modules)
- [Installation](#installation)
- [Examples](#examples)
- [Performance](#performance)
- [Why SafeHeaders-Go?](#why-safeheaders-go)
- [Production Usage](#production-usage)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Features

- ✅ **Memory Safe** - No buffer overflows, bounds-checked, zero unsafe pointers
- ✅ **Concurrent** - Built-in goroutine support for parallel processing
- ✅ **Zero Dependencies** - Pure Go stdlib, no CGO required
- ✅ **Easy Integration** - Drop-in packages for any Go project
- ✅ **Production Ready** - Comprehensive tests, fuzz tests, security scanning
- ✅ **Well Documented** - Full godoc, examples, and usage guides
- ✅ **Actively Maintained** - Regular updates, security patches, community support

## Quick Start

### Installation

```bash
# Install a specific module
go get github.com/alikatgh/safeheaders-go/jsmn-go

# Or multiple modules
go get github.com/alikatgh/safeheaders-go/jsmn-go \
       github.com/alikatgh/safeheaders-go/stb-image-go \
       github.com/alikatgh/safeheaders-go/tinyxml2-go
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
    json := []byte(`{"name": "SafeHeaders-Go", "version": "0.5.0", "stable": true}`)

    // Option 1: Serial parsing (for small inputs)
    p := jsmngo.NewParser(100)
    count, err := p.Parse(json)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed %d tokens\n", count)

    // Option 2: Parallel parsing (for large inputs)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    tokens, err := jsmngo.ParseParallel(ctx, json)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed %d tokens in parallel\n", len(tokens))
}
```

## Available Modules

Coverage figures below are the measured `go test -cover` totals for each module
(enforced at a 70% minimum in CI).

| Module | Status | Version | Coverage | Description |
|--------|--------|---------|----------|-------------|
| [cgltf-go](./cgltf-go) | 🟢 **Stable** | v0.5.0 | 93% | glTF 3D model loading with parallel assets |
| [tinyxml2-go](./tinyxml2-go) | 🟢 **Stable** | v0.5.0 | 89% | XML DOM parsing with element traversal |
| [stb-image-go](./stb-image-go) | 🟢 **Stable** | v0.5.0 | 89% | Image loading with batch decoding (PNG, JPEG, GIF) |
| [jsmn-go](./jsmn-go) | 🟢 **Stable** | v0.5.0 | 88% | Fast JSON tokenizer with parallel parsing |
| [cjson-go](./cjson-go) | 🟢 **Stable** | v0.5.0 | 83% | JSON marshaling/unmarshaling with parallel processing |
| [dr-wav-go](./dr-wav-go) | 🟢 **Stable** | v0.5.0 | 82% | WAV audio (RIFF/PCM) parsing with concurrent decoding |
| [stb-truetype-go](./stb-truetype-go) | 🟡 **Beta** | v0.5.0 | 81% | TrueType font file loading + LRU glyph cache (rasterizer is a placeholder¹) |
| [miniz-go](./miniz-go) | 🟢 **Stable** | v0.5.0 | 79% | ZIP compression with concurrent chunking |
| [linenoise-go](./linenoise-go) | 🟢 **Stable** | v0.1.0 | 77% | Minimal line editing library for CLI apps |

> ¹ `stb-truetype-go` parses font files and provides a thread-safe, bounded LRU
> glyph cache, but ships a **placeholder rasterizer** that returns a blank
> bitmap. Supply your own rasterizer for real glyph rendering. Treat the
> glyph-image output as not-yet-production.

**Status Legend:**
- 🟢 **Stable** - Production-ready, full test coverage, security-audited
- 🟡 **Beta** - Core features complete, API may change
- 🔴 **Alpha** - Experimental, not recommended for production

**8 of 9 modules are production-ready; `stb-truetype-go` is Beta** (see the note above).

## Examples

Comprehensive examples are available in the [`examples/`](./examples) directory:

### Available Examples

1. **[JSON Parser](./examples/json-parser/)** - Production-ready JSON parsing with validation
   ```bash
   cd examples/json-parser && go run main.go
   ```

2. **[Production HTTP Service](./examples/production-usage/)** - Complete HTTP server with SafeHeaders-Go
   ```bash
   cd examples/production-usage && go run main.go
   ```

3. **[Linenoise REPL](./examples/linenoise-repl/)** - Interactive command-line with history and completion
   ```bash
   cd examples/linenoise-repl && go run main.go
   ```

4. **More Examples** - See [`examples/README.md`](./examples/README.md) for the full list

### Quick Example Snippets

<details>
<summary><b>Parse Large JSON in Parallel</b></summary>

```go
import (
    "context"
    "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func parseJSON(data []byte) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    tokens, err := jsmngo.ParseParallel(ctx, data)
    if err != nil {
        return err
    }

    fmt.Printf("Parsed %d tokens\n", len(tokens))
    return nil
}
```
</details>

<details>
<summary><b>Load Images Concurrently</b></summary>

```go
import (
    "context"
    "github.com/alikatgh/safeheaders-go/stb-image-go"
)

func loadImages(files [][]byte) error {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    images, err := stbimagego.LoadBatchConcurrent(ctx, files)
    if err != nil {
        return err
    }

    fmt.Printf("Loaded %d images\n", len(images))
    return nil
}
```
</details>

<details>
<summary><b>Parse XML Document</b></summary>

```go
import "github.com/alikatgh/safeheaders-go/tinyxml2-go"

func parseXML(data []byte) error {
    doc := tinyxml2go.NewDocument()
    if err := doc.Parse(data); err != nil {
        return err
    }

    root := doc.RootElement()
    fmt.Printf("Root element: %s\n", root.Name())
    return nil
}
```
</details>

## Performance

Each parser/codec module ships benchmarks. Run them on your own hardware:

```bash
make bench                       # jsmn-go, stb-image-go, stb-truetype-go
cd jsmn-go && go test -bench=. -benchmem -run='^$' ./...
```

**Where the concurrency helps (and where it doesn't):**

- `jsmn-go` `ParseParallel` / `ParseWithConfig` split the input at **top-level
  delimiters** and tokenize chunks concurrently. This pays off on large streams
  of many top-level values; a single large object or array has no top-level
  split points and **transparently falls back to serial parsing**. Inputs below
  4 KB always parse serially (the goroutine overhead is not worth it).
- `stb-image-go`, `cgltf-go`, `dr-wav-go`, `miniz-go` parallelize across
  **independent items** (a batch of images / assets / files), so speedup scales
  with the number of items, not the size of any single one.

Because throughput depends heavily on CPU count, input shape, and allocator
behavior, this README intentionally does not quote fixed numbers — measure on
your target hardware with the commands above.

## Why SafeHeaders-Go?

Traditional C libraries are fast but unsafe. Go stdlib is safe but doesn't parallelize parsing. SafeHeaders-Go gives you both:

### vs C Libraries

| Feature | C Libraries | SafeHeaders-Go |
|---------|-------------|----------------|
| Memory Safety | ❌ Manual | ✅ Automatic |
| CGO Required | ✅ Yes | ❌ No |
| Cross-Compilation | ❌ Hard | ✅ Easy |
| Concurrency | ❌ Manual | ✅ Built-in |
| Deployment | ❌ Complex | ✅ Simple |

### vs Go Stdlib

| Feature | Go Stdlib | SafeHeaders-Go |
|---------|-----------|----------------|
| Safety | ✅ Yes | ✅ Yes |
| Parallel Parsing | ❌ No | ✅ Yes |
| Context Support | ✅ Yes | ✅ Yes |
| Zero Dependencies | ✅ Yes | ✅ Yes |
| Performance | ⚠️ Good | ✅ Better (for large inputs) |

### vs Other Go Ports

- **Concurrency-First Design** - Built for parallel processing from the ground up
- **Production Ready** - Comprehensive testing, security scanning, examples
- **Well Maintained** - Regular updates, active community support
- **Zero Dependencies** - Pure stdlib, no external packages

## Production Usage

SafeHeaders-Go is designed for production use. Here's how to use it safely:

### Input Validation

```go
const MaxInputSize = 10 * 1024 * 1024 // 10MB

func validateAndParse(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty input")
    }
    if len(data) > MaxInputSize {
        return fmt.Errorf("input too large: %d bytes", len(data))
    }

    // Parse with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    _, err := jsmngo.ParseParallel(ctx, data)
    return err
}
```

### Error Handling

```go
tokens, err := jsmngo.ParseParallel(ctx, data)
if err != nil {
    if errors.Is(err, context.Canceled) {
        return errors.New("parsing canceled")
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return errors.New("parsing timeout")
    }
    return fmt.Errorf("parse failed: %w", err)
}
```

### Best Practices

1. **Always use context timeouts** for long-running operations
2. **Validate input size** before processing (see [SECURITY.md](./SECURITY.md))
3. **Handle errors gracefully** and provide meaningful messages
4. **Monitor memory usage** in production (see examples)
5. **Use parallel mode** only for inputs >4KB (overhead otherwise)

See [examples/production-usage](./examples/production-usage) for a complete production-ready HTTP service.

## Documentation

- 📖 [**CONTRIBUTING.md**](./CONTRIBUTING.md) - How to contribute, coding standards, architecture
- 🔒 [**SECURITY.md**](./SECURITY.md) - Security policy, vulnerability reporting, best practices
- 🐛 [**ISSUES.md**](./ISSUES.md) - Known issues, roadmap, improvement tracker
- 📝 [**CHANGELOG.md**](./CHANGELOG.md) - Version history, release notes
- 🤝 [**CODE_OF_CONDUCT.md**](./CODE_OF_CONDUCT.md) - Community guidelines
- 💡 [**Examples**](./examples/) - Comprehensive usage examples
- 📦 [**Module READMEs**](./jsmn-go/README.md) - Detailed documentation for each module
- 🧪 [**Test Data**](./testdata/) - Benchmark and test data files

### Module Documentation

Each module has comprehensive documentation:

- **Installation & Quick Start**
- **API Reference** with godoc
- **Performance Benchmarks**
- **Known Limitations**
- **Examples & Use Cases**

Visit the module directories for detailed docs.

## Contributing

We welcome contributions! 🎉

### How to Contribute

1. **Report Bugs** - Use our [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml)
2. **Request Features** - Use our [feature request template](.github/ISSUE_TEMPLATE/feature_request.yml)
3. **Port New Libraries** - Use our [port request template](.github/ISSUE_TEMPLATE/port_request.yml)
4. **Submit PRs** - Follow our [pull request template](.github/PULL_REQUEST_TEMPLATE.md)
5. **Improve Docs** - Documentation improvements are always welcome
6. **Add Tests** - Increase coverage, add fuzz tests
7. **Optimize Performance** - Improve parallel algorithms

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed guidelines.

### Development Setup

```bash
# Clone repository
git clone https://github.com/alikatgh/safeheaders-go.git
cd safeheaders-go

# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run linter
golangci-lint run --config .golangci.yml
```

### Wishlist - Port New Libraries

Want to port a new C library? Pick one:

- [ ] [linenoise.h](https://github.com/antirez/linenoise) - CLI input editing
- [ ] [stb_vorbis.h](https://github.com/nothings/stb/blob/master/stb_vorbis.c) - Ogg Vorbis decoder
- [ ] [tinyobjloader.h](https://github.com/tinyobjloader/tinyobjloader) - OBJ 3D model loader
- [ ] [stb_perlin.h](https://github.com/nothings/stb/blob/master/stb_perlin.h) - Perlin noise
- [ ] [utf8.h](https://github.com/sheredom/utf8.h) - UTF-8 utilities

See [CONTRIBUTING.md](./CONTRIBUTING.md) for porting guidelines.

## Security

Security is a top priority for SafeHeaders-Go.

### Reporting Vulnerabilities

**DO NOT** open a public issue for security vulnerabilities. Instead:

1. **Email:** security@safeheaders.dev (or use GitHub Security Advisories)
2. **Include:** Detailed description, proof of concept, impact assessment
3. **Timeline:** We aim to respond within 48 hours

See [SECURITY.md](./SECURITY.md) for complete details.

### Security Features

- ✅ **Input Validation** - Configurable size limits (coming in v1.0)
- ✅ **Bounds Checking** - No buffer overflows
- ✅ **Context Timeouts** - Prevent DoS attacks
- ✅ **Memory Safety** - Pure Go, no unsafe pointers
- ✅ **Security Scanning** - Automated gosec and govulncheck in CI
- ✅ **Dependency Management** - Zero external dependencies
- ✅ **Regular Audits** - Automated security scans weekly

### Known Security Considerations

See [SECURITY.md](./SECURITY.md) for:
- DoS prevention guidelines
- Input size recommendations
- Context timeout best practices
- Memory usage monitoring

## CI/CD & Quality

- ✅ **Automated Testing** - All modules tested on every commit
- ✅ **Race Detection** - Tests run with `-race` flag
- ✅ **Coverage Tracking** - Minimum 70% coverage enforced
- ✅ **Security Scanning** - gosec and govulncheck on every PR
- ✅ **Linting** - golangci-lint with 50+ linters
- ✅ **Fuzz Testing** - Automated fuzzing for parser modules
- ✅ **Benchmarking** - Performance regression detection
- ✅ **Multi-OS Testing** - Linux, macOS, Windows
- ✅ **Dependabot** - Automated dependency updates

## Roadmap

See [ISSUES.md](./ISSUES.md) for detailed roadmap. Highlights:

### v1.0.0 (Q1 2026)
- Stable API guarantee
- Input validation with configurable limits
- Improved error handling consistency
- Smart chunking for better parallel performance
- Official security audit

### v1.1.0 (Q2 2026)
- Streaming APIs for large files
- WebAssembly support
- Additional C library ports
- Performance optimizations

## Community & Support

- 💬 [GitHub Discussions](https://github.com/alikatgh/safeheaders-go/discussions) - Ask questions, share ideas
- 🐛 [Issue Tracker](https://github.com/alikatgh/safeheaders-go/issues) - Report bugs, request features
- 📧 [Email](mailto:support@safeheaders.dev) - Direct support
- 🐦 [Twitter](https://twitter.com/safeheadersgo) - Updates and announcements

## License

MIT License - See [LICENSE](./LICENSE) for details.

Original C libraries retain their respective licenses (MIT, BSD, Public Domain, zlib).

### Attribution

SafeHeaders-Go is a reimplementation of the following excellent C libraries:

- [jsmn](https://github.com/zserge/jsmn) by Serge Zaitsev (MIT)
- [stb](https://github.com/nothings/stb) by Sean Barrett (Public Domain / MIT)
- [cJSON](https://github.com/DaveGamble/cJSON) by Dave Gamble (MIT)
- [tinyxml2](https://github.com/leethomason/tinyxml2) by Lee Thomason (zlib)
- [cgltf](https://github.com/jkuhlmann/cgltf) by Johannes Kuhlmann (MIT)
- [dr_wav](https://github.com/mackron/dr_libs) by David Reid (Public Domain)

Thank you to all the original authors for their amazing work! 🙏

---

**Made with ❤️ by the SafeHeaders-Go team**

**Star this repo if you find it useful!** ⭐
