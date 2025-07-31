package tinyxml2go

import "testing"

func TestParse(t *testing.T) {
	data := []byte(`<root><child>value</child></root>`)
	_, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTraverseConcurrent(t *testing.T) {
	root := &Node{Name: "root"}
	_, err := TraverseConcurrent(root)
	if err != nil {
		t.Fatal(err)
	}
}
