// jsmngo/jsmn_bench_test.go
package jsmngo

import (
	"os"
	"testing"
)

// loadBenchmarkData is a helper to load the benchmark file.
// It assumes a large JSON file exists at "testdata/bench.json".
func loadBenchmarkData(b *testing.B) []byte {
	data, err := os.ReadFile("testdata/bench.json")
	if err != nil {
		b.Skipf("Skipping benchmark: could not read testdata/bench.json: %v", err)
	}
	return data
}

// BenchmarkParseSingle benchmarks the single-threaded parser.
func BenchmarkParseSingle(b *testing.B) {
	data := loadBenchmarkData(b)
	// Pre-allocate a reasonable number of tokens.
	p := NewParser(len(data) / 4)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p.Parse(data)
	}
}

// BenchmarkParseParallel benchmarks the robust parallel parser.
func BenchmarkParseParallel(b *testing.B) {
	data := loadBenchmarkData(b)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ParseParallel(data)
	}
}
