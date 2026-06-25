package cjsongo

import "testing"

// TestUnmarshalArrayParallelItemCap verifies the memory-amplification guard
// (audit M2).
func TestUnmarshalArrayParallelItemCap(t *testing.T) {
	orig := MaxArrayItems
	defer func() { MaxArrayItems = orig }()

	MaxArrayItems = 5
	if _, err := UnmarshalArrayParallel([]byte("[0,0,0,0,0,0,0,0,0,0]")); err == nil {
		t.Error("expected an error for an array exceeding MaxArrayItems")
	}

	MaxArrayItems = 0 // disabled
	if _, err := UnmarshalArrayParallel([]byte(`[{"a":1},{"b":2}]`)); err != nil {
		t.Errorf("with the cap disabled: %v", err)
	}
}
