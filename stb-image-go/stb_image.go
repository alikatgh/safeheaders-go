package stbimagego

import (
	"bytes"
	"errors"
	"image"
	"io"
	"sync"
)

// Load decodes an image from data (stubbed with stdlib; expand for full stb features like HDR).
func Load(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("failed to decode image: " + err.Error())
	}
	return img, nil
}

// LoadBatchConcurrent decodes multiple images in parallel using goroutines.
func LoadBatchConcurrent(datas [][]byte) ([]image.Image, error) {
	numWorkers := 4 // Cap for simplicity.
	var wg sync.WaitGroup
	results := make([]image.Image, len(datas))
	errs := make(chan error, len(datas))

	for i := 0; i < len(datas); i += numWorkers {
		end := i + numWorkers
		if end > len(datas) {
			end = len(datas)
		}
		for j := i; j < end; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				img, err := Load(datas[idx])
				if err != nil {
					errs <- err
					return
				}
				results[idx] = img
			}(j)
		}
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	return results, nil
}

// LoadStream decodes from an io.Reader.
func LoadStream(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Load(data)
}
