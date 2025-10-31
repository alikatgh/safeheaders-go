package cgltfgo

import (
	"context"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	data := []byte(`{
		"asset": {"version": "2.0"},
		"meshes": [{"primitives": [{"attributes": {"POSITION": 0}}]}],
		"accessors": [{"bufferView": 0, "componentType": 5126, "count": 100, "type": "VEC3"}]
	}`)
	for i := 0; i < b.N; i++ {
		Parse(data)
	}
}

func BenchmarkParseBatch(b *testing.B) {
	dataList := make([][]byte, 100)
	testData := []byte(`{
		"asset": {"version": "2.0"},
		"meshes": [{"primitives": [{"attributes": {"POSITION": 0}}]}]
	}`)
	for i := range dataList {
		dataList[i] = testData
	}
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		ParseBatch(ctx, dataList)
	}
}
