package cjsongo

import (
	"bytes"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	data := []byte(`{"key": "value"}`)
	_, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalParallel(t *testing.T) {
	data := []byte(`{"key1": "value1", "key2": "value2", "key3": {"nested": "value"}}`)
	_, err := UnmarshalParallel(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalStream(t *testing.T) {
	data := []byte(`{"key": "value"}`)
	reader := bytes.NewReader(data)
	_, err := UnmarshalStream(reader)
	if err != nil {
		t.Fatal(err)
	}
}
