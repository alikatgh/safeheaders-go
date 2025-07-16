package cgltfgo

import (
	"bytes"
	"testing"
)

func BenchmarkLoadAssetConcurrent(b *testing.B) {
	// Dummy large glTF data.
	baseData := []byte("glTF" + "\x02\x00\x00\x00" + "\x0c\x00\x00\x00JSON" + "{\"asset\":{\"version\":\"2.0\"}}")
	data := bytes.Repeat(baseData, 10000) // ~300KB dummy.
	for i := 0; i < b.N; i++ {
		LoadAssetConcurrent(data)
	}
}
