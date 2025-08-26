package stbimagego

import (
	"context" // Import the context package
	"os"
	"testing"
)

func BenchmarkLoadBatchConcurrent(b *testing.B) {
	// This benchmark requires a real image file in a `testdata` directory.
	// Example: testdata/4k.jpg
	data, err := os.ReadFile("testdata/4k.jpg")
	if err != nil {
		// Using b.Skip instead of b.Fatal allows tests to run even if the file is missing.
		b.Skipf("Skipping benchmark: failed to read benchmark image 'testdata/4k.jpg': %v", err)
	}

	const N = 100
	datas := make([][]byte, N)
	for i := 0; i < N; i++ {
		datas[i] = data
	}

	// Create a reusable context for the benchmark.
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Pass the context to the function being benchmarked.
		_, _ = LoadBatchConcurrent(ctx, datas)
	}
}
