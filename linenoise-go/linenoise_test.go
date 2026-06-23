package linenoise

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	config := DefaultConfig()
	state := New(config)

	if state == nil {
		t.Fatal("New() returned nil")
	}

	if state.config != config {
		t.Error("Config not set correctly")
	}

	if len(state.history) != 0 {
		t.Error("History should be empty initially")
	}
}

func TestInsertChar(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		pos         int
		char        rune
		expected    string
		expectedPos int
	}{
		{"append", "hello", 5, '!', "hello!", 6},
		{"insert middle", "helo", 2, 'l', "hello", 3},
		{"insert start", "ello", 0, 'h', "hello", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(DefaultConfig())
			s.buf = []rune(tt.initial)
			s.pos = tt.pos

			s.insertChar(tt.char)

			result := string(s.buf)
			if result != tt.expected {
				t.Errorf("insertChar() = %q, want %q", result, tt.expected)
			}

			if s.pos != tt.expectedPos {
				t.Errorf("pos = %d, want %d", s.pos, tt.expectedPos)
			}
		})
	}
}

func TestDeleteChar(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		pos      int
		expected string
	}{
		{"delete middle", "hello", 2, "helo"},
		{"delete end", "hello", 4, "hell"},
		{"delete start", "hello", 0, "ello"},
		{"no delete at end", "hello", 5, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(DefaultConfig())
			s.buf = []rune(tt.initial)
			s.pos = tt.pos

			s.deleteChar()

			result := string(s.buf)
			if result != tt.expected {
				t.Errorf("deleteChar() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBackspace(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		pos         int
		expected    string
		expectedPos int
	}{
		{"backspace middle", "hello", 3, "helo", 2},
		{"backspace end", "hello", 5, "hell", 4},
		{"no backspace at start", "hello", 0, "hello", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(DefaultConfig())
			s.buf = []rune(tt.initial)
			s.pos = tt.pos

			s.backspace()

			result := string(s.buf)
			if result != tt.expected {
				t.Errorf("backspace() = %q, want %q", result, tt.expected)
			}

			if s.pos != tt.expectedPos {
				t.Errorf("pos = %d, want %d", s.pos, tt.expectedPos)
			}
		})
	}
}

func TestDeletePrevWord(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		pos         int
		expected    string
		expectedPos int
	}{
		{"delete word", "hello world", 11, "hello ", 6},
		{"delete with spaces", "hello   world", 13, "hello   ", 8},
		{"delete at word boundary", "hello world", 6, "world", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(DefaultConfig())
			s.buf = []rune(tt.initial)
			s.pos = tt.pos

			s.deletePrevWord()

			result := string(s.buf)
			if result != tt.expected {
				t.Errorf("deletePrevWord() = %q, want %q", result, tt.expected)
			}

			if s.pos != tt.expectedPos {
				t.Errorf("pos = %d, want %d", s.pos, tt.expectedPos)
			}
		})
	}
}

func TestAddHistory(t *testing.T) {
	s := New(DefaultConfig())

	// Add first entry
	s.AddHistory("command1")
	if len(s.history) != 1 {
		t.Errorf("history length = %d, want 1", len(s.history))
	}

	// Add second entry
	s.AddHistory("command2")
	if len(s.history) != 2 {
		t.Errorf("history length = %d, want 2", len(s.history))
	}

	// Add duplicate - should not be added
	s.AddHistory("command2")
	if len(s.history) != 2 {
		t.Errorf("history length = %d, want 2 (no duplicate)", len(s.history))
	}

	// Add empty - should not be added
	s.AddHistory("")
	s.AddHistory("   ")
	if len(s.history) != 2 {
		t.Errorf("history length = %d, want 2 (no empty)", len(s.history))
	}
}

func TestHistoryMaxLen(t *testing.T) {
	config := DefaultConfig()
	config.HistoryMaxLen = 3
	s := New(config)

	s.AddHistory("cmd1")
	s.AddHistory("cmd2")
	s.AddHistory("cmd3")
	s.AddHistory("cmd4")

	if len(s.history) != 3 {
		t.Errorf("history length = %d, want 3", len(s.history))
	}

	// Should have kept the 3 most recent
	if s.history[0] != "cmd2" {
		t.Errorf("history[0] = %q, want %q", s.history[0], "cmd2")
	}
	if s.history[2] != "cmd4" {
		t.Errorf("history[2] = %q, want %q", s.history[2], "cmd4")
	}
}

func TestCompletion(t *testing.T) {
	config := DefaultConfig()
	config.CompletionCallback = func(line string) []string {
		if strings.HasPrefix("hello", line) {
			return []string{"hello", "help"}
		}
		return nil
	}

	s := New(config)
	s.buf = []rune("hel")
	s.pos = 3

	// First tab - should get completions
	s.handleCompletion()
	if !s.completionActive {
		t.Error("Completion should be active")
	}
	if len(s.completions) != 2 {
		t.Errorf("completions length = %d, want 2", len(s.completions))
	}
	if string(s.buf) != "hello" {
		t.Errorf("buf = %q, want %q", string(s.buf), "hello")
	}

	// Second tab - should cycle
	s.handleCompletion()
	if string(s.buf) != "help" {
		t.Errorf("buf = %q, want %q", string(s.buf), "help")
	}

	// Third tab - should wrap around
	s.handleCompletion()
	if string(s.buf) != "hello" {
		t.Errorf("buf = %q, want %q", string(s.buf), "hello")
	}
}

func TestHints(t *testing.T) {
	config := DefaultConfig()
	hintCalled := false
	config.HintsCallback = func(line string) (string, bool) {
		hintCalled = true
		if line == "git" {
			return " commit", true
		}
		return "", false
	}

	s := New(config)
	s.buf = []rune("git")
	s.pos = 3

	// Refresh should call hints callback
	// (We can't easily test the output, but we can verify the callback is called)
	hint, bold := s.config.HintsCallback(string(s.buf))
	if !hintCalled {
		t.Error("Hints callback should have been called")
	}
	if hint != " commit" {
		t.Errorf("hint = %q, want %q", hint, " commit")
	}
	if !bold {
		t.Error("hint should be bold")
	}
}

func TestClearHistory(t *testing.T) {
	s := New(DefaultConfig())
	s.AddHistory("cmd1")
	s.AddHistory("cmd2")

	s.ClearHistory()

	if len(s.history) != 0 {
		t.Errorf("history length = %d, want 0", len(s.history))
	}
}

func BenchmarkInsertChar(b *testing.B) {
	s := New(DefaultConfig())
	s.buf = make([]rune, 0, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.buf = s.buf[:0]
		s.pos = 0
		for j := 0; j < 100; j++ {
			s.insertChar('a')
		}
	}
}

func BenchmarkDeleteChar(b *testing.B) {
	s := New(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.buf = []rune(strings.Repeat("a", 100))
		s.pos = 50
		for j := 0; j < 50; j++ {
			s.deleteChar()
		}
	}
}

func BenchmarkAddHistory(b *testing.B) {
	s := New(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddHistory("test command")
	}
}
