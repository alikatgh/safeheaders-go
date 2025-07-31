package tinyxml2go

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	// Ensure bench.xml exists
	if _, err := os.Stat("../bench.xml"); os.IsNotExist(err) {
		genBenchXML()
	}
	data, err := os.ReadFile("../bench.xml")
	if err != nil {
		b.Fatalf("Failed to read bench.xml: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(data)
	}
}

// genBenchXML writes a 1 MB synthetic file.
func genBenchXML() {
	const size = 10_000
	var sb strings.Builder
	sb.WriteString(`<root>`)
	for i := 0; i < size; i++ {
		sb.WriteString(`<item id="` + strconv.Itoa(i) + `">text</item>`)
	}
	sb.WriteString(`</root>`)
	os.WriteFile("../bench.xml", []byte(sb.String()), 0644)
}

func BenchmarkTraverseConcurrent(b *testing.B) {
	data, err := os.ReadFile("../bench.xml")
	if err != nil {
		b.Fatalf("Failed to read bench.xml: %v. Please generate it first.", err)
	}
	root, err := Parse(data)
	if err != nil {
		b.Fatalf("Parse failed: %v", err)
	}
	rootNode := root.Root
	if rootNode == nil {
		b.Fatalf("no root node")
	}
	for i := 0; i < b.N; i++ {
		TraverseConcurrent(rootNode)
	}
}
