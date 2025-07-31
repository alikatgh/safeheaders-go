package tinyxml2go

import (
	"bytes"
	"errors"
)

// XMLElement mirrors the structure of tinyxml2::XMLElement.
type XMLElement struct {
	Name string
	Text string
	// Children, Parent, Attributes to be added later.
}

// XMLDocument mirrors the tinyxml2::XMLDocument.
type XMLDocument struct {
	Declaration string
	RootElement *XMLElement
}

// trimWhitespace trims leading whitespace from a byte slice.
func trimWhitespace(data []byte) []byte {
	return bytes.TrimLeft(data, " \t\n\r")
}

// Parse is the main entry point. This version is a minimal but real
// implementation that parses the declaration and root element.
func Parse(data []byte) (*XMLDocument, error) {
	doc := &XMLDocument{}
	data = trimWhitespace(data)

	// 1. A real (though simple) piece of parsing: The XML Declaration
	if bytes.HasPrefix(data, []byte("<?xml")) {
		endDecl := bytes.Index(data, []byte("?>"))
		if endDecl == -1 {
			return nil, errors.New("unclosed XML declaration")
		}
		doc.Declaration = string(data[:endDecl+2])
		data = trimWhitespace(data[endDecl+2:])
	}

	// 2. A second real piece of parsing: Find the root element name
	if !bytes.HasPrefix(data, []byte("<")) {
		return nil, errors.New("expected '<' for root element")
	}
	endRootName := bytes.IndexAny(data, " \t\n\r>")
	if endRootName == -1 {
		return nil, errors.New("unclosed root element tag")
	}

	doc.RootElement = &XMLElement{
		Name: string(data[1:endRootName]),
	}

	// The rest of the document is not yet parsed.
	return doc, nil
}
