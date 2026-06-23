// Package jsmngo provides a fast JSON tokenizer with parallel processing capabilities.
package jsmngo

import (
	"context"
	"errors"
	"fmt"
	"runtime"
)

// TokenType represents the type of JSON token.
type TokenType int

const (
	// Object represents a JSON object token: { ... }.
	Object TokenType = iota
	// Array represents a JSON array token: [ ... ].
	Array
	// String represents a JSON string token: "...".
	String
	// Primitive represents a JSON primitive token (e.g., number, boolean, null).
	Primitive
)

// Token holds information about a parsed JSON token.
type Token struct {
	Type      TokenType
	Start     int
	End       int
	Size      int
	ParentIdx int
}

// Parser is the JSON tokenizer state.
type Parser struct {
	pos      int
	toknext  int
	toksuper int
	tokens   []Token
}

// NewParser creates a new parser with space for numTokens.
func NewParser(numTokens int) *Parser {
	return &Parser{
		tokens: make([]Token, numTokens),
	}
}

// isPrimitiveStart checks if a byte can be the beginning of a JSON primitive.
func isPrimitiveStart(c byte) bool {
	return c == 't' || c == 'f' || c == 'n' || (c >= '0' && c <= '9') || c == '-'
}

// Parse tokenizes the JSON input, returning the number of tokens or an error.
func (p *Parser) Parse(json []byte) (int, error) {
	p.pos = 0
	p.toknext = 0
	p.toksuper = -1

	for p.pos < len(json) {
		c := json[p.pos]
		switch c {
		case '{', '[':
			tok := Token{Start: p.pos, End: -1, Size: 0, ParentIdx: p.toksuper}
			if c == '{' {
				tok.Type = Object
			} else {
				tok.Type = Array
			}
			if err := p.allocToken(tok); err != nil {
				return 0, err
			}
			p.toksuper = p.toknext - 1
			p.pos++
		case '}', ']':
			if p.toksuper != -1 {
				p.tokens[p.toksuper].End = p.pos + 1
				p.toksuper = p.tokens[p.toksuper].ParentIdx
			}
			p.pos++
		case '"':
			if err := p.parseString(json); err != nil {
				return 0, err
			}
		case '\t', '\r', '\n', ' ', ':', ',':
			p.pos++
		default:
			if !isPrimitiveStart(c) {
				return 0, fmt.Errorf("invalid character '%c' at position %d", c, p.pos)
			}
			if err := p.parsePrimitive(json); err != nil {
				return 0, err
			}
		}
	}
	for i := 0; i < p.toknext; i++ {
		if p.tokens[i].End == -1 && p.tokens[i].Start != -1 {
			p.tokens[i].End = len(json)
		}
	}
	if p.toksuper != -1 {
		return 0, errors.New("unclosed object or array")
	}
	return p.toknext, nil
}

// Tokens returns the parsed tokens.
func (p *Parser) Tokens() []Token {
	return p.tokens[:p.toknext]
}

// allocToken appends a token to the parser's token slice.
// It will grow the slice if the capacity is exceeded.
func (p *Parser) allocToken(tok Token) error {
	if p.toknext >= len(p.tokens) {
		// Instead of returning an error, grow the slice. This is safer for workers.
		p.tokens = append(p.tokens, Token{})
	}
	p.tokens[p.toknext] = tok
	if p.toksuper != -1 {
		p.tokens[p.toksuper].Size++
	}
	p.toknext++
	return nil
}

func (p *Parser) parseString(json []byte) error {
	p.pos++ // Skip opening quote
	tok := Token{Type: String, Start: p.pos, End: -1, ParentIdx: p.toksuper}
	for p.pos < len(json) {
		c := json[p.pos]
		if c == '"' {
			tok.End = p.pos
			if err := p.allocToken(tok); err != nil {
				return err
			}
			p.pos++
			return nil
		}
		if c == '\\' && p.pos+1 < len(json) {
			p.pos += 2
			continue
		}
		p.pos++
	}
	return errors.New("unclosed string")
}

func (p *Parser) parsePrimitive(json []byte) error {
	tok := Token{Type: Primitive, Start: p.pos, End: -1, ParentIdx: p.toksuper}
	start := p.pos
	for p.pos < len(json) {
		c := json[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']' || c == '}' {
			break
		}
		p.pos++
	}
	tok.End = p.pos
	if tok.End == tok.Start {
		return fmt.Errorf("invalid character at %d", start)
	}
	return p.allocToken(tok)
}

// findSplitPoints scans the JSON and returns a slice of byte offsets
// corresponding to top-level commas, which are safe split points.
func findSplitPoints(data []byte) []int {
	var splits []int
	depth := 0
	inStr := false
	escaped := false

	for i, c := range data {
		if inStr {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				splits = append(splits, i)
			}
		}
	}
	return splits
}

// ParseParallel tokenizes JSON in parallel. It is most effective on large JSON arrays.
// For single large JSON objects, it will correctly fall back to single-threaded parsing.
func ParseParallel(json []byte) ([]Token, error) {
	if len(json) < 4096 { // Fallback for small payloads
		p := NewParser(len(json) / 4) // Initial heuristic capacity
		_, err := p.Parse(json)
		return p.Tokens(), err
	}

	// Not enough top-level split points to justify parallelism.
	if len(findSplitPoints(json)) < runtime.NumCPU() {
		p := NewParser(len(json) / 4)
		_, err := p.Parse(json)
		return p.Tokens(), err
	}

	// Delegate the chunked worker-pool + merge to the shared implementation so
	// there is a single source of truth for the parallel tokenizer. Unlimited
	// config = no size/token caps and no context, matching this entry point's
	// historical contract.
	return parseParallelWithConfig(context.Background(), json, UnlimitedConfig())
}
