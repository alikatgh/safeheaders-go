// Package stbimagego provides image loading (PNG, JPEG, GIF) with support for
// decoding batches of images concurrently. It mirrors the role of the
// stb_image C library (github.com/nothings/stb) using the Go image stdlib.
package stbimagego

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"io"
	"runtime"
	"sync"
)

// ImageInfo contains metadata about an image without decoding the full image.
type ImageInfo struct {
	Width  int
	Height int
	Format string
}

// GetInfo returns image metadata without fully decoding the image.
func GetInfo(data []byte) (*ImageInfo, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &ImageInfo{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
	}, nil
}

// MaxImagePixels caps the number of pixels Load will decode, guarding against
// decode bombs — a tiny file whose header declares enormous dimensions can drive
// image.Decode to allocate gigabytes. The default is 64 megapixels (e.g.
// 8192x8192). Set it to 0 to disable the guard.
var MaxImagePixels = 64 << 20

// checkPixelLimit rejects images whose declared dimensions exceed MaxImagePixels,
// reading only the (cheap) header. A header that won't even decode is left for
// the full decode to report.
func checkPixelLimit(data []byte) error {
	if MaxImagePixels <= 0 {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil //nolint:nilerr // let the full decode surface the real error
	}
	if cfg.Width > 0 && cfg.Height > 0 && int64(cfg.Width)*int64(cfg.Height) > int64(MaxImagePixels) {
		return fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit (adjust MaxImagePixels)",
			cfg.Width, cfg.Height, MaxImagePixels)
	}
	return nil
}

// Load decodes an image from data. It rejects images larger than MaxImagePixels
// before decoding, so untrusted input cannot trigger a decode-bomb allocation.
func Load(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data")
	}
	if err := checkPixelLimit(data); err != nil {
		return nil, err
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	if img == nil {
		return nil, fmt.Errorf("decoded nil image (format: %s)", format)
	}

	return img, nil
}

// LoadBatchConcurrent decodes multiple images in parallel with context support and full error reporting.
func LoadBatchConcurrent(ctx context.Context, datas [][]byte) ([]image.Image, error) {
	numWorkers := runtime.NumCPU()
	if len(datas) < numWorkers {
		numWorkers = len(datas)
	}

	// jobs channel sends indices of images to be processed.
	jobs := make(chan int, len(datas))
	for i := 0; i < len(datas); i++ {
		jobs <- i
	}
	close(jobs)

	results := make([]image.Image, len(datas))
	// Buffer the worst case so no worker blocks on send: up to len(datas) decode
	// failures plus up to numWorkers cancellation sends. An under-sized buffer
	// deadlocks wg.Wait when cancellation coincides with decode failures.
	errs := make(chan error, len(datas)+numWorkers)

	var wg sync.WaitGroup

	// Start workers.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				// Check cancellation first: a bare select races between a ready
				// job and ctx.Done() (Go picks randomly), so an already-canceled
				// context would only be honored intermittently.
				if err := ctx.Err(); err != nil {
					errs <- err
					return
				}
				select {
				case idx, ok := <-jobs:
					// 'ok' will be false if the jobs channel is closed and empty.
					if !ok {
						return
					}
					img, err := Load(datas[idx])
					if err != nil {
						errs <- fmt.Errorf("failed to decode image at index %d: %w", idx, err)
					} else {
						results[idx] = img
					}
				case <-ctx.Done():
					// The context was canceled, so stop processing.
					errs <- ctx.Err()
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs) // Close the error channel after all workers are done.

	// Collect all errors into a slice.
	multiErr := make([]error, 0, len(errs))
	for err := range errs {
		multiErr = append(multiErr, err)
	}

	if len(multiErr) > 0 {
		// If the context was canceled, return context.Canceled directly
		// (multiple workers may report the same cancellation)
		if errors.Is(multiErr[0], context.Canceled) || errors.Is(multiErr[0], context.DeadlineExceeded) {
			return nil, multiErr[0]
		}
		// For other errors, aggregate them into a formatted error message
		return nil, fmt.Errorf("multiple errors occurred: %v", multiErr)
	}

	return results, nil
}

// LoadStream decodes an image from an io.Reader. Like Load, it enforces the
// MaxImagePixels decode-bomb guard: the header is peeked to check the declared
// dimensions before the full image is decoded.
func LoadStream(r io.Reader) (image.Image, error) {
	if r == nil {
		return nil, errors.New("nil reader")
	}
	if MaxImagePixels > 0 {
		// Tee the header bytes consumed by DecodeConfig into a buffer so the full
		// image can still be decoded (DecodeConfig reads only the header).
		var header bytes.Buffer
		cfg, _, cfgErr := image.DecodeConfig(io.TeeReader(r, &header))
		if cfgErr == nil && cfg.Width > 0 && cfg.Height > 0 &&
			int64(cfg.Width)*int64(cfg.Height) > int64(MaxImagePixels) {
			return nil, fmt.Errorf("image %dx%d exceeds the %d-pixel decode limit (adjust MaxImagePixels)",
				cfg.Width, cfg.Height, MaxImagePixels)
		}
		// Replay the consumed header, then the remainder of the stream.
		r = io.MultiReader(&header, r)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image stream: %w", err)
	}
	return img, nil
}
