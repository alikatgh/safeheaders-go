package cgltfgo

import (
	"context"
	"testing"
	"time"
)

func createTestGLTF() []byte {
	return []byte(`{
		"asset": {
			"version": "2.0",
			"generator": "test"
		},
		"scene": 0,
		"scenes": [
			{
				"name": "TestScene",
				"nodes": [0]
			}
		],
		"nodes": [
			{
				"name": "TestNode",
				"mesh": 0
			}
		],
		"meshes": [
			{
				"name": "TestMesh",
				"primitives": [
					{
						"attributes": {
							"POSITION": 0
						}
					}
				]
			}
		],
		"accessors": [
			{
				"bufferView": 0,
				"componentType": 5126,
				"count": 3,
				"type": "VEC3"
			}
		],
		"bufferViews": [
			{
				"buffer": 0,
				"byteLength": 36
			}
		],
		"buffers": [
			{
				"byteLength": 36
			}
		]
	}`)
}

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantError bool
	}{
		{
			name:      "Valid glTF",
			data:      createTestGLTF(),
			wantError: false,
		},
		{
			name:      "Empty data",
			data:      []byte{},
			wantError: true,
		},
		{
			name:      "Invalid JSON",
			data:      []byte("{invalid json}"),
			wantError: true,
		},
		{
			name:      "Missing version",
			data:      []byte(`{"asset": {}}`),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gltf, err := Parse(tt.data)
			if (err != nil) != tt.wantError {
				t.Fatalf("Parse() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && gltf == nil {
				t.Fatal("Expected non-nil glTF")
			}
		})
	}
}

func TestValidateGLTF(t *testing.T) {
	validData := createTestGLTF()
	validGLTF, err := Parse(validData)
	if err != nil {
		t.Fatalf("Failed to parse valid glTF: %v", err)
	}

	tests := []struct {
		name      string
		gltf      *GLTF
		wantError bool
	}{
		{
			name:      "Valid glTF",
			gltf:      validGLTF,
			wantError: false,
		},
		{
			name:      "Nil glTF",
			gltf:      nil,
			wantError: true,
		},
		{
			name: "Invalid version",
			gltf: &GLTF{
				Asset: Asset{Version: "1.0"},
			},
			wantError: true,
		},
		{
			name: "Invalid scene index",
			gltf: &GLTF{
				Asset:  Asset{Version: "2.0"},
				Scene:  5,
				Scenes: []Scene{},
			},
			wantError: true,
		},
		{
			// `scene` omitted (zero) with no scenes is a valid mesh-library glTF.
			name: "No scenes, scene omitted",
			gltf: &GLTF{
				Asset: Asset{Version: "2.0"},
			},
			wantError: false,
		},
		{
			name: "Scene in range",
			gltf: &GLTF{
				Asset:  Asset{Version: "2.0"},
				Scene:  1,
				Scenes: []Scene{{Name: "a"}, {Name: "b"}},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGLTF(tt.gltf)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateGLTF() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGLTF_GetMeshCount(t *testing.T) {
	data := createTestGLTF()
	gltf, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse glTF: %v", err)
	}

	count := gltf.GetMeshCount()
	if count != 1 {
		t.Errorf("Expected 1 mesh, got %d", count)
	}
}

func TestGLTF_GetNodeCount(t *testing.T) {
	data := createTestGLTF()
	gltf, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse glTF: %v", err)
	}

	count := gltf.GetNodeCount()
	if count != 1 {
		t.Errorf("Expected 1 node, got %d", count)
	}
}

func TestGLTF_GetMesh(t *testing.T) {
	data := createTestGLTF()
	gltf, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse glTF: %v", err)
	}

	tests := []struct {
		name      string
		index     int
		wantError bool
	}{
		{"Valid index", 0, false},
		{"Negative index", -1, true},
		{"Out of bounds", 999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mesh, err := gltf.GetMesh(tt.index)
			if (err != nil) != tt.wantError {
				t.Fatalf("GetMesh() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && mesh == nil {
				t.Fatal("Expected non-nil mesh")
			}
		})
	}
}

func TestParseBatch(t *testing.T) {
	gltf1 := createTestGLTF()
	gltf2 := []byte(`{
		"asset": {"version": "2.0"},
		"meshes": []
	}`)
	gltf3 := []byte(`{
		"asset": {"version": "2.0", "generator": "another test"},
		"nodes": []
	}`)

	dataList := [][]byte{gltf1, gltf2, gltf3}

	ctx := context.Background()
	results, err := ParseBatch(ctx, dataList)
	if err != nil {
		t.Fatalf("ParseBatch() error = %v", err)
	}

	if len(results) != len(dataList) {
		t.Fatalf("Expected %d results, got %d", len(dataList), len(results))
	}

	for i, result := range results {
		if result == nil {
			t.Errorf("Result %d is nil", i)
		}
	}
}

func TestParseBatch_Errors(t *testing.T) {
	tests := []struct {
		name      string
		dataList  [][]byte
		wantError bool
	}{
		{
			name:      "Empty list",
			dataList:  [][]byte{},
			wantError: true,
		},
		{
			name: "Contains invalid data",
			dataList: [][]byte{
				createTestGLTF(),
				[]byte("invalid"),
			},
			wantError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBatch(ctx, tt.dataList)
			if (err != nil) != tt.wantError {
				t.Fatalf("ParseBatch() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseBatch_Context(t *testing.T) {
	// Create many test files
	dataList := make([][]byte, 100)
	for i := range dataList {
		dataList[i] = createTestGLTF()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Ensure timeout

	_, err := ParseBatch(ctx, dataList)
	if err == nil {
		t.Fatal("Expected context timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestSerializeGLTF(t *testing.T) {
	data := createTestGLTF()
	gltf, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse glTF: %v", err)
	}

	serialized, err := SerializeGLTF(gltf)
	if err != nil {
		t.Fatalf("SerializeGLTF() error = %v", err)
	}

	if len(serialized) == 0 {
		t.Fatal("Expected non-empty serialized data")
	}

	// Verify it can be parsed back
	reparsed, err := Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to reparse serialized glTF: %v", err)
	}

	if reparsed.Asset.Version != gltf.Asset.Version {
		t.Error("Version mismatch after round-trip")
	}
}

func TestSerializeGLTF_Nil(t *testing.T) {
	_, err := SerializeGLTF(nil)
	if err == nil {
		t.Fatal("Expected error for nil glTF")
	}
}

func TestSerializeGLTFIndent(t *testing.T) {
	data := createTestGLTF()
	gltf, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse glTF: %v", err)
	}

	serialized, err := SerializeGLTFIndent(gltf)
	if err != nil {
		t.Fatalf("SerializeGLTFIndent() error = %v", err)
	}

	if len(serialized) == 0 {
		t.Fatal("Expected non-empty serialized data")
	}

	// Indented should be longer than compact
	compact, _ := SerializeGLTF(gltf)
	if len(serialized) <= len(compact) {
		t.Error("Expected indented format to be longer than compact")
	}
}

func TestRoundTrip(t *testing.T) {
	// Parse original
	original := createTestGLTF()
	gltf1, err := Parse(original)
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	// Validate
	if err := ValidateGLTF(gltf1); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Serialize
	serialized, err := SerializeGLTF(gltf1)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	// Parse again
	gltf2, err := Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized: %v", err)
	}

	// Compare key fields
	if gltf1.Asset.Version != gltf2.Asset.Version {
		t.Error("Version mismatch")
	}
	if gltf1.GetMeshCount() != gltf2.GetMeshCount() {
		t.Error("Mesh count mismatch")
	}
	if gltf1.GetNodeCount() != gltf2.GetNodeCount() {
		t.Error("Node count mismatch")
	}
}

func TestComplexGLTF(t *testing.T) {
	complexGLTF := []byte(`{
		"asset": {
			"version": "2.0",
			"generator": "Complex Test"
		},
		"scene": 0,
		"scenes": [
			{
				"name": "Scene",
				"nodes": [0, 1, 2]
			}
		],
		"nodes": [
			{
				"name": "Node1",
				"mesh": 0,
				"translation": [1.0, 2.0, 3.0]
			},
			{
				"name": "Node2",
				"mesh": 1,
				"rotation": [0.0, 0.0, 0.0, 1.0]
			},
			{
				"name": "Node3",
				"children": [0, 1],
				"scale": [1.0, 1.0, 1.0]
			}
		],
		"meshes": [
			{
				"name": "Mesh1",
				"primitives": [
					{
						"attributes": {"POSITION": 0, "NORMAL": 1},
						"indices": 2,
						"material": 0
					}
				]
			},
			{
				"name": "Mesh2",
				"primitives": [
					{
						"attributes": {"POSITION": 3}
					}
				]
			}
		],
		"materials": [
			{
				"name": "Material1",
				"pbrMetallicRoughness": {
					"baseColorFactor": [1.0, 0.5, 0.0, 1.0],
					"metallicFactor": 0.8,
					"roughnessFactor": 0.2
				},
				"doubleSided": true
			}
		]
	}`)

	gltf, err := Parse(complexGLTF)
	if err != nil {
		t.Fatalf("Failed to parse complex glTF: %v", err)
	}

	if gltf.GetMeshCount() != 2 {
		t.Errorf("Expected 2 meshes, got %d", gltf.GetMeshCount())
	}

	if gltf.GetNodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", gltf.GetNodeCount())
	}

	if len(gltf.Materials) != 1 {
		t.Errorf("Expected 1 material, got %d", len(gltf.Materials))
	}

	// Verify node properties
	if len(gltf.Nodes[0].Translation) != 3 {
		t.Error("Node1 should have translation")
	}

	if len(gltf.Nodes[1].Rotation) != 4 {
		t.Error("Node2 should have rotation")
	}
}
