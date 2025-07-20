// jsmngo/jsmn_test.go
package jsmngo

import (
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
