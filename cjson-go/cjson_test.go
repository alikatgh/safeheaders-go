package cjsongo

import (
	"bytes"
	"strings"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"Simple object", `{"key": "value"}`},
		{"Nested object", `{"outer": {"inner": "value"}}`},
		{"Array", `{"items": [1, 2, 3]}`},
		{"Mixed types", `{"string": "text", "number": 42, "bool": true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := Unmarshal([]byte(tt.json), &result)
			if err != nil {
				t.Fatalf("Unmarshal() failed: %v", err)
			}
			if result == nil {
				t.Error("Result is nil")
			}
		})
	}
}

func TestUnmarshal_Errors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"Empty", ""},
		{"Invalid JSON", `{invalid}`},
		{"Unclosed", `{"key": `},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := Unmarshal([]byte(tt.json), &result)
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestUnmarshalToMap(t *testing.T) {
	json := `{"name": "test", "value": 123}`
	m, err := UnmarshalToMap([]byte(json))
	if err != nil {
		t.Fatalf("UnmarshalToMap() failed: %v", err)
	}
	if m["name"] != "test" {
		t.Error("Wrong value for 'name'")
	}
	if m["value"].(float64) != 123 {
		t.Error("Wrong value for 'value'")
	}
}

func TestUnmarshalToSlice(t *testing.T) {
	json := `[1, 2, 3, "four", true]`
	s, err := UnmarshalToSlice([]byte(json))
	if err != nil {
		t.Fatalf("UnmarshalToSlice() failed: %v", err)
	}
	if len(s) != 5 {
		t.Errorf("Expected 5 items, got %d", len(s))
	}
}

func TestMarshal(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"value": 123,
		"items": []int{1, 2, 3},
	}

	result, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("Marshal() returned empty data")
	}

	// Verify it can be unmarshaled back
	var decoded map[string]interface{}
	if err := Unmarshal(result, &decoded); err != nil {
		t.Error("Failed to unmarshal marshaled data")
	}
}

func TestMarshalIndent(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
	}

	result, err := MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() failed: %v", err)
	}

	if !strings.Contains(string(result), "  ") {
		t.Error("Result doesn't contain indentation")
	}
}

func TestUnmarshalStream(t *testing.T) {
	json := `{"key": "value", "number": 42}`
	reader := bytes.NewReader([]byte(json))

	var result map[string]interface{}
	err := UnmarshalStream(reader, &result)
	if err != nil {
		t.Fatalf("UnmarshalStream() failed: %v", err)
	}
	if result["key"] != "value" {
		t.Error("Wrong value")
	}
}

func TestMarshalStream(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
	}

	var buf bytes.Buffer
	err := MarshalStream(&buf, data)
	if err != nil {
		t.Fatalf("MarshalStream() failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("No data written to stream")
	}
}

func TestUnmarshalArrayParallel(t *testing.T) {
	json := `[
		{"id": 1, "name": "item1"},
		{"id": 2, "name": "item2"},
		{"id": 3, "name": "item3"},
		{"id": 4, "name": "item4"},
		{"id": 5, "name": "item5"}
	]`

	results, err := UnmarshalArrayParallel([]byte(json))
	if err != nil {
		t.Fatalf("UnmarshalArrayParallel() failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Expected 5 items, got %d", len(results))
	}

	for i, item := range results {
		expectedID := float64(i + 1)
		if item["id"].(float64) != expectedID {
			t.Errorf("Item %d has wrong ID", i)
		}
	}
}

func TestUnmarshalArrayParallel_Empty(t *testing.T) {
	json := `[]`
	results, err := UnmarshalArrayParallel([]byte(json))
	if err != nil {
		t.Fatalf("UnmarshalArrayParallel() failed: %v", err)
	}
	if len(results) != 0 {
		t.Error("Expected empty result")
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		valid bool
	}{
		{"Valid object", `{"key": "value"}`, true},
		{"Valid array", `[1, 2, 3]`, true},
		{"Invalid", `{invalid}`, false},
		{"Empty", ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Valid([]byte(tt.json))
			if result != tt.valid {
				t.Errorf("Valid() = %v, want %v", result, tt.valid)
			}
		})
	}
}

func TestCompact(t *testing.T) {
	json := `{
		"key": "value",
		"number": 123
	}`

	result, err := Compact([]byte(json))
	if err != nil {
		t.Fatalf("Compact() failed: %v", err)
	}

	// Compacted should have no newlines or extra spaces
	if strings.Contains(string(result), "\n") || strings.Contains(string(result), "  ") {
		t.Error("Result not properly compacted")
	}

	// Should still be valid JSON
	if !Valid(result) {
		t.Error("Compacted result is not valid JSON")
	}
}
