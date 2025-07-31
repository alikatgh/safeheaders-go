package tinyxml2go

import "testing"

func TestParse(t *testing.T) {
	xml := `<?xml version="1.0"?><root attr="val"><child>text</child></root>`
	doc, err := Parse([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root.Name != "root" {
		t.Fatalf("expected root, got %s", doc.Root.Name)
	}
	if doc.Root.Attributes["attr"] != "val" {
		t.Fatalf("attr mismatch")
	}
	if len(doc.Root.Children) != 1 || doc.Root.Children[0].Text != "text" {
		t.Fatalf("child/text mismatch")
	}
}

func TestTraverseConcurrent(t *testing.T) {
	xml := `<root><a/><b/><c/></root>`
	doc, _ := Parse([]byte(xml))
	names, _ := TraverseConcurrent(doc.Root)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
}
