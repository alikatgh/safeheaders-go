package minizgo

import "testing"

func TestCompressChunksConcurrent(t *testing.T) {
	data := []byte("repeat data repeat data") // Larger in practice.
	_, err := CompressChunksConcurrent(data)
	if err != nil {
		t.Fatal(err)
	}
}
