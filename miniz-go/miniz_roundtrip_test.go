package minizgo

import (
	"bytes"
	"context"
	"testing"
)

// TestConcurrentArchiveRoundTrip guards against the double-compression
// regression: CreateArchiveConcurrent must produce an archive whose extracted
// contents are byte-for-byte identical to the inputs.
func TestConcurrentArchiveRoundTrip(t *testing.T) {
	files := []FileEntry{
		{Name: "short.txt", Data: []byte("hello world")},
		{Name: "repetitive.bin", Data: bytes.Repeat([]byte("ABCD"), 1024)},
		{Name: "empty.txt", Data: []byte{}},
		{Name: "binary.dat", Data: []byte{0, 1, 2, 3, 255, 254, 0, 0, 42}},
	}

	archive, err := CreateArchiveConcurrent(context.Background(), files)
	if err != nil {
		t.Fatalf("CreateArchiveConcurrent: %v", err)
	}

	extracted, err := ExtractArchive(archive)
	if err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}
	if len(extracted) != len(files) {
		t.Fatalf("extracted %d files, want %d", len(extracted), len(files))
	}

	for i, got := range extracted {
		if got.Name != files[i].Name {
			t.Errorf("file %d name = %q, want %q", i, got.Name, files[i].Name)
		}
		if !bytes.Equal(got.Data, files[i].Data) {
			t.Errorf("file %q did not round-trip: got %d bytes, want %d bytes",
				got.Name, len(got.Data), len(files[i].Data))
		}
	}

	// The listing must report true uncompressed sizes.
	listed, err := ListArchive(archive)
	if err != nil {
		t.Fatalf("ListArchive: %v", err)
	}
	for i, l := range listed {
		if l.Size != int64(len(files[i].Data)) {
			t.Errorf("listed size for %q = %d, want %d", l.Name, l.Size, len(files[i].Data))
		}
	}
}

func TestConcurrentArchiveContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	files := make([]FileEntry, 64)
	for i := range files {
		files[i] = FileEntry{Name: "f", Data: bytes.Repeat([]byte("x"), 256)}
	}
	if _, err := CreateArchiveConcurrent(ctx, files); err == nil {
		t.Fatal("expected an error from a canceled context, got nil")
	}
}
