// Package tinyxml2go provides a concurrent XML DOM parser with optional
// DoS-protection limits (input size, node count, nesting depth).
package tinyxml2go

import "errors"

// Common errors returned by configured parsing.
var (
	// ErrInputTooLarge is returned when input exceeds Config.MaxInputSize.
	ErrInputTooLarge = errors.New("input size exceeds maximum allowed")

	// ErrTooManyNodes is returned when the parsed node count exceeds Config.MaxNodeCount.
	ErrTooManyNodes = errors.New("node count exceeds maximum allowed")

	// ErrNestingTooDeep is returned when element nesting exceeds Config.MaxNestingDepth.
	ErrNestingTooDeep = errors.New("nesting depth exceeds maximum allowed")

	// ErrEmptyInput is returned when input is empty.
	ErrEmptyInput = errors.New("empty input")
)

// Config holds parsing limits used to guard against malicious or oversized
// input (DoS protection). The zero value of any individual limit means
// "unlimited" for that dimension; prefer DefaultConfig or StrictConfig for
// untrusted input.
type Config struct {
	// MaxInputSize limits the maximum XML input size in bytes.
	// Default: 100MB. Set to 0 for unlimited (not recommended).
	MaxInputSize int

	// MaxNodeCount limits the maximum number of element nodes parsed.
	// Default: 1,000,000. Set to 0 for unlimited (not recommended).
	MaxNodeCount int

	// MaxNestingDepth limits how deeply elements may nest. This prevents
	// stack exhaustion from deeply nested XML.
	// Default: 1,000. Set to 0 for unlimited (not recommended).
	MaxNestingDepth int
}

// DefaultConfig returns the default configuration with sensible limits.
func DefaultConfig() *Config {
	return &Config{
		MaxInputSize:    100 * 1024 * 1024, // 100MB
		MaxNodeCount:    1_000_000,         // 1 million nodes
		MaxNestingDepth: 1_000,             // 1000 levels deep
	}
}

// StrictConfig returns a stricter configuration suitable for untrusted input.
func StrictConfig() *Config {
	return &Config{
		MaxInputSize:    10 * 1024 * 1024, // 10MB
		MaxNodeCount:    100_000,          // 100k nodes
		MaxNestingDepth: 100,              // 100 levels deep
	}
}

// UnlimitedConfig returns a configuration with no limits (use with caution).
func UnlimitedConfig() *Config {
	return &Config{
		MaxInputSize:    0, // Unlimited
		MaxNodeCount:    0, // Unlimited
		MaxNestingDepth: 0, // Unlimited
	}
}

// Validate checks that the configuration values are not negative.
func (c *Config) Validate() error {
	if c.MaxInputSize < 0 {
		return errors.New("MaxInputSize cannot be negative")
	}
	if c.MaxNodeCount < 0 {
		return errors.New("MaxNodeCount cannot be negative")
	}
	if c.MaxNestingDepth < 0 {
		return errors.New("MaxNestingDepth cannot be negative")
	}
	return nil
}

// validateInput checks that the input meets the size constraints.
func (c *Config) validateInput(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyInput
	}
	if c.MaxInputSize > 0 && len(data) > c.MaxInputSize {
		return ErrInputTooLarge
	}
	return nil
}
