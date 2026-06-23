// Package truetype provides a Go-native implementation for parsing, querying,
// and rasterizing TrueType fonts, inspired by the stb_truetype.h C library.
package truetype

import (
	"container/list"
	"fmt"
	"image"
	"os"
	"sync"
)

// rasterizerFunc defines the signature for a function that can render a glyph.
type rasterizerFunc func(font *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error)

// immutableGray wraps an image.Image to prevent type assertions back to a mutable type.
// It satisfies the image.Image interface by embedding it.
type immutableGray struct{ image.Image }

// GlyphMetrics holds positioning information for a glyph.
type GlyphMetrics struct {
	AdvanceWidth int
	BearingX     int
	BearingY     int
	Scale        float64
}

// Glyph provides read-only access to a rendered glyph's bitmap and metrics.
type Glyph struct {
	bitmap  image.Image
	metrics GlyphMetrics
}

// Bitmap returns the glyph's image. It is wrapped in a read-only type.
func (g *Glyph) Bitmap() image.Image {
	return g.bitmap
}

// Metrics returns the glyph's positioning metrics.
func (g *Glyph) Metrics() GlyphMetrics {
	return g.metrics
}

// Bounds returns the glyph's bounding rectangle.
func (g *Glyph) Bounds() image.Rectangle {
	return g.bitmap.Bounds()
}

// Font represents a loaded TrueType font file. It is parsed once at load time
// and immutable thereafter, so it is safe for concurrent use.
type Font struct {
	rawData     []byte
	tables      map[string]tableRec // sfnt table directory
	loca        []uint32            // glyph offsets into glyf (numGlyphs+1)
	cmapData    []byte              // selected cmap subtable
	unitsPerEm  uint16
	indexToLoc  int16
	numGlyphs   uint16
	numHMetrics uint16
}

// LoadFont reads a .ttf file from disk. The path is supplied by the caller by
// design (this is a font loader), so the variable-path read is intentional.
func LoadFont(path string) (*Font, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("failed to read font file: %w", err)
	}
	return LoadFontFromBytes(data)
}

// LoadFontFromBytes prepares a font from an in-memory byte slice.
func LoadFontFromBytes(data []byte) (*Font, error) {
	copiedData := make([]byte, len(data))
	copy(copiedData, data)
	font := &Font{rawData: copiedData}
	if err := font.parseSFNT(); err != nil {
		return nil, err
	}
	return font, nil
}

// defaultRasterizer renders a glyph by decoding the font's glyf outlines and
// scan-filling them with anti-aliasing (see sfnt.go). It is used when
// NewGlyphCache is given a nil rasterizer.
func defaultRasterizer(font *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error) {
	return rasterizeGlyph(font, r, size)
}

// cacheEntry stores the glyph and a pointer to its element in the LRU list.
type cacheEntry struct {
	glyph   *Glyph
	element *list.Element
}

// GlyphCache provides a thread-safe, bounded, O(1) LRU cache for rendered glyphs.
type GlyphCache struct {
	font       *Font
	size       float64
	rasterizer rasterizerFunc
	mu         sync.RWMutex
	cache      map[rune]*cacheEntry
	lru        *list.List
	maxEntries int
}

// NewGlyphCache creates a new cache. If rasterizer is nil, a default is used.
func NewGlyphCache(font *Font, size float64, maxEntries int, rasterizer rasterizerFunc) *GlyphCache {
	if rasterizer == nil {
		rasterizer = defaultRasterizer
	}
	return &GlyphCache{
		font:       font,
		size:       size,
		rasterizer: rasterizer,
		cache:      make(map[rune]*cacheEntry),
		lru:        list.New(),
		maxEntries: maxEntries,
	}
}

// GetGlyph retrieves a glyph, rasterizing if necessary.
func (gc *GlyphCache) GetGlyph(r rune) (*Glyph, error) {
	gc.mu.RLock()
	if entry, found := gc.cache[r]; found {
		gc.mu.RUnlock()
		gc.mu.Lock()
		gc.lru.MoveToFront(entry.element) // O(1) recency update
		gc.mu.Unlock()
		return entry.glyph, nil
	}
	gc.mu.RUnlock()

	gc.mu.Lock()
	defer gc.mu.Unlock()

	if entry, found := gc.cache[r]; found {
		gc.lru.MoveToFront(entry.element)
		return entry.glyph, nil
	}

	bitmap, metrics, err := gc.rasterizer(gc.font, r, gc.size)
	if err != nil {
		return nil, fmt.Errorf("failed to rasterize glyph '%c': %w", r, err)
	}

	glyph := &Glyph{
		bitmap:  immutableGray{bitmap}, // Wrap bitmap to make it read-only
		metrics: metrics,
	}

	element := gc.lru.PushFront(r)
	gc.cache[r] = &cacheEntry{glyph, element}

	if gc.maxEntries > 0 && gc.lru.Len() > gc.maxEntries {
		lruElement := gc.lru.Back()
		if lruElement != nil {
			if evictRune, ok := gc.lru.Remove(lruElement).(rune); ok {
				delete(gc.cache, evictRune)
			}
		}
	}

	return glyph, nil
}
