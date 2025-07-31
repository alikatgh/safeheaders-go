package nukleargo

import "testing"

func BenchmarkRenderConcurrent(b *testing.B) {
	ctx := Init()
	for i := 0; i < b.N; i++ {
		RenderConcurrent(ctx, 100)
	}
}
