package linenoise

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newSinkState returns a State whose Output is a throwaway temp file, so the
// drawing routines (refresh) can run without a real terminal.
func newSinkState(t *testing.T) *State {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "out-*")
	if err != nil {
		t.Fatalf("temp output: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })
	cfg := DefaultConfig()
	cfg.Output = out
	return New(cfg)
}

// tempInput writes content to a temp file and returns it opened for reading.
func tempInput(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open input: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestProcessCharTerminators(t *testing.T) {
	t.Run("enter returns the line", func(t *testing.T) {
		s := newSinkState(t)
		s.buf = []rune("done")
		cont, result, err := s.processChar('\r')
		if cont || err != nil || result != "done" {
			t.Fatalf("got cont=%v result=%q err=%v", cont, result, err)
		}
	})

	t.Run("ctrl-c interrupts", func(t *testing.T) {
		s := newSinkState(t)
		s.buf = []rune("x")
		cont, _, err := s.processChar('\x03')
		if cont || !errors.Is(err, ErrInterrupted) {
			t.Fatalf("got cont=%v err=%v", cont, err)
		}
	})

	t.Run("ctrl-d on empty buffer is EOF", func(t *testing.T) {
		s := newSinkState(t)
		cont, _, err := s.processChar('\x04')
		if cont || !errors.Is(err, io.EOF) {
			t.Fatalf("got cont=%v err=%v", cont, err)
		}
	})

	t.Run("ctrl-d with text deletes under cursor", func(t *testing.T) {
		s := newSinkState(t)
		s.buf = []rune("abc")
		s.pos = 0
		cont, _, err := s.processChar('\x04')
		if !cont || err != nil {
			t.Fatalf("got cont=%v err=%v", cont, err)
		}
		if string(s.buf) != "bc" {
			t.Fatalf("buf = %q, want %q", string(s.buf), "bc")
		}
	})
}

func TestHandleEditKeys(t *testing.T) {
	mk := func() *State {
		s := newSinkState(t)
		s.buf = []rune("hello world")
		s.pos = len(s.buf)
		return s
	}

	tests := []struct {
		name    string
		key     rune
		wantBuf string
		wantPos int
	}{
		{"insert printable", 'X', "hello worldX", 12},
		{"backspace", '\x7f', "hello worl", 10},
		{"ctrl-a home", '\x01', "hello world", 0},
		{"home key", keyHome, "hello world", 0},
		{"ctrl-k kill to end", '\x0b', "hello world", 11},
		{"ctrl-u kill line", '\x15', "", 0},
		{"ctrl-w del word", '\x17', "hello ", 6},
		{"left arrow", keyLeft, "hello world", 10},
		{"delete key", keyDelete, "hello world", 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := mk()
			s.handleEditKey(tt.key)
			if string(s.buf) != tt.wantBuf {
				t.Errorf("buf = %q, want %q", string(s.buf), tt.wantBuf)
			}
			if s.pos != tt.wantPos {
				t.Errorf("pos = %d, want %d", s.pos, tt.wantPos)
			}
		})
	}
}

func TestHandleEditKeyMovementBounds(t *testing.T) {
	s := newSinkState(t)
	s.buf = []rune("ab")
	s.pos = 0
	s.handleEditKey(keyLeft) // already at start, stays
	if s.pos != 0 {
		t.Fatalf("left at start moved to %d", s.pos)
	}
	s.pos = len(s.buf)
	s.handleEditKey(keyRight) // already at end, stays
	if s.pos != len(s.buf) {
		t.Fatalf("right at end moved to %d", s.pos)
	}
	s.handleEditKey('\x05') // ctrl-e end
	if s.pos != len(s.buf) {
		t.Fatalf("ctrl-e pos = %d", s.pos)
	}
	s.handleEditKey('\x0c') // ctrl-l clear screen (just must not panic)
}

func TestReadCharEscapeSequences(t *testing.T) {
	tests := []struct {
		in   string
		want rune
	}{
		{"a", 'a'},
		{"\x1b[A", keyUp},
		{"\x1b[B", keyDown},
		{"\x1b[C", keyRight},
		{"\x1b[D", keyLeft},
		{"\x1b[H", keyHome},
		{"\x1b[F", keyEnd},
		{"\x1b[1~", keyHome},
		{"\x1b[3~", keyDelete},
		{"\x1b[4~", keyEnd},
		{"\x1bZ", '\x1b'}, // unknown sequence falls back to ESC
		{"é", 'é'},        // multi-byte UTF-8
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.in, "\x1b", "ESC"), func(t *testing.T) {
			s := newSinkState(t)
			r := bufio.NewReader(strings.NewReader(tt.in))
			got, err := s.readChar(r)
			if err != nil {
				t.Fatalf("readChar(%q) err = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("readChar(%q) = %q (%d), want %q (%d)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestReadCharEOF(t *testing.T) {
	s := newSinkState(t)
	r := bufio.NewReader(strings.NewReader(""))
	_, err := s.readChar(r)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readChar at EOF err = %v, want io.EOF", err)
	}
}

func TestHistoryNavigation(t *testing.T) {
	s := newSinkState(t)
	s.history = []string{"one", "two", "three"}
	s.historyIndex = len(s.history)
	s.buf = []rune("draft")
	s.pos = len(s.buf)

	steps := []struct {
		move func()
		want string
	}{
		{s.historyPrev, "three"},
		{s.historyPrev, "two"},
		{s.historyPrev, "one"},
		{s.historyPrev, "one"}, // clamp at oldest
		{s.historyNext, "two"},
		{s.historyNext, "three"},
		{s.historyNext, "draft"}, // back to the stashed live line
		{s.historyNext, "draft"}, // clamp at newest
	}
	for i, st := range steps {
		st.move()
		if string(s.buf) != st.want {
			t.Fatalf("step %d: buf = %q, want %q", i, string(s.buf), st.want)
		}
	}
}

func TestHistoryNavigationEmpty(t *testing.T) {
	s := newSinkState(t)
	s.historyIndex = 0
	s.buf = []rune("keep")
	s.historyPrev()
	s.historyNext()
	if string(s.buf) != "keep" {
		t.Fatalf("navigation on empty history changed buf to %q", string(s.buf))
	}
}

func TestReadLineNoTTY(t *testing.T) {
	t.Run("full line", func(t *testing.T) {
		s := New(DefaultConfig())
		s.config.Input = tempInput(t, "hello\nworld\n")
		got, err := s.ReadLine("> ")
		if err != nil || got != "hello" {
			t.Fatalf("got %q err %v", got, err)
		}
	})

	t.Run("final line without newline", func(t *testing.T) {
		s := New(DefaultConfig())
		s.config.Input = tempInput(t, "partial")
		got, err := s.ReadLine("> ")
		if err != nil || got != "partial" {
			t.Fatalf("got %q err %v, want \"partial\" nil", got, err)
		}
	})

	t.Run("empty input is EOF", func(t *testing.T) {
		s := New(DefaultConfig())
		s.config.Input = tempInput(t, "")
		_, err := s.ReadLine("> ")
		if !errors.Is(err, io.EOF) {
			t.Fatalf("err = %v, want io.EOF", err)
		}
	})
}

func TestRefreshBranches(t *testing.T) {
	// Mask mode and a hints callback exercise the conditional branches in refresh.
	s := newSinkState(t)
	s.prompt = "pw> "
	s.buf = []rune("secret")
	s.pos = len(s.buf)
	s.config.MaskMode = true
	s.refresh()

	s.config.MaskMode = false
	s.config.HintsCallback = func(line string) (string, bool) { return " <hint>", true }
	s.refresh()
}

func TestHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")

	s := New(DefaultConfig())
	s.AddHistory("alpha")
	s.AddHistory("beta")
	s.AddHistory("beta") // duplicate of last, ignored
	if err := s.SaveHistory(path); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}

	loaded := New(DefaultConfig())
	if err := loaded.LoadHistory(path); err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if got := strings.Join(loaded.history, ","); got != "alpha,beta" {
		t.Fatalf("loaded history = %q, want %q", got, "alpha,beta")
	}

	// Loading a missing file is not an error.
	if err := loaded.LoadHistory(filepath.Join(t.TempDir(), "absent")); err != nil {
		t.Fatalf("LoadHistory(missing) = %v, want nil", err)
	}
}

func TestRestoreNoTerm(t *testing.T) {
	s := newSinkState(t)
	s.oldTerm = nil
	s.restore() // must be a safe no-op when not in raw mode
}

func TestGlobalConvenienceFns(t *testing.T) {
	ClearHistory()
	AddHistory("g1")
	AddHistory("g2")

	path := filepath.Join(t.TempDir(), "global-history")
	if err := SaveHistory(path); err != nil {
		t.Fatalf("global SaveHistory: %v", err)
	}
	if err := LoadHistory(path); err != nil {
		t.Fatalf("global LoadHistory: %v", err)
	}

	SetCompletionCallback(func(string) []string { return []string{"x"} })
	SetHintsCallback(func(string) (string, bool) { return "", false })
	SetMultiLine(true)
	SetMaskMode(true)
	ClearHistory()
}
