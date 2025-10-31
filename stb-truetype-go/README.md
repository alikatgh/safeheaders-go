# stb-truetype-go

TrueType font parsing with concurrent glyph caching.

## Status

🟢 **Stable** - Production ready

## Features

- Load TrueType fonts from files or memory
- Thread-safe LRU glyph cache
- O(1) cache lookups and updates
- Immutable glyph bitmaps (safe for concurrent access)
- Zero external dependencies

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/stb-truetype-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/stb-truetype-go"
)

func main() {
    // Load font
    font, err := truetype.LoadFont("path/to/font.ttf")
    if err != nil {
        panic(err)
    }

    // Create glyph cache (100 glyphs max, 12pt size)
    cache := truetype.NewGlyphCache(font, 12.0, 100, nil)

    // Get glyph (cached after first call)
    glyph, err := cache.GetGlyph('A')
    if err != nil {
        panic(err)
    }

    // Access glyph data
    bitmap := glyph.Bitmap()
    metrics := glyph.Metrics()

    fmt.Printf("Glyph 'A': %dx%d pixels, advance: %d\n",
        bitmap.Bounds().Dx(),
        bitmap.Bounds().Dy(),
        metrics.AdvanceWidth)
}
```

## Custom Rasterizer

You can provide your own glyph rendering function:

```go
func myRasterizer(font *truetype.Font, r rune, size float64) (*image.Gray, truetype.GlyphMetrics, error) {
    // Your custom rendering logic
    img := image.NewGray(image.Rect(0, 0, 24, 24))
    metrics := truetype.GlyphMetrics{
        AdvanceWidth: 24,
        BearingX: 0,
        BearingY: 18,
        Scale: size,
    }
    return img, metrics, nil
}

cache := truetype.NewGlyphCache(font, 12.0, 100, myRasterizer)
```

## API Reference

### Types

```go
type Font struct {
    // Immutable, safe for concurrent use
}

type GlyphCache struct {
    // Thread-safe LRU cache
}

type Glyph struct {
    // Read-only glyph data
}

type GlyphMetrics struct {
    AdvanceWidth int
    BearingX     int
    BearingY     int
    Scale        float64
}
```

### Functions

```go
// LoadFont reads a TrueType font from disk
func LoadFont(path string) (*Font, error)

// LoadFontFromBytes creates font from in-memory data
func LoadFontFromBytes(data []byte) (*Font, error)

// NewGlyphCache creates a thread-safe LRU cache
func NewGlyphCache(font *Font, size float64, maxEntries int, rasterizer rasterizerFunc) *GlyphCache

// GetGlyph retrieves or renders a glyph
func (gc *GlyphCache) GetGlyph(r rune) (*Glyph, error)

// Glyph methods
func (g *Glyph) Bitmap() image.Image
func (g *Glyph) Metrics() GlyphMetrics
func (g *Glyph) Bounds() image.Rectangle
```

## Performance

- **LRU Cache**: O(1) lookups and evictions
- **Thread Safety**: Lock-free reads for cache hits
- **Memory Efficient**: Bounded cache prevents memory bloat

## Thread Safety

- ✅ `Font` is **immutable** and safe for concurrent use
- ✅ `GlyphCache` is **thread-safe** (multiple goroutines can call `GetGlyph`)
- ✅ `Glyph.Bitmap()` returns **immutable** image (safe to share)

## Design Patterns

### Immutable Bitmap

The cache wraps glyph bitmaps in an immutable type to prevent accidental modification:

```go
// This prevents type assertion back to *image.Gray
type immutableGray struct{ image.Image }
```

### Double-Checked Locking

Cache lookups use optimized double-checked locking:

1. First check with read lock (fast path)
2. Upgrade to write lock if needed (slow path)
3. Re-check after acquiring write lock (prevents races)

## Testing

```bash
cd stb-truetype-go
go test -v
go test -race  # Verify thread safety
go test -bench . -benchmem
```

## License

MIT - See [LICENSE](../LICENSE)

Inspired by [stb_truetype.h](https://github.com/nothings/stb/blob/master/stb_truetype.h) by Sean Barrett (Public Domain)
