package tinyxml2go

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
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

// Parse builds a full DOM tree.
func Parse(data []byte) (*XMLDocument, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	doc := &XMLDocument{}

	// Find the first token to get the XML declaration if it exists.
	// This approach is simpler than the previous one.
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err // Return the original, accurate error from the decoder.
		}

		switch v := tok.(type) {
		case xml.ProcInst:
			if v.Target == "xml" {
				doc.Declaration = fmt.Sprintf("<?%s %s?>", v.Target, string(v.Inst))
			}
		case xml.Comment:
			// Skip
		case xml.StartElement:
			// Once we find the first element, we start parsing the tree.
			root, err := parseElement(dec, v) // Pass the decoder and the first token.
			if err != nil {
				return nil, err
			}
			doc.Root = root
			return doc, nil
		}
	}

	return nil, errors.New("no root element found")
}

// ParseWithConfig builds a full DOM tree while enforcing the limits in config
// (input size, total node count, and nesting depth) to guard against
// denial-of-service from oversized or maliciously nested XML.
//
// If config is nil, DefaultConfig is used. The original Parse function is
// unchanged and applies no limits; use ParseWithConfig for untrusted input.
func ParseWithConfig(data []byte, config *Config) (*XMLDocument, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := config.validateInput(data); err != nil {
		return nil, err
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	doc := &XMLDocument{}
	nodeCount := 0

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch v := tok.(type) {
		case xml.ProcInst:
			if v.Target == "xml" {
				doc.Declaration = fmt.Sprintf("<?%s %s?>", v.Target, string(v.Inst))
			}
		case xml.Comment:
			// Skip
		case xml.StartElement:
			root, err := parseElementLimited(dec, v, config, 1, &nodeCount)
			if err != nil {
				return nil, err
			}
			doc.Root = root
			return doc, nil
		}
	}

	return nil, errors.New("no root element found")
}

// parseElementLimited mirrors parseElement but enforces depth and node-count
// limits. depth is the depth of se (root == 1). nodeCount is shared across the
// whole document and incremented once per element node.
func parseElementLimited(
	dec *xml.Decoder,
	se xml.StartElement,
	config *Config,
	depth int,
	nodeCount *int,
) (*Node, error) {
	if config.MaxNestingDepth > 0 && depth > config.MaxNestingDepth {
		return nil, ErrNestingTooDeep
	}

	*nodeCount++
	if config.MaxNodeCount > 0 && *nodeCount > config.MaxNodeCount {
		return nil, ErrTooManyNodes
	}

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
			child, err := parseElementLimited(dec, v, config, depth+1, nodeCount)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)

		case xml.CharData:
			text := strings.TrimSpace(string(v))
			if text != "" {
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

// The recursive parseElement helper needs only a minor change
// to remove the 'parser' struct dependency.
// func parseElement(dec *xml.Decoder, se xml.StartElement) (*Node, error) { ... }

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

// Find searches for the first child node with the given name.
func (n *Node) Find(name string) *Node {
	if n == nil {
		return nil
	}
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// FindAll searches for all child nodes with the given name.
func (n *Node) FindAll(name string) []*Node {
	if n == nil {
		return nil
	}
	var results []*Node
	for _, child := range n.Children {
		if child.Name == name {
			results = append(results, child)
		}
	}
	return results
}

// FindDeep recursively searches for the first node with the given name in the entire subtree.
func (n *Node) FindDeep(name string) *Node {
	if n == nil {
		return nil
	}
	if n.Name == name {
		return n
	}
	for _, child := range n.Children {
		if found := child.FindDeep(name); found != nil {
			return found
		}
	}
	return nil
}

// FindAllDeep recursively searches for all nodes with the given name in the entire subtree.
func (n *Node) FindAllDeep(name string) []*Node {
	if n == nil {
		return nil
	}
	var results []*Node
	if n.Name == name {
		results = append(results, n)
	}
	for _, child := range n.Children {
		results = append(results, child.FindAllDeep(name)...)
	}
	return results
}

// GetAttribute returns the value of an attribute, or empty string if not found.
func (n *Node) GetAttribute(key string) string {
	if n == nil || n.Attributes == nil {
		return ""
	}
	return n.Attributes[key]
}

// HasAttribute checks if an attribute exists.
func (n *Node) HasAttribute(key string) bool {
	if n == nil || n.Attributes == nil {
		return false
	}
	_, ok := n.Attributes[key]
	return ok
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
	chunk := (len(children) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	results := make([][]string, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start, end := i*chunk, (i+1)*chunk
		if start >= len(children) {
			wg.Done()
			continue
		}
		if end > len(children) {
			end = len(children)
		}
		results[i] = make([]string, 0, end-start)
		go func(slice []*Node, res *[]string) {
			defer wg.Done()
			for _, n := range slice {
				*res = append(*res, n.Name)
			}
		}(children[start:end], &results[i])
	}
	wg.Wait()

	var out []string
	for _, r := range results {
		out = append(out, r...)
	}
	return out, nil
}
