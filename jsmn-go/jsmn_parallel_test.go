package jsmngo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
