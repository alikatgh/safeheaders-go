// Package main provides an example CLI tool for parsing JSON files using jsmn-go.
package main

import (
	"log"
	"os"

	jsmngo "github.com/alikatgh/safeheaders-go/jsmn-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage: jsmn-go <json_file>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	tokens, err := jsmngo.ParseParallel(data, 1000) // Use parallel for demo.
	if err != nil {
		panic(err)
	}
	for _, tok := range tokens {
		log.Printf("Token: Type=%v, Start=%d, End=%d, Size=%d\n", tok.Type, tok.Start, tok.End, tok.Size)
	}
}
