// Package jsmngo provides benchmarks for the JSON tokenizer.
package jsmngo

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// loadBenchmarkData returns the benchmark payload. It prefers a committed
// fixture at testdata/bench.json if present, otherwise it generates a
// representative payload in-memory so the benchmark always runs.
func loadBenchmarkData(b *testing.B) []byte {
	b.Helper()
	if data, err := os.ReadFile("testdata/bench.json"); err == nil {
		return data
	}
	return generateBenchJSON(20000) // ~1MB
}

// generateBenchJSON builds n comma-separated top-level objects. The depth-0
// commas become split points, so this payload exercises the chunked parallel
// tokenizer rather than collapsing to the serial fallback.
func generateBenchJSON(n int) []byte {
	var b strings.Builder
	b.Grow(n * 56)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":%d,"name":"item-%d","active":%t,"score":%d}`, i, i, i%2 == 0, i*7)
	}
	return []byte(b.String())
}

// BenchmarkParseSingle benchmarks the single-threaded parser.
func BenchmarkParseSingle(b *testing.B) {
	data := loadBenchmarkData(b)
	p := NewParser(len(data) / 4)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := p.Parse(data); err != nil {
			b.Fatal(err) // Check for errors.
		}
	}
}

// BenchmarkParseParallel benchmarks the robust parallel parser.
func BenchmarkParseParallel(b *testing.B) {
	data := loadBenchmarkData(b)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := ParseParallel(data); err != nil {
			b.Fatal(err) // Check for errors.
		}
	}
}
