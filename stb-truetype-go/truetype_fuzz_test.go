package truetype

import "testing"

// FuzzLoadFont ensures parsing and rasterizing arbitrary (malformed) font data
// never panics — font files are untrusted input.
func FuzzLoadFont(f *testing.F) {
	f.Add(testFontData)
	f.Add([]byte("not a font"))
	f.Add([]byte{0, 1, 0, 0})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		font, err := LoadFontFromBytes(data)
		if err != nil || font == nil {
			return
		}
		cache := NewGlyphCache(font, 18.0, 16, nil)
		for _, r := range []rune{'A', 'a', '0', ' ', '@', 0x00E9, 0x4E2D} {
			_, _ = cache.GetGlyph(r) // must not panic on any parsed font
		}
	})
}
