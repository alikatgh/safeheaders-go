package cgltfgo

import (
	"bytes"
	"testing"
)

func TestLoadGLTF(t *testing.T) {
	// Dummy glTF header + data.
	data := []byte("glTF" + "\x02\x00\x00\x00" + "\x0c\x00\x00\x00JSON" + "{\"asset\":{\"version\":\"2.0\"}}")
	_, err := LoadGLTF(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadAssetConcurrent(t *testing.T) {
	// Larger dummy glTF (repeat for chunks).
	baseData := []byte("glTF" + "\x02\x00\x00\x00" + "\x0c\x00\x00\x00JSON" + "{\"asset\":{\"version\":\"2.0\"}}")
	data := bytes.Repeat(baseData, 1000) // ~30KB for multi-chunk.
	_, err := LoadAssetConcurrent(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadStream(t *testing.T) {
	data := []byte("glTF" + "\x02\x00\x00\x00" + "\x0c\x00\x00\x00JSON" + "{\"asset\":{\"version\":\"2.0\"}}")
	reader := bytes.NewReader(data)
	_, err := LoadStream(reader)
	if err != nil {
		t.Fatal(err)
	}
}
