# stb-image-go

[](https://goreportcard.com/report/github.com/alikatgh/stb-image-go)
[](https://github.com/alikatgh/stb-image-go/actions/workflows/go.yml)

> **Zero-dependency, high-performance image loader for Go.**
> A faithful **Go port** of `stb_image.h` (`stb_image 2.x`) with **concurrent batch decoding**, **HDR support**, and **raw pixel access**.

-----

## Project Goals

| Goal | Status |
|---|---|
| **Memory-safe** | [ ] No C, no CGO, no buffer overflows |
| **Concurrent** | [ ] `LoadBatchConcurrent` uses goroutines |
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

| Format | Bits / Channel | Status |
|---|---|---|
| JPEG | 8 / 16 / 24 | [ ] Planned |
| PNG  | 8 / 16       | [ ] Planned |
| GIF  | 8            | [ ] Planned |
| TGA  | 8 / 16 / 24  | [ ] Planned ([\#2](https://github.com/alikatgh/stb-image-go/issues/2)) |
| BMP  | 8 / 16 / 24  | [ ] Planned ([\#3](https://github.com/alikatgh/stb-image-go/issues/3)) |
| HDR  | 32-bit float | [ ] Planned ([\#4](https://github.com/alikatgh/stb-image-go/issues/4)) |

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