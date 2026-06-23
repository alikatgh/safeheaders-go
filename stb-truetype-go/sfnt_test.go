package truetype

import (
	"encoding/binary"
	"testing"
)

func TestCmapFormat0(t *testing.T) {
	d := make([]byte, 6+256)
	binary.BigEndian.PutUint16(d[0:], 0)
	d[6+'A'] = 65
	if got := cmapFormat0(d, 'A'); got != 65 {
		t.Errorf("format0('A') = %d, want 65", got)
	}
	if got := cmapFormat0(d, 0x100); got != 0 {
		t.Errorf("format0 out-of-range = %d, want 0", got)
	}
}

func TestCmapFormat6(t *testing.T) {
	d := make([]byte, 10+3*2)
	binary.BigEndian.PutUint16(d[0:], 6)
	binary.BigEndian.PutUint16(d[6:], 'A') // firstCode
	binary.BigEndian.PutUint16(d[8:], 3)   // entryCount
	binary.BigEndian.PutUint16(d[10:], 10)
	binary.BigEndian.PutUint16(d[12:], 11)
	binary.BigEndian.PutUint16(d[14:], 12)
	if got := cmapFormat6(d, 'B'); got != 11 {
		t.Errorf("format6('B') = %d, want 11", got)
	}
	if got := cmapFormat6(d, 'Z'); got != 0 {
		t.Errorf("format6 out-of-range = %d, want 0", got)
	}
}

func TestCmapFormat12(t *testing.T) {
	d := make([]byte, 16+12)
	binary.BigEndian.PutUint16(d[0:], 12)
	binary.BigEndian.PutUint32(d[12:], 1) // nGroups
	binary.BigEndian.PutUint32(d[16:], 0x1F600)
	binary.BigEndian.PutUint32(d[20:], 0x1F610)
	binary.BigEndian.PutUint32(d[24:], 100)
	if got := cmapFormat12(d, 0x1F605); got != 105 {
		t.Errorf("format12(U+1F605) = %d, want 105", got)
	}
	if got := cmapFormat12(d, '0'); got != 0 {
		t.Errorf("format12 out-of-range = %d, want 0", got)
	}
}

// TestGlyphIndexResolves checks the real font maps common runes to non-zero glyphs.
func TestGlyphIndexResolves(t *testing.T) {
	f := testFont()
	for _, r := range []rune{'A', 'z', '0', ' '} {
		if f.glyphIndex(r) == 0 && r != ' ' {
			t.Errorf("glyphIndex(%q) returned 0 (.notdef)", r)
		}
	}
}

// TestWideRuneSweep renders a broad range of runes and asserts none error or panic.
func TestWideRuneSweep(t *testing.T) {
	cache := NewGlyphCache(testFont(), 24.0, 0, nil)
	for r := rune(0x20); r <= 0x24F; r++ { // ASCII + Latin-1 + Latin Extended-A
		if _, err := cache.GetGlyph(r); err != nil {
			t.Fatalf("GetGlyph(%q / U+%04X): %v", r, r, err)
		}
	}
}

// TestCompositeGlyph exercises the composite-glyph decode path if the font
// contains any composite glyphs (accented characters usually are).
func TestCompositeGlyph(t *testing.T) {
	f := testFont()
	glyf, ok := f.tableData("glyf")
	if !ok {
		t.Fatal("no glyf table")
	}
	for gid := uint16(0); int(gid)+1 < len(f.loca); gid++ {
		start, end := f.loca[gid], f.loca[gid+1]
		if start >= end || int(end) > len(glyf) || int(start)+10 > len(glyf) {
			continue
		}
		if i16(glyf[start:]) >= 0 { // not composite
			continue
		}
		contours, err := f.glyphContours(gid, 0)
		if err != nil {
			t.Fatalf("composite glyph %d: %v", gid, err)
		}
		if len(contours) == 0 {
			t.Errorf("composite glyph %d produced no contours", gid)
		}
		return // exercised one composite glyph; done
	}
	t.Skip("font contains no composite glyphs")
}

// TestParseRejectsBadHeaders covers the loader's validation paths.
func TestParseRejectsBadHeaders(t *testing.T) {
	cases := map[string][]byte{
		"too short":     {0, 1},
		"OTTO/CFF":      {0x4F, 0x54, 0x54, 0x4F, 0, 0, 0, 0, 0, 0, 0, 0},
		"bad version":   {0xDE, 0xAD, 0xBE, 0xEF, 0, 0, 0, 0, 0, 0, 0, 0},
		"truncated dir": {0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}, // claims 1 table, no room
	}
	for name, data := range cases {
		if _, err := LoadFontFromBytes(data); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}
