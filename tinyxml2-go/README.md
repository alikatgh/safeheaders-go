# tinyxml2-go

Fast XML DOM parser with XPath-like query methods and concurrent traversal support.

## Status

🟢 **Stable** - Production ready

## Features

- Full DOM tree parsing
- XPath-like query methods (Find, FindAll, FindDeep, FindAllDeep)
- Attribute access and manipulation
- Concurrent tree traversal
- XML declaration support
- Zero external dependencies (uses Go stdlib only)
- Memory safe

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/tinyxml2-go
```

## Quick Start

### Parse XML

```go
package main

import (
    "fmt"
    "github.com/alikatgh/safeheaders-go/tinyxml2-go"
)

func main() {
    xml := []byte(`<?xml version="1.0"?>
    <root>
        <person id="1">
            <name>Alice</name>
            <age>30</age>
        </person>
        <person id="2">
            <name>Bob</name>
            <age>25</age>
        </person>
    </root>`)

    doc, err := tinyxml2go.Parse(xml)
    if err != nil {
        panic(err)
    }

    fmt.Println("Root:", doc.Root.Name)
    fmt.Println("Declaration:", doc.Declaration)
}
```

### Query Nodes

```go
doc, _ := tinyxml2go.Parse(xmlData)

// Find first child with name "person"
person := doc.Root.Find("person")
if person != nil {
    fmt.Println("First person ID:", person.GetAttribute("id"))

    name := person.Find("name")
    if name != nil {
        fmt.Println("Name:", name.Text)
    }
}

// Find all children with name "person"
allPeople := doc.Root.FindAll("person")
fmt.Printf("Found %d people\n", len(allPeople))

for _, p := range allPeople {
    fmt.Printf("Person %s: %s\n",
        p.GetAttribute("id"),
        p.Find("name").Text)
}
```

### Deep Search

Search recursively through entire subtree:

```go
doc, _ := tinyxml2go.Parse(xmlData)

// Find first "name" anywhere in tree
name := doc.Root.FindDeep("name")
if name != nil {
    fmt.Println("Found name:", name.Text)
}

// Find all "name" nodes anywhere in tree
allNames := doc.Root.FindAllDeep("name")
for _, n := range allNames {
    fmt.Println("Name:", n.Text)
}
```

### Attributes

```go
node := doc.Root.Find("person")

// Get attribute value
id := node.GetAttribute("id")
fmt.Println("ID:", id)

// Check if attribute exists
if node.HasAttribute("email") {
    email := node.GetAttribute("email")
    fmt.Println("Email:", email)
} else {
    fmt.Println("No email attribute")
}

// List all attributes
for key, value := range node.Attributes {
    fmt.Printf("%s = %s\n", key, value)
}
```

## Concurrent Traversal

Process child nodes in parallel:

```go
doc, _ := tinyxml2go.Parse(xmlData)

// Traverse all direct children concurrently
results, err := tinyxml2go.TraverseConcurrent(doc.Root)
if err != nil {
    panic(err)
}

for _, name := range results {
    fmt.Println("Child node:", name)
}
```

## API Reference

### Types

```go
type Node struct {
    Name       string              // Element name
    Attributes map[string]string   // Element attributes
    Text       string              // Text content
    Children   []*Node             // Child nodes
}

type XMLDocument struct {
    Declaration string  // XML declaration
    Root        *Node   // Root element
}
```

### Functions

```go
// Parse parses XML data and builds DOM tree
func Parse(data []byte) (*XMLDocument, error)

// TraverseConcurrent processes children in parallel
func TraverseConcurrent(root *Node) ([]string, error)
```

### Node Methods

```go
// Find returns first child with given name
func (n *Node) Find(name string) *Node

// FindAll returns all children with given name
func (n *Node) FindAll(name string) []*Node

// FindDeep recursively searches for first node with name
func (n *Node) FindDeep(name string) *Node

// FindAllDeep recursively searches for all nodes with name
func (n *Node) FindAllDeep(name string) []*Node

// GetAttribute returns attribute value or empty string
func (n *Node) GetAttribute(key string) string

// HasAttribute checks if attribute exists
func (n *Node) HasAttribute(key string) bool
```

## Testing

```bash
cd tinyxml2-go
go test -v
```

## License

MIT - See [LICENSE](../LICENSE)

Based on [TinyXML-2](https://github.com/leethomason/tinyxml2) by Lee Thomason (zlib License)