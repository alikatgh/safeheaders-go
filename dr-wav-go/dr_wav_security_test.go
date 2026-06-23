package drwavgo

import (
	"encoding/binary"
	"testing"
)

// TestParseOversizedDataChunkIsCapped guards against the OOM regression: a WAV
// whose data subchunk header claims a huge size must not drive a giant
// allocation. The read is capped to the bytes actually present.
func TestParseOversizedDataChunkIsCapped(t *testing.T) {
	wav := &WAV{
		Header: WAVHeader{
			AudioFormat:   1,
			NumChannels:   1,
			SampleRate:    8000,
			ByteRate:      8000,
			BlockAlign:    1,
			BitsPerSample: 8,
		},
		Data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}

	raw, err := Serialize(wav)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Corrupt the data-subchunk size field (bytes 40..43) to claim ~4 GB.
	binary.LittleEndian.PutUint32(raw[40:44], 0xFFFFFFFF)

	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Data) != len(wav.Data) {
		t.Fatalf("data length = %d, want %d (capped to bytes available)", len(got.Data), len(wav.Data))
	}
}

// TestParseSerializeRoundTrip exercises the happy path end-to-end.
func TestParseSerializeRoundTrip(t *testing.T) {
	original := &WAV{
		Header: WAVHeader{
			AudioFormat:   1,
			NumChannels:   2,
			SampleRate:    44100,
			ByteRate:      176400,
			BlockAlign:    4,
			BitsPerSample: 16,
		},
		Data: []byte{0, 1, 2, 3, 4, 5, 6, 7},
	}

	raw, err := Serialize(original)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Header != original.Header {
		t.Errorf("header = %+v, want %+v", got.Header, original.Header)
	}
	if string(got.Data) != string(original.Data) {
		t.Errorf("data round-trip mismatch")
	}
	if err := ValidateWAV(got); err != nil {
		t.Errorf("ValidateWAV: %v", err)
	}
}
