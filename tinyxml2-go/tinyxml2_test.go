package tinyxml2go

import "testing"

func TestParse(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><root attr="value"><child1>text1</child1><child2/></root>`)
	root, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if root.Name != "root" {
		t.Errorf("Expected root name 'root', got '%s'", root.Name)
	}
	if val, ok := root.Attributes["attr"]; !ok || val != "value" {
		t.Errorf("Expected attribute 'attr=value', got '%s'", val)
	}
	if len(root.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(root.Children))
	}
	if root.Children[0].Name != "child1" {
		t.Errorf("Expected child1, got %s", root.Children[0].Name)
	}
	if root.Children[0].Text != "text1" {
		t.Errorf("Expected text 'text1', got '%s'", root.Children[0].Text)
	}
}

func TestTraverseConcurrent(t *testing.T) {
	root := &Node{
		Name: "root",
		Children: []Node{
			{Name: "child1"},
			{Name: "child2"},
			{Name: "child3"},
			{Name: "child4"},
		},
	}
	names, err := TraverseConcurrent(root)
	if err != nil {
		t.Fatalf("TraverseConcurrent() failed: %v", err)
	}
	if len(names) != 4 {
		t.Errorf("Expected 4 child names, got %d", len(names))
	}
}
