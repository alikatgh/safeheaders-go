// tinyxml2go/tinyxml2_fuzz_test.go
package tinyxml2go

import (
	"bytes"
	"testing"
)

// FuzzParse tests the Parse function with random XML inputs.
func FuzzParse(f *testing.F) {
	// Seed corpus with various XML inputs
	f.Add([]byte(`<root/>`))
	f.Add([]byte(`<root></root>`))
	f.Add([]byte(`<root><child/></root>`))
	f.Add([]byte(`<root attr="value"/>`))
	f.Add([]byte(`<root><child>text</child></root>`))
	f.Add([]byte(`<root><a><b><c/></b></a></root>`))
	f.Add([]byte(`<?xml version="1.0"?><root/>`))
	f.Add([]byte(`<!-- comment --><root/>`))
	f.Add([]byte(`<root><![CDATA[data]]></root>`))

	// Malformed inputs
	f.Add([]byte(`<root>`))
	f.Add([]byte(`</root>`))
	f.Add([]byte(`<root attr=value/>`))
	f.Add([]byte(`<root><child></root>`))
	f.Add([]byte(`<<root>>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parse panicked on input %q: %v", data, r)
			}
		}()

		doc, err := Parse(data)
		_ = err // We don't care if parsing fails, just that it doesn't crash

		// If parsing succeeded, verify the document structure by traversing
		if err == nil && doc != nil && doc.Root != nil {
			var traverse func(*Node)
			traverse = func(n *Node) {
				_ = n.Name
				_ = n.Text
				for _, child := range n.Children {
					traverse(child)
				}
			}
			traverse(doc.Root)
		}
	})
}

// FuzzParseLarge tests with larger XML inputs.
func FuzzParseLarge(f *testing.F) {
	f.Add([]byte(`<root><item>1</item></root>`))
	f.Add([]byte(`<root a="1" b="2" c="3"/>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Limit size to prevent OOM
		if len(data) > 5*1024*1024 { // 5MB limit
			t.Skip("Input too large")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Large input panicked (len=%d): %v", len(data), r)
			}
		}()

		_, err := Parse(data)
		_ = err
	})
}

// FuzzSpecialChars tests XML with special characters and entities.
func FuzzSpecialChars(f *testing.F) {
	f.Add([]byte(`<root>&lt;&gt;&amp;&quot;&apos;</root>`))
	f.Add([]byte(`<root>Hello, 世界</root>`))
	f.Add([]byte(`<root attr="value with &quot;quotes&quot;"/>`))
	f.Add([]byte(`<root><![CDATA[<>&"']]></root>`))
	f.Add(bytes.Repeat([]byte(`<`), 1000))
	f.Add(bytes.Repeat([]byte(`&`), 1000))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Special chars test panicked: %v", r)
			}
		}()

		_, err := Parse(data)
		_ = err
	})
}

// FuzzAttributes tests parsing of elements with many attributes.
func FuzzAttributes(f *testing.F) {
	f.Add([]byte(`<root a="1"/>`))
	f.Add([]byte(`<root a="1" b="2" c="3"/>`))
	f.Add([]byte(`<root key="value with spaces"/>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Attributes test panicked: %v", r)
			}
		}()

		doc, err := Parse(data)
		if err == nil && doc != nil && doc.Root != nil {
			// Access attributes without crashing
			for key, val := range doc.Root.Attributes {
				_ = key
				_ = val
			}
		}
	})
}

// FuzzDeepNesting tests deeply nested XML structures.
func FuzzDeepNesting(f *testing.F) {
	// Create deeply nested XML
	deep := bytes.Repeat([]byte(`<a>`), 100)
	deep = append(deep, bytes.Repeat([]byte(`</a>`), 100)...)
	f.Add(deep)

	f.Add([]byte(`<a><b><c><d><e/></d></c></b></a>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Deep nesting test panicked: %v", r)
			}
		}()

		_, err := Parse(data)
		_ = err
	})
}
