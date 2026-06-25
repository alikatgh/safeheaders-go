package jsmngo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// topLevelStream builds n comma-separated top-level JSON objects. findSplitPoints
// splits on depth-0 commas, so this shape produces n-1 split points and, when
// large enough, exercises the chunked parallel code path.
func topLevelStream(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"index":%d,"name":"item-%d","value":%d}`, i, i, i)
	}
	return []byte(b.String())
}

func TestParseParallelChunked(t *testing.T) {
	data := topLevelStream(300) // ~12KB, 299 split points (>> NumCPU)
	if len(data) < 4096 {
		t.Fatalf("test input too small (%d bytes) to trigger the parallel path", len(data))
	}

	// Baseline: serial token count.
	serial := NewParser(len(data) / 4)
	serialCount, err := serial.Parse(data)
	if err != nil {
		t.Fatalf("serial parse: %v", err)
	}

	t.Run("ParseParallel matches serial", func(t *testing.T) {
		toks, err := ParseParallel(data)
		if err != nil {
			t.Fatalf("ParseParallel: %v", err)
		}
		if len(toks) != serialCount {
			t.Errorf("ParseParallel produced %d tokens, serial produced %d", len(toks), serialCount)
		}
	})

	t.Run("ParseWithConfig parallel matches serial", func(t *testing.T) {
		toks, err := ParseWithConfig(context.Background(), data, DefaultConfig())
		if err != nil {
			t.Fatalf("ParseWithConfig: %v", err)
		}
		if len(toks) != serialCount {
			t.Errorf("parallel produced %d tokens, serial produced %d", len(toks), serialCount)
		}
	})
}

// TestParallelTokensMatchSerial checks that the chunked parallel tokenizer
// produces byte-for-byte identical tokens (type and global offsets) to a serial
// pass across input sizes that span the worker-count boundary. This guards the
// chunk-grouping math in buildChunkJobs.
func TestParallelTokensMatchSerial(t *testing.T) {
	for _, n := range []int{90, 100, 250, 1000, 5000} {
		data := topLevelStream(n)
		if len(data) < 4096 {
			continue // below the parallel threshold; not exercising chunking
		}

		serial := NewParser(len(data) / 4)
		if _, err := serial.Parse(data); err != nil {
			t.Fatalf("n=%d serial parse: %v", n, err)
		}
		want := serial.Tokens()

		got, err := ParseParallel(data)
		if err != nil {
			t.Fatalf("n=%d ParseParallel: %v", n, err)
		}
		if len(got) != len(want) {
			t.Fatalf("n=%d: got %d tokens, want %d", n, len(got), len(want))
		}
		for i := range want {
			if got[i].Type != want[i].Type || got[i].Start != want[i].Start || got[i].End != want[i].End ||
				got[i].ParentIdx != want[i].ParentIdx || got[i].Size != want[i].Size {
				t.Fatalf("n=%d token %d: got %+v, want %+v", n, i, got[i], want[i])
			}
		}
	}
}

// TestParseParallelCancellationNoDeadlock guards against the under-buffered
// results channel: canceling concurrently with a parse must never wedge the
// worker pool. The watchdog fails fast if a call hangs.
func TestParseParallelCancellationNoDeadlock(t *testing.T) {
	data := topLevelStream(2000) // large enough to take the parallel path
	for i := 0; i < 300; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			_, _ = ParseWithConfig(ctx, data, DefaultConfig())
			close(done)
		}()
		cancel() // race the cancellation against worker startup/execution
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("ParseWithConfig deadlocked on cancellation (iteration %d)", i)
		}
	}
}

func TestParseWithConfigTokenLimit(t *testing.T) {
	data := topLevelStream(300)

	cfg := DefaultConfig()
	cfg.MaxTokens = 50 // far below the real token count

	_, err := ParseWithConfig(context.Background(), data, cfg)
	if !errors.Is(err, ErrTooManyTokens) {
		t.Fatalf("err = %v, want ErrTooManyTokens", err)
	}
}

func TestParseWithConfigInputLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxInputSize = 16

	_, err := ParseWithConfig(context.Background(), topLevelStream(50), cfg)
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("err = %v, want ErrInputTooLarge", err)
	}
}

func TestParseWithConfigCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled before parsing starts

	_, err := ParseWithConfig(ctx, topLevelStream(300), DefaultConfig())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestParseWithConfigEmptyAndNilConfig(t *testing.T) {
	if _, err := ParseWithConfig(context.Background(), nil, nil); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("empty input err = %v, want ErrEmptyInput", err)
	}
	// A nil config must default rather than panic.
	if _, err := ParseWithConfig(context.Background(), []byte(`{"ok":true}`), nil); err != nil {
		t.Fatalf("nil config parse: %v", err)
	}
}
