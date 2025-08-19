package truetype

import (
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"sync"
	"testing"
)

//go:embed testdata/font.ttf
var testFontData []byte

// testFont is a thread-safe helper that loads the embedded font once.
var testFont = sync.OnceValue(func() *Font {
	font, err := LoadFontFromBytes(testFontData)
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded test font: %v", err))
	}
	return font
})

// errRasterizationFailed is a sentinel error used for testing.
var errRasterizationFailed = errors.New("rasterization failed")

// fastRasterizer is a no-op rasterizer for benchmarks.
func fastRasterizer(font *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error) {
	return image.NewGray(image.Rect(0, 0, 1, 1)), GlyphMetrics{}, nil
}

// errorRasterizer is a rasterizer that always fails with our sentinel error.
func errorRasterizer(font *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error) {
	return nil, GlyphMetrics{}, errRasterizationFailed
}

// TestLRUEviction verifies that the least-recently-used glyph is evicted.
func TestLRUEviction(t *testing.T) {
	cache := NewGlyphCache(testFont(), 16.0, 2, nil) // Max size: 2

	_, _ = cache.GetGlyph('A')
	_, _ = cache.GetGlyph('B')
	if cache.lru.Len() != 2 {
		t.Fatalf("expected cache size 2, got %d", cache.lru.Len())
	}

	_, _ = cache.GetGlyph('C')
	if _, found := cache.cache['A']; found {
		t.Error("'A' should have been evicted, but was found")
	}
	if _, found := cache.cache['B']; !found {
		t.Error("'B' was evicted, but should not have been")
	}

	_, _ = cache.GetGlyph('B')
	if cache.lru.Front().Value.(rune) != 'B' {
		t.Error("'B' should be the most recently used item")
	}
}

// TestErrorPropagation verifies that rasterizer errors are passed to the caller.
func TestErrorPropagation(t *testing.T) {
	cache := NewGlyphCache(testFont(), 16.0, 0, errorRasterizer)
	_, err := cache.GetGlyph('A')
	if err == nil {
		t.Fatal("expected an error from GetGlyph, but got nil")
	}
	// CORRECTED: We now check if the error chain CONTAINS our sentinel error.
	if !errors.Is(err, errRasterizationFailed) {
		t.Errorf("expected error wrapping '%v', but it was not in the chain", errRasterizationFailed)
	}
}

// TestBitmapImmutability verifies a caller cannot mutate the cached bitmap.
func TestBitmapImmutability(t *testing.T) {
	cache := NewGlyphCache(testFont(), 16.0, 0, nil)
	glyph, _ := cache.GetGlyph('A')
	originalColor, _, _, _ := glyph.Bitmap().At(0, 0).RGBA()

	if mutableBitmap, ok := glyph.Bitmap().(*image.Gray); ok {
		mutableBitmap.Set(0, 0, color.White)
		t.Fatal("Bitmap was mutable: successfully type-asserted to *image.Gray")
	}

	glyph2, _ := cache.GetGlyph('A')
	newColor, _, _, _ := glyph2.Bitmap().At(0, 0).RGBA()

	if originalColor != newColor {
		t.Error("Bitmap was mutated despite wrapper")
	}
}

// ... the rest of the file (benchmarks, example) is unchanged ...

// CORRECTED BenchmarkGetGlyphCacheMiss using b.RunParallel.
func BenchmarkGetGlyphCacheMiss(b *testing.B) {
	font := testFont()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		// Each parallel goroutine gets its own cache, guaranteeing a true miss every time.
		cache := NewGlyphCache(font, 16.0, 0, fastRasterizer)
		for pb.Next() {
			_, _ = cache.GetGlyph('X')
		}
	})
}
