package tinyxml2go

import (
	"strings"
	"testing"
)

// TestParseDepthCeiling: Parse and UnlimitedConfig must reject XML deeper than
// the absolute ceiling with an error rather than crashing with a fatal stack
// overflow (audit M7, L5).
func TestParseDepthCeiling(t *testing.T) {
	depth := maxNestingDepth + 5
	data := []byte(strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth))

	if _, err := Parse(data); err == nil {
		t.Error("Parse accepted XML deeper than the ceiling")
	}
	if _, err := ParseWithConfig(data, UnlimitedConfig()); err == nil {
		t.Error("ParseWithConfig(UnlimitedConfig) accepted over-deep XML")
	}
}

// TestFindDeepIterative confirms the iterative Find*Deep rewrite preserves
// pre-order search semantics (audit L6).
func TestFindDeepIterative(t *testing.T) {
	doc, err := Parse([]byte(`<r><a><b><c>x</c></b></a><c>y</c></r>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Root.FindDeep("c"); got == nil || got.Text != "x" {
		t.Errorf("FindDeep('c') = %v, want first (pre-order) match with text x", got)
	}
	if got := doc.Root.FindAllDeep("c"); len(got) != 2 {
		t.Errorf("FindAllDeep('c') = %d nodes, want 2", len(got))
	}
	if doc.Root.FindDeep("zzz") != nil {
		t.Error("FindDeep of a missing name should be nil")
	}
}
