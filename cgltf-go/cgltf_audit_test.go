package cgltfgo

import "testing"

// TestValidateGLTFReferences covers the reference checks added for audit M1/L1.
func TestValidateGLTFReferences(t *testing.T) {
	cases := []struct {
		name    string
		g       *GLTF
		wantErr bool
	}{
		{"negative mesh", &GLTF{Asset: Asset{Version: "2.0"}, Meshes: []Mesh{{}}, Nodes: []Node{{Mesh: -5}}}, true},
		{"mesh nonzero, no meshes", &GLTF{Asset: Asset{Version: "2.0"}, Nodes: []Node{{Mesh: 3}}}, true},
		{"valid mesh 0", &GLTF{Asset: Asset{Version: "2.0"}, Meshes: []Mesh{{}}, Nodes: []Node{{Mesh: 0}}}, false},
		{"scene node out of range", &GLTF{Asset: Asset{Version: "2.0"}, Scenes: []Scene{{Nodes: []int{9}}}, Nodes: []Node{{}}}, true},
		{"negative child", &GLTF{Asset: Asset{Version: "2.0"}, Nodes: []Node{{Children: []int{-1}}}}, true},
		{"valid child", &GLTF{Asset: Asset{Version: "2.0"}, Nodes: []Node{{Children: []int{1}}, {}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateGLTF(c.g); (err != nil) != c.wantErr {
				t.Errorf("ValidateGLTF() err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}
