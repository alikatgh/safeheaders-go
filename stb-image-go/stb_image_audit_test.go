package stbimagego

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// TestLoadBatchConcurrentCancellationNoDeadlock guards against the under-buffered
// errs channel: cancellation coinciding with decode failures must not wedge the
// worker pool (audit H2).
func TestLoadBatchConcurrentCancellationNoDeadlock(t *testing.T) {
	bad := make([][]byte, 64)
	for i := range bad {
		bad[i] = []byte{0, 1, 2, 3} // invalid image data -> decode failure
	}
	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			_, _ = LoadBatchConcurrent(ctx, bad)
			close(done)
		}()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("LoadBatchConcurrent deadlocked (iteration %d)", i)
		}
	}
}

// TestLoadStreamPixelLimit verifies LoadStream enforces MaxImagePixels (audit M6).
func TestLoadStreamPixelLimit(t *testing.T) {
	img := createTestPNG(200, 200) // 40,000 pixels

	if _, err := LoadStream(bytes.NewReader(img)); err != nil {
		t.Fatalf("LoadStream under default limit: %v", err)
	}

	orig := MaxImagePixels
	defer func() { MaxImagePixels = orig }()

	MaxImagePixels = 1000 // below 40,000
	if _, err := LoadStream(bytes.NewReader(img)); err == nil {
		t.Error("LoadStream did not enforce MaxImagePixels")
	}

	MaxImagePixels = 0 // disabled
	if _, err := LoadStream(bytes.NewReader(img)); err != nil {
		t.Errorf("LoadStream with guard disabled: %v", err)
	}
}
