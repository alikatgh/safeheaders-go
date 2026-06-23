# stb-image-go

Fast image loader with batch decoding support for PNG, JPEG, and GIF formats.

## Status

🟢 **Stable** - Production ready

## Features

- Decode PNG, JPEG, and GIF images
- Concurrent batch decoding for multiple images
- Stream-based loading for memory efficiency
- Image metadata extraction without full decode
- Context-based cancellation support
- Zero external dependencies (uses Go stdlib only)

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/stb-image-go
```

## Quick Start

### Load Single Image

```go
package main

import (
    "fmt"
    "os"
    "github.com/alikatgh/safeheaders-go/stb-image-go"
)

func main() {
    // Read image file
    data, _ := os.ReadFile("photo.jpg")

    // Decode image
    img, err := stbimagego.Load(data)
    if err != nil {
        panic(err)
    }

    bounds := img.Bounds()
    fmt.Printf("Image: %dx%d\n", bounds.Dx(), bounds.Dy())
}
```

### Get Image Info

Get image dimensions and format without full decode:

```go
data, _ := os.ReadFile("photo.jpg")

info, err := stbimagego.GetInfo(data)
if err != nil {
    panic(err)
}

fmt.Printf("Format: %s\n", info.Format)     // "jpeg"
fmt.Printf("Size: %dx%d\n", info.Width, info.Height)
```

## Batch Processing

Decode multiple images concurrently for better performance:

```go
import "context"

files := []string{"img1.png", "img2.jpg", "img3.gif"}
dataList := make([][]byte, len(files))

for i, file := range files {
    dataList[i], _ = os.ReadFile(file)
}

ctx := context.Background()

// Decode all images in parallel
images, err := stbimagego.LoadBatchConcurrent(ctx, dataList)
if err != nil {
    panic(err)
}

for i, img := range images {
    bounds := img.Bounds()
    fmt.Printf("Image %d: %dx%d\n", i, bounds.Dx(), bounds.Dy())
}
```

### Context Cancellation

```go
import (
    "context"
    "time"
)

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

images, err := stbimagego.LoadBatchConcurrent(ctx, dataList)
if err == context.DeadlineExceeded {
    fmt.Println("Image loading timed out")
}
```

## Stream Loading

Load images from streams without buffering entire file:

```go
file, _ := os.Open("large-photo.jpg")
defer file.Close()

img, err := stbimagego.LoadStream(file)
if err != nil {
    panic(err)
}
```

## Performance

Throughput depends heavily on input shape, CPU count, and allocator behavior, so
this README does not quote fixed numbers — measure on your target hardware:

```bash
go test -bench=. -benchmem ./...
```

`LoadBatchConcurrent` decodes independent images concurrently. Speedup scales with the number of images.

## API Reference

### Types

```go
type ImageInfo struct {
    Width  int
    Height int
    Format string  // "png", "jpeg", or "gif"
}
```

### Functions

```go
// Load decodes an image from bytes
func Load(data []byte) (image.Image, error)

// GetInfo returns image metadata without decoding
func GetInfo(data []byte) (*ImageInfo, error)

// LoadBatchConcurrent decodes multiple images in parallel
func LoadBatchConcurrent(ctx context.Context, datas [][]byte) ([]image.Image, error)

// LoadStream decodes from a reader
func LoadStream(r io.Reader) (image.Image, error)
```

## Supported Formats

- ✅ PNG (Portable Network Graphics)
- ✅ JPEG (JFIF/EXIF)
- ✅ GIF (Graphics Interchange Format)

## When to Use Batch Processing

Use `LoadBatchConcurrent()` when:
- Loading 4+ images
- Images are small-to-medium (<10MB each)
- Maximum throughput needed

Use regular `Load()` when:
- Loading single image
- Image is very large (>50MB)
- Simplicity preferred

## Thread Safety

- All functions are safe to call concurrently
- `LoadBatchConcurrent()` automatically uses `runtime.NumCPU()` workers
- Returned `image.Image` values are safe to read from multiple goroutines

## Error Handling

```go
img, err := stbimagego.Load(data)
if err != nil {
    // Handle decode error
    fmt.Println("Failed to decode image:", err)
    return
}
```

## Testing

```bash
cd stb-image-go
go test -v
go test -bench . -benchmem
```

## Examples

### Create Thumbnail

```go
import (
    "image"
    "golang.org/x/image/draw"
)

func createThumbnail(data []byte, maxWidth, maxHeight int) (image.Image, error) {
    img, err := stbimagego.Load(data)
    if err != nil {
        return nil, err
    }

    bounds := img.Bounds()
    width := bounds.Dx()
    height := bounds.Dy()

    // Calculate new dimensions
    scale := math.Min(
        float64(maxWidth)/float64(width),
        float64(maxHeight)/float64(height),
    )

    newWidth := int(float64(width) * scale)
    newHeight := int(float64(height) * scale)

    // Create thumbnail
    thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
    draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)

    return thumb, nil
}
```

### Process Images Concurrently

```go
func processImages(files []string) error {
    dataList := make([][]byte, len(files))
    for i, file := range files {
        data, err := os.ReadFile(file)
        if err != nil {
            return err
        }
        dataList[i] = data
    }

    ctx := context.Background()
    images, err := stbimagego.LoadBatchConcurrent(ctx, dataList)
    if err != nil {
        return err
    }

    // Process each image
    for i, img := range images {
        fmt.Printf("Processing %s: %v\n", files[i], img.Bounds())
        // ... apply filters, transformations, etc.
    }

    return nil
}
```

## License

MIT - See [LICENSE](../LICENSE)

Based on [stb_image.h](https://github.com/nothings/stb) by Sean Barrett (Public Domain)