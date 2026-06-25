// Package cjsongo provides JSON marshaling and unmarshaling helpers, including
// parallel processing of large JSON arrays. It is a port of the cJSON C library
// (github.com/DaveGamble/cJSON).
package cjsongo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// Unmarshal parses JSON data into the provided interface.
func Unmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return errors.New("empty JSON data")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}
	return nil
}

// UnmarshalToMap parses JSON into a map.
func UnmarshalToMap(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// UnmarshalToSlice parses JSON array into a slice.
func UnmarshalToSlice(data []byte) ([]interface{}, error) {
	var s []interface{}
	if err := Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// Marshal serializes the provided value to JSON.
func Marshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	return data, nil
}

// MarshalIndent serializes with indentation for readability.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	data, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, fmt.Errorf("marshal indent error: %w", err)
	}
	return data, nil
}

// UnmarshalStream parses JSON from an io.Reader. It does not impose a size
// limit, so for untrusted input callers MUST wrap r in an io.LimitReader (or
// http.MaxBytesReader) to bound memory.
func UnmarshalStream(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("stream unmarshal error: %w", err)
	}
	return nil
}

// MarshalStream writes JSON to an io.Writer.
func MarshalStream(w io.Writer, v interface{}) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("stream marshal error: %w", err)
	}
	return nil
}

// MaxArrayItems caps how many elements UnmarshalArrayParallel will process,
// guarding against memory-amplification (a tiny "[0,0,0,...]" body declares
// millions of elements, each eagerly committing slice/channel/map slots). The
// default is 1,048,576. Set it to 0 to disable the cap.
var MaxArrayItems = 1 << 20

// UnmarshalArrayParallel deserializes JSON array items in parallel.
// Useful for large arrays where each item can be processed independently.
//
// On a malformed item it returns a single error wrapping the first failure
// observed by the worker pool; which item that is, is nondeterministic.
func UnmarshalArrayParallel(data []byte) ([]map[string]interface{}, error) {
	// First unmarshal to raw array
	var rawArray []json.RawMessage
	if err := json.Unmarshal(data, &rawArray); err != nil {
		return nil, fmt.Errorf("failed to parse array: %w", err)
	}

	if MaxArrayItems > 0 && len(rawArray) > MaxArrayItems {
		return nil, fmt.Errorf("array has %d items, exceeding the %d-item limit (adjust MaxArrayItems)",
			len(rawArray), MaxArrayItems)
	}

	if len(rawArray) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Process items in parallel
	numWorkers := runtime.NumCPU()
	if len(rawArray) < numWorkers {
		numWorkers = len(rawArray)
	}

	results := make([]map[string]interface{}, len(rawArray))
	errs := make(chan error, numWorkers)
	jobs := make(chan int, len(rawArray))

	var wg sync.WaitGroup

	// Send jobs
	for i := range rawArray {
		jobs <- i
	}
	close(jobs)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				var item map[string]interface{}
				if err := json.Unmarshal(rawArray[idx], &item); err != nil {
					errs <- fmt.Errorf("failed to unmarshal item %d: %w", idx, err)
					return
				}
				results[idx] = item
			}
		}()
	}

	wg.Wait()
	close(errs)

	// Check for errors
	if err := <-errs; err != nil {
		return nil, err
	}

	return results, nil
}

// Valid checks if the data is valid JSON without fully parsing it.
func Valid(data []byte) bool {
	return json.Valid(data)
}

// Compact removes insignificant whitespace from JSON.
func Compact(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, fmt.Errorf("compact error: %w", err)
	}
	return buf.Bytes(), nil
}
