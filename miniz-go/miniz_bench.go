package minizgo

import (
	"context"
	"testing"
)

func BenchmarkCreateArchive(b *testing.B) {
	files := []FileEntry{
		{Name: "file1.txt", Data: make([]byte, 1<<18)},
		{Name: "file2.txt", Data: make([]byte, 1<<18)},
		{Name: "file3.txt", Data: make([]byte, 1<<18)},
		{Name: "file4.txt", Data: make([]byte, 1<<18)},
	}
	for i := 0; i < b.N; i++ {
		CreateArchive(files)
	}
}

func BenchmarkCreateArchiveConcurrent(b *testing.B) {
	files := []FileEntry{
		{Name: "file1.txt", Data: make([]byte, 1<<18)},
		{Name: "file2.txt", Data: make([]byte, 1<<18)},
		{Name: "file3.txt", Data: make([]byte, 1<<18)},
		{Name: "file4.txt", Data: make([]byte, 1<<18)},
	}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		CreateArchiveConcurrent(ctx, files)
	}
}
