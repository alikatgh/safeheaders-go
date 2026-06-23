package minizgo

import (
	"bytes"
	"testing"
)

// FuzzExtract ensures the ZIP/DEFLATE readers survive arbitrary input without
// panicking, and that Compress/Decompress round-trips.
func FuzzExtract(f *testing.F) {
	arc, _ := CreateArchive([]FileEntry{{Name: "a.txt", Data: []byte("hello")}})
	f.Add(arc)
	comp, _ := CompressData([]byte("the quick brown fox"))
	f.Add(comp)
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// None of these may panic on arbitrary bytes.
		_, _ = ExtractArchive(data)
		_, _ = ListArchive(data)

		if dec, err := DecompressData(data); err == nil {
			// Round-trip: recompressing then decompressing must reproduce it.
			if rc, err := CompressData(dec); err == nil {
				if rt, err := DecompressData(rc); err == nil && !bytes.Equal(rt, dec) {
					t.Fatalf("decompress/recompress not stable")
				}
			}
		}
	})
}
