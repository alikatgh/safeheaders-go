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

// Load decodes an image from data.
func Load(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data")
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
	errs := make(chan error, len(datas))

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

// LoadStream decodes from an io.Reader without buffering the entire stream.
func LoadStream(r io.Reader) (image.Image, error) {
	// Decodes directly from the stream without buffering the whole input.
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image stream: %w", err)
	}
	return img, nil
}
