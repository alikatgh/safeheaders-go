package stbimagego

import (
	"context" // Import the context package
	"os"
	"testing"
)

func BenchmarkLoadBatchConcurrent(b *testing.B) {
	// Prefer a real image at testdata/4k.jpg if the user supplies one; otherwise
	// generate a synthetic JPEG in-memory so the benchmark always runs.
	data, err := os.ReadFile("testdata/4k.jpg")
	if err != nil {
		data = createTestJPEG(512, 512)
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
