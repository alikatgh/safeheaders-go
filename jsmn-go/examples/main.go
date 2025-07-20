// Package main demonstrates usage of the jsmngo tokenizer.
package main

import (
	"fmt"
	"log"

	jsmngo "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	// Use a JSON payload large enough to trigger the parallel logic.
	// A single object will demonstrate the fallback to single-threaded parsing.
	json := []byte(`[
		{"id": 1, "action": "login", "user": "alice"},
		{"id": 2, "action": "query", "user": "bob"},
		{"id": 3, "action": "logout", "user": "alice"}
	]`)

	fmt.Println("--- Single-Threaded Parser Example ---")
	// Basic parsing example
	p := jsmngo.NewParser(len(json) / 4) // Allocate a reasonable starting capacity
	_, err := p.Parse(json)
	if err != nil {
		log.Fatalln("Error parsing JSON:", err)
	}
	tokens := p.Tokens()
	log.Printf("Single-threaded parser found %d tokens.\n", len(tokens))
	for i, tok := range tokens {
		log.Printf("Token %d: Type=%v, Start=%d, End=%d, Size=%d, Parent=%d\n", i, tok.Type, tok.Start, tok.End, tok.Size, tok.ParentIdx)
	}

	fmt.Println("\n--- Parallel Parser Example ---")
	// Parallel mode example with the corrected function signature
	tokensParallel, err := jsmngo.ParseParallel(json)
	if err != nil {
		log.Fatalln("Error in parallel parsing:", err)
	}
	log.Printf("Parallel parser found %d tokens.\n", len(tokensParallel))
	// The output should be identical to the single-threaded version.
}
