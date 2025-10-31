package drwavgo

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
	"time"
)

func createTestWAV(sampleRate uint32, numChannels uint16, bitsPerSample uint16, numSamples int) []byte {
	var buf bytes.Buffer

	// RIFF header
	buf.Write([]byte("RIFF"))
	dataSize := numSamples * int(numChannels) * int(bitsPerSample) / 8
	chunkSize := uint32(36 + dataSize)
	binary.Write(&buf, binary.LittleEndian, chunkSize)
	buf.Write([]byte("WAVE"))

	// fmt subchunk
	buf.Write([]byte("fmt "))
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // subchunk1Size
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // audioFormat (PCM)
	binary.Write(&buf, binary.LittleEndian, numChannels)
	binary.Write(&buf, binary.LittleEndian, sampleRate)
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8
	binary.Write(&buf, binary.LittleEndian, byteRate)
	blockAlign := numChannels * bitsPerSample / 8
	binary.Write(&buf, binary.LittleEndian, blockAlign)
	binary.Write(&buf, binary.LittleEndian, bitsPerSample)

	// data subchunk
	buf.Write([]byte("data"))
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	// Write sample data
	for i := 0; i < numSamples*int(numChannels); i++ {
		switch bitsPerSample {
		case 8:
			buf.WriteByte(byte(i % 256))
		case 16:
			binary.Write(&buf, binary.LittleEndian, uint16(i%65536))
		case 32:
			binary.Write(&buf, binary.LittleEndian, uint32(i))
		}
	}

	return buf.Bytes()
}

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantError bool
	}{
		{
			name:      "Valid 16-bit mono",
			data:      createTestWAV(44100, 1, 16, 100),
			wantError: false,
		},
		{
			name:      "Valid 16-bit stereo",
			data:      createTestWAV(44100, 2, 16, 100),
			wantError: false,
		},
		{
			name:      "Valid 8-bit mono",
			data:      createTestWAV(22050, 1, 8, 100),
			wantError: false,
		},
		{
			name:      "Too short",
			data:      []byte("short"),
			wantError: true,
		},
		{
			name:      "Invalid RIFF",
			data:      append([]byte("JUNK"), make([]byte, 40)...),
			wantError: true,
		},
		{
			name:      "Empty data",
			data:      []byte{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wav, err := Parse(tt.data)
			if (err != nil) != tt.wantError {
				t.Fatalf("Parse() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && wav == nil {
				t.Fatal("Expected non-nil WAV")
			}
		})
	}
}

func TestWAV_GetDuration(t *testing.T) {
	// 44100 Hz, 1 channel, 16-bit, 44100 samples = 1 second
	data := createTestWAV(44100, 1, 16, 44100)
	wav, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse WAV: %v", err)
	}

	duration := wav.GetDuration()
	if duration < 0.99 || duration > 1.01 {
		t.Errorf("Expected duration ~1.0s, got %.2f", duration)
	}
}

func TestWAV_GetSampleCount(t *testing.T) {
	numSamples := 1000
	data := createTestWAV(44100, 2, 16, numSamples)
	wav, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse WAV: %v", err)
	}

	count := wav.GetSampleCount()
	if count != numSamples {
		t.Errorf("Expected %d samples, got %d", numSamples, count)
	}
}

func TestValidateWAV(t *testing.T) {
	validData := createTestWAV(44100, 2, 16, 100)
	validWAV, err := Parse(validData)
	if err != nil {
		t.Fatalf("Failed to parse valid WAV: %v", err)
	}

	tests := []struct {
		name      string
		wav       *WAV
		wantError bool
	}{
		{
			name:      "Valid WAV",
			wav:       validWAV,
			wantError: false,
		},
		{
			name:      "Nil WAV",
			wav:       nil,
			wantError: true,
		},
		{
			name: "Invalid audio format",
			wav: &WAV{
				Header: WAVHeader{
					AudioFormat:   2, // not PCM
					NumChannels:   1,
					SampleRate:    44100,
					BitsPerSample: 16,
				},
			},
			wantError: true,
		},
		{
			name: "Zero channels",
			wav: &WAV{
				Header: WAVHeader{
					AudioFormat:   1,
					NumChannels:   0,
					SampleRate:    44100,
					BitsPerSample: 16,
				},
			},
			wantError: true,
		},
		{
			name: "Zero sample rate",
			wav: &WAV{
				Header: WAVHeader{
					AudioFormat:   1,
					NumChannels:   2,
					SampleRate:    0,
					BitsPerSample: 16,
				},
			},
			wantError: true,
		},
		{
			name: "Invalid bits per sample",
			wav: &WAV{
				Header: WAVHeader{
					AudioFormat:   1,
					NumChannels:   2,
					SampleRate:    44100,
					BitsPerSample: 12,
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWAV(tt.wav)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateWAV() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseBatch(t *testing.T) {
	wav1 := createTestWAV(44100, 1, 16, 100)
	wav2 := createTestWAV(22050, 2, 8, 200)
	wav3 := createTestWAV(48000, 2, 16, 150)

	dataList := [][]byte{wav1, wav2, wav3}

	ctx := context.Background()
	results, err := ParseBatch(ctx, dataList)
	if err != nil {
		t.Fatalf("ParseBatch() error = %v", err)
	}

	if len(results) != len(dataList) {
		t.Fatalf("Expected %d results, got %d", len(dataList), len(results))
	}

	// Verify each result
	if results[0].Header.SampleRate != 44100 {
		t.Errorf("WAV 0: expected sample rate 44100, got %d", results[0].Header.SampleRate)
	}
	if results[1].Header.NumChannels != 2 {
		t.Errorf("WAV 1: expected 2 channels, got %d", results[1].Header.NumChannels)
	}
	if results[2].Header.BitsPerSample != 16 {
		t.Errorf("WAV 2: expected 16 bits per sample, got %d", results[2].Header.BitsPerSample)
	}
}

func TestParseBatch_Errors(t *testing.T) {
	tests := []struct {
		name      string
		dataList  [][]byte
		wantError bool
	}{
		{
			name:      "Empty list",
			dataList:  [][]byte{},
			wantError: true,
		},
		{
			name: "Contains invalid data",
			dataList: [][]byte{
				createTestWAV(44100, 1, 16, 100),
				[]byte("invalid"),
			},
			wantError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBatch(ctx, tt.dataList)
			if (err != nil) != tt.wantError {
				t.Fatalf("ParseBatch() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseBatch_Context(t *testing.T) {
	dataList := make([][]byte, 100)
	for i := range dataList {
		dataList[i] = createTestWAV(44100, 2, 16, 1000)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Ensure timeout

	_, err := ParseBatch(ctx, dataList)
	if err == nil {
		t.Fatal("Expected context timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestWAV_ExtractChannels(t *testing.T) {
	// Create stereo WAV
	data := createTestWAV(44100, 2, 16, 100)
	wav, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse WAV: %v", err)
	}

	channels, err := wav.ExtractChannels()
	if err != nil {
		t.Fatalf("ExtractChannels() error = %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("Expected 2 channels, got %d", len(channels))
	}

	// Each channel should have 100 samples * 2 bytes
	expectedSize := 100 * 2
	for i, ch := range channels {
		if len(ch) != expectedSize {
			t.Errorf("Channel %d: expected size %d, got %d", i, expectedSize, len(ch))
		}
	}
}

func TestWAV_ExtractChannels_Mono(t *testing.T) {
	data := createTestWAV(44100, 1, 16, 100)
	wav, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse WAV: %v", err)
	}

	channels, err := wav.ExtractChannels()
	if err != nil {
		t.Fatalf("ExtractChannels() error = %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("Expected 1 channel, got %d", len(channels))
	}
}

func TestSerialize(t *testing.T) {
	// Parse original
	original := createTestWAV(44100, 2, 16, 100)
	wav, err := Parse(original)
	if err != nil {
		t.Fatalf("Failed to parse WAV: %v", err)
	}

	// Serialize
	serialized, err := Serialize(wav)
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if len(serialized) == 0 {
		t.Fatal("Expected non-empty serialized data")
	}

	// Parse again
	wav2, err := Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized WAV: %v", err)
	}

	// Compare headers
	if wav.Header.NumChannels != wav2.Header.NumChannels {
		t.Error("NumChannels mismatch")
	}
	if wav.Header.SampleRate != wav2.Header.SampleRate {
		t.Error("SampleRate mismatch")
	}
	if wav.Header.BitsPerSample != wav2.Header.BitsPerSample {
		t.Error("BitsPerSample mismatch")
	}
	if !bytes.Equal(wav.Data, wav2.Data) {
		t.Error("Data mismatch")
	}
}

func TestSerialize_Nil(t *testing.T) {
	_, err := Serialize(nil)
	if err == nil {
		t.Fatal("Expected error for nil WAV")
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		sampleRate   uint32
		numChannels  uint16
		bitsPerSample uint16
		numSamples   int
	}{
		{"8-bit mono 22050Hz", 22050, 1, 8, 100},
		{"16-bit stereo 44100Hz", 44100, 2, 16, 500},
		{"16-bit mono 48000Hz", 48000, 1, 16, 250},
		{"32-bit stereo 96000Hz", 96000, 2, 32, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create
			original := createTestWAV(tt.sampleRate, tt.numChannels, tt.bitsPerSample, tt.numSamples)

			// Parse
			wav1, err := Parse(original)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Validate
			if err := ValidateWAV(wav1); err != nil {
				t.Fatalf("Validation failed: %v", err)
			}

			// Serialize
			serialized, err := Serialize(wav1)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			// Parse again
			wav2, err := Parse(serialized)
			if err != nil {
				t.Fatalf("Second parse failed: %v", err)
			}

			// Compare
			if wav1.Header != wav2.Header {
				t.Error("Header mismatch")
			}
			if !bytes.Equal(wav1.Data, wav2.Data) {
				t.Error("Data mismatch")
			}

			// Check sample count
			if wav1.GetSampleCount() != tt.numSamples {
				t.Errorf("Expected %d samples, got %d", tt.numSamples, wav1.GetSampleCount())
			}
		})
	}
}

func TestMultipleFormats(t *testing.T) {
	formats := []struct {
		sampleRate   uint32
		numChannels  uint16
		bitsPerSample uint16
	}{
		{8000, 1, 8},
		{11025, 1, 16},
		{22050, 2, 16},
		{44100, 1, 16},
		{44100, 2, 16},
		{48000, 2, 24},
		{96000, 2, 32},
	}

	for _, format := range formats {
		data := createTestWAV(format.sampleRate, format.numChannels, format.bitsPerSample, 100)
		wav, err := Parse(data)
		if err != nil {
			t.Errorf("Failed to parse %dHz %dch %dbit: %v",
				format.sampleRate, format.numChannels, format.bitsPerSample, err)
			continue
		}

		if wav.Header.SampleRate != format.sampleRate {
			t.Errorf("SampleRate mismatch: expected %d, got %d",
				format.sampleRate, wav.Header.SampleRate)
		}
		if wav.Header.NumChannels != format.numChannels {
			t.Errorf("NumChannels mismatch: expected %d, got %d",
				format.numChannels, wav.Header.NumChannels)
		}
		if wav.Header.BitsPerSample != format.bitsPerSample {
			t.Errorf("BitsPerSample mismatch: expected %d, got %d",
				format.bitsPerSample, wav.Header.BitsPerSample)
		}
	}
}
