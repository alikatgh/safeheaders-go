package stbimagego

import "testing"

// TestMaxImagePixels verifies the decode-bomb guard: an image whose pixel count
// exceeds MaxImagePixels is rejected before the expensive full decode.
func TestMaxImagePixels(t *testing.T) {
	img := createTestPNG(20, 20) // 400 pixels

	if _, err := Load(img); err != nil {
		t.Fatalf("Load under the default limit failed: %v", err)
	}

	orig := MaxImagePixels
	defer func() { MaxImagePixels = orig }()

	MaxImagePixels = 100 // below 400
	if _, err := Load(img); err == nil {
		t.Fatal("expected Load to reject a 20x20 image under a 100-pixel limit")
	}

	MaxImagePixels = 0 // disabled
	if _, err := Load(img); err != nil {
		t.Fatalf("Load with the guard disabled failed: %v", err)
	}
}
