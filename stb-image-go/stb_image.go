package stbimagego

import (
	"bytes"
	"context" // Imported context package
	"errors"
	"fmt"
	"image"
	"io"
	"runtime"
	"sync"
)

// Load decodes an image from data.
func Load(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("failed to decode image: " + err.Error())
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
					// The context was cancelled, so stop processing.
					errs <- ctx.Err()
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs) // Close the error channel after all workers are done.

	// Collect all errors into a slice.
	var multiErr []error
	for err := range errs {
		multiErr = append(multiErr, err)
	}

	if len(multiErr) > 0 {
		// If the context was cancelled, return context.Canceled directly
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
	// P2 FIX: This now decodes directly from the stream.
	img, _, err := image.Decode(r)
	return img, err
}
