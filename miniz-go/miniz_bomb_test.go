package minizgo

import (
	"bytes"
	"context"
	"fmt"
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

// TestExtractArchiveAggregateBombGuard verifies the cap is on the archive total,
// not per entry: many entries each under the cap must still fail in aggregate
// (audit M5).
func TestExtractArchiveAggregateBombGuard(t *testing.T) {
	entry := make([]byte, 256*1024) // 256 KB of zeros per entry (compresses tiny)
	files := make([]FileEntry, 16)  // 16 * 256KB = 4 MB total
	for i := range files {
		files[i] = FileEntry{Name: fmt.Sprintf("f%d.bin", i), Data: entry}
	}
	archive, err := CreateArchive(files)
	if err != nil {
		t.Fatal(err)
	}

	orig := MaxDecompressedSize
	defer func() { MaxDecompressedSize = orig }()

	// Each entry (256 KB) is under a 1 MiB cap, but the 4 MB total must be rejected.
	MaxDecompressedSize = 1 << 20
	if _, err := ExtractArchive(archive); err == nil {
		t.Error("ExtractArchive did not enforce the aggregate decompression limit")
	}

	// A cap above the total succeeds.
	MaxDecompressedSize = 64 << 20
	if _, err := ExtractArchive(archive); err != nil {
		t.Errorf("ExtractArchive under a sufficient cap: %v", err)
	}
}

// TestMaxBatchSize verifies the aggregate DoS guard on the create path: a
// batch whose file count exceeds MaxBatchSize is rejected before any
// compression work begins, even though every individual file is tiny.
func TestMaxBatchSize(t *testing.T) {
	files := []FileEntry{
		{Name: "a.txt", Data: []byte("a")},
		{Name: "b.txt", Data: []byte("b")},
		{Name: "c.txt", Data: []byte("c")},
	}

	if _, err := CreateArchiveConcurrent(context.Background(), files); err != nil {
		t.Fatalf("CreateArchiveConcurrent under the default limit failed: %v", err)
	}

	orig := MaxBatchSize
	defer func() { MaxBatchSize = orig }()

	MaxBatchSize = 2 // below len(files) == 3
	if _, err := CreateArchiveConcurrent(context.Background(), files); err == nil {
		t.Fatal("expected CreateArchiveConcurrent to reject a 3-file batch under a 2-file limit")
	}

	MaxBatchSize = 0 // disabled
	if _, err := CreateArchiveConcurrent(context.Background(), files); err != nil {
		t.Fatalf("CreateArchiveConcurrent with the guard disabled failed: %v", err)
	}
}
