package minizgo

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestCreateArchive(t *testing.T) {
	tests := []struct {
		name      string
		files     []FileEntry
		wantError bool
	}{
		{
			name: "Single file",
			files: []FileEntry{
				{Name: "test.txt", Data: []byte("hello world")},
			},
			wantError: false,
		},
		{
			name: "Multiple files",
			files: []FileEntry{
				{Name: "file1.txt", Data: []byte("content 1")},
				{Name: "file2.txt", Data: []byte("content 2")},
				{Name: "file3.txt", Data: []byte("content 3")},
			},
			wantError: false,
		},
		{
			name:      "Empty list",
			files:     []FileEntry{},
			wantError: true,
		},
		{
			name: "Empty filename",
			files: []FileEntry{
				{Name: "", Data: []byte("data")},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive, err := CreateArchive(tt.files)
			if (err != nil) != tt.wantError {
				t.Fatalf("CreateArchive() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && len(archive) == 0 {
				t.Fatal("Expected non-empty archive")
			}
		})
	}
}

func TestExtractArchive(t *testing.T) {
	// Create test archive
	files := []FileEntry{
		{Name: "test1.txt", Data: []byte("hello")},
		{Name: "test2.txt", Data: []byte("world")},
	}

	archive, err := CreateArchive(files)
	if err != nil {
		t.Fatalf("Failed to create test archive: %v", err)
	}

	// Extract
	extracted, err := ExtractArchive(archive)
	if err != nil {
		t.Fatalf("ExtractArchive() error = %v", err)
	}

	if len(extracted) != len(files) {
		t.Fatalf("Expected %d files, got %d", len(files), len(extracted))
	}

	// Verify content
	for i, file := range files {
		if extracted[i].Name != file.Name {
			t.Errorf("File %d: expected name %s, got %s", i, file.Name, extracted[i].Name)
		}
		if !bytes.Equal(extracted[i].Data, file.Data) {
			t.Errorf("File %d: data mismatch", i)
		}
	}
}

func TestExtractArchive_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid data", []byte("not a zip file")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractArchive(tt.data)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
		})
	}
}

func TestCreateArchiveConcurrent(t *testing.T) {
	files := []FileEntry{
		{Name: "file1.txt", Data: bytes.Repeat([]byte("a"), 1000)},
		{Name: "file2.txt", Data: bytes.Repeat([]byte("b"), 1000)},
		{Name: "file3.txt", Data: bytes.Repeat([]byte("c"), 1000)},
		{Name: "file4.txt", Data: bytes.Repeat([]byte("d"), 1000)},
	}

	ctx := context.Background()
	archive, err := CreateArchiveConcurrent(ctx, files)
	if err != nil {
		t.Fatalf("CreateArchiveConcurrent() error = %v", err)
	}

	if len(archive) == 0 {
		t.Fatal("Expected non-empty archive")
	}

	// Verify archive can be extracted
	extracted, err := ExtractArchive(archive)
	if err != nil {
		t.Fatalf("Failed to extract concurrent archive: %v", err)
	}

	if len(extracted) != len(files) {
		t.Fatalf("Expected %d files, got %d", len(files), len(extracted))
	}
}

func TestCreateArchiveConcurrent_Context(t *testing.T) {
	files := make([]FileEntry, 100)
	for i := range files {
		files[i] = FileEntry{
			Name: "file.txt",
			Data: bytes.Repeat([]byte("x"), 10000),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Ensure timeout occurs

	_, err := CreateArchiveConcurrent(ctx, files)
	if err == nil {
		t.Fatal("Expected context timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestListArchive(t *testing.T) {
	files := []FileEntry{
		{Name: "file1.txt", Data: []byte("content 1")},
		{Name: "file2.txt", Data: []byte("content 2")},
	}

	archive, err := CreateArchive(files)
	if err != nil {
		t.Fatalf("Failed to create archive: %v", err)
	}

	list, err := ListArchive(archive)
	if err != nil {
		t.Fatalf("ListArchive() error = %v", err)
	}

	if len(list) != len(files) {
		t.Fatalf("Expected %d entries, got %d", len(files), len(list))
	}

	for i, entry := range list {
		if entry.Name != files[i].Name {
			t.Errorf("Entry %d: expected name %s, got %s", i, files[i].Name, entry.Name)
		}
		if entry.Size != int64(len(files[i].Data)) {
			t.Errorf("Entry %d: expected size %d, got %d", i, len(files[i].Data), entry.Size)
		}
	}
}

func TestCompressData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Small data", []byte("hello world")},
		{"Repeated data", bytes.Repeat([]byte("test"), 100)},
		{"Large data", bytes.Repeat([]byte("x"), 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := CompressData(tt.data)
			if err != nil {
				t.Fatalf("CompressData() error = %v", err)
			}

			if len(compressed) == 0 {
				t.Fatal("Expected non-empty compressed data")
			}

			// Verify it can be decompressed
			decompressed, err := DecompressData(compressed)
			if err != nil {
				t.Fatalf("DecompressData() error = %v", err)
			}

			if !bytes.Equal(decompressed, tt.data) {
				t.Error("Decompressed data doesn't match original")
			}
		})
	}
}

func TestCompressData_Empty(t *testing.T) {
	_, err := CompressData([]byte{})
	if err == nil {
		t.Fatal("Expected error for empty data")
	}
}

func TestDecompressData_Invalid(t *testing.T) {
	_, err := DecompressData([]byte("not compressed"))
	if err == nil {
		t.Fatal("Expected error for invalid data")
	}
}

func TestRoundTrip(t *testing.T) {
	// Create archive with various file types
	files := []FileEntry{
		{Name: "text.txt", Data: []byte("Hello, World!")},
		{Name: "binary.bin", Data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
		{Name: "empty.txt", Data: []byte{}},
		{Name: "large.dat", Data: bytes.Repeat([]byte("ABCD"), 2500)},
	}

	// Create
	archive, err := CreateArchive(files)
	if err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}

	// List
	list, err := ListArchive(archive)
	if err != nil {
		t.Fatalf("ListArchive() error = %v", err)
	}

	if len(list) != len(files) {
		t.Fatalf("Expected %d files in list, got %d", len(files), len(list))
	}

	// Extract
	extracted, err := ExtractArchive(archive)
	if err != nil {
		t.Fatalf("ExtractArchive() error = %v", err)
	}

	// Verify
	for i, original := range files {
		if extracted[i].Name != original.Name {
			t.Errorf("File %d: name mismatch", i)
		}
		if !bytes.Equal(extracted[i].Data, original.Data) {
			t.Errorf("File %d: data mismatch", i)
		}
	}
}
