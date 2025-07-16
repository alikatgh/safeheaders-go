package cjsongo

import "testing"

func BenchmarkUnmarshalParallel(b *testing.B) {
	data := []byte(`{"key1": "value1", "key2": "value2"}`) // Larger in practice.
	for i := 0; i < b.N; i++ {
		UnmarshalParallel(data)
	}
}
