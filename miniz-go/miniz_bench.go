package minizgo

import "testing"

func BenchmarkCompressChunksConcurrent(b *testing.B) {
	data := make([]byte, 1<<20) // 1MB dummy.
	for i := 0; i < b.N; i++ {
		CompressChunksConcurrent(data)
	}
}
