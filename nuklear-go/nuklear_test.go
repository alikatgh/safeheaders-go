package nukleargo

import "testing"

func TestRenderConcurrent(t *testing.T) {
	ctx := Init()
	err := RenderConcurrent(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
}
