package minizgo

import (
	"bytes"
	"compress/flate"
	"io"
	"sync"
)

// Compress compresses data using deflate (stubbed with stdlib).
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestCompression)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	w.Close()
	return buf.Bytes(), nil
}

// CompressChunksConcurrent compresses large data in parallel chunks.
func CompressChunksConcurrent(data []byte) ([]byte, error) {
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
			compressed, err := Compress(chunk)
			if err != nil {
				errs <- err
				return
			}
			results[i] = compressed
		}(i, data[start:end])
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}

	// Merge compressed chunks (add headers if full ZIP).
	var merged bytes.Buffer
	for _, res := range results {
		merged.Write(res)
	}
	return merged.Bytes(), nil
}

// DecompressStream decompresses from reader.
func DecompressStream(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	reader := flate.NewReader(r)
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
