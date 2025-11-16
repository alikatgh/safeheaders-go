# Test Data

This directory contains test data files used for benchmarking and testing across all modules.

## Files

### JSON Test Data

- **small.json** (1KB) - Small JSON object for quick tests
- **medium.json** (100KB) - Medium-sized JSON array for standard benchmarks
- **large.json** (1MB) - Large JSON array with 10,000 objects for performance testing

### XML Test Data

- **small.xml** (1KB) - Small XML document
- **medium.xml** (50KB) - Medium-sized XML with nested elements
- **large.xml** (500KB) - Large XML document with many elements

### Image Test Data

*Note: Binary files not included in repository. Generate using scripts in examples/generate-testdata*

- **test.png** - Sample PNG image (100x100)
- **test.jpg** - Sample JPEG image (100x100)
- **test.gif** - Sample GIF image (100x100)

### Font Test Data

- **test.ttf** - Sample TrueType font file

### Audio Test Data

- **test.wav** - Sample WAV audio file (16-bit PCM, 44.1kHz)

## Usage

```go
package mytest

import (
    "os"
    "testing"
)

func BenchmarkParse(b *testing.B) {
    data, err := os.ReadFile("../testdata/large.json")
    if err != nil {
        b.Fatal(err)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Parse data
    }
}
```

## Generating Test Data

To regenerate test data files, use the scripts in `examples/generate-testdata/`:

```bash
cd examples/generate-testdata
go run generate.go
```

This will create fresh test data files in the testdata directory.

## License

Test data files are in the public domain or use permissive licenses compatible with the project.
