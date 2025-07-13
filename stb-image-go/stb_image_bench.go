package stbimagego

import "testing"

func BenchmarkLoadBatchConcurrent(b *testing.B) {
	datas := make([][]byte, 10) // Dummy; use real images.
	for i := 0; i < len(datas); i++ {
		datas[i] = []byte("\x89PNG\r\n\x1a\n")
	}
	for i := 0; i < b.N; i++ {
		LoadBatchConcurrent(datas)
	}
}
