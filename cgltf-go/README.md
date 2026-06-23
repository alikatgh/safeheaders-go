# cgltf-go

Fast glTF 2.0 3D model parser with parallel batch processing support.

## Status

🟢 **Stable** - Production ready

## Features

- Full glTF 2.0 JSON parsing
- Complete scene graph support (nodes, meshes, materials)
- PBR material properties
- Batch parsing with parallel processing
- Model validation and querying
- Serialization back to JSON
- Zero external dependencies (uses Go stdlib only)

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/cgltf-go
```

## What is glTF?

glTF (GL Transmission Format) is a 3D file format for efficient transmission and loading of 3D models. It's the "JPEG of 3D" - a standard format used in web, AR/VR, games, and 3D applications.

## Quick Start

### Parse glTF File

```go
package main

import (
    "fmt"
    "os"
    "github.com/alikatgh/safeheaders-go/cgltf-go"
)

func main() {
    // Read glTF file
    data, _ := os.ReadFile("model.gltf")

    // Parse glTF
    gltf, err := cgltfgo.Parse(data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("glTF Version: %s\n", gltf.Asset.Version)
    fmt.Printf("Meshes: %d\n", gltf.GetMeshCount())
    fmt.Printf("Nodes: %d\n", gltf.GetNodeCount())
}
```

### Access Scene Data

```go
gltf, _ := cgltfgo.Parse(data)

// Iterate through scenes
for i, scene := range gltf.Scenes {
    fmt.Printf("Scene %d: %s\n", i, scene.Name)
    fmt.Printf("  Nodes: %v\n", scene.Nodes)
}

// Iterate through nodes
for i, node := range gltf.Nodes {
    fmt.Printf("Node %d: %s\n", i, node.Name)
    if len(node.Translation) > 0 {
        fmt.Printf("  Position: %v\n", node.Translation)
    }
    if node.Mesh >= 0 {
        fmt.Printf("  Mesh: %d\n", node.Mesh)
    }
}

// Iterate through meshes
for i, mesh := range gltf.Meshes {
    fmt.Printf("Mesh %d: %s\n", i, mesh.Name)
    fmt.Printf("  Primitives: %d\n", len(mesh.Primitives))
}
```

### Query Meshes

```go
gltf, _ := cgltfgo.Parse(data)

// Get specific mesh
mesh, err := gltf.GetMesh(0)
if err != nil {
    panic(err)
}

fmt.Printf("Mesh: %s\n", mesh.Name)
for i, prim := range mesh.Primitives {
    fmt.Printf("  Primitive %d:\n", i)
    fmt.Printf("    Attributes: %v\n", prim.Attributes)
    fmt.Printf("    Material: %d\n", prim.Material)
}
```

### Validate Model

```go
gltf, _ := cgltfgo.Parse(data)

if err := cgltfgo.ValidateGLTF(gltf); err != nil {
    fmt.Println("Validation error:", err)
} else {
    fmt.Println("Valid glTF 2.0 model")
}
```

## Parallel Batch Processing

Parse multiple glTF files concurrently for maximum throughput:

```go
import "context"

files := []string{"model1.gltf", "model2.gltf", "model3.gltf"}
dataList := make([][]byte, len(files))

for i, file := range files {
    dataList[i], _ = os.ReadFile(file)
}

ctx := context.Background()

// Parse all files in parallel
models, err := cgltfgo.ParseBatch(ctx, dataList)
if err != nil {
    panic(err)
}

for i, model := range models {
    fmt.Printf("Model %d: %d meshes, %d nodes\n",
        i, model.GetMeshCount(), model.GetNodeCount())
}
```

### Context Cancellation

```go
import (
    "context"
    "time"
)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

models, err := cgltfgo.ParseBatch(ctx, dataList)
if err == context.DeadlineExceeded {
    fmt.Println("Parsing timed out")
}
```

## Materials and PBR

Access PBR material properties:

```go
gltf, _ := cgltfgo.Parse(data)

for i, material := range gltf.Materials {
    fmt.Printf("Material %d: %s\n", i, material.Name)

    if pbr := material.PBRMetallicRoughness; pbr != nil {
        fmt.Printf("  Base Color: %v\n", pbr.BaseColorFactor)
        fmt.Printf("  Metallic: %.2f\n", pbr.MetallicFactor)
        fmt.Printf("  Roughness: %.2f\n", pbr.RoughnessFactor)
    }

    fmt.Printf("  Alpha Mode: %s\n", material.AlphaMode)
    fmt.Printf("  Double Sided: %v\n", material.DoubleSided)
}
```

## Serialization

Convert glTF models back to JSON:

```go
gltf, _ := cgltfgo.Parse(data)

// Compact JSON
compact, err := cgltfgo.SerializeGLTF(gltf)
if err != nil {
    panic(err)
}

os.WriteFile("output.gltf", compact, 0644)

// Pretty-printed JSON
pretty, err := cgltfgo.SerializeGLTFIndent(gltf)
if err != nil {
    panic(err)
}

os.WriteFile("output-pretty.gltf", pretty, 0644)
```

## Performance

Throughput depends heavily on input shape, CPU count, and allocator behavior, so
this README does not quote fixed numbers — measure on your target hardware:

```bash
go test -bench=. -benchmem ./...
```

`ParseBatch` parses independent models concurrently. Speedup scales with the number of models, not the size of any single model.

## API Reference

### Data Types

```go
type GLTF struct {
    Asset       Asset
    Scene       int          // Default scene index
    Scenes      []Scene
    Nodes       []Node
    Meshes      []Mesh
    Accessors   []Accessor
    BufferViews []BufferView
    Buffers     []Buffer
    Materials   []Material
}

type Asset struct {
    Version    string
    Generator  string
    Copyright  string
    MinVersion string
}

type Scene struct {
    Name  string
    Nodes []int  // Node indices
}

type Node struct {
    Name        string
    Mesh        int        // Mesh index
    Children    []int      // Child node indices
    Translation []float64  // [x, y, z]
    Rotation    []float64  // [x, y, z, w] quaternion
    Scale       []float64  // [x, y, z]
    Matrix      []float64  // 4x4 transform matrix
}

type Mesh struct {
    Name       string
    Primitives []Primitive
}

type Primitive struct {
    Attributes map[string]int  // "POSITION": accessor_index
    Indices    int             // Accessor index
    Material   int             // Material index
    Mode       int             // GL primitive mode
}

type Material struct {
    Name                 string
    PBRMetallicRoughness *PBRMetallicRoughness
    EmissiveFactor       []float64
    AlphaMode            string  // "OPAQUE", "MASK", "BLEND"
    DoubleSided          bool
}

type PBRMetallicRoughness struct {
    BaseColorFactor []float64  // RGBA [0-1]
    MetallicFactor  float64    // 0-1
    RoughnessFactor float64    // 0-1
}
```

### Functions

```go
// Parse parses glTF JSON data
func Parse(data []byte) (*GLTF, error)

// ValidateGLTF validates a glTF model
func ValidateGLTF(gltf *GLTF) error

// ParseBatch parses multiple glTF files concurrently
func ParseBatch(ctx context.Context, dataList [][]byte) ([]*GLTF, error)

// SerializeGLTF converts glTF to compact JSON
func SerializeGLTF(gltf *GLTF) ([]byte, error)

// SerializeGLTFIndent converts glTF to formatted JSON
func SerializeGLTFIndent(gltf *GLTF) ([]byte, error)
```

### Methods

```go
// GetMeshCount returns number of meshes
func (g *GLTF) GetMeshCount() int

// GetNodeCount returns number of nodes
func (g *GLTF) GetNodeCount() int

// GetMesh returns mesh by index
func (g *GLTF) GetMesh(index int) (*Mesh, error)
```

## glTF Coordinate System

- Right-handed coordinate system
- Y-up (Y axis points up)
- Distances in meters
- Angles in radians
- Quaternions for rotations (x, y, z, w)

## Supported glTF Features

✅ Scenes and nodes
✅ Meshes and primitives
✅ Materials (PBR Metallic-Roughness)
✅ Accessors and buffer views
✅ Node transforms (TRS and matrix)
✅ Multiple scenes

## Limitations

- Only JSON format supported (not GLB binary)
- Buffer data URIs not loaded (structure only)
- Textures not loaded (references only)
- Animations not supported
- Skins/morphs not supported

## When to Use Batch Processing

Use `ParseBatch()` when:
- Loading 10+ glTF files
- Files are small-to-medium (<10MB)
- Maximum throughput needed

Use regular `Parse()` when:
- Loading single file
- File is very large (>100MB)
- Low memory usage preferred

## Thread Safety

- All functions are safe to call concurrently
- `ParseBatch()` automatically uses `runtime.NumCPU()` workers
- Parsed `GLTF` structs are safe to read from multiple goroutines

## Error Handling

```go
gltf, err := cgltfgo.Parse(data)
if err != nil {
    // Handle parse error
    fmt.Println("Failed to parse glTF:", err)
    return
}

if err := cgltfgo.ValidateGLTF(gltf); err != nil {
    // Handle validation error
    fmt.Println("Invalid glTF:", err)
    return
}
```

## Testing

```bash
cd cgltf-go
go test -v
go test -bench . -benchmem
```

## Examples

### Load and Display Model Info

```go
func printModelInfo(filename string) {
    data, _ := os.ReadFile(filename)
    gltf, err := cgltfgo.Parse(data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Model: %s\n", filename)
    fmt.Printf("Generator: %s\n", gltf.Asset.Generator)
    fmt.Printf("Scenes: %d\n", len(gltf.Scenes))
    fmt.Printf("Nodes: %d\n", gltf.GetNodeCount())
    fmt.Printf("Meshes: %d\n", gltf.GetMeshCount())
    fmt.Printf("Materials: %d\n", len(gltf.Materials))

    for i, mesh := range gltf.Meshes {
        primitives := 0
        for _, prim := range mesh.Primitives {
            primitives += len(prim.Attributes)
        }
        fmt.Printf("  Mesh %d: %s (%d primitives)\n",
            i, mesh.Name, primitives)
    }
}
```

## Resources

- [glTF 2.0 Specification](https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html)
- [glTF Sample Models](https://github.com/KhronosGroup/glTF-Sample-Models)
- [glTF Validator](https://github.khronos.org/glTF-Validator/)

## License

MIT - See [LICENSE](../LICENSE)

Based on [cgltf](https://github.com/jkuhlmann/cgltf) by Johannes Kuhlmann (MIT License)
