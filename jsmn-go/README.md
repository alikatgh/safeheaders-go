# jsmn-go

Fast, lightweight JSON tokenizer with built-in parallel processing support.

## Status

🟢 **Stable** - Production ready

## Features

- Fast JSON tokenization (2x speedup with parallelization)
- Zero external dependencies
- Memory safe (no buffer overflows)
- Concurrent processing for large JSON files
- Compatible with original jsmn C library

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/jsmn-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
    json := []byte(`{"name": "John", "age": 30, "items": [1, 2, 3]}`)

    // Create parser
    p := jsmngo.NewParser(100)

    // Parse JSON
    n, err := p.Parse(json)
    if err != nil {
        panic(err)
    }

    // Get tokens
    tokens := p.Tokens()
    fmt.Printf("Parsed %d tokens\n", n)

    for i, tok := range tokens {
        value := string(json[tok.Start:tok.End])
        fmt.Printf("Token %d: %v = %s\n", i, tok.Type, value)
    }
}
```

## Parallel Processing

For large JSON files (>4KB), use parallel parsing:

```go
json := []byte(`[{"id":1}, {"id":2}, ... many objects]`)

// Automatically uses all CPU cores
tokens, err := jsmngo.ParseParallel(json)
if err != nil {
    panic(err)
}

// Process tokens...
```

## Performance

Benchmarks on Apple M3 Pro (1MB JSON, 10K objects):

| CPUs | Time | Speedup | Throughput |
|------|------|---------|------------|
| 1    | 150ms | 1.0x | 6.7 MB/s |
| 2    | 100ms | 1.5x | 10 MB/s |
| 4    | 75ms  | 2.0x | 13.3 MB/s |
| 8    | 70ms  | 2.1x | 14.3 MB/s |

## API Reference

### Types

```go
type TokenType int

const (
    Object    // JSON object {}
    Array     // JSON array []
    String    // JSON string ""
    Primitive // number, true, false, null
)

type Token struct {
    Type      TokenType
    Start     int  // Start position in JSON
    End       int  // End position in JSON
    Size      int  // Number of children
    ParentIdx int  // Parent token index
}

type Parser struct {
    // Internal state
}
```

### Functions

```go
// NewParser creates a new parser with capacity for numTokens
func NewParser(numTokens int) *Parser

// Parse tokenizes JSON and returns number of tokens
func (p *Parser) Parse(json []byte) (int, error)

// Tokens returns the parsed tokens
func (p *Parser) Tokens() []Token

// ParseParallel tokenizes large JSON using multiple CPU cores
func ParseParallel(json []byte) ([]Token, error)
```

## Limitations

- Parallel mode works best for large JSON arrays
- Complex nested objects may not parallelize efficiently
- Token positions reference the original JSON byte slice

## Thread Safety

- `Parser` is **not** thread-safe (create one per goroutine)
- `ParseParallel()` is safe to call concurrently from multiple goroutines

## Testing

```bash
cd jsmn-go
go test -v
go test -bench . -benchmem
```

## License

MIT - See [LICENSE](../LICENSE)

Based on [jsmn](https://github.com/zserge/jsmn) by Serge Zaitsev (MIT License)
