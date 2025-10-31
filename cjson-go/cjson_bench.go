package cjsongo

import "testing"

func BenchmarkUnmarshalArrayParallel(b *testing.B) {
	data := []byte(`[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		UnmarshalArrayParallel(data)
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	data := []byte(`{"key1": "value1", "key2": "value2"}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]interface{}
		Unmarshal(data, &m)
	}
}
