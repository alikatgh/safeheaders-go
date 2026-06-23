package minizgo

import (
	"bytes"
	"testing"
)

// TestMaxDecompressedSize verifies the decompression-bomb guard on both the raw
// DEFLATE path and the ZIP extraction path.
func TestMaxDecompressedSize(t *testing.T) {
	payload := make([]byte, 1<<20) // 1 MiB of zeros -> compresses tiny
	comp, err := CompressData(payload)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := CreateArchive([]FileEntry{{Name: "big.bin", Data: payload}})
	if err != nil {
		t.Fatal(err)
	}

	orig := MaxDecompressedSize
	defer func() { MaxDecompressedSize = orig }()

	// Default limit (256 MiB) allows a 1 MiB payload.
	if out, err := DecompressData(comp); err != nil || !bytes.Equal(out, payload) {
		t.Fatalf("default DecompressData: err=%v len=%d", err, len(out))
	}

	// A limit below the payload size must be enforced on both paths.
	MaxDecompressedSize = 1024
	if _, err := DecompressData(comp); err == nil {
		t.Error("DecompressData did not enforce the limit")
	}
	if _, err := ExtractArchive(archive); err == nil {
		t.Error("ExtractArchive did not enforce the limit")
	}

	// Disabling the guard restores unbounded behavior.
	MaxDecompressedSize = 0
	if _, err := DecompressData(comp); err != nil {
		t.Errorf("disabled guard: %v", err)
	}
}
