package tinyxml2go

import (
	"os"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	data, err := os.ReadFile("../bench.xml")
	if err != nil {
		b.Fatalf("Failed to read bench.xml: %v. Please generate it first.", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(data)
	}
}

func BenchmarkTraverseConcurrent(b *testing.B) {
	data, err := os.ReadFile("../bench.xml")
	if err != nil {
		b.Fatalf("Failed to read bench.xml: %v. Please generate it first.", err)
	}
	root, err := Parse(data)
	if err != nil {
		b.Fatalf("Parse failed during benchmark setup: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TraverseConcurrent(root)
	}
}
