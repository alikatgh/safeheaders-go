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
	fmt.Println("Example 2: Parse with Input Validation (NEW API)")
	fmt.Println("------------------------------------------------")

	json := []byte(`{"status": "ok", "data": [1, 2, 3]}`)

	// Use the new Config API with strict limits
	ctx, cancel := context.WithTimeout(context.Background(), ParseTimeout)
	defer cancel()

	config := jsmngo.StrictConfig() // 10MB limit, 100K tokens
	tokens, err := jsmngo.ParseWithConfig(ctx, json, config)
	if err != nil {
		log.Fatalf("❌ Parse failed: %v", err)
	}

	fmt.Printf("✅ Validated and parsed %d tokens\n", len(tokens))
	fmt.Printf("   Input size: %d bytes\n", len(json))
	fmt.Printf("   Config: MaxInputSize=%d, MaxTokens=%d\n\n",
		config.MaxInputSize, config.MaxTokens)
}

func parallelExample() {
	fmt.Println("Example 3: Parallel Parsing with Context (NEW API)")
	fmt.Println("-------------------------------------------------")

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
	// Use new context-aware API
	tokens, err := jsmngo.ParseParallelWithContext(ctx, json)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("❌ Parallel parse failed: %v", err)
	}

	fmt.Printf("✅ Parsed %d tokens in %v (parallel, context-aware)\n", len(tokens), elapsed)
	fmt.Printf("   Input size: %d bytes\n", len(json))
	fmt.Printf("   Context: Timeout protection enabled\n\n")
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
// - Input validation with configurable limits
// - Size and token limits
// - Timeout protection
// - Automatic parallel mode for large inputs
// - Context cancellation support
func parseJSON(data []byte) (*ParseResult, error) {
	start := time.Now()

	// Use the new Config API for production safety
	config := jsmngo.DefaultConfig()
	config.MaxInputSize = MaxJSONSize
	config.ParallelThreshold = 4 * 1024 // 4KB

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ParseTimeout)
	defer cancel()

	// Parse with validation and automatic strategy selection
	tokens, err := jsmngo.ParseWithConfig(ctx, data, config)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	// Determine which method was used
	method := "serial"
	if len(data) >= config.ParallelThreshold {
		method = "parallel (context-aware)"
	}

	return &ParseResult{
		TokenCount: len(tokens),
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
