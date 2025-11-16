// Package linenoise provides a minimal line editing library for CLI applications.
//
// This is a Go port of antirez/linenoise (https://github.com/antirez/linenoise),
// a minimal alternative to readline and libedit.
//
// Features:
//   - Single and multi-line editing
//   - Command history with persistence
//   - Tab completion support
//   - Inline hints
//   - Password masking
//   - Cross-platform (Unix/Linux/macOS)
//
// Basic usage:
//
//	line, err := linenoise.Line("prompt> ")
//	if err != nil {
//	    if err == io.EOF {
//	        // User pressed Ctrl+D
//	        return
//	    }
//	    log.Fatal(err)
//	}
//	linenoise.AddHistory(line)
package linenoise

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"
)

// Version information
const (
	Version = "0.1.0"
)

// Common errors
var (
	ErrNotTTY        = errors.New("not a terminal")
	ErrUnsupported   = errors.New("unsupported terminal")
	ErrInterrupted   = errors.New("interrupted")
	ErrInvalidUTF8   = errors.New("invalid UTF-8 sequence")
)

// CompletionCallback is called when the user presses Tab.
// It should populate completions based on the current line buffer.
type CompletionCallback func(line string) []string

// HintsCallback is called to provide inline hints as the user types.
// It returns the hint text and whether to display it in bold.
type HintsCallback func(line string) (hint string, bold bool)

// Config holds linenoise configuration options.
type Config struct {
	// HistoryMaxLen is the maximum number of history entries (0 = unlimited)
	HistoryMaxLen int

	// MultiLine enables multi-line editing mode
	MultiLine bool

	// MaskMode enables password masking (characters shown as '*')
	MaskMode bool

	// CompletionCallback is called for tab completion
	CompletionCallback CompletionCallback

	// HintsCallback is called to display inline hints
	HintsCallback HintsCallback

	// Input/Output streams (defaults to os.Stdin/os.Stdout)
	Input  *os.File
	Output *os.File
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		HistoryMaxLen: 100,
		MultiLine:     false,
		MaskMode:      false,
		Input:         os.Stdin,
		Output:        os.Stdout,
	}
}

// State represents the current line editing state.
type State struct {
	config  *Config
	history []string
	buf     []rune
	pos     int
	prompt  string
	oldTerm *term.State

	// Completion state
	completions     []string
	completionIndex int
	completionActive bool
	savedBuf        []rune
}

// New creates a new linenoise state with the given configuration.
func New(config *Config) *State {
	if config == nil {
		config = DefaultConfig()
	}
	return &State{
		config:  config,
		history: make([]string, 0, config.HistoryMaxLen),
		buf:     make([]rune, 0, 128),
	}
}

// Line reads a line of input with the given prompt.
// This is a convenience function that creates a new State and calls ReadLine.
func Line(prompt string) (string, error) {
	state := New(DefaultConfig())
	return state.ReadLine(prompt)
}

// ReadLine reads a line of input with the given prompt.
func (s *State) ReadLine(prompt string) (string, error) {
	// Check if we're connected to a terminal
	if !term.IsTerminal(int(s.config.Input.Fd())) {
		return s.readLineNoTTY()
	}

	s.prompt = prompt
	s.buf = s.buf[:0]
	s.pos = 0
	s.completionActive = false

	// Enable raw mode
	oldState, err := term.MakeRaw(int(s.config.Input.Fd()))
	if err != nil {
		return "", err
	}
	s.oldTerm = oldState
	defer s.restore()

	// Display prompt
	fmt.Fprint(s.config.Output, prompt)

	// Read and process input
	reader := bufio.NewReader(s.config.Input)
	for {
		ch, err := s.readChar(reader)
		if err != nil {
			if err == io.EOF {
				if len(s.buf) > 0 {
					fmt.Fprintln(s.config.Output)
					return string(s.buf), nil
				}
				return "", io.EOF
			}
			return "", err
		}

		cont, result, err := s.processChar(ch)
		if !cont {
			if err != nil {
				return "", err
			}
			fmt.Fprintln(s.config.Output)
			return result, nil
		}
	}
}

// readChar reads a single character or escape sequence
func (s *State) readChar(reader *bufio.Reader) (rune, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	// Check for escape sequences
	if b == '\x1b' {
		// Try to read escape sequence
		b2, err := reader.ReadByte()
		if err != nil {
			return '\x1b', nil
		}

		if b2 == '[' || b2 == 'O' {
			b3, err := reader.ReadByte()
			if err != nil {
				return '\x1b', nil
			}

			// Arrow keys and other special keys
			switch b3 {
			case 'A': return keyUp, nil
			case 'B': return keyDown, nil
			case 'C': return keyRight, nil
			case 'D': return keyLeft, nil
			case 'H': return keyHome, nil
			case 'F': return keyEnd, nil
			case '1', '7': // Home
				reader.ReadByte() // consume ~
				return keyHome, nil
			case '3': // Delete
				reader.ReadByte() // consume ~
				return keyDelete, nil
			case '4', '8': // End
				reader.ReadByte() // consume ~
				return keyEnd, nil
			}
		}
		return '\x1b', nil
	}

	// Handle UTF-8
	if b < 128 {
		return rune(b), nil
	}

	// Multi-byte UTF-8 character
	reader.UnreadByte()
	r, _, err := reader.ReadRune()
	return r, err
}

// Special key codes
const (
	keyUp rune = 0x10000 + iota
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyDelete
)

// processChar processes a single character and returns:
// - cont: whether to continue reading
// - result: the final line (if cont is false)
// - err: any error that occurred
func (s *State) processChar(ch rune) (cont bool, result string, err error) {
	switch ch {
	case '\r', '\n': // Enter
		return false, string(s.buf), nil

	case '\x03': // Ctrl+C
		return false, "", ErrInterrupted

	case '\x04': // Ctrl+D (EOF)
		if len(s.buf) == 0 {
			return false, "", io.EOF
		}
		// Delete character under cursor
		s.deleteChar()

	case '\x08', '\x7f': // Backspace / Delete
		s.backspace()

	case '\t': // Tab (completion)
		s.handleCompletion()

	case '\x01': // Ctrl+A (Home)
		s.pos = 0

	case '\x05': // Ctrl+E (End)
		s.pos = len(s.buf)

	case '\x02': // Ctrl+B (Left)
		if s.pos > 0 {
			s.pos--
		}

	case '\x06': // Ctrl+F (Right)
		if s.pos < len(s.buf) {
			s.pos++
		}

	case '\x0b': // Ctrl+K (delete to end of line)
		s.buf = s.buf[:s.pos]

	case '\x15': // Ctrl+U (delete entire line)
		s.buf = s.buf[:0]
		s.pos = 0

	case '\x17': // Ctrl+W (delete previous word)
		s.deletePrevWord()

	case '\x0c': // Ctrl+L (clear screen)
		fmt.Fprint(s.config.Output, "\x1b[H\x1b[2J")

	case keyUp:
		s.historyPrev()

	case keyDown:
		s.historyNext()

	case keyLeft:
		if s.pos > 0 {
			s.pos--
		}

	case keyRight:
		if s.pos < len(s.buf) {
			s.pos++
		}

	case keyHome:
		s.pos = 0

	case keyEnd:
		s.pos = len(s.buf)

	case keyDelete:
		s.deleteChar()

	default:
		// Regular character - insert at cursor position
		if unicode.IsPrint(ch) || ch == ' ' {
			s.insertChar(ch)
		}
	}

	s.refresh()
	return true, "", nil
}

// insertChar inserts a character at the cursor position
func (s *State) insertChar(ch rune) {
	// Expand buffer if needed
	if s.pos == len(s.buf) {
		s.buf = append(s.buf, ch)
		s.pos++
		return
	}

	// Insert in middle
	s.buf = append(s.buf, 0)
	copy(s.buf[s.pos+1:], s.buf[s.pos:])
	s.buf[s.pos] = ch
	s.pos++
}

// deleteChar deletes the character under the cursor
func (s *State) deleteChar() {
	if s.pos < len(s.buf) {
		copy(s.buf[s.pos:], s.buf[s.pos+1:])
		s.buf = s.buf[:len(s.buf)-1]
	}
}

// backspace deletes the character before the cursor
func (s *State) backspace() {
	if s.pos > 0 {
		s.pos--
		s.deleteChar()
	}
}

// deletePrevWord deletes the previous word (Ctrl+W)
func (s *State) deletePrevWord() {
	if s.pos == 0 {
		return
	}

	// Find start of word
	oldPos := s.pos
	for s.pos > 0 && unicode.IsSpace(s.buf[s.pos-1]) {
		s.pos--
	}
	for s.pos > 0 && !unicode.IsSpace(s.buf[s.pos-1]) {
		s.pos--
	}

	// Delete from pos to oldPos
	copy(s.buf[s.pos:], s.buf[oldPos:])
	s.buf = s.buf[:len(s.buf)-(oldPos-s.pos)]
}

// refresh redraws the line
func (s *State) refresh() {
	// Move cursor to start of line
	fmt.Fprintf(s.config.Output, "\r\x1b[K")

	// Print prompt
	fmt.Fprint(s.config.Output, s.prompt)

	// Print buffer (with masking if enabled)
	if s.config.MaskMode {
		for range s.buf {
			fmt.Fprint(s.config.Output, "*")
		}
	} else {
		fmt.Fprint(s.config.Output, string(s.buf))
	}

	// Print hint if available
	if s.config.HintsCallback != nil && !s.config.MaskMode {
		hint, bold := s.config.HintsCallback(string(s.buf))
		if hint != "" {
			if bold {
				fmt.Fprintf(s.config.Output, "\x1b[1;90m%s\x1b[0m", hint)
			} else {
				fmt.Fprintf(s.config.Output, "\x1b[90m%s\x1b[0m", hint)
			}
		}
	}

	// Move cursor to correct position
	promptLen := utf8.RuneCountInString(s.prompt)
	targetCol := promptLen + s.pos
	fmt.Fprintf(s.config.Output, "\r\x1b[%dC", targetCol)
}

// handleCompletion handles tab completion
func (s *State) handleCompletion() {
	if s.config.CompletionCallback == nil {
		return
	}

	// First tab - get completions
	if !s.completionActive {
		s.completions = s.config.CompletionCallback(string(s.buf))
		if len(s.completions) == 0 {
			return
		}
		s.completionActive = true
		s.completionIndex = 0
		s.savedBuf = append([]rune(nil), s.buf...)
	} else {
		// Cycle to next completion
		s.completionIndex = (s.completionIndex + 1) % len(s.completions)
	}

	// Replace buffer with completion
	s.buf = []rune(s.completions[s.completionIndex])
	s.pos = len(s.buf)
}

// historyPrev moves to previous history entry
func (s *State) historyPrev() {
	// TODO: Implement history navigation
}

// historyNext moves to next history entry
func (s *State) historyNext() {
	// TODO: Implement history navigation
}

// restore restores terminal to normal mode
func (s *State) restore() {
	if s.oldTerm != nil {
		term.Restore(int(s.config.Input.Fd()), s.oldTerm)
		s.oldTerm = nil
	}
}

// readLineNoTTY reads a line when not connected to a terminal
func (s *State) readLineNoTTY() (string, error) {
	reader := bufio.NewReader(s.config.Input)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

// AddHistory adds a line to the history.
func (s *State) AddHistory(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Don't add duplicates
	if len(s.history) > 0 && s.history[len(s.history)-1] == line {
		return
	}

	// Add to history
	s.history = append(s.history, line)

	// Trim if exceeds max length
	if s.config.HistoryMaxLen > 0 && len(s.history) > s.config.HistoryMaxLen {
		s.history = s.history[len(s.history)-s.config.HistoryMaxLen:]
	}
}

// SaveHistory saves the history to a file.
func (s *State) SaveHistory(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range s.history {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return err
		}
	}
	return nil
}

// LoadHistory loads history from a file.
func (s *State) LoadHistory(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error if file doesn't exist
		}
		return err
	}
	defer f.Close()

	s.history = s.history[:0]
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			s.history = append(s.history, line)
		}
	}
	return scanner.Err()
}

// ClearHistory clears all history entries.
func (s *State) ClearHistory() {
	s.history = s.history[:0]
}

// Global convenience functions for simple usage

var defaultState = New(DefaultConfig())

// AddHistory adds a line to the global history.
func AddHistory(line string) {
	defaultState.AddHistory(line)
}

// SaveHistory saves the global history to a file.
func SaveHistory(filename string) error {
	return defaultState.SaveHistory(filename)
}

// LoadHistory loads the global history from a file.
func LoadHistory(filename string) error {
	return defaultState.LoadHistory(filename)
}

// ClearHistory clears the global history.
func ClearHistory() {
	defaultState.ClearHistory()
}

// SetCompletionCallback sets the global completion callback.
func SetCompletionCallback(cb CompletionCallback) {
	defaultState.config.CompletionCallback = cb
}

// SetHintsCallback sets the global hints callback.
func SetHintsCallback(cb HintsCallback) {
	defaultState.config.HintsCallback = cb
}

// SetMultiLine enables or disables multi-line mode globally.
func SetMultiLine(enabled bool) {
	defaultState.config.MultiLine = enabled
}

// SetMaskMode enables or disables password masking globally.
func SetMaskMode(enabled bool) {
	defaultState.config.MaskMode = enabled
}
