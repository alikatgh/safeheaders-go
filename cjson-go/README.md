# cjson-go

Fast JSON marshaling and unmarshaling with parallel array processing support.

## Status

🟢 **Stable** - Production ready

## Features

- Full JSON marshal/unmarshal support
- Parallel array processing for large datasets
- Stream-based processing for memory efficiency
- JSON validation and compaction utilities
- Zero external dependencies (uses Go stdlib only)
- Type-safe operations

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/cjson-go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/cjson-go"
)

type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func main() {
    // Unmarshal JSON to struct
    data := []byte(`{"name":"Alice","email":"alice@example.com","age":30}`)

    var user User
    if err := cjsongo.Unmarshal(data, &user); err != nil {
        panic(err)
    }
    fmt.Printf("User: %+v\n", user)

    // Marshal struct to JSON
    jsonData, err := cjsongo.Marshal(user)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(jsonData))
}
```

### Unmarshal to Map

```go
data := []byte(`{"status":"ok","count":42,"tags":["go","json"]}`)

result, err := cjsongo.UnmarshalToMap(data)
if err != nil {
    panic(err)
}

fmt.Println(result["status"]) // "ok"
fmt.Println(result["count"])  // 42.0 (float64)
```

### Unmarshal to Slice

```go
data := []byte(`[1, 2, 3, 4, 5]`)

result, err := cjsongo.UnmarshalToSlice(data)
if err != nil {
    panic(err)
}

fmt.Println(result) // [1 2 3 4 5]
```

## Parallel Array Processing

For large JSON arrays (thousands of objects), use parallel processing:

```go
// JSON array with many objects
jsonArray := []byte(`[
    {"id":1,"name":"Item 1","value":100},
    {"id":2,"name":"Item 2","value":200},
    ... thousands more ...
]`)

// Process array in parallel using all CPU cores
results, err := cjsongo.UnmarshalArrayParallel(jsonArray)
if err != nil {
    panic(err)
}

// results is []map[string]interface{}
for _, item := range results {
    fmt.Printf("ID: %.0f, Name: %s\n", item["id"], item["name"])
}
```

## Stream Processing

Process large JSON files without loading everything into memory:

```go
import "os"

// Read from stream
file, _ := os.Open("large.json")
defer file.Close()

var data map[string]interface{}
if err := cjsongo.UnmarshalStream(file, &data); err != nil {
    panic(err)
}

// Write to stream
outFile, _ := os.Create("output.json")
defer outFile.Close()

if err := cjsongo.MarshalStream(outFile, data); err != nil {
    panic(err)
}
```

## Utilities

### Validate JSON

```go
data := []byte(`{"valid": true}`)

if cjsongo.Valid(data) {
    fmt.Println("Valid JSON")
} else {
    fmt.Println("Invalid JSON")
}
```

### Compact JSON

Remove whitespace from JSON:

```go
prettyJSON := []byte(`{
    "name": "Alice",
    "age": 30
}`)

compact, err := cjsongo.Compact(prettyJSON)
if err != nil {
    panic(err)
}

fmt.Println(string(compact))
// Output: {"name":"Alice","age":30}
```

## Performance

Throughput depends heavily on input shape, CPU count, and allocator behavior, so
this README does not quote fixed numbers — measure on your target hardware:

```bash
go test -bench=. -benchmem ./...
```

`UnmarshalArrayParallel` decodes array elements concurrently. It helps when the array has many independently-decodable items.

## API Reference

### Core Functions

```go
// Unmarshal parses JSON into a Go value
func Unmarshal(data []byte, v interface{}) error

// UnmarshalToMap parses JSON into a map
func UnmarshalToMap(data []byte) (map[string]interface{}, error)

// UnmarshalToSlice parses JSON into a slice
func UnmarshalToSlice(data []byte) ([]interface{}, error)

// Marshal encodes a Go value as JSON
func Marshal(v interface{}) ([]byte, error)

// MarshalIndent encodes a Go value as formatted JSON
func MarshalIndent(v interface{}) ([]byte, error)

// UnmarshalStream parses JSON from a reader
func UnmarshalStream(r io.Reader, v interface{}) error

// MarshalStream writes JSON to a writer
func MarshalStream(w io.Writer, v interface{}) error

// UnmarshalArrayParallel parses large JSON arrays in parallel
func UnmarshalArrayParallel(data []byte) ([]map[string]interface{}, error)

// Valid reports whether data is valid JSON
func Valid(data []byte) bool

// Compact removes whitespace from JSON
func Compact(data []byte) ([]byte, error)
```

## When to Use Parallel Processing

Use `UnmarshalArrayParallel()` when:
- JSON array has 1000+ objects
- Each object is relatively small (<10KB)
- You need maximum throughput

Use regular `Unmarshal()` when:
- JSON is not an array
- Array has <1000 objects
- Objects are very large (>100KB each)

## Thread Safety

- All functions are safe to call concurrently from multiple goroutines
- `UnmarshalArrayParallel()` automatically uses `runtime.NumCPU()` workers

## Error Handling

All functions return descriptive errors:

```go
if err := cjsongo.Unmarshal(data, &result); err != nil {
    if errors.Is(err, io.EOF) {
        // Handle EOF
    } else {
        // Handle parse error
        fmt.Println("JSON parse error:", err)
    }
}
```

## Testing

```bash
cd cjson-go
go test -v
go test -bench . -benchmem
```

## License

MIT - See [LICENSE](../LICENSE)
