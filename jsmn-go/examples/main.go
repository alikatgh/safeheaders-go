package main

import (
	"bytes"
	"fmt"
	"os"

	jsmngo "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)

	// Basic parsing example
	p := jsmngo.NewParser(10)
	_, err := p.Parse(json)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		os.Exit(1) // Or return if not in main.
	}
	tokens := p.Tokens()
	for _, tok := range tokens {
		fmt.Printf("Token: Type=%v, Start=%d, End=%d, Size=%d\n", tok.Type, tok.Start, tok.End, tok.Size)
	}

	// Parallel mode example
	tokensParallel, err := jsmngo.ParseParallel(json, 1000)
	if err != nil {
		fmt.Println("Error in parallel parsing:", err)
		os.Exit(1)
	}
	fmt.Println("Parallel tokens count:", len(tokensParallel))

	// Streaming example
	reader := bytes.NewReader(json)
	tokensStream, err := jsmngo.ParseStream(reader, 1000)
	if err != nil {
		fmt.Println("Error in streaming parsing:", err)
		os.Exit(1)
	}
	fmt.Println("Stream tokens count:", len(tokensStream))
}
