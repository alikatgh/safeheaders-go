package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alikatgh/safeheaders-go/jsmn-go"
)

const (
	// MaxJSONSize limits input to prevent DoS attacks
	MaxJSONSize = 10 * 1024 * 1024 // 10MB

	// ParseTimeout limits parsing time
	ParseTimeout = 30 * time.Second
)

func main() {
	fmt.Println("SafeHeaders-Go JSON Parser Example")
	fmt.Println("===================================\n")

	// Example 1: Parse small JSON
	smallExample()

	// Example 2: Parse with validation
	validatedExample()

	// Example 3: Parse in parallel
	parallelExample()

	// Example 4: Handle errors gracefully
	errorHandlingExample()

	// Example 5: Production-ready parsing
	productionExample()
}

func smallExample() {
	fmt.Println("Example 1: Basic JSON Parsing")
	fmt.Println("------------------------------")

	json := []byte(`{
		"name": "SafeHeaders-Go",
		"version": "1.0.0",
		"stable": true,
		"modules": ["jsmn-go", "stb-image-go", "tinyxml2-go"]
	}`)

	p := jsmngo.NewParser(100)
	count, err := p.Parse(json)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("✅ Parsed %d tokens successfully\n", count)
	tokens := p.Tokens()
	fmt.Printf("   First token: Type=%v, Range=[%d:%d]\n\n",
		tokens[0].Type, tokens[0].Start, tokens[0].End)
}

func validatedExample() {
	fmt.Println("Example 2: Parse with Input Validation")
	fmt.Println("---------------------------------------")

	json := []byte(`{"status": "ok", "data": [1, 2, 3]}`)

	// Validate input size
	if len(json) > MaxJSONSize {
		log.Fatalf("❌ Input too large: %d bytes (max: %d)", len(json), MaxJSONSize)
	}

	// Basic validation
	if len(json) == 0 {
		log.Fatal("❌ Empty input")
	}

	p := jsmngo.NewParser(1000)
	count, err := p.Parse(json)
	if err != nil {
		log.Fatalf("❌ Parse failed: %v", err)
	}

	fmt.Printf("✅ Validated and parsed %d tokens\n", count)
	fmt.Printf("   Input size: %d bytes\n\n", len(json))
}

func parallelExample() {
	fmt.Println("Example 3: Parallel Parsing")
	fmt.Println("----------------------------")

	// Generate a larger JSON array
	json := []byte(`[
		{"id": 1, "name": "Item 1", "value": 100},
		{"id": 2, "name": "Item 2", "value": 200},
		{"id": 3, "name": "Item 3", "value": 300},
		{"id": 4, "name": "Item 4", "value": 400},
		{"id": 5, "name": "Item 5", "value": 500}
	]`)

	ctx, cancel := context.WithTimeout(context.Background(), ParseTimeout)
	defer cancel()

	start := time.Now()
	tokens, err := jsmngo.ParseParallel(ctx, json)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("❌ Parallel parse failed: %v", err)
	}

	fmt.Printf("✅ Parsed %d tokens in %v (parallel)\n", len(tokens), elapsed)
	fmt.Printf("   Input size: %d bytes\n\n", len(json))
}

func errorHandlingExample() {
	fmt.Println("Example 4: Error Handling")
	fmt.Println("-------------------------")

	malformedInputs := []struct {
		name string
		json string
	}{
		{"Unclosed object", `{"key": "value"`},
		{"Invalid character", `{"key": @}`},
		{"Trailing comma", `{"key": "value",}`},
		{"Empty input", ``},
	}

	for _, test := range malformedInputs {
		p := jsmngo.NewParser(100)
		_, err := p.Parse([]byte(test.json))
		if err != nil {
			fmt.Printf("✅ Correctly rejected: %s\n   Error: %v\n", test.name, err)
		} else {
			fmt.Printf("⚠️  Unexpectedly accepted: %s\n", test.name)
		}
	}
	fmt.Println()
}

func productionExample() {
	fmt.Println("Example 5: Production-Ready Parsing")
	fmt.Println("------------------------------------")

	// Simulate reading from file or API
	jsonData := []byte(`{
		"timestamp": "2025-01-01T00:00:00Z",
		"status": "success",
		"results": [
			{"id": 1, "score": 95.5},
			{"id": 2, "score": 87.2},
			{"id": 3, "score": 92.8}
		],
		"metadata": {
			"count": 3,
			"version": "2.0"
		}
	}`)

	result, err := parseJSON(jsonData)
	if err != nil {
		log.Fatalf("❌ Production parse failed: %v", err)
	}

	fmt.Printf("✅ Production parse successful:\n")
	fmt.Printf("   Tokens: %d\n", result.TokenCount)
	fmt.Printf("   Parse time: %v\n", result.Duration)
	fmt.Printf("   Input size: %d bytes\n", result.InputSize)
	fmt.Printf("   Method: %s\n\n", result.Method)
}

// ParseResult contains parsing statistics
type ParseResult struct {
	TokenCount int
	Duration   time.Duration
	InputSize  int
	Method     string
}

// parseJSON is a production-ready JSON parsing function with:
// - Input validation
// - Size limits
// - Timeout protection
// - Automatic parallel mode for large inputs
func parseJSON(data []byte) (*ParseResult, error) {
	start := time.Now()

	// 1. Validate input
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if len(data) > MaxJSONSize {
		return nil, fmt.Errorf("input too large: %d bytes (max: %d)", len(data), MaxJSONSize)
	}

	// 2. Choose parsing strategy
	const parallelThreshold = 4 * 1024 // 4KB
	var tokenCount int
	var err error
	method := "serial"

	if len(data) >= parallelThreshold {
		// Use parallel parsing for large inputs
		ctx, cancel := context.WithTimeout(context.Background(), ParseTimeout)
		defer cancel()

		tokens, parseErr := jsmngo.ParseParallel(ctx, data)
		tokenCount = len(tokens)
		err = parseErr
		method = "parallel"
	} else {
		// Use serial parsing for small inputs
		p := jsmngo.NewParser(10000)
		tokenCount, err = p.Parse(data)
		method = "serial"
	}

	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	return &ParseResult{
		TokenCount: tokenCount,
		Duration:   time.Since(start),
		InputSize:  len(data),
		Method:     method,
	}, nil
}

// init sets up the example
func init() {
	// In production, configure logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime | log.Lshortfile)
}
