// Package drwavgo provides a pure-Go parser and serializer for WAV (RIFF) audio
// files, with support for decoding multiple files concurrently. It is a port of
// the dr_wav C library (github.com/mackron/dr_libs).
package drwavgo

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
)

// WAVHeader represents the WAV file header.
type WAVHeader struct {
	AudioFormat   uint16 // 1 = PCM
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
}

// WAV represents a parsed WAV audio file.
type WAV struct {
	Header WAVHeader
	Data   []byte // Raw PCM data
}

// Parse parses WAV file data.
func Parse(data []byte) (*WAV, error) {
	if len(data) < 44 {
		return nil, errors.New("data too short for WAV header")
	}

	r := bytes.NewReader(data)

	// Read RIFF header
	var riff [4]byte
	if err := binary.Read(r, binary.LittleEndian, &riff); err != nil {
		return nil, fmt.Errorf("failed to read RIFF: %w", err)
	}
	if string(riff[:]) != "RIFF" {
		return nil, errors.New("invalid RIFF header")
	}

	// Read chunk size
	var chunkSize uint32
	if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
		return nil, fmt.Errorf("failed to read chunk size: %w", err)
	}

	// Read WAVE header
	var wave [4]byte
	if err := binary.Read(r, binary.LittleEndian, &wave); err != nil {
		return nil, fmt.Errorf("failed to read WAVE: %w", err)
	}
	if string(wave[:]) != "WAVE" {
		return nil, errors.New("invalid WAVE header")
	}

	// Read fmt subchunk
	var fmtTag [4]byte
	if err := binary.Read(r, binary.LittleEndian, &fmtTag); err != nil {
		return nil, fmt.Errorf("failed to read fmt: %w", err)
	}
	if string(fmtTag[:]) != "fmt " {
		return nil, errors.New("invalid fmt subchunk")
	}

	var subchunk1Size uint32
	if err := binary.Read(r, binary.LittleEndian, &subchunk1Size); err != nil {
		return nil, fmt.Errorf("failed to read subchunk1 size: %w", err)
	}

	// Read format details
	var header WAVHeader
	if err := binary.Read(r, binary.LittleEndian, &header.AudioFormat); err != nil {
		return nil, fmt.Errorf("failed to read audio format: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.NumChannels); err != nil {
		return nil, fmt.Errorf("failed to read num channels: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.SampleRate); err != nil {
		return nil, fmt.Errorf("failed to read sample rate: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.ByteRate); err != nil {
		return nil, fmt.Errorf("failed to read byte rate: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.BlockAlign); err != nil {
		return nil, fmt.Errorf("failed to read block align: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.BitsPerSample); err != nil {
		return nil, fmt.Errorf("failed to read bits per sample: %w", err)
	}

	// Skip any extra format bytes
	if subchunk1Size > 16 {
		extra := make([]byte, subchunk1Size-16)
		if _, err := r.Read(extra); err != nil {
			return nil, fmt.Errorf("failed to skip extra format bytes: %w", err)
		}
	}

	pcmData, err := readDataChunk(r)
	if err != nil {
		return nil, err
	}
	return &WAV{Header: header, Data: pcmData}, nil
}

// readDataChunk scans subchunks until it finds the "data" chunk and returns its
// PCM payload. The allocation is capped at the bytes actually remaining in the
// reader so a malformed or malicious header that declares a huge data size
// cannot trigger an out-of-memory allocation.
func readDataChunk(r *bytes.Reader) ([]byte, error) {
	for {
		var subchunkID [4]byte
		if err := binary.Read(r, binary.LittleEndian, &subchunkID); err != nil {
			return nil, fmt.Errorf("failed to find data subchunk: %w", err)
		}

		var subchunkSize uint32
		if err := binary.Read(r, binary.LittleEndian, &subchunkSize); err != nil {
			return nil, fmt.Errorf("failed to read subchunk size: %w", err)
		}

		if string(subchunkID[:]) == "data" {
			allocSize := int(subchunkSize)
			if allocSize > r.Len() {
				allocSize = r.Len() // never trust the declared size past EOF
			}
			pcmData := make([]byte, allocSize)
			if _, err := io.ReadFull(r, pcmData); err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read PCM data: %w", err)
			}
			return pcmData, nil
		}

		// Skip this subchunk.
		if _, err := r.Seek(int64(subchunkSize), io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("failed to skip subchunk: %w", err)
		}
	}
}

// GetDuration returns the duration of the audio in seconds.
func (w *WAV) GetDuration() float64 {
	if w.Header.ByteRate == 0 {
		return 0
	}
	return float64(len(w.Data)) / float64(w.Header.ByteRate)
}

// GetSampleCount returns the total number of samples.
func (w *WAV) GetSampleCount() int {
	bytesPerSample := int(w.Header.BitsPerSample) / 8
	if bytesPerSample == 0 {
		return 0
	}
	return len(w.Data) / bytesPerSample / int(w.Header.NumChannels)
}

// ValidateWAV performs basic validation on WAV data.
func ValidateWAV(wav *WAV) error {
	if wav == nil {
		return errors.New("nil WAV")
	}

	// Check audio format (1 = PCM)
	if wav.Header.AudioFormat != 1 {
		return fmt.Errorf("unsupported audio format: %d (only PCM supported)", wav.Header.AudioFormat)
	}

	// Check channels
	if wav.Header.NumChannels == 0 {
		return errors.New("invalid number of channels: 0")
	}

	// Check sample rate
	if wav.Header.SampleRate == 0 {
		return errors.New("invalid sample rate: 0")
	}

	// Check bits per sample
	if wav.Header.BitsPerSample != 8 && wav.Header.BitsPerSample != 16 &&
		wav.Header.BitsPerSample != 24 && wav.Header.BitsPerSample != 32 {
		return fmt.Errorf("unsupported bits per sample: %d", wav.Header.BitsPerSample)
	}

	return nil
}

// ParseBatch parses multiple WAV files concurrently.
func ParseBatch(ctx context.Context, dataList [][]byte) ([]*WAV, error) {
	if len(dataList) == 0 {
		return nil, errors.New("empty data list")
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(dataList) {
		numWorkers = len(dataList)
	}

	type result struct {
		wav   *WAV
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

					wav, err := Parse(work.data)
					resultChan <- result{
						wav:   wav,
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

	results := make([]*WAV, len(dataList))
	for res := range resultChan {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if res.err != nil {
			return nil, fmt.Errorf("failed to parse WAV at index %d: %w", res.index, res.err)
		}
		results[res.index] = res.wav
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return results, nil
}

// ExtractChannels splits multi-channel WAV data into separate channel slices.
func (w *WAV) ExtractChannels() ([][]byte, error) {
	if w.Header.NumChannels == 0 {
		return nil, errors.New("no channels")
	}

	bytesPerSample := int(w.Header.BitsPerSample) / 8
	if bytesPerSample == 0 {
		return nil, errors.New("invalid bits per sample")
	}

	numChannels := int(w.Header.NumChannels)
	sampleCount := len(w.Data) / bytesPerSample / numChannels

	channels := make([][]byte, numChannels)
	for i := range channels {
		channels[i] = make([]byte, sampleCount*bytesPerSample)
	}

	// Deinterleave channels
	for sample := 0; sample < sampleCount; sample++ {
		for ch := 0; ch < numChannels; ch++ {
			srcIdx := (sample*numChannels + ch) * bytesPerSample
			dstIdx := sample * bytesPerSample

			copy(channels[ch][dstIdx:dstIdx+bytesPerSample],
				w.Data[srcIdx:srcIdx+bytesPerSample])
		}
	}

	return channels, nil
}

// maxWAVDataSize is the largest PCM payload that fits in the 32-bit RIFF size
// fields (total file size must also fit, hence the 44-byte header allowance).
const maxWAVDataSize = math.MaxUint32 - 44

// Serialize converts a WAV structure back to WAV file format.
func Serialize(wav *WAV) ([]byte, error) {
	if wav == nil {
		return nil, errors.New("nil WAV")
	}
	if len(wav.Data) > maxWAVDataSize {
		return nil, fmt.Errorf("WAV data too large to serialize: %d bytes", len(wav.Data))
	}

	var buf bytes.Buffer

	// All binary.Write calls below target a bytes.Buffer with fixed-size values,
	// so they cannot actually fail; the closure records the first error anyway.
	var werr error
	put := func(v any) {
		if werr == nil {
			werr = binary.Write(&buf, binary.LittleEndian, v)
		}
	}

	// RIFF header. len(wav.Data) is bounded by maxWAVDataSize above, so the
	// uint32 conversions below cannot overflow.
	buf.WriteString("RIFF")
	put(uint32(36 + len(wav.Data))) //nolint:gosec // G115: len bounded by maxWAVDataSize
	buf.WriteString("WAVE")

	// fmt subchunk.
	buf.WriteString("fmt ")
	put(uint32(16))
	put(wav.Header.AudioFormat)
	put(wav.Header.NumChannels)
	put(wav.Header.SampleRate)
	put(wav.Header.ByteRate)
	put(wav.Header.BlockAlign)
	put(wav.Header.BitsPerSample)

	// data subchunk.
	buf.WriteString("data")
	put(uint32(len(wav.Data))) //nolint:gosec // G115: len bounded by maxWAVDataSize
	buf.Write(wav.Data)

	if werr != nil {
		return nil, fmt.Errorf("serialize WAV: %w", werr)
	}
	return buf.Bytes(), nil
}
