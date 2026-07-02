# dr-wav-go

Fast WAV audio file parser with concurrent batch processing support.

## Status

🟢 **Stable** - Production ready

## Features

- Full WAV/RIFF header parsing
- Support for PCM audio (8, 16, 24, 32-bit)
- Mono and multi-channel audio
- Channel extraction (de-interleave)
- Batch parsing with parallel processing
- WAV file serialization (write)
- Audio metadata (duration, sample count)
- Zero external dependencies (uses Go stdlib only)

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/dr-wav-go
```

## What is WAV?

WAV (Waveform Audio File Format) is a standard audio file format for storing uncompressed PCM audio data. It's widely used for high-quality audio recording and editing.

## Quick Start

### Parse WAV File

```go
package main

import (
    "fmt"
    "os"
    "github.com/alikatgh/safeheaders-go/dr-wav-go"
)

func main() {
    // Read WAV file
    data, _ := os.ReadFile("audio.wav")

    // Parse WAV
    wav, err := drwavgo.Parse(data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Sample Rate: %d Hz\n", wav.Header.SampleRate)
    fmt.Printf("Channels: %d\n", wav.Header.NumChannels)
    fmt.Printf("Bit Depth: %d-bit\n", wav.Header.BitsPerSample)
    fmt.Printf("Duration: %.2f seconds\n", wav.GetDuration())
    fmt.Printf("Samples: %d\n", wav.GetSampleCount())
}
```

### Access Audio Data

```go
wav, _ := drwavgo.Parse(data)

// Access header info
fmt.Printf("Audio Format: %d (1=PCM)\n", wav.Header.AudioFormat)
fmt.Printf("Byte Rate: %d bytes/sec\n", wav.Header.ByteRate)
fmt.Printf("Block Align: %d\n", wav.Header.BlockAlign)

// Access raw PCM data
pcmData := wav.Data
fmt.Printf("PCM Data Size: %d bytes\n", len(pcmData))

// Process samples...
```

### Validate WAV File

```go
wav, _ := drwavgo.Parse(data)

if err := drwavgo.ValidateWAV(wav); err != nil {
    fmt.Println("Invalid WAV:", err)
} else {
    fmt.Println("Valid PCM WAV file")
}
```

## Channel Extraction

Extract individual channels from stereo or multi-channel audio:

```go
// Parse stereo WAV file
data, _ := os.ReadFile("stereo.wav")
wav, _ := drwavgo.Parse(data)

// Extract left and right channels
channels, err := wav.ExtractChannels()
if err != nil {
    panic(err)
}

leftChannel := channels[0]   // Left channel PCM data
rightChannel := channels[1]  // Right channel PCM data

fmt.Printf("Left: %d bytes\n", len(leftChannel))
fmt.Printf("Right: %d bytes\n", len(rightChannel))

// Save individual channels
os.WriteFile("left.raw", leftChannel, 0644)
os.WriteFile("right.raw", rightChannel, 0644)
```

## Parallel Batch Processing

Parse multiple WAV files concurrently:

```go
import "context"

files := []string{"audio1.wav", "audio2.wav", "audio3.wav"}
dataList := make([][]byte, len(files))

for i, file := range files {
    dataList[i], _ = os.ReadFile(file)
}

ctx := context.Background()

// Parse all files in parallel
wavs, err := drwavgo.ParseBatch(ctx, dataList)
if err != nil {
    panic(err)
}

for i, wav := range wavs {
    fmt.Printf("File %d: %.2fs, %d Hz, %d channels\n",
        i, wav.GetDuration(),
        wav.Header.SampleRate,
        wav.Header.NumChannels)
}
```

`drwavgo.MaxBatchSize` (default 10,000, set to `0` to disable) rejects a
`ParseBatch` call outright if handed more files than that — many small-but-valid
files can still exhaust memory/CPU in aggregate even though each one parses fine
on its own.

### Context Cancellation

```go
import (
    "context"
    "time"
)

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

wavs, err := drwavgo.ParseBatch(ctx, dataList)
if err == context.DeadlineExceeded {
    fmt.Println("Parsing timed out")
}
```

## WAV File Creation

Write WAV files from PCM data:

```go
// Create WAV structure
wav := &drwavgo.WAV{
    Header: drwavgo.WAVHeader{
        AudioFormat:   1,      // PCM
        NumChannels:   2,      // Stereo
        SampleRate:    44100,  // 44.1 kHz
        BitsPerSample: 16,     // 16-bit
        ByteRate:      44100 * 2 * 2,  // SampleRate * Channels * BytesPerSample
        BlockAlign:    4,      // Channels * BytesPerSample
    },
    Data: pcmData,  // Your PCM audio data
}

// Serialize to WAV format
wavBytes, err := drwavgo.Serialize(wav)
if err != nil {
    panic(err)
}

// Save to file
os.WriteFile("output.wav", wavBytes, 0644)
```

## Audio Metadata

```go
wav, _ := drwavgo.Parse(data)

// Get duration in seconds
duration := wav.GetDuration()
fmt.Printf("Duration: %.2f seconds\n", duration)

// Get total number of samples (per channel)
samples := wav.GetSampleCount()
fmt.Printf("Samples per channel: %d\n", samples)

// Calculate file size
fileSize := 44 + len(wav.Data)  // Header + PCM data
fmt.Printf("File size: %d bytes\n", fileSize)
```

## Performance

Throughput depends heavily on input shape, CPU count, and allocator behavior, so
this README does not quote fixed numbers — measure on your target hardware:

```bash
go test -bench=. -benchmem ./...
```

`ParseBatch` decodes independent files concurrently. Speedup scales with the number of files.

## API Reference

### Types

```go
type WAVHeader struct {
    AudioFormat   uint16  // 1 = PCM
    NumChannels   uint16  // 1 = mono, 2 = stereo, etc.
    SampleRate    uint32  // Samples per second (Hz)
    ByteRate      uint32  // Bytes per second
    BlockAlign    uint16  // Bytes per sample across all channels
    BitsPerSample uint16  // 8, 16, 24, or 32
}

type WAV struct {
    Header WAVHeader
    Data   []byte  // Raw PCM audio data
}
```

### Functions

```go
// Parse parses WAV file data
func Parse(data []byte) (*WAV, error)

// ValidateWAV validates WAV structure
func ValidateWAV(wav *WAV) error

// ParseBatch parses multiple WAV files concurrently
func ParseBatch(ctx context.Context, dataList [][]byte) ([]*WAV, error)

// Serialize converts WAV to file format
func Serialize(wav *WAV) ([]byte, error)
```

### Methods

```go
// GetDuration returns audio duration in seconds
func (w *WAV) GetDuration() float64

// GetSampleCount returns total samples per channel
func (w *WAV) GetSampleCount() int

// ExtractChannels splits multi-channel audio
func (w *WAV) ExtractChannels() ([][]byte, error)
```

## Supported Formats

### Audio Formats
- ✅ PCM (uncompressed)
- ❌ ADPCM, MP3, AAC (not supported)

### Bit Depths
- ✅ 8-bit (unsigned)
- ✅ 16-bit (signed)
- ✅ 24-bit (signed)
- ✅ 32-bit (signed/float)

### Channel Configurations
- ✅ Mono (1 channel)
- ✅ Stereo (2 channels)
- ✅ Multi-channel (3+ channels)

### Sample Rates
All standard rates supported:
- 8000 Hz (telephone)
- 16000 Hz (wideband)
- 22050 Hz (radio)
- 44100 Hz (CD quality)
- 48000 Hz (professional)
- 96000 Hz (high-res)
- 192000 Hz (ultra high-res)

## Audio Data Format

PCM data is stored as interleaved samples:

**Mono (1 channel):**
```
[Sample1] [Sample2] [Sample3] ...
```

**Stereo (2 channels):**
```
[L1] [R1] [L2] [R2] [L3] [R3] ...
```

**Multi-channel (N channels):**
```
[Ch1_S1] [Ch2_S1] ... [ChN_S1] [Ch1_S2] [Ch2_S2] ... [ChN_S2] ...
```

## When to Use Batch Processing

Use `ParseBatch()` when:
- Loading 10+ WAV files
- Files are small-to-medium (<50MB)
- Maximum throughput needed

Use regular `Parse()` when:
- Loading single file
- File is very large (>500MB)
- Low memory usage preferred

## Thread Safety

- All functions are safe to call concurrently
- `ParseBatch()` automatically uses `runtime.NumCPU()` workers
- Parsed `WAV` structs are safe to read from multiple goroutines

## Error Handling

```go
wav, err := drwavgo.Parse(data)
if err != nil {
    // Handle parse error
    fmt.Println("Failed to parse WAV:", err)
    return
}

if err := drwavgo.ValidateWAV(wav); err != nil {
    // Handle validation error
    fmt.Println("Invalid WAV:", err)
    return
}
```

## Testing

```bash
cd dr-wav-go
go test -v
go test -bench . -benchmem
```

## Examples

### Convert Stereo to Mono

```go
func stereoToMono(stereoData []byte, wav *drwavgo.WAV) ([]byte, error) {
    channels, err := wav.ExtractChannels()
    if err != nil {
        return nil, err
    }

    left := channels[0]
    right := channels[1]

    // Average left and right channels
    mono := make([]byte, len(left))
    for i := 0; i < len(left); i += 2 {
        l := int16(binary.LittleEndian.Uint16(left[i:]))
        r := int16(binary.LittleEndian.Uint16(right[i:]))
        avg := (l + r) / 2
        binary.LittleEndian.PutUint16(mono[i:], uint16(avg))
    }

    return mono, nil
}
```

### Analyze Audio Levels

```go
func analyzeAudio(wav *drwavgo.WAV) {
    if wav.Header.BitsPerSample != 16 {
        fmt.Println("Only 16-bit supported for this example")
        return
    }

    var maxAmplitude int16
    for i := 0; i < len(wav.Data); i += 2 {
        sample := int16(binary.LittleEndian.Uint16(wav.Data[i:]))
        if sample < 0 {
            sample = -sample
        }
        if sample > maxAmplitude {
            maxAmplitude = sample
        }
    }

    // Calculate dB relative to max
    percentage := float64(maxAmplitude) / float64(32767) * 100
    fmt.Printf("Peak amplitude: %.1f%%\n", percentage)
}
```

### Create Sine Wave

```go
import "math"

func createSineWave(frequency, duration float64, sampleRate int) *drwavgo.WAV {
    samples := int(duration * float64(sampleRate))
    data := make([]byte, samples*2)  // 16-bit = 2 bytes per sample

    for i := 0; i < samples; i++ {
        t := float64(i) / float64(sampleRate)
        value := math.Sin(2 * math.Pi * frequency * t)
        sample := int16(value * 32767)  // Scale to 16-bit range
        binary.LittleEndian.PutUint16(data[i*2:], uint16(sample))
    }

    return &drwavgo.WAV{
        Header: drwavgo.WAVHeader{
            AudioFormat:   1,
            NumChannels:   1,
            SampleRate:    uint32(sampleRate),
            BitsPerSample: 16,
            ByteRate:      uint32(sampleRate * 2),
            BlockAlign:    2,
        },
        Data: data,
    }
}
```

## Limitations

- Only PCM format supported (no compression)
- Maximum file size: 4GB (WAV format limit)
- Floating-point PCM requires manual handling
- BWF metadata not parsed
- No automatic resampling

## Resources

- [WAV File Format](http://soundfile.sapp.org/doc/WaveFormat/)
- [RIFF Specification](https://www.mmsp.ece.mcgill.ca/Documents/AudioFormats/WAVE/WAVE.html)
- [Audio Programming](https://www.objc.io/issues/24-audio/)

## License

MIT - See [LICENSE](../LICENSE)

Based on [dr_wav](https://github.com/mackron/dr_libs) by David Reid (Public Domain)
