package stbimagego

import (
	"os"
	"testing"
)

func BenchmarkLoadBatchConcurrent(b *testing.B) {
	// This benchmark requires a real image file in a `testdata` directory.
	// Example: testdata/4k.jpg
	data, err := os.ReadFile("testdata/4k.jpg")
	if err != nil {
		b.Fatalf("Skipping benchmark: failed to read benchmark image 'testdata/4k.jpg': %v", err)
	}

	const N = 100
	datas := make([][]byte, N)
	for i := 0; i < N; i++ {
		datas[i] = data
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadBatchConcurrent(datas)
	}
}
