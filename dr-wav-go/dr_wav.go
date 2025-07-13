package drwavgo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

// LoadWAV loads WAV audio samples (basic RIFF header parsing).
func LoadWAV(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)
	var header [4]byte
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	if string(header[:]) != "RIFF" {
		return nil, errors.New("invalid WAV header")
	}
	// Skip size, WAVE, fmt, etc. (stub; full would read channels, sample rate).
	// For MVP, return raw data after header.
	samples := data[44:] // Typical header size; adjust for real.
	return samples, nil
}

// LoadStreamConcurrent loads and decodes WAV from a stream in parallel chunks.
func LoadStreamConcurrent(r io.Reader) ([]byte, error) {
	// Read full data sequentially to handle header.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// Parse header and get samples.
	samples, parseErr := LoadWAV(data)
	if parseErr != nil {
		return nil, parseErr
	}

	// Concurrent "decoding" on sample chunks (stub for parallelism).
	numWorkers := 4
	chunkSize := len(samples) / numWorkers
	var wg sync.WaitGroup
	results := make([][]byte, numWorkers)
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if i == numWorkers-1 {
			end = len(samples)
		}
		go func(i int, chunk []byte) {
			defer wg.Done()
			// "Decode" chunk (stub; real could process channels/samples).
			results[i] = chunk // Return as-is for MVP.
		}(i, samples[start:end])
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
