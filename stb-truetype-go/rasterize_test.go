package truetype

import "testing"

// inkedPixels counts pixels with meaningful coverage in a glyph bitmap.
func inkedPixels(g *Glyph) (w, h, inked int) {
	b := g.Bitmap().Bounds()
	w, h = b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r, _, _, _ := g.Bitmap().At(x, y).RGBA(); r>>8 > 32 {
				inked++
			}
		}
	}
	return w, h, inked
}

// TestRasterizeRendersRealGlyphs verifies the rasterizer produces real, inked
// glyph bitmaps (not the old blank placeholder) with sensible metrics.
func TestRasterizeRendersRealGlyphs(t *testing.T) {
	cache := NewGlyphCache(testFont(), 48.0, 0, nil)
	for _, r := range []rune{'A', 'B', 'g', 'I', 'o', '@', '5'} {
		gl, err := cache.GetGlyph(r)
		if err != nil {
			t.Fatalf("GetGlyph(%q): %v", r, err)
		}
		w, h, inked := inkedPixels(gl)
		if w < 4 || h < 8 {
			t.Errorf("%q: bitmap %dx%d too small to be a real glyph", r, w, h)
		}
		if inked == 0 {
			t.Errorf("%q: zero inked pixels — not rendered", r)
		}
		if gl.Metrics().AdvanceWidth <= 0 {
			t.Errorf("%q: advance width %d should be positive", r, gl.Metrics().AdvanceWidth)
		}
	}
}

// TestRasterizeGlyphShapes checks distinguishing shape properties so a future
// regression that renders garbage (e.g. all glyphs identical) is caught.
func TestRasterizeGlyphShapes(t *testing.T) {
	cache := NewGlyphCache(testFont(), 48.0, 0, nil)
	get := func(r rune) *Glyph {
		g, err := cache.GetGlyph(r)
		if err != nil {
			t.Fatalf("GetGlyph(%q): %v", r, err)
		}
		return g
	}

	// 'I' is much narrower than 'M'.
	iw := get('I').Bitmap().Bounds().Dx()
	mw := get('M').Bitmap().Bounds().Dx()
	if iw >= mw {
		t.Errorf("'I' width %d should be less than 'M' width %d", iw, mw)
	}

	// A capital is taller (extends higher above the baseline) than a short
	// lowercase letter like 'o' — bearingY is the top above the baseline.
	if get('H').Metrics().BearingY <= get('o').Metrics().BearingY {
		t.Errorf("cap-height bearingY (%d) should exceed x-height bearingY (%d)",
			get('H').Metrics().BearingY, get('o').Metrics().BearingY)
	}

	// Space renders blank but advances the cursor.
	sp := get(' ')
	if _, _, inked := inkedPixels(sp); inked > 4 {
		t.Errorf("space should be blank, got %d inked pixels", inked)
	}
	if sp.Metrics().AdvanceWidth <= 0 {
		t.Error("space should have a positive advance width")
	}
}

// TestRasterizeScales verifies a larger size yields a proportionally larger bitmap.
func TestRasterizeScales(t *testing.T) {
	small := NewGlyphCache(testFont(), 16.0, 0, nil)
	large := NewGlyphCache(testFont(), 64.0, 0, nil)
	gs, _ := small.GetGlyph('A')
	gl, _ := large.GetGlyph('A')
	if gl.Bitmap().Bounds().Dy() <= gs.Bitmap().Bounds().Dy() {
		t.Errorf("64px 'A' (%d) should be taller than 16px 'A' (%d)",
			gl.Bitmap().Bounds().Dy(), gs.Bitmap().Bounds().Dy())
	}
}

// TestRasterizeInvalidSize rejects a non-positive size instead of panicking.
func TestRasterizeInvalidSize(t *testing.T) {
	if _, _, err := rasterizeGlyph(testFont(), 'A', 0); err == nil {
		t.Error("expected an error for size 0")
	}
}

// TestRasterizeUnmappedRune renders .notdef for an unmapped rune without error.
func TestRasterizeUnmappedRune(t *testing.T) {
	cache := NewGlyphCache(testFont(), 32.0, 0, nil)
	if _, err := cache.GetGlyph('￿'); err != nil {
		t.Errorf("unmapped rune should render .notdef, got error: %v", err)
	}
}

// TestLoadFontRejectsNonFont ensures the loader validates input.
func TestLoadFontRejectsNonFont(t *testing.T) {
	if _, err := LoadFontFromBytes([]byte("not a font")); err == nil {
		t.Error("expected LoadFontFromBytes to reject non-font data")
	}
}
