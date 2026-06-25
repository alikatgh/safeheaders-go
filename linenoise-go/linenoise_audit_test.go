package linenoise

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func tempOutState(t *testing.T) *State {
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

// TestGlobalHistoryConcurrent exercises the shared defaultState from many
// goroutines; run with -race it must not report a data race (audit H3).
func TestGlobalHistoryConcurrent(t *testing.T) {
	ClearHistory()
	t.Cleanup(ClearHistory)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				AddHistory(fmt.Sprintf("cmd-%d-%d", i, j))
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 400; j++ {
			defaultState.historyPrev()
			defaultState.historyNext()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		path := filepath.Join(t.TempDir(), "h")
		for j := 0; j < 50; j++ {
			_ = defaultState.SaveHistory(path)
		}
	}()
	wg.Wait()
}

// TestLoadHistoryLongLine: a line over the 64KB bufio.Scanner cap must load (not
// abort), and a load must not destroy existing history on failure (audit M4).
func TestLoadHistoryLongLine(t *testing.T) {
	s := New(DefaultConfig())
	s.AddHistory("pre-existing")

	path := filepath.Join(t.TempDir(), "history")
	long := strings.Repeat("x", 100*1024) // 100 KB, well over the 64 KB scanner cap
	content := "short\n" + long + "\nafter\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.LoadHistory(path); err != nil {
		t.Fatalf("LoadHistory failed on a long line: %v", err)
	}
	if len(s.history) != 3 || s.history[0] != "short" || len(s.history[1]) != len(long) || s.history[2] != "after" {
		t.Fatalf("loaded history wrong: len=%d", len(s.history))
	}

	// A load from a non-existent file is a no-op and must not wipe history.
	if err := s.LoadHistory(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("LoadHistory(missing): %v", err)
	}
	if len(s.history) != 3 {
		t.Errorf("history changed by a missing-file load: len=%d", len(s.history))
	}
}

// TestCompletionResetsOnEdit: editing after a Tab must exit completion mode so a
// later Tab does not overwrite the edit with a stale completion (audit L3).
func TestCompletionResetsOnEdit(t *testing.T) {
	s := tempOutState(t)
	s.config.CompletionCallback = func(string) []string { return []string{"foobar", "foobaz"} }
	s.buf = []rune("foo")
	s.pos = 3

	s.handleCompletion() // first Tab
	if !s.completionActive {
		t.Fatal("expected completion active after first Tab")
	}
	s.handleEditKey('X') // edit
	if s.completionActive {
		t.Error("completion should reset after a non-Tab edit")
	}
}

// TestRefreshNoZeroCUF: refresh must not emit ESC[0C on the empty-prompt edge
// (audit L4).
func TestRefreshNoZeroCUF(t *testing.T) {
	s := tempOutState(t)
	s.prompt = ""
	s.buf = nil
	s.pos = 0
	s.refresh()
	_ = s.config.Output.Sync()
	data, err := os.ReadFile(s.config.Output.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "[0C") {
		t.Errorf("refresh emitted ESC[0C (off-by-one CUF): %q", string(data))
	}
}
