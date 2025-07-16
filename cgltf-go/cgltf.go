package cgltfgo

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

// LoadGLTF loads glTF model data (stubbed parsing).
func LoadGLTF(data []byte) ([]byte, error) {
	// Stub: Validate and return (real would parse JSON/binary for meshes/textures).
	if !bytes.HasPrefix(data, []byte("glTF")) {
		return nil, errors.New("invalid glTF header")
	}
	return data, nil
}

// LoadAssetConcurrent loads glTF assets in parallel (e.g., meshes, textures).
func LoadAssetConcurrent(data []byte) ([]byte, error) {
	numWorkers := 4
	chunkSize := len(data) / numWorkers
	var wg sync.WaitGroup
	results := make([][]byte, numWorkers)
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if i == numWorkers-1 {
			end = len(data)
		}
		go func(i int, chunk []byte) {
			defer wg.Done()
			loaded, err := LoadGLTF(chunk)
			if err != nil {
				errs <- err
				return
			}
			results[i] = loaded
		}(i, data[start:end])
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	var merged []byte
	for _, res := range results {
		merged = append(merged, res...)
	}
	return merged, nil
}

// LoadStream loads from reader.
func LoadStream(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return LoadGLTF(data)
}
