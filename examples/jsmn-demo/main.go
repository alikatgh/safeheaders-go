package main

import (
	"fmt"
	"log"

	"github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	// Example JSON data
	json := []byte(`{
		"name": "SafeHeaders-Go",
		"version": "1.0",
		"features": ["fast", "safe", "concurrent"],
		"stats": {
			"modules": 9,
			"stars": 100
		}
	}`)

	fmt.Println("=== JSON Tokenizer Demo ===\n")
	fmt.Printf("Input JSON:\n%s\n\n", string(json))

	// Parse JSON
	p := jsmngo.NewParser(100)
	n, err := p.Parse(json)
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("✓ Parsed %d tokens\n\n", n)

	// Display tokens
	tokens := p.Tokens()
	for i, tok := range tokens {
		value := string(json[tok.Start:tok.End])
		typeStr := tokenTypeString(tok.Type)
		fmt.Printf("Token %2d: %-10s | %s\n", i, typeStr, value)
	}

	// Parallel parsing demo (for large JSON)
	fmt.Println("\n=== Parallel Parsing Demo ===\n")

	largeJSON := []byte(`[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]`)
	tokens, err = jsmngo.ParseParallel(largeJSON)
	if err != nil {
		log.Fatalf("Parallel parse error: %v", err)
	}

	fmt.Printf("✓ Parallel parsed %d tokens from array\n", len(tokens))
}

func tokenTypeString(t jsmngo.TokenType) string {
	switch t {
	case jsmngo.Object:
		return "Object"
	case jsmngo.Array:
		return "Array"
	case jsmngo.String:
		return "String"
	case jsmngo.Primitive:
		return "Primitive"
	default:
		return "Unknown"
	}
}
