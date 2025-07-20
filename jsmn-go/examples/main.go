// Package main demonstrates usage of the jsmngo tokenizer.
package main

import (
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

	log.Println("--- Single-Threaded Parser Example ---")
	p := jsmngo.NewParser(len(json) / 4)
	_, err := p.Parse(json)
	if err != nil {
		log.Fatalln("Error parsing JSON:", err)
	}
	tokens := p.Tokens()
	log.Printf("Single-threaded parser found %d tokens.\n", len(tokens))
	for i, tok := range tokens {
		// This line has been shortened to pass the linter.
		log.Printf("Token %d: Type=%v, Start=%d, End=%d, Size=%d, Parent=%d\n",
			i, tok.Type, tok.Start, tok.End, tok.Size, tok.ParentIdx)
	}

	log.Println("\n--- Parallel Parser Example ---")
	tokensParallel, err := jsmngo.ParseParallel(json)
	if err != nil {
		log.Fatalln("Error in parallel parsing:", err)
	}
	log.Printf("Parallel parser found %d tokens.\n", len(tokensParallel))
}
