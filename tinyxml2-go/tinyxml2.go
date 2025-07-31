package tinyxml2go

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
)

// Node represents an XML DOM node.
type Node struct {
	Name       string
	Attributes map[string]string
	Text       string
	Children   []*Node
}

// XMLDocument is the top-level container.
type XMLDocument struct {
	Declaration string
	Root        *Node
}

// Parse builds a complete DOM tree.
func Parse(data []byte) (*XMLDocument, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))

	doc := &XMLDocument{
		Declaration: "",
		Root:        nil,
	}

	// 1. Declaration
	if bytes.HasPrefix(data, []byte("<?xml")) {
		end := bytes.Index(data, []byte("?>"))
		if end == -1 {
			return nil, errors.New("unclosed XML declaration")
		}
		doc.Declaration = strings.TrimSpace(string(data[:end+2]))
		data = data[end+2:]
		dec = xml.NewDecoder(bytes.NewReader(data))
	}

	// 2. Root element
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			root, err := parseElement(dec, se)
			if err != nil {
				return nil, err
			}
			doc.Root = root
			break
		}
	}

	if doc.Root == nil {
		return nil, errors.New("no root element found")
	}
	return doc, nil
}

// parseElement recursively builds the tree.
func parseElement(dec *xml.Decoder, se xml.StartElement) (*Node, error) {
	node := &Node{
		Name:       se.Name.Local,
		Attributes: make(map[string]string),
	}

	for _, a := range se.Attr {
		node.Attributes[a.Name.Local] = a.Value
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, errors.New("unexpected EOF")
		}
		if err != nil {
			return nil, err
		}

		switch v := tok.(type) {
		case xml.StartElement:
			child, err := parseElement(dec, v)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)

		case xml.CharData:
			text := strings.TrimSpace(string(v))
			if text != "" {
				// Append to existing text if already present
				if node.Text == "" {
					node.Text = text
				} else {
					node.Text += text
				}
			}

		case xml.EndElement:
			if v.Name.Local == se.Name.Local {
				return node, nil
			}
		}
	}
}

// TraverseConcurrent walks direct children in parallel.
func TraverseConcurrent(root *Node) ([]string, error) {
	if root == nil || len(root.Children) == 0 {
		return nil, nil
	}
	children := root.Children
	numWorkers := runtime.NumCPU()
	if len(children) < numWorkers {
		numWorkers = len(children)
	}

	chunkSize := (len(children) + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	results := make([][]string, numWorkers)
	errs := make(chan error, 1)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(children) {
			wg.Done()
			continue
		}
		if end > len(children) {
			end = len(children)
		}
		results[i] = make([]string, 0, end-start)
		go func(children []*Node, res *[]string) {
			defer wg.Done()
			for _, c := range children {
				*res = append(*res, c.Name)
			}
		}(children[start:end], &results[i])
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}

	// Flatten
	var out []string
	for _, r := range results {
		out = append(out, r...)
	}
	return out, nil
}
