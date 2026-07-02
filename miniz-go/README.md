# miniz-go

High-performance ZIP archive manipulation with concurrent compression support.

## Status

🟢 **Stable** - Production ready

## Features

- Create and extract ZIP archives
- List archive contents without extraction
- Concurrent file compression for faster archiving
- DEFLATE compression/decompression
- Context-based cancellation support
- Zero external dependencies (uses Go stdlib only)

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/miniz-go
```

## Quick Start

### Create ZIP Archive

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/miniz-go"
)

func main() {
    files := []minizgo.FileEntry{
        {Name: "document.txt", Data: []byte("Hello, World!")},
        {Name: "data.json", Data: []byte(`{"status":"ok"}`)},
        {Name: "notes.md", Data: []byte("# Notes\n\nSome content")},
    }

    archive, err := minizgo.CreateArchive(files)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Created archive: %d bytes\n", len(archive))
}
```

### Extract ZIP Archive

```go
import "os"

// Read ZIP file
archiveData, _ := os.ReadFile("archive.zip")

// Extract all files
files, err := minizgo.ExtractArchive(archiveData)
if err != nil {
    panic(err)
}

for _, file := range files {
    fmt.Printf("File: %s (%d bytes)\n", file.Name, len(file.Data))

    // Save to disk
    os.WriteFile(file.Name, file.Data, 0644)
}
```

### List Archive Contents

```go
archiveData, _ := os.ReadFile("archive.zip")

files, err := minizgo.ListArchive(archiveData)
if err != nil {
    panic(err)
}

for _, file := range files {
    fmt.Printf("%s - %d bytes\n", file.Name, file.Size)
}
```

## Concurrent Compression

For large archives with many files, use concurrent compression for better performance:

```go
import "context"

files := []minizgo.FileEntry{
    {Name: "large1.dat", Data: makeLargeData(10*1024*1024)}, // 10MB
    {Name: "large2.dat", Data: makeLargeData(10*1024*1024)},
    {Name: "large3.dat", Data: makeLargeData(10*1024*1024)},
    {Name: "large4.dat", Data: makeLargeData(10*1024*1024)},
}

ctx := context.Background()

// Compress files in parallel using all CPU cores
archive, err := minizgo.CreateArchiveConcurrent(ctx, files)
if err != nil {
    panic(err)
}

fmt.Printf("Created %d byte archive using parallel compression\n", len(archive))
```

`minizgo.MaxBatchSize` (default 10,000, set to `0` to disable) rejects a
`CreateArchiveConcurrent` call outright if handed more files than that — many
small files still cost per-worker compression buffers and goroutine overhead
in aggregate. (`MaxDecompressedSize`, documented above, is the separate guard
on the *extract* path.)

### Context Cancellation

```go
import (
    "context"
    "time"
)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

archive, err := minizgo.CreateArchiveConcurrent(ctx, files)
if err == context.DeadlineExceeded {
    fmt.Println("Compression timed out")
} else if err != nil {
    panic(err)
}
```

## DEFLATE Compression

Compress/decompress data without creating ZIP archives:

```go
// Compress data
original := []byte("This is some text that will be compressed")

compressed, err := minizgo.CompressData(original)
if err != nil {
    panic(err)
}

fmt.Printf("Compressed: %d -> %d bytes (%.1f%%)\n",
    len(original), len(compressed),
    100.0*float64(len(compressed))/float64(len(original)))

// Decompress data
decompressed, err := minizgo.DecompressData(compressed)
if err != nil {
    panic(err)
}

fmt.Println(string(decompressed)) // Original text
```

## Performance

Throughput depends heavily on input shape, CPU count, and allocator behavior, so
this README does not quote fixed numbers — measure on your target hardware:

```bash
go test -bench=. -benchmem ./...
```

`CreateArchiveConcurrent` compresses archive entries in parallel and then assembles them without recompressing. Speedup scales with the number of files, not the size of any single one.

## API Reference

### Types

```go
type FileEntry struct {
    Name string  // File name in archive
    Data []byte  // File contents
}

type ZipFile struct {
    Name string  // File name
    Data []byte  // File contents (only populated by ExtractArchive)
    Size int64   // Uncompressed size
}
```

### Archive Functions

```go
// CreateArchive creates a ZIP archive from files
func CreateArchive(files []FileEntry) ([]byte, error)

// ExtractArchive extracts all files from a ZIP archive
func ExtractArchive(data []byte) ([]ZipFile, error)

// ListArchive returns file names and sizes without extraction
func ListArchive(data []byte) ([]ZipFile, error)

// CreateArchiveConcurrent creates archive using parallel compression
func CreateArchiveConcurrent(ctx context.Context, files []FileEntry) ([]byte, error)
```

### Compression Functions

```go
// CompressData compresses data using DEFLATE
func CompressData(data []byte) ([]byte, error)

// DecompressData decompresses DEFLATE-compressed data
func DecompressData(data []byte) ([]byte, error)
```

## When to Use Concurrent Compression

Use `CreateArchiveConcurrent()` when:
- Archive has 4+ files
- Files are large (>1MB each)
- CPU cores are available
- Maximum performance is needed

Use regular `CreateArchive()` when:
- Archive has few files (<4)
- Files are small (<1MB)
- Simplicity is preferred

## Best Practices

### Memory Management

```go
// For very large archives, extract files one at a time
archiveData, _ := os.ReadFile("huge.zip")

files, _ := minizgo.ListArchive(archiveData)
for _, file := range files {
    // Extract individual files as needed
    // instead of loading all into memory
}
```

### Error Handling

```go
if err := minizgo.CreateArchive(files); err != nil {
    if errors.Is(err, context.Canceled) {
        // Handle cancellation
    } else {
        // Handle compression error
        fmt.Println("Archive error:", err)
    }
}
```

## Limitations

- Maximum file size: 4GB (ZIP64 not supported)
- Archive paths use forward slashes (/)
- File permissions are not preserved
- Compression level is fixed at best compression

## Thread Safety

- All functions are safe to call concurrently from multiple goroutines
- `CreateArchiveConcurrent()` uses `runtime.NumCPU()` workers automatically

## Testing

```bash
cd miniz-go
go test -v
go test -bench . -benchmem
```

## Examples

### Backup Directory

```go
import (
    "os"
    "path/filepath"
)

func backupDirectory(dir string) ([]byte, error) {
    var files []minizgo.FileEntry

    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        data, err := os.ReadFile(path)
        if err != nil {
            return err
        }

        relPath, _ := filepath.Rel(dir, path)
        files = append(files, minizgo.FileEntry{
            Name: relPath,
            Data: data,
        })
        return nil
    })

    if err != nil {
        return nil, err
    }

    return minizgo.CreateArchiveConcurrent(context.Background(), files)
}
```

### Extract Specific File

```go
func extractFile(archiveData []byte, filename string) ([]byte, error) {
    files, err := minizgo.ExtractArchive(archiveData)
    if err != nil {
        return nil, err
    }

    for _, file := range files {
        if file.Name == filename {
            return file.Data, nil
        }
    }

    return nil, fmt.Errorf("file not found: %s", filename)
}
```

## License

MIT - See [LICENSE](../LICENSE)

Based on [miniz](https://github.com/richgel999/miniz) by Rich Geldreich (Public Domain)
