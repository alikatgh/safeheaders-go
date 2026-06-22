package tinyxml2go

import (
	"errors"
	"strings"
	"testing"
)

func TestParseWithConfig_NilUsesDefault(t *testing.T) {
	xml := `<?xml version="1.0"?><root attr="val"><child>text</child></root>`
	doc, err := ParseWithConfig([]byte(xml), nil)
	if err != nil {
		t.Fatalf("ParseWithConfig() with nil config failed: %v", err)
	}
	if doc.Root.Name != "root" {
		t.Fatalf("expected root, got %s", doc.Root.Name)
	}
	if doc.Root.Children[0].Text != "text" {
		t.Fatalf("child text mismatch: %q", doc.Root.Children[0].Text)
	}
}

func TestParseWithConfig_EmptyInput(t *testing.T) {
	_, err := ParseWithConfig([]byte(""), DefaultConfig())
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestParseWithConfig_InputTooLarge(t *testing.T) {
	xml := []byte(`<root><child>text</child></root>`)
	cfg := &Config{MaxInputSize: 10} // far smaller than input
	_, err := ParseWithConfig(xml, cfg)
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("expected ErrInputTooLarge, got %v", err)
	}
}

func TestParseWithConfig_InputSizeBoundary(t *testing.T) {
	xml := []byte(`<a/>`)
	// Exactly at the limit must succeed.
	if _, err := ParseWithConfig(xml, &Config{MaxInputSize: len(xml)}); err != nil {
		t.Fatalf("input exactly at MaxInputSize should parse, got %v", err)
	}
	// One byte over the limit must fail.
	if _, err := ParseWithConfig(xml, &Config{MaxInputSize: len(xml) - 1}); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("expected ErrInputTooLarge one byte over limit, got %v", err)
	}
}

func TestParseWithConfig_TooManyNodes(t *testing.T) {
	// 1 root + 3 children = 4 element nodes.
	xml := []byte(`<root><a/><b/><c/></root>`)
	cfg := &Config{MaxNodeCount: 3}
	_, err := ParseWithConfig(xml, cfg)
	if !errors.Is(err, ErrTooManyNodes) {
		t.Fatalf("expected ErrTooManyNodes, got %v", err)
	}

	// Exactly at the node limit must succeed.
	if _, err := ParseWithConfig(xml, &Config{MaxNodeCount: 4}); err != nil {
		t.Fatalf("4 nodes with MaxNodeCount=4 should succeed, got %v", err)
	}
}

func TestParseWithConfig_NestingTooDeep(t *testing.T) {
	// Build deeply nested XML: <n0><n1>...<n49/></n1></n0> = depth 50.
	const depth = 50
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString("<n>")
	}
	for i := 0; i < depth; i++ {
		sb.WriteString("</n>")
	}
	xml := []byte(sb.String())

	// Limit below actual depth must fail with ErrNestingTooDeep.
	if _, err := ParseWithConfig(xml, &Config{MaxNestingDepth: 10}); !errors.Is(err, ErrNestingTooDeep) {
		t.Fatalf("expected ErrNestingTooDeep, got %v", err)
	}

	// Limit at exactly the actual depth must succeed.
	if _, err := ParseWithConfig(xml, &Config{MaxNestingDepth: depth}); err != nil {
		t.Fatalf("depth==limit should succeed, got %v", err)
	}

	// Limit one below the actual depth must fail.
	if _, err := ParseWithConfig(xml, &Config{MaxNestingDepth: depth - 1}); !errors.Is(err, ErrNestingTooDeep) {
		t.Fatalf("expected ErrNestingTooDeep at depth-1, got %v", err)
	}
}

func TestParseWithConfig_UnlimitedAllowsDeepNesting(t *testing.T) {
	const depth = 500
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString("<n>")
	}
	for i := 0; i < depth; i++ {
		sb.WriteString("</n>")
	}
	xml := []byte(sb.String())

	doc, err := ParseWithConfig(xml, UnlimitedConfig())
	if err != nil {
		t.Fatalf("UnlimitedConfig should parse deep nesting, got %v", err)
	}
	// Walk to the bottom to confirm the full tree was built.
	n := doc.Root
	got := 1
	for len(n.Children) == 1 {
		n = n.Children[0]
		got++
	}
	if got != depth {
		t.Fatalf("expected nesting depth %d, walked %d", depth, got)
	}
}

func TestConfig_Presets(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *Config
	}{
		{"Default", DefaultConfig()},
		{"Strict", StrictConfig()},
		{"Unlimited", UnlimitedConfig()},
	} {
		if err := tc.cfg.Validate(); err != nil {
			t.Errorf("%s config should validate, got %v", tc.name, err)
		}
	}

	// Strict is tighter than Default on every dimension.
	d, s := DefaultConfig(), StrictConfig()
	if s.MaxInputSize >= d.MaxInputSize || s.MaxNodeCount >= d.MaxNodeCount || s.MaxNestingDepth >= d.MaxNestingDepth {
		t.Error("StrictConfig should be tighter than DefaultConfig on all limits")
	}
}

func TestConfig_ValidateNegative(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *Config
	}{
		{"NegInputSize", &Config{MaxInputSize: -1}},
		{"NegNodeCount", &Config{MaxNodeCount: -1}},
		{"NegNestingDepth", &Config{MaxNestingDepth: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Error("expected validation error for negative limit")
			}
		})
	}
}

func TestParseWithConfig_InvalidConfigRejected(t *testing.T) {
	_, err := ParseWithConfig([]byte(`<root/>`), &Config{MaxInputSize: -5})
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
	if errors.Is(err, ErrEmptyInput) || errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("expected config validation error, got input error: %v", err)
	}
}

// TestParseWithConfig_MatchesParse confirms ParseWithConfig builds the same
// tree as the legacy Parse for valid input within limits.
func TestParseWithConfig_MatchesParse(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?>
<library>
	<book id="1"><title>A</title></book>
	<book id="2"><title>B</title></book>
</library>`)

	got, err := ParseWithConfig(xml, DefaultConfig())
	if err != nil {
		t.Fatalf("ParseWithConfig() failed: %v", err)
	}
	want, err := Parse(xml)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	gb := got.Root.FindAll("book")
	wb := want.Root.FindAll("book")
	if len(gb) != len(wb) || len(gb) != 2 {
		t.Fatalf("book count mismatch: got %d, want %d", len(gb), len(wb))
	}
	if gb[1].GetAttribute("id") != wb[1].GetAttribute("id") {
		t.Error("attribute mismatch between ParseWithConfig and Parse")
	}
	if gb[0].Find("title").Text != "A" {
		t.Errorf("title mismatch: %q", gb[0].Find("title").Text)
	}
}
