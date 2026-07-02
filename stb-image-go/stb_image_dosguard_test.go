package stbimagego

import (
	"context"
	"testing"
)

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

// TestMaxBatchSize verifies the aggregate DoS guard: a batch whose item count
// exceeds MaxBatchSize is rejected before any decoding work begins, even
// though every individual image would pass MaxImagePixels on its own.
func TestMaxBatchSize(t *testing.T) {
	datas := [][]byte{createTestPNG(4, 4), createTestPNG(4, 4), createTestPNG(4, 4)}

	if _, err := LoadBatchConcurrent(context.Background(), datas); err != nil {
		t.Fatalf("LoadBatchConcurrent under the default limit failed: %v", err)
	}

	orig := MaxBatchSize
	defer func() { MaxBatchSize = orig }()

	MaxBatchSize = 2 // below len(datas) == 3
	if _, err := LoadBatchConcurrent(context.Background(), datas); err == nil {
		t.Fatal("expected LoadBatchConcurrent to reject a 3-image batch under a 2-image limit")
	}

	MaxBatchSize = 0 // disabled
	if _, err := LoadBatchConcurrent(context.Background(), datas); err != nil {
		t.Fatalf("LoadBatchConcurrent with the guard disabled failed: %v", err)
	}
}
