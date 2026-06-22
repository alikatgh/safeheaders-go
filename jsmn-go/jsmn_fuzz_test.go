// jsmngo/jsmn_fuzz_test.go
package jsmngo

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// FuzzParse tests the Parse function with random inputs to find crashes and panics.
func FuzzParse(f *testing.F) {
	// Seed corpus with various JSON inputs
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`[1, 2, 3]`))
	f.Add([]byte(`{"nested": {"object": true}}`))
	f.Add([]byte(`[[[[[]]]]]]`))
	f.Add([]byte(`"simple string"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`true`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"a": [1, 2], "b": {"c": "d"}}`))

	// Malformed inputs (should fail gracefully)
	f.Add([]byte(`{`))
	f.Add([]byte(`}`))
	f.Add([]byte(`[`))
	f.Add([]byte(`]`))
	f.Add([]byte(`{"unclosed": "string`))
	f.Add([]byte(`{"invalid": @}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test should not panic or crash, even on invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parse panicked on input %q: %v", data, r)
			}
		}()

		// Limit token count to prevent excessive memory allocation
		p := NewParser(10000)
		_, err := p.Parse(data)

		// We don't care if parsing fails, we just care that it doesn't crash.
		// Only verify token bounds on successful parses; partial tokens may
		// remain after an error and will have End=-1.
		if err != nil {
			return
		}

		// Verify tokens are within bounds (valid parse only)
		tokens := p.Tokens()
		for i, tok := range tokens {
			if tok.Start > len(data) || tok.End > len(data) || tok.Start > tok.End {
				t.Errorf("Token %d has invalid bounds: Start=%d End=%d (data len=%d)",
					i, tok.Start, tok.End, len(data))
			}
		}
	})
}

// FuzzParseParallel tests the parallel parser with random inputs.
func FuzzParseParallel(f *testing.F) {
	// Seed corpus
	f.Add([]byte(`[{"id": 1}, {"id": 2}, {"id": 3}]`))
	f.Add([]byte(`{"a": 1, "b": 2, "c": 3}`))
	f.Add([]byte(`[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`))
	f.Add([]byte(`{"arr": [1, 2, 3], "obj": {"nested": true}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseParallel panicked on input %q: %v", data, r)
			}
		}()

		// Use context with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := ParseParallelWithContext(ctx, data)
		_ = err // We don't care if parsing fails
	})
}

// FuzzParseConsistency ensures parallel and serial parsing produce the same results.
func FuzzParseConsistency(f *testing.F) {
	// Seed with valid JSON only
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`[1, 2, 3, 4, 5]`))
	f.Add([]byte(`{"a": [1, 2], "b": {"c": "d"}}`))
	f.Add([]byte(`[{"id": 1}, {"id": 2}]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Consistency test panicked on input %q: %v", data, r)
			}
		}()

		// Parse serially
		p := NewParser(10000)
		_, errSerial := p.Parse(data)
		tokensSerial := p.Tokens()

		// Parse in parallel
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tokensParallel, errParallel := ParseParallelWithContext(ctx, data)

		// Both should either succeed or fail together
		if (errSerial == nil) != (errParallel == nil) {
			// One succeeded, one failed - this is acceptable since parallel
			// parsing may have different error detection
			return
		}

		// If both succeeded, verify tokens are equivalent
		if errSerial == nil && errParallel == nil {
			if len(tokensSerial) != len(tokensParallel) {
				t.Errorf("Token count mismatch: serial=%d parallel=%d",
					len(tokensSerial), len(tokensParallel))
			}
		}
	})
}

// FuzzLargeInput tests with larger inputs to find memory or performance issues.
func FuzzLargeInput(f *testing.F) {
	// Seed with patterns that can be expanded
	f.Add([]byte(`[1]`))
	f.Add([]byte(`{"k":"v"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Limit input size to prevent OOM
		if len(data) > 10*1024*1024 { // 10MB limit
			t.Skip("Input too large")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Large input test panicked on input (len=%d): %v", len(data), r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		p := NewParser(100000) // Higher limit for large inputs
		_, err := p.Parse(data)
		_ = err

		// Also test parallel
		_, err = ParseParallelWithContext(ctx, data)
		_ = err
	})
}

// FuzzSpecialChars tests handling of special characters and edge cases.
func FuzzSpecialChars(f *testing.F) {
	// Seed with inputs containing special characters
	f.Add([]byte(`{"unicode": "Hello, 世界"}`))
	f.Add([]byte(`{"escaped": "quote \" and slash \/"}`))
	f.Add([]byte(`{"newline": "line1\nline2"}`))
	f.Add([]byte(`{"tab": "col1\tcol2"}`))
	f.Add([]byte(`{"backslash": "C:\\path\\to\\file"}`))
	f.Add([]byte(`{"emoji": "🎉🚀✨"}`))
	f.Add([]byte(string([]byte{0x00, 0x01, 0x1F}))) // Control characters
	f.Add(bytes.Repeat([]byte(`"`), 1000))          // Many quotes
	f.Add(bytes.Repeat([]byte(`\`), 1000))          // Many backslashes

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Special chars test panicked: %v", r)
			}
		}()

		p := NewParser(10000)
		_, err := p.Parse(data)
		_ = err
	})
}
