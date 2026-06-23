# Test Data

This directory contains test data files used for benchmarking and testing across all modules.

## Files

### JSON Test Data

- **small.json** (1KB) - Small JSON object for quick tests
- **medium.json** (100KB) - Medium-sized JSON array for standard benchmarks
- **large.json** (~10MB) - Large JSON array for performance testing — **generated on
  demand, not committed** (run `make testdata`)

### XML Test Data

- **small.xml** (1KB) - Small XML document
- **medium.xml** (50KB) - Medium-sized XML with nested elements
- **large.xml** (~5MB) - Large XML document — **generated on demand, not committed**
  (run `make testdata`)

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

Large fixtures are regenerable and intentionally not committed. Recreate them
from the repository root with:

```bash
make testdata
# or, directly:
go run scripts/generate-testdata.go
```

This writes fresh `large.json` / `large.xml` into this directory. Benchmarks
that need other large fixtures generate them on first run (and skip gracefully
if they cannot).

## License

Test data files are in the public domain or use permissive licenses compatible with the project.
