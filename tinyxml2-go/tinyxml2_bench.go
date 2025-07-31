package tinyxml2go

import "testing"

func BenchmarkTraverseConcurrent(b *testing.B) {
	root := &Node{Name: "root"}
	for i := 0; i < b.N; i++ {
		TraverseConcurrent(root)
	}
}
