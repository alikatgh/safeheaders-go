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

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{"Empty", ""},
		{"No root", "<?xml version='1.0'?>"},
		{"Unclosed tag", "<root><child></root>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.xml))
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestFind(t *testing.T) {
	xml := `<root><item id="1"/><item id="2"/><other/></root>`
	doc, _ := Parse([]byte(xml))

	item := doc.Root.Find("item")
	if item == nil {
		t.Fatal("Find() returned nil")
	}
	if item.GetAttribute("id") != "1" {
		t.Error("Found wrong item")
	}

	missing := doc.Root.Find("missing")
	if missing != nil {
		t.Error("Find() should return nil for missing nodes")
	}
}

func TestFindAll(t *testing.T) {
	xml := `<root><item id="1"/><item id="2"/><item id="3"/><other/></root>`
	doc, _ := Parse([]byte(xml))

	items := doc.Root.FindAll("item")
	if len(items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(items))
	}

	missing := doc.Root.FindAll("missing")
	if len(missing) != 0 {
		t.Error("FindAll() should return empty slice for missing nodes")
	}
}

func TestFindDeep(t *testing.T) {
	xml := `<root><a><b><target id="1"/></b></a><c><target id="2"/></c></root>`
	doc, _ := Parse([]byte(xml))

	target := doc.Root.FindDeep("target")
	if target == nil {
		t.Fatal("FindDeep() returned nil")
	}
	if target.GetAttribute("id") != "1" {
		t.Error("Found wrong target")
	}
}

func TestFindAllDeep(t *testing.T) {
	xml := `<root><a><item/><b><item/></b></a><item/><c><item/></c></root>`
	doc, _ := Parse([]byte(xml))

	items := doc.Root.FindAllDeep("item")
	if len(items) != 4 {
		t.Fatalf("Expected 4 items, got %d", len(items))
	}
}

func TestGetAttribute(t *testing.T) {
	xml := `<root name="test" value="123"/>`
	doc, _ := Parse([]byte(xml))

	if doc.Root.GetAttribute("name") != "test" {
		t.Error("GetAttribute() returned wrong value")
	}
	if doc.Root.GetAttribute("value") != "123" {
		t.Error("GetAttribute() returned wrong value")
	}
	if doc.Root.GetAttribute("missing") != "" {
		t.Error("GetAttribute() should return empty string for missing attributes")
	}
}

func TestHasAttribute(t *testing.T) {
	xml := `<root name="test"/>`
	doc, _ := Parse([]byte(xml))

	if !doc.Root.HasAttribute("name") {
		t.Error("HasAttribute() should return true for existing attribute")
	}
	if doc.Root.HasAttribute("missing") {
		t.Error("HasAttribute() should return false for missing attribute")
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

func TestComplexXML(t *testing.T) {
	xml := `<?xml version="1.0"?>
<library>
	<book id="1" genre="fiction">
		<title>The Great Gatsby</title>
		<author>F. Scott Fitzgerald</author>
	</book>
	<book id="2" genre="science">
		<title>A Brief History of Time</title>
		<author>Stephen Hawking</author>
	</book>
</library>`

	doc, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	books := doc.Root.FindAll("book")
	if len(books) != 2 {
		t.Fatalf("Expected 2 books, got %d", len(books))
	}

	if books[0].GetAttribute("id") != "1" {
		t.Error("Wrong book ID")
	}

	title := books[0].Find("title")
	if title == nil || title.Text != "The Great Gatsby" {
		t.Error("Wrong title")
	}

	allTitles := doc.Root.FindAllDeep("title")
	if len(allTitles) != 2 {
		t.Fatalf("Expected 2 titles, got %d", len(allTitles))
	}
}
