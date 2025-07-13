package stbimagego

import "testing"

func TestLoadBatchConcurrent(t *testing.T) {
	// Dummy data (use real PNG/JPG bytes for proper testing).
	datas := [][]byte{[]byte("\x89PNG\r\n\x1a\n"), []byte("\x89PNG\r\n\x1a\n")}
	_, err := LoadBatchConcurrent(datas)
	if err == nil {
		t.Error("expected error for invalid image data")
	}
}
