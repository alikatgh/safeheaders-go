package cjsongo

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// Unmarshal parses JSON to a map (stubbed with stdlib; expand for custom).
func Unmarshal(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, errors.New("unmarshal error: " + err.Error())
	}
	return m, nil
}

// UnmarshalParallel deserializes JSON objects in parallel (for large/nested data).
func UnmarshalParallel(data []byte) (map[string]interface{}, error) {
	// First, unmarshal top-level to map.
	m, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	// Concurrent deserialization on sub-objects (stub: process values in parallel).
	numWorkers := 4
	var wg sync.WaitGroup
	errs := make(chan error, numWorkers)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	chunkSize := len(keys) / numWorkers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if i == numWorkers-1 {
			end = len(keys)
		}
		go func(keys []string) {
			defer wg.Done()
			for _, k := range keys {
				// "Deserialize" sub-value (stub; real would recurse if object).
				if _, ok := m[k].(map[string]interface{}); ok {
					// Parallel processing placeholder.
				}
			}
		}(keys[start:end])
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	return m, nil
}

// Marshal serializes to JSON.
func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalStream parses from reader.
func UnmarshalStream(r io.Reader) (map[string]interface{}, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Unmarshal(data)
}
