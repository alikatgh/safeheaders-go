package jsmngo

import (
	"testing"
)

// BenchmarkParse benchmarks the standard JSON parsing performance.
func BenchmarkParse(b *testing.B) {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`) // Or load a large file.
	p := NewParser(10)
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(json)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseParallel benchmarks the parallel JSON parsing performance.
func BenchmarkParseParallel(b *testing.B) {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`) // Use large for real gains.
	for i := 0; i < b.N; i++ {
		_, err := ParseParallel(json, 10)
		if err != nil {
			b.Fatal(err)
		}
	}
}
