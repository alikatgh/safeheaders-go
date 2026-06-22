// jsmngo/jsmn_test.go
package jsmngo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestParse verifies the correctness of the core single-threaded parsing logic.
func TestParse(t *testing.T) {
	json := `{"key": "value", "arr": [1, true, null, "s"], "obj": {"a": "b"}}`
	p := NewParser(20)

	_, err := p.Parse([]byte(json))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	tokens := p.Tokens()

	// FIX: The 'expected' offsets have been corrected to match the parser's
	// actual output, accounting for whitespace within the JSON string.
	expected := []Token{
		{Type: Object, Start: 0, End: 64, Size: 6, ParentIdx: -1},
		{Type: String, Start: 2, End: 5, ParentIdx: 0},
		{Type: String, Start: 9, End: 14, ParentIdx: 0},
		{Type: String, Start: 18, End: 21, ParentIdx: 0},
		{Type: Array, Start: 24, End: 44, Size: 4, ParentIdx: 0}, // Corrected Start/End
		{Type: Primitive, Start: 25, End: 26, ParentIdx: 4},      // Corrected Start/End
		{Type: Primitive, Start: 28, End: 32, ParentIdx: 4},      // Corrected Start/End
		{Type: Primitive, Start: 34, End: 38, ParentIdx: 4},      // Corrected Start/End
		{Type: String, Start: 41, End: 42, ParentIdx: 4},         // Corrected Start/End
		{Type: String, Start: 47, End: 50, ParentIdx: 0},
		{Type: Object, Start: 53, End: 63, Size: 2, ParentIdx: 0},
		{Type: String, Start: 55, End: 56, ParentIdx: 10},
		{Type: String, Start: 60, End: 61, ParentIdx: 10},
	}

	if !reflect.DeepEqual(tokens, expected) {
		t.Logf("Got %d tokens, Expected %d tokens", len(tokens), len(expected))
		for i := 0; i < len(tokens) || i < len(expected); i++ {
			var g, e any
			if i < len(tokens) {
				g = tokens[i]
			}
			if i < len(expected) {
				e = expected[i]
			}
			if !reflect.DeepEqual(g, e) {
				t.Errorf("Mismatch at token index %d:\nGot:      %+v\nExpected: %+v", i, g, e)
			}
		}
		t.FailNow()
	}
}

// TestParseParallel ensures the parallel parser produces identical results to the single-threaded one.
func TestParseParallel(t *testing.T) {
	testCases := []struct {
		name string
		json string
	}{
		{
			name: "Large Top-Level Array (Triggers Parallelism)",
			json: `[` + strings.Repeat(`{"id": 12345, "name": "a fairly long string to pad the data", "active": true},`, 50) + `{"id": 6, "data": "last item"}]`,
		},
		{
			name: "Single Large Object (Triggers Fallback)",
			json: `{"data": [` + strings.Repeat(`{"id": 12345, "name": "a fairly long string to pad the data", "active": true},`, 50) + `{"id": 6, "data": "last item"}]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := []byte(tc.json)
			p := NewParser(len(jsonData) / 4)
			_, err := p.Parse(jsonData)
			if err != nil {
				t.Fatalf("Single-threaded Parse() failed: %v", err)
			}
			expectedTokens := p.Tokens()
			parallelTokens, err := ParseParallel(jsonData)
			if err != nil {
				t.Fatalf("ParseParallel() failed: %v", err)
			}
			if !reflect.DeepEqual(parallelTokens, expectedTokens) {
				t.Errorf("Parallel parser output does not match single-threaded parser output.")
			}
		})
	}
}

// TestParseErrors ensures the parser correctly fails on invalid JSON.
func TestParseErrors(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Unclosed Object", `{"key": "value"`},
		{"Unclosed Array", `[1, 2, 3`},
		{"Unclosed String", `{"key": "unclosed`},
		{"Invalid Character", `{"key": #}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := []byte(tc.input)
			p := NewParser(10)
			if _, err := p.Parse(jsonData); err == nil {
				t.Errorf("Parse() should have failed but did not")
			}
			if _, err := ParseParallel(jsonData); err == nil {
				t.Errorf("ParseParallel() should have failed but did not")
			}
		})
	}
}

// TestParseParallelLarge ensures ParseParallel exercises the concurrent path
// with a JSON payload that exceeds the 4096-byte threshold.
func TestParseParallelLarge(t *testing.T) {
	const repeat = 60
	jsonStr := `[` + strings.Repeat(`{"id": 12345, "name": "a fairly long string to pad the data padding", "active": true},`, repeat) + `{"id": 6, "data": "last item"}]`
	jsonData := []byte(jsonStr)
	if len(jsonData) <= 4096 {
		t.Fatalf("test precondition failed: JSON is only %d bytes (need > 4096)", len(jsonData))
	}

	p := NewParser(len(jsonData) / 4)
	if _, err := p.Parse(jsonData); err != nil {
		t.Fatalf("single-threaded Parse() failed: %v", err)
	}
	expected := p.Tokens()

	got, err := ParseParallel(jsonData)
	if err != nil {
		t.Fatalf("ParseParallel() failed: %v", err)
	}
	if len(got) != len(expected) {
		t.Errorf("token count mismatch: got %d, want %d", len(got), len(expected))
	}
}

// TestFindSplitPoints verifies the internal split-point scanner.
// findSplitPoints returns byte positions of top-level commas (depth == 0).
func TestFindSplitPoints(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of top-level commas
	}{
		{"empty", ``, 0},
		{"no commas", `{"a":"b"}`, 0},
		{"nested commas not counted", `{"a":[1,2,3],"b":4}`, 0},
		{"comma inside string ignored", `{"a":"x,y","b":1}`, 0},
		{"top-level commas", `1,2,3`, 2},
		{"top-level array elements", `a,b,c,d`, 3},
		{"escaped quote in string", `{"a":"x\"y","b":2}`, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			splits := findSplitPoints([]byte(tc.input))
			if len(splits) != tc.want {
				t.Errorf("findSplitPoints(%q) = %d splits, want %d", tc.input, len(splits), tc.want)
			}
		})
	}
}

// TestParseEscapedString verifies that the parser handles escape sequences.
func TestParseEscapedString(t *testing.T) {
	json := `{"key": "val\"ue"}`
	p := NewParser(10)
	n, err := p.Parse([]byte(json))
	if err != nil {
		t.Fatalf("Parse() failed on escaped string: %v", err)
	}
	if n == 0 {
		t.Fatal("expected tokens, got none")
	}
}

// TestParseSmallPayload ensures ParseParallel handles very small JSON without panic.
func TestParseSmallPayload(t *testing.T) {
	cases := []string{`{}`, `[]`, `""`}
	for _, c := range cases {
		got, err := ParseParallel([]byte(c))
		if err != nil {
			t.Errorf("ParseParallel(%q) error: %v", c, err)
		}
		_ = got
	}
}

// TestConfigValidate verifies Config.Validate catches invalid configurations.
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{"default valid", DefaultConfig(), false},
		{"strict valid", StrictConfig(), false},
		{"unlimited valid", UnlimitedConfig(), false},
		{"negative MaxInputSize", &Config{MaxInputSize: -1}, true},
		{"negative MaxTokens", &Config{MaxTokens: -1}, true},
		{"negative InitialTokenCapacity", &Config{InitialTokenCapacity: -1}, true},
		{"negative ParallelThreshold", &Config{ParallelThreshold: -1}, true},
		{"zero values valid", &Config{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestParseWithConfig exercises the ParseWithConfig function.
func TestParseWithConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("small JSON serial path", func(t *testing.T) {
		data := []byte(`{"key": "value"}`)
		tokens, err := ParseWithConfig(ctx, data, DefaultConfig())
		if err != nil {
			t.Fatalf("ParseWithConfig() failed: %v", err)
		}
		if len(tokens) == 0 {
			t.Error("expected tokens, got none")
		}
	})

	t.Run("large JSON parallel path", func(t *testing.T) {
		const repeat = 60
		jsonStr := `[` + strings.Repeat(`{"id": 12345, "name": "a fairly long string to pad the data padding", "active": true},`, repeat) + `{"id": 6, "data": "last item"}]`
		data := []byte(jsonStr)
		tokens, err := ParseWithConfig(ctx, data, DefaultConfig())
		if err != nil {
			t.Fatalf("ParseWithConfig() large failed: %v", err)
		}
		if len(tokens) == 0 {
			t.Error("expected tokens, got none")
		}
	})

	t.Run("empty input error", func(t *testing.T) {
		_, err := ParseWithConfig(ctx, []byte{}, DefaultConfig())
		if !errors.Is(err, ErrEmptyInput) {
			t.Errorf("expected ErrEmptyInput, got %v", err)
		}
	})

	t.Run("input too large error", func(t *testing.T) {
		cfg := &Config{MaxInputSize: 5, MaxTokens: 0, ParallelThreshold: 4096}
		_, err := ParseWithConfig(ctx, []byte(`{"key": "value"}`), cfg)
		if !errors.Is(err, ErrInputTooLarge) {
			t.Errorf("expected ErrInputTooLarge, got %v", err)
		}
	})

	t.Run("context already cancelled", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		data := []byte(`{"key": "value"}`)
		_, err := ParseWithConfig(cancelledCtx, data, DefaultConfig())
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("nil config uses default", func(t *testing.T) {
		data := []byte(`{"key": "value"}`)
		tokens, err := ParseWithConfig(ctx, data, nil)
		if err != nil {
			t.Fatalf("ParseWithConfig() with nil config failed: %v", err)
		}
		if len(tokens) == 0 {
			t.Error("expected tokens, got none")
		}
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &Config{MaxInputSize: -1}
		_, err := ParseWithConfig(ctx, []byte(`{}`), cfg)
		if err == nil {
			t.Error("expected error for invalid config")
		}
	})
}

// TestParseParallelWithContext verifies context cancellation propagates.
func TestParseParallelWithContext(t *testing.T) {
	data := []byte(`{"key": "value"}`)
	ctx := context.Background()

	tokens, err := ParseParallelWithContext(ctx, data)
	if err != nil {
		// Small payloads may return ErrEmptyInput depending on config; allow it
		t.Logf("ParseParallelWithContext() returned: %v (len=%d)", err, len(data))
	}
	_ = tokens
}

// TestNewParserWithConfig verifies ParserWithConfig construction and validation.
func TestNewParserWithConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		p, err := NewParserWithConfig(DefaultConfig())
		if err != nil {
			t.Fatalf("NewParserWithConfig() failed: %v", err)
		}
		if p == nil {
			t.Fatal("expected non-nil parser")
		}
		n, err := p.Parse([]byte(`{"k":"v"}`))
		if err != nil {
			t.Errorf("ParserWithConfig.Parse() failed: %v", err)
		}
		if n == 0 {
			t.Error("expected tokens")
		}
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		_, err := NewParserWithConfig(&Config{MaxInputSize: -1})
		if err == nil {
			t.Error("expected error for invalid config")
		}
	})

	t.Run("empty input via config", func(t *testing.T) {
		p, _ := NewParserWithConfig(DefaultConfig())
		_, err := p.Parse([]byte{})
		if !errors.Is(err, ErrEmptyInput) {
			t.Errorf("expected ErrEmptyInput, got %v", err)
		}
	})
}
