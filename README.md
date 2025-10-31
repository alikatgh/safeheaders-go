# SafeHeaders-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/alikatgh/safeheaders-go/jsmn-go)](https://goreportcard.com/report/github.com/alikatgh/safeheaders-go/jsmn-go)
[![Tests](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml/badge.svg)](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml)
[![Coverage](https://codecov.io/gh/alikatgh/safeheaders-go/branch/main/graph/badge.svg)](https://codecov.io/gh/alikatgh/safeheaders-go)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Pure Go implementations of popular single-header C libraries with built-in concurrency support and zero CGO dependencies.

## Features

- **Memory Safe** - No buffer overflows or undefined behavior
- **Concurrent** - Built-in goroutine support for parallel processing
- **Zero Dependencies** - Pure Go stdlib, no CGO required
- **Easy Integration** - Drop-in packages for any Go project

## Quick Start

```bash
go get github.com/alikatgh/safeheaders-go/jsmn-go
```

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
    json := []byte(`{"name": "SafeHeaders", "cool": true}`)
    p := jsmngo.NewParser(10)
    _, err := p.Parse(json)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed %d tokens\n", len(p.Tokens()))
}
```

## Available Modules

| Module | Status | Description |
|--------|--------|-------------|
| [jsmn-go](./jsmn-go) | 🟢 **Stable** | JSON tokenizer with parallel parsing |
| [stb-truetype-go](./stb-truetype-go) | 🟢 **Stable** | TrueType font parsing with LRU glyph cache |
| [stb-image-go](./stb-image-go) | 🟢 **Stable** | Image loading with batch decoding (PNG, JPEG, GIF) |
| [tinyxml2-go](./tinyxml2-go) | 🟢 **Stable** | XML DOM parsing with XPath-like queries |
| [cjson-go](./cjson-go) | 🟢 **Stable** | JSON marshaling/unmarshaling with parallel array processing |
| [miniz-go](./miniz-go) | 🟢 **Stable** | ZIP compression with concurrent chunking |
| [cgltf-go](./cgltf-go) | 🟢 **Stable** | glTF 3D model loading with parallel assets |
| [dr-wav-go](./dr-wav-go) | 🟢 **Stable** | WAV audio file parsing with concurrent decoding |

**All 8 modules are production-ready!** 🎉

## Performance

Benchmarks on Apple M3 Pro with 1MB JSON (10K objects):

| Mode | Time | Speedup | Throughput |
|------|------|---------|------------|
| Single-threaded | 150ms | 1.0x | 6.7 MB/s |
| Parallel (2 CPU) | 100ms | 1.5x | 10 MB/s |
| Parallel (4 CPU) | 75ms | 2.0x | 13.3 MB/s |
| Parallel (8 CPU) | 70ms | 2.1x | 14.3 MB/s |

Comparison with stdlib `encoding/json`: Similar single-threaded performance, but parallel mode provides 2x speedup on multi-core systems.

## Why SafeHeaders-Go?

Traditional C libraries are fast but unsafe. Go stdlib is safe but doesn't parallelize parsing. SafeHeaders-Go gives you both:

- **vs C Libraries**: Memory safe, no CGO complexity, easier deployment
- **vs Go Stdlib**: Built-in parallelism for large data processing
- **vs Other Ports**: Focus on concurrency patterns, production-ready quality

## Documentation

- [Contributing Guide](./CONTRIBUTING.md) - How to contribute
- [Issues & Roadmap](./ISSUES.md) - Known issues and planned features
- Module READMEs - Detailed docs in each module directory

## Contributing

We welcome contributions! Here's how to help:

1. **Complete Alpha Modules** - Help finish experimental modules
2. **Improve Performance** - Optimize chunking and parallel algorithms
3. **Add Tests** - Increase coverage and add fuzz tests
4. **Port New Libraries** - See wishlist below

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed guidelines.

### Wishlist

Want to port a new library? Pick one:

- [ ] [linenoise.h](https://github.com/antirez/linenoise) - CLI input editing
- [ ] [stb_vorbis.h](https://github.com/nothings/stb/blob/master/stb_vorbis.c) - Ogg Vorbis decoder
- [ ] [tinyobjloader.h](https://github.com/tinyobjloader/tinyobjloader) - OBJ 3D model loader
- [ ] [stb_perlin.h](https://github.com/nothings/stb/blob/master/stb_perlin.h) - Perlin noise
- [ ] [utf8.h](https://github.com/sheredom/utf8.h) - UTF-8 utilities

## License

MIT License - See [LICENSE](./LICENSE) for details.

Original C libraries retain their respective licenses (MIT/BSD/Public Domain).
