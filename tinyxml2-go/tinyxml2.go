package tinyxml2go

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"runtime"
	"sync"
)

// Node represents a node in the XML DOM tree.
type Node struct {
	Name       string
	Attributes map[string]string
	Text       string
	Children   []Node
}

// Parse builds a DOM tree from XML data using a recursive helper.
func Parse(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// Skip to the first real token, ignoring declarations etc.
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if _, ok := t.(xml.StartElement); ok {
			// Found the root, start parsing from here.
			// We need to construct a "fake" StartElement token to re-feed the parser.
			return parseElement(dec, t.(xml.StartElement))
		}
	}
	return nil, errors.New("no root element found in XML")
}

// parseElement recursively parses tokens into a Node tree.
func parseElement(dec *xml.Decoder, se xml.StartElement) (*Node, error) {
	node := &Node{Name: se.Name.Local, Attributes: make(map[string]string)}
	for _, attr := range se.Attr {
		node.Attributes[attr.Name.Local] = attr.Value
	}

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break // Should be handled by matching EndElement
		}
		if err != nil {
			return nil, err
		}

		switch v := t.(type) {
		case xml.StartElement:
			child, err := parseElement(dec, v)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, *child)
		case xml.CharData:
			node.Text = string(v)
		case xml.EndElement:
			// When we find the end tag matching our start tag, we're done.
			if v.Name.Local == se.Name.Local {
				return node, nil
			}
		}
	}
	return nil, errors.New("unexpected EOF - unclosed tag " + se.Name.Local)
}

// TraverseConcurrent traverses the direct children of a node concurrently.
func TraverseConcurrent(root *Node) ([]string, error) {
	if root == nil || len(root.Children) == 0 {
		return []string{}, nil
	}
	numWorkers := runtime.NumCPU()
	if len(root.Children) < numWorkers {
		numWorkers = len(root.Children)
	}

	var wg sync.WaitGroup
	results := make([]string, 0, len(root.Children))
	mu := sync.Mutex{}
	errs := make(chan error, numWorkers)

	chunkSize := (len(root.Children) + numWorkers - 1) / numWorkers // Ceiling division
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(root.Children) {
			wg.Done()
			continue
		}
		if end > len(root.Children) {
			end = len(root.Children)
		}
		go func(children []Node) {
			defer wg.Done()
			localResults := make([]string, 0, len(children))
			for _, child := range children {
				// In a real scenario, you would do more work here.
				localResults = append(localResults, child.Name)
			}
			mu.Lock()
			results = append(results, localResults...)
			mu.Unlock()
		}(root.Children[start:end])
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	return results, nil
}
