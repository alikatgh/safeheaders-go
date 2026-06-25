// Package cgltfgo provides a pure-Go loader for glTF 2.0 3D models, with
// support for parsing multiple assets concurrently. It is a port of the
// cgltf C library (github.com/jkuhlmann/cgltf).
package cgltfgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
)

// GLTF represents a parsed glTF 3D model.
type GLTF struct {
	Asset       Asset        `json:"asset"`
	Scene       int          `json:"scene,omitempty"`
	Scenes      []Scene      `json:"scenes,omitempty"`
	Nodes       []Node       `json:"nodes,omitempty"`
	Meshes      []Mesh       `json:"meshes,omitempty"`
	Accessors   []Accessor   `json:"accessors,omitempty"`
	BufferViews []BufferView `json:"bufferViews,omitempty"`
	Buffers     []Buffer     `json:"buffers,omitempty"`
	Materials   []Material   `json:"materials,omitempty"`
}

// Asset contains metadata about the glTF asset.
type Asset struct {
	Version    string `json:"version"`
	Generator  string `json:"generator,omitempty"`
	Copyright  string `json:"copyright,omitempty"`
	MinVersion string `json:"minVersion,omitempty"`
}

// Scene represents a scene in the glTF model.
type Scene struct {
	Name  string `json:"name,omitempty"`
	Nodes []int  `json:"nodes,omitempty"`
}

// Node represents a node in the scene graph.
type Node struct {
	Name        string    `json:"name,omitempty"`
	Mesh        int       `json:"mesh,omitempty"`
	Children    []int     `json:"children,omitempty"`
	Translation []float64 `json:"translation,omitempty"`
	Rotation    []float64 `json:"rotation,omitempty"`
	Scale       []float64 `json:"scale,omitempty"`
	Matrix      []float64 `json:"matrix,omitempty"`
}

// Mesh represents a 3D mesh.
type Mesh struct {
	Name       string      `json:"name,omitempty"`
	Primitives []Primitive `json:"primitives"`
}

// Primitive represents a mesh primitive.
type Primitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices,omitempty"`
	Material   int            `json:"material,omitempty"`
	Mode       int            `json:"mode,omitempty"`
}

// Accessor defines how to read data from a buffer view.
type Accessor struct {
	BufferView    int       `json:"bufferView,omitempty"`
	ByteOffset    int       `json:"byteOffset,omitempty"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Max           []float64 `json:"max,omitempty"`
	Min           []float64 `json:"min,omitempty"`
}

// BufferView represents a view into a buffer.
type BufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset,omitempty"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride,omitempty"`
	Target     int `json:"target,omitempty"`
}

// Buffer represents a data buffer.
type Buffer struct {
	ByteLength int    `json:"byteLength"`
	URI        string `json:"uri,omitempty"`
}

// Material represents a material definition.
type Material struct {
	Name                 string                `json:"name,omitempty"`
	PBRMetallicRoughness *PBRMetallicRoughness `json:"pbrMetallicRoughness,omitempty"`
	EmissiveFactor       []float64             `json:"emissiveFactor,omitempty"`
	AlphaMode            string                `json:"alphaMode,omitempty"`
	DoubleSided          bool                  `json:"doubleSided,omitempty"`
}

// PBRMetallicRoughness contains PBR material parameters.
type PBRMetallicRoughness struct {
	BaseColorFactor []float64 `json:"baseColorFactor,omitempty"`
	MetallicFactor  float64   `json:"metallicFactor,omitempty"`
	RoughnessFactor float64   `json:"roughnessFactor,omitempty"`
}

// Parse parses glTF JSON data.
func Parse(data []byte) (*GLTF, error) {
	if len(data) == 0 {
		return nil, errors.New("empty glTF data")
	}

	var gltf GLTF
	if err := json.Unmarshal(data, &gltf); err != nil {
		return nil, fmt.Errorf("failed to parse glTF: %w", err)
	}

	// Validate required fields
	if gltf.Asset.Version == "" {
		return nil, errors.New("missing required field: asset.version")
	}

	return &gltf, nil
}

// ValidateGLTF performs structural sanity checks on a glTF model: the asset
// version, the default-scene index, scene/node graph references, and node mesh
// references. It deliberately does NOT verify the full buffer graph (accessor →
// bufferView → buffer) or material references, so a passing result is not a
// guarantee of complete referential integrity — callers that index into those
// arrays must still bounds-check.
func ValidateGLTF(gltf *GLTF) error {
	if gltf == nil {
		return errors.New("nil glTF")
	}

	// Check version
	if gltf.Asset.Version != "2.0" {
		return fmt.Errorf("unsupported glTF version: %s (only 2.0 supported)", gltf.Asset.Version)
	}

	// Validate the default-scene reference. `scene` is optional in glTF: a model
	// with no scenes (e.g. a mesh library) is valid as long as it doesn't point
	// at a non-existent scene. Note the Go zero value can't distinguish an
	// omitted `scene` from `scene: 0`.
	if len(gltf.Scenes) > 0 {
		if gltf.Scene < 0 || gltf.Scene >= len(gltf.Scenes) {
			return fmt.Errorf("invalid scene index: %d", gltf.Scene)
		}
	} else if gltf.Scene != 0 {
		return fmt.Errorf("scene index %d set but no scenes are defined", gltf.Scene)
	}

	// Scene node lists are explicit references into Nodes.
	for si, scene := range gltf.Scenes {
		for _, n := range scene.Nodes {
			if n < 0 || n >= len(gltf.Nodes) {
				return fmt.Errorf("scene %d references invalid node: %d", si, n)
			}
		}
	}

	// Validate node references (mesh index and child node indices).
	for i, node := range gltf.Nodes {
		// mesh is optional; reject negative and out-of-range, allowing the
		// "omitted" zero only when no meshes are defined.
		if len(gltf.Meshes) > 0 {
			if node.Mesh < 0 || node.Mesh >= len(gltf.Meshes) {
				return fmt.Errorf("node %d references invalid mesh: %d", i, node.Mesh)
			}
		} else if node.Mesh != 0 {
			return fmt.Errorf("node %d references mesh %d but no meshes are defined", i, node.Mesh)
		}
		for _, c := range node.Children {
			if c < 0 || c >= len(gltf.Nodes) {
				return fmt.Errorf("node %d references invalid child: %d", i, c)
			}
		}
	}

	return nil
}

// GetMeshCount returns the number of meshes in the model.
func (g *GLTF) GetMeshCount() int {
	return len(g.Meshes)
}

// GetNodeCount returns the number of nodes in the model.
func (g *GLTF) GetNodeCount() int {
	return len(g.Nodes)
}

// GetMesh returns a mesh by index.
func (g *GLTF) GetMesh(index int) (*Mesh, error) {
	if index < 0 || index >= len(g.Meshes) {
		return nil, fmt.Errorf("invalid mesh index: %d", index)
	}
	return &g.Meshes[index], nil
}

// ParseBatch parses multiple glTF files concurrently.
func ParseBatch(ctx context.Context, dataList [][]byte) ([]*GLTF, error) {
	if len(dataList) == 0 {
		return nil, errors.New("empty data list")
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(dataList) {
		numWorkers = len(dataList)
	}

	type result struct {
		gltf  *GLTF
		err   error
		index int
	}

	dataChan := make(chan struct {
		data  []byte
		index int
	}, len(dataList))
	resultChan := make(chan result, len(dataList))

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case work, ok := <-dataChan:
					if !ok {
						return
					}

					gltf, err := Parse(work.data)
					resultChan <- result{
						gltf:  gltf,
						err:   err,
						index: work.index,
					}
				}
			}
		}()
	}

	// Send work
	go func() {
		for i, data := range dataList {
			select {
			case <-ctx.Done():
				close(dataChan)
				return
			case dataChan <- struct {
				data  []byte
				index int
			}{data, i}:
			}
		}
		close(dataChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]*GLTF, len(dataList))
	for res := range resultChan {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if res.err != nil {
			return nil, fmt.Errorf("failed to parse glTF at index %d: %w", res.index, res.err)
		}
		results[res.index] = res.gltf
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return results, nil
}

// SerializeGLTF converts a glTF model to JSON.
func SerializeGLTF(gltf *GLTF) ([]byte, error) {
	if gltf == nil {
		return nil, errors.New("nil glTF")
	}

	data, err := json.Marshal(gltf)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize glTF: %w", err)
	}

	return data, nil
}

// SerializeGLTFIndent converts a glTF model to indented JSON.
func SerializeGLTFIndent(gltf *GLTF) ([]byte, error) {
	if gltf == nil {
		return nil, errors.New("nil glTF")
	}

	data, err := json.MarshalIndent(gltf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize glTF: %w", err)
	}

	return data, nil
}
