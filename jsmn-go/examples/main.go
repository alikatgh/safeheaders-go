// Package main demonstrates usage of jsmn-go.
package main

import (
	"bytes"
	"log"

	jsmngo "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)

	// Basic parsing example
	p := jsmngo.NewParser(10)
	_, err := p.Parse(json)
	if err != nil {
		log.Println("Error parsing JSON:", err)
		return
	}
	tokens := p.Tokens()
	for _, tok := range tokens {
		log.Printf("Token: Type=%v, Start=%d, End=%d, Size=%d\n", tok.Type, tok.Start, tok.End, tok.Size)
	}

	// Parallel mode example
	tokensParallel, err := jsmngo.ParseParallel(json, 1000)
	if err != nil {
		log.Println("Error in parallel parsing:", err)
		return
	}
	log.Println("Parallel tokens count:", len(tokensParallel))

	// Streaming example
	reader := bytes.NewReader(json)
	tokensStream, err := jsmngo.ParseStream(reader, 1000)
	if err != nil {
		log.Println("Error in streaming parsing:", err)
		return
	}
	log.Println("Stream tokens count:", len(tokensStream))
}
