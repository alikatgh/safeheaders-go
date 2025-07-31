package tinyxml2go

import (
	"bytes"
	"encoding/xml"
	"io"
	"sync"
)

// Node represents an XML node (stub for tree).
type Node struct {
	Name     string
	Children []Node
}

// Parse parses XML to tree (basic).
func Parse(data []byte) (*Node, error) {
	var root Node
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if se, ok := t.(xml.StartElement); ok {
			root.Name = se.Name.Local // Stub; build tree.
		}
	}
	return &root, nil
}

// TraverseConcurrent traverses XML tree concurrently (for large nodes).
func TraverseConcurrent(root *Node) ([]string, error) {
	numWorkers := 4
	var wg sync.WaitGroup
	results := make([]string, 0)
	mu := sync.Mutex{}
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Stub: Traverse subtree (real would recurse children).
			mu.Lock()
			results = append(results, root.Name)
			mu.Unlock()
		}()
	}
	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	return results, nil
}
