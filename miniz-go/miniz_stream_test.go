package minizgo

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompressDecompressStreamRoundTrip(t *testing.T) {
	original := strings.Repeat("SafeHeaders-Go streaming round-trip. ", 5000)

	var compressed bytes.Buffer
	if err := CompressStream(&compressed, strings.NewReader(original)); err != nil {
		t.Fatalf("CompressStream: %v", err)
	}
	if compressed.Len() >= len(original) {
		t.Errorf("stream did not compress: %d >= %d", compressed.Len(), len(original))
	}

	var out bytes.Buffer
	if err := DecompressStream(&out, &compressed); err != nil {
		t.Fatalf("DecompressStream: %v", err)
	}
	if out.String() != original {
		t.Error("stream round-trip mismatch")
	}
}

func TestDecompressStreamEnforcesLimit(t *testing.T) {
	var compressed bytes.Buffer
	if err := CompressStream(&compressed, bytes.NewReader(make([]byte, 1<<20))); err != nil {
		t.Fatal(err)
	}

	orig := MaxDecompressedSize
	defer func() { MaxDecompressedSize = orig }()
	MaxDecompressedSize = 4096

	if err := DecompressStream(&bytes.Buffer{}, bytes.NewReader(compressed.Bytes())); err == nil {
		t.Error("DecompressStream did not enforce MaxDecompressedSize")
	}
}

func TestStreamNilArgs(t *testing.T) {
	if err := CompressStream(nil, strings.NewReader("x")); err == nil {
		t.Error("CompressStream(nil, ...) should error")
	}
	if err := DecompressStream(&bytes.Buffer{}, nil); err == nil {
		t.Error("DecompressStream(..., nil) should error")
	}
}
