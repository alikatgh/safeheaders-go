// Package jsmngo provides a fast JSON tokenizer with parallel processing capabilities.
package jsmngo

import (
	"context"
	"errors"
)

// Common errors
var (
	// ErrInputTooLarge is returned when input exceeds MaxInputSize
	ErrInputTooLarge = errors.New("input size exceeds maximum allowed")

	// ErrTooManyTokens is returned when token count exceeds MaxTokens
	ErrTooManyTokens = errors.New("token count exceeds maximum allowed")

	// ErrEmptyInput is returned when input is empty
	ErrEmptyInput = errors.New("empty input")
)

// Config holds parsing configuration and limits.
type Config struct {
	// MaxInputSize limits the maximum JSON input size in bytes.
	// Default: 100MB. Set to 0 for unlimited (not recommended).
	MaxInputSize int

	// MaxTokens limits the maximum number of tokens that can be parsed.
	// Default: 1,000,000. Set to 0 for unlimited (not recommended).
	MaxTokens int

	// InitialTokenCapacity is the initial capacity for the token slice.
	// Default: inputSize / 4. The slice will grow automatically if needed.
	InitialTokenCapacity int

	// ParallelThreshold is the minimum input size (in bytes) to enable parallel parsing.
	// Default: 4KB. Parallel parsing has overhead, so small inputs are faster serially.
	ParallelThreshold int
}

// DefaultConfig returns the default configuration with sensible limits.
func DefaultConfig() *Config {
	return &Config{
		MaxInputSize:         100 * 1024 * 1024, // 100MB
		MaxTokens:            1_000_000,         // 1 million tokens
		InitialTokenCapacity: 0,                 // Auto-calculated
		ParallelThreshold:    4 * 1024,          // 4KB
	}
}

// StrictConfig returns a stricter configuration suitable for untrusted input.
func StrictConfig() *Config {
	return &Config{
		MaxInputSize:         10 * 1024 * 1024, // 10MB
		MaxTokens:            100_000,          // 100k tokens
		InitialTokenCapacity: 0,                // Auto-calculated
		ParallelThreshold:    4 * 1024,         // 4KB
	}
}

// UnlimitedConfig returns a configuration with no limits (use with caution).
func UnlimitedConfig() *Config {
	return &Config{
		MaxInputSize:         0,        // Unlimited
		MaxTokens:            0,        // Unlimited
		InitialTokenCapacity: 0,        // Auto-calculated
		ParallelThreshold:    4 * 1024, // 4KB
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.MaxInputSize < 0 {
		return errors.New("MaxInputSize cannot be negative")
	}
	if c.MaxTokens < 0 {
		return errors.New("MaxTokens cannot be negative")
	}
	if c.InitialTokenCapacity < 0 {
		return errors.New("InitialTokenCapacity cannot be negative")
	}
	if c.ParallelThreshold < 0 {
		return errors.New("ParallelThreshold cannot be negative")
	}
	return nil
}

// validateInput checks if the input meets the configuration constraints.
func (c *Config) validateInput(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyInput
	}

	if c.MaxInputSize > 0 && len(data) > c.MaxInputSize {
		return ErrInputTooLarge
	}

	return nil
}

// getInitialCapacity returns the initial token capacity based on input size.
func (c *Config) getInitialCapacity(inputSize int) int {
	if c.InitialTokenCapacity > 0 {
		return c.InitialTokenCapacity
	}

	// Heuristic: JSON typically has ~1 token per 4 bytes
	capacity := inputSize / 4
	if capacity < 10 {
		capacity = 10
	}
	if c.MaxTokens > 0 && capacity > c.MaxTokens {
		capacity = c.MaxTokens
	}
	return capacity
}

// shouldUseParallel determines if parallel parsing should be used.
func (c *Config) shouldUseParallel(inputSize int) bool {
	return inputSize >= c.ParallelThreshold
}

// ParserWithConfig creates a new parser with the given configuration.
type ParserWithConfig struct {
	*Parser
	config *Config
}

// NewParserWithConfig creates a new parser with the given configuration.
func NewParserWithConfig(config *Config) (*ParserWithConfig, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &ParserWithConfig{
		Parser: &Parser{
			tokens: make([]Token, 0, 100), // Will grow as needed
		},
		config: config,
	}, nil
}

// Parse tokenizes the JSON input with validation.
func (p *ParserWithConfig) Parse(json []byte) (int, error) {
	if err := p.config.validateInput(json); err != nil {
		return 0, err
	}

	return p.Parser.Parse(json)
}

// ParseWithConfig parses JSON with the given configuration.
func ParseWithConfig(ctx context.Context, data []byte, config *Config) ([]Token, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := config.validateInput(data); err != nil {
		return nil, err
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Choose parsing strategy
	if config.shouldUseParallel(len(data)) {
		return parseParallelWithConfig(ctx, data, config)
	}

	// Serial parsing
	capacity := config.getInitialCapacity(len(data))
	p := NewParser(capacity)

	// Check token limit during parsing
	count, err := p.Parse(data)
	if err != nil {
		return nil, err
	}

	if config.MaxTokens > 0 && count > config.MaxTokens {
		return nil, ErrTooManyTokens
	}

	return p.Tokens(), nil
}
