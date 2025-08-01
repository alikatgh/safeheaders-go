# stb-image-go

> **Zero-dependency, high-performance image loader for Go.**
> A faithful **Go port** of `stb_image.h` (`stb_image 2.x`) with **concurrent batch decoding**, **HDR support**, and **raw pixel access**.

-----

## Project Goals

| Goal | Status |
|---|---|
| **Memory-safe** | [x] No C, no CGO, no buffer overflows |
| **Concurrent** | [x] `LoadBatchConcurrent` uses goroutines |
| **Format Parity** | [ ] Port JPEG, PNG, GIF, TGA, BMP, PSD, HDR |
| **Zero Dependencies** | [ ] Pure Go, no external libraries |

-----

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/stb-image-go
```

-----

## Usage

### Quick Load

```go
img, err := stbimagego.Load(data) // []byte or io.Reader
```

### Batch Decode (Concurrency)

```go
imgs, err := stbimagego.LoadBatchConcurrent(datas) // [][]byte
```

### Raw Pixel Access

```go
rgba, width, height, err := stbimagego.LoadRaw(data) // []uint8, int, int
```

-----

## Current Status

The stable `v0.1.0` tag represents a fast, concurrent loader for standard formats (JPEG, PNG, GIF) that wraps Go's standard library.

The `main` branch now contains the foundational skeleton for a **pure-Go port** of `stb_image.h`. The primary goal is to implement these decoders to achieve full feature-parity with the original C library.

You can follow the porting progress in our main tracking issue: **[#5 Port stb_image.h Decoders](https://github.com/alikatgh/stb-image-go/issues/5)**.

-----

## Performance Targets

The following are target benchmarks for the library.

| Workload | `Load` (Target) | `LoadBatchConcurrent` (Target) | Target Speed-up |
|---|---|---|---|
| 1 MB × 10 JPEG | \~110 ms | \~28 ms | **\~4×** |

Once implemented, you will be able to run benchmarks locally:

```bash
go test -bench=. -benchmem
```

-----

## Contributing

1.  **Port a format** – pick an open issue and open a pull request.
2.  **Add tests** – include sample images in the `testdata/` directory.
3.  **Add benchmarks** – show before and after CPU and memory usage.

-----

## License

MIT