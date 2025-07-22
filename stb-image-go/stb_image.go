package stbimagego

import (
	"bytes"
	"errors"
	"image"
	"io"
	"runtime" // Imported runtime package
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

// LoadBatchConcurrent decodes multiple images in parallel using goroutines.
func LoadBatchConcurrent(datas [][]byte) ([]image.Image, error) {
	// P2 FIX: numWorkers is now dynamic.
	numWorkers := runtime.NumCPU()
	if len(datas) < numWorkers {
		numWorkers = len(datas) // Don't spin up more workers than jobs.
	}

	var wg sync.WaitGroup
	results := make([]image.Image, len(datas))
	errs := make(chan error, len(datas))
	jobs := make(chan int, len(datas))

	// Fill the jobs channel with indices.
	for i := 0; i < len(datas); i++ {
		jobs <- i
	}
	close(jobs)

	// Start workers.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				img, err := Load(datas[idx])
				if err != nil {
					errs <- err
					return
				}
				results[idx] = img
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errs:
		return nil, err // Return the first error encountered.
	default:
	}
	return results, nil
}

// LoadStream decodes from an io.Reader without buffering the entire stream.
func LoadStream(r io.Reader) (image.Image, error) {
	// P2 FIX: This now decodes directly from the stream.
	img, _, err := image.Decode(r)
	return img, err
}
