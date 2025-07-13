# SafeHeaders-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/alikatgh/safeheaders-go/jsmn-go)](https://goreportcard.com/report/github.com/alikatgh/safeheaders-go/jsmn-go)
[![Tests](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml/badge.svg)](https://github.com/alikatgh/safeheaders-go/actions/workflows/go-ci.yaml)

A collection of idiomatic Go ports of popular single-header C libraries, enhanced with Go's concurrency and safety features. These ports eliminate C's raw pointer risks using Go's slices and bounds checking, while adding novel twists like parallel processing for high-throughput scenarios.

## Why?
- **Safety**: No buffer overflows or undefined behavior.
- **Performance**: Leverage goroutines for concurrency, e.g., parallel tokenizing.
- **Simplicity**: Drop-in packages for embedded, web, or edge apps.
- **Modernized**: Application of Go-idiomatic features like streaming I/O to classic C libraries for real-time data (e.g., IoT JSON streams).

## Current Ports
- [jsmn-go](./jsmn-go): Lightweight JSON tokenizer with parallel and streaming support.
- [stb-image-go](./stb-image-go): Image loading with concurrent batch decoding.
- [miniz-go](./miniz-go): ZIP compression with concurrent chunk processing.

## Usage Snippets
Basic parsing:
```go
package main

import (
	"fmt"
	"github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)
	p := jsmngo.NewParser(10)
	_, err := p.Parse(json)
	if err != nil {
		panic(err)
	}
	tokens := p.Tokens()
	for _, tok := range tokens {
		fmt.Printf("Token: Type=%v, Start=%d, End=%d, Size=%d\n", tok.Type, tok.Start, tok.End, tok.Size)
	}
}
```

Parallel mode (for large JSON):
```go
tokens, err := jsmngo.ParseParallel(json, 1000)
if err != nil {
	panic(err)
}
// Use tokens...
```

Streaming from reader:
```go
reader := bytes.NewReader(json)
tokens, err := jsmngo.ParseStream(reader, 1000)
if err != nil {
	panic(err)
}
// Use tokens...
```

## Benchmark Results
Run `go test -bench . ./jsmn-go` for details. On a sample 1MB JSON array:
- BenchmarkParse (single-threaded): ~150ms
- BenchmarkParseParallel: ~75ms (2x faster on 4 cores)

(Note: Results may vary by hardware; replace with your local runs for accuracy.)

## Limitations
- Parallel chunking is naive (simple splits without boundary alignment); works best for large, uniform arrays/objects. May produce misaligned tokens on complex JSON—PR improvements welcome (e.g., add smart chunk boundary scanning)!

## Contributing
Pick a single-header C lib from the wishlist below, port it to pure Go, add one concurrent enhancement, and PR!

- [x] stb_image.h (images): Enhancement idea: Goroutine-based batch decoding.
- [ ] stb_truetype.h (fonts): Enhancement: Concurrent glyph caching.
- [x] miniz.h (compression): Enhancement: Parallel compression of chunks.
- [ ] linenoise.h (CLI input): Enhancement: Async history search with goroutines.
- [ ] nuklear.h (GUI): Enhancement: Concurrent rendering of UI elements.
- [ ] cJSON.h (JSON parsing): Enhancement: Parallel object deserialization.
- [ ] dr_wav.h (WAV audio loading): Enhancement: Goroutine-based audio stream decoding.
- [ ] tinyxml2.h (XML parsing): Enhancement: Concurrent node traversal and querying.
- [ ] cgltf.h (glTF 3D model loading): Enhancement: Parallel asset loading for models.
- [ ] stb_vorbis.h (Ogg Vorbis audio decoding): Enhancement: Multi-channel parallel decoding.
- [ ] easytab.h (table layout): Enhancement: Async data filling for dynamic tables.

Guidelines: Keep it allocation-free where possible, include benchmarks/tests vs. original C, and ensure safety with Go's features.

## License
MIT