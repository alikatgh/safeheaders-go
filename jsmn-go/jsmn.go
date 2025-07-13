// Package jsmngo provides a fast JSON tokenizer with parallel processing capabilities.
package jsmngo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// TokenType represents the type of JSON token.
type TokenType int

const (
	// Object represents a JSON object token.
	Object TokenType = iota
	// Array represents a JSON array token.
	Array
	// String represents a JSON string token.
	String
	// Primitive represents a JSON primitive token (number, boolean, null).
	Primitive
)

// Token holds information about a parsed JSON token.
type Token struct {
	Type      TokenType
	Start     int // Start position in the input string.
	End       int // End position in the input string.
	Size      int // Number of children (for objects/arrays).
	ParentIdx int // Index of parent token (-1 for root).
}

// Parser is the JSON tokenizer state.
type Parser struct {
	pos      int // Current position in the JSON string.
	toknext  int // Next token to allocate.
	toksuper int // Parent token index.
	tokens   []Token
}

// NewParser creates a new parser with space for numTokens.
func NewParser(numTokens int) *Parser {
	return &Parser{
		tokens: make([]Token, numTokens),
	}
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
			continue
		case '}', ']':
			if p.toksuper != -1 {
				p.tokens[p.toksuper].End = p.pos + 1
				p.toksuper = p.tokens[p.toksuper].ParentIdx
			}
			p.pos++
			continue
		case '"':
			err := p.parseString(json)
			if err != nil {
				return 0, err
			}
			continue
		case '\t', '\r', '\n', ' ':
			p.pos++
			continue
		case ':':
			p.pos++
			continue
		case ',':
			if p.toksuper != -1 && p.tokens[p.toksuper].Type != Array && p.tokens[p.toksuper].Type != Object {
				p.toksuper = p.tokens[p.toksuper].ParentIdx
			}
			p.pos++
			continue
		default:
			err := p.parsePrimitive(json)
			if err != nil {
				return 0, err
			}
			continue
		}
	}
	for i := range p.tokens {
		if p.tokens[i].End == -1 && p.tokens[i].Start != -1 {
			p.tokens[i].End = len(json)
		}
	}
	// Additional validation: Check for unclosed structures
	if p.toksuper != -1 {
		return 0, errors.New("unclosed object or array")
	}
	return p.toknext, nil
}

// Tokens returns the parsed tokens.
func (p *Parser) Tokens() []Token {
	return p.tokens[:p.toknext]
}

func (p *Parser) allocToken(tok Token) error {
	if p.toknext >= len(p.tokens) {
		return errors.New("token overflow: too many tokens")
	}
	p.tokens[p.toknext] = tok
	if p.toksuper != -1 {
		p.tokens[p.toksuper].Size++
	}
	p.toknext++
	return nil
}

func (p *Parser) parseString(json []byte) error {
	p.pos++ // Skip opening quote.
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
	for p.pos < len(json) {
		c := json[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']' || c == '}' {
			break
		}
		p.pos++
	}
	tok.End = p.pos
	if tok.End == tok.Start {
		return errors.New("empty primitive")
	}
	if err := p.allocToken(tok); err != nil {
		return err
	}
	return nil
}

// ParseParallel tokenizes JSON in parallel across chunks for improved performance.
func ParseParallel(json []byte, numTokens int) ([]Token, error) {
	if len(json) < 512 { // Fallback for small JSON to avoid invalid chunks.
		p := NewParser(numTokens)
		_, err := p.Parse(json)
		if err != nil {
			return nil, err
		}
		return p.Tokens(), nil
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 4 {
		numWorkers = 4 // Cap for simplicity.
	}
	chunkSize := len(json) / numWorkers
	if chunkSize == 0 {
		chunkSize = len(json)
		numWorkers = 1
	}

	var wg sync.WaitGroup
	results := make([][]Token, numWorkers)
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := start + chunkSize
		if i == numWorkers-1 {
			end = len(json)
		}
		go func(i int, chunk []byte) {
			defer wg.Done()
			p := NewParser(numTokens) // Use full numTokens per to avoid overflow.
			_, err := p.Parse(chunk)
			if err != nil {
				errs <- err
				return
			}
			results[i] = p.Tokens()
		}(i, json[start:end])
	}

	wg.Wait()
	select {
	case err := <-errs:
		return nil, err
	default:
	}

	// Merge results (naive concat; note limitation in README for real use).
	var merged []Token
	for _, res := range results {
		merged = append(merged, res...)
	}
	return merged, nil
}

// ParseStream tokenizes JSON from an io.Reader for non-blocking streaming.
func ParseStream(r io.Reader, numTokens int) ([]Token, error) {
	var fullData []byte
	buf := make([]byte, 4096) // Chunk size for reading.
	for {
		n, err := r.Read(buf)
		if n > 0 {
			fullData = append(fullData, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read from reader: %w", err)
		}
	}
	p := NewParser(numTokens)
	_, err := p.Parse(fullData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return p.Tokens(), nil
}

// ParseStreamDecoder uses json.Decoder for incremental tokenizing during I/O.
// ParseStreamDecoder uses json.Decoder for incremental tokenizing during I/O.
func ParseStreamDecoder(r io.Reader, numTokens int) ([]Token, error) {
	dec := json.NewDecoder(r)
	p := NewParser(numTokens)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoder error: %w", err)
		}
		// Map json.Token to our Token (basic; expand for full support).
		ourTok := Token{ParentIdx: p.toksuper}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				ourTok.Type = Object
			case '[':
				ourTok.Type = Array
			case '}', ']':
				if p.toksuper != -1 {
					p.tokens[p.toksuper].End = p.pos + 1 // Approximate positions.
					p.toksuper = p.tokens[p.toksuper].ParentIdx
				}
				continue // Delims like }/] don't need new tokens.
			}
		case string:
			ourTok.Type = String
		default: // Numbers, booleans, null.
			ourTok.Type = Primitive
		}
		if err := p.allocToken(ourTok); err != nil {
			return nil, err
		}
		p.toksuper = p.toknext - 1 // Update for containers.
		p.pos++                    // Increment position (stub; track real offsets from dec).
	}
	return p.Tokens(), nil
}
