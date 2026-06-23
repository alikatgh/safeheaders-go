package cgltfgo

import "testing"

// FuzzParse ensures Parse and the downstream validation/accessor/serialize paths
// survive arbitrary input without panicking (e.g. out-of-range mesh indexing).
func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"asset":{"version":"2.0"},"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}]}`))
	f.Add([]byte(`{"asset":{"version":"2.0"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`{"asset":{"version":"2.0"},"scene":99,"scenes":[]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		g, err := Parse(data)
		if err != nil || g == nil {
			return
		}
		_ = ValidateGLTF(g)
		n := g.GetMeshCount()
		_ = g.GetNodeCount()
		// Exercise the bounds checks on either side of the valid range.
		for i := -1; i <= n; i++ {
			_, _ = g.GetMesh(i)
		}
		_, _ = SerializeGLTF(g)
		_, _ = SerializeGLTFIndent(g)
	})
}
