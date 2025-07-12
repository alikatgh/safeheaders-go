package jsmngo

import (
	"bytes"
	"testing"
)

func TestParse(t *testing.T) {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)
	p := NewParser(10)
	n, err := p.Parse(json)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 { // Corrected: Root object + "key" + "value" + "arr" + array + 3 primitives.
		t.Errorf("expected 8 tokens, got %d", n)
	}
}

func TestParseParallel(t *testing.T) {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)
	tokens, err := ParseParallel(json, 10)
	if err != nil {
		t.Fatal(err) // Now falls back to single, so no error.
	}
	if len(tokens) != 8 {
		t.Errorf("expected 8 tokens, got %d", len(tokens))
	}
}

func TestParseStream(t *testing.T) {
	json := []byte(`{"key": "value"}`)
	reader := bytes.NewReader(json)
	tokens, err := ParseStream(reader, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(tokens))
	}
}
