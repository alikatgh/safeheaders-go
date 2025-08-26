package stbimagego

import (
	"bytes"
	"context" // Import the context package
	"image"
	"image/color"
	"image/png"
	"testing"
)

// createTestPNG generates a simple 1x1 PNG image in memory for testing.
func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("failed to create test png: " + err.Error())
	}
	return buf.Bytes()
}

func TestLoad(t *testing.T) {
	pngData := createTestPNG()
	img, err := Load(pngData)
	if err != nil {
		t.Fatalf("Load() failed with error: %v", err)
	}
	if img == nil {
		t.Fatal("Load() returned a nil image")
	}
	if bounds := img.Bounds(); bounds.Dx() != 1 || bounds.Dy() != 1 {
		t.Errorf("Expected image bounds 1x1, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestLoadStream(t *testing.T) {
	pngData := createTestPNG()
	reader := bytes.NewReader(pngData)
	img, err := LoadStream(reader)
	if err != nil {
		t.Fatalf("LoadStream() failed with error: %v", err)
	}
	if img == nil {
		t.Fatal("LoadStream() returned a nil image")
	}
}

func TestLoadBatchConcurrent(t *testing.T) {
	pngData := createTestPNG()
	datas := [][]byte{pngData, pngData, pngData}

	// Pass a background context for the standard test case.
	images, err := LoadBatchConcurrent(context.Background(), datas)
	if err != nil {
		t.Fatalf("LoadBatchConcurrent() failed with error: %v", err)
	}
	if len(images) != len(datas) {
		t.Fatalf("Expected %d images, got %d", len(datas), len(images))
	}
	for i, img := range images {
		if img == nil {
			t.Errorf("Image at index %d is nil", i)
		}
	}
}

// NEW TEST: Verify that context cancellation works correctly.
func TestLoadBatchConcurrent_Cancellation(t *testing.T) {
	pngData := createTestPNG()
	// Create a large batch to ensure the operation takes some time.
	datas := make([][]byte, 100)
	for i := range datas {
		datas[i] = pngData
	}

	// Create a context that we can cancel.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately.
	cancel()

	_, err := LoadBatchConcurrent(ctx, datas)
	if err == nil {
		t.Fatal("LoadBatchConcurrent() did not return an error on a cancelled context")
	}

	// Check if the error is a context cancellation error.
	if err != context.Canceled {
		t.Errorf("Expected error to be context.Canceled, but got: %v", err)
	}
}
