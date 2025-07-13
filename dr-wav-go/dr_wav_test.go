package drwavgo

import (
	"bytes"
	"testing"
)

func TestLoadWAV(t *testing.T) {
	// Dummy WAV header + data.
	data := []byte("RIFF" + "\x00\x00\x00\x00WAVEfmt " + "\x10\x00\x00\x00" + "\x01\x00\x01\x00\x44\xac\x00\x00\x88\x58\x01\x00\x02\x00\x10\x00data" + "\x04\x00\x00\x00" + "\x01\x02\x03\x04")
	_, err := LoadWAV(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadStreamConcurrent(t *testing.T) {
	// Larger dummy WAV (repeat for chunks).
	baseData := []byte("RIFF" + "\x00\x00\x00\x00WAVEfmt " + "\x10\x00\x00\x00" + "\x01\x00\x01\x00\x44\xac\x00\x00\x88\x58\x01\x00\x02\x00\x10\x00data" + "\x04\x00\x00\x00" + "\x01\x02\x03\x04")
	data := bytes.Repeat(baseData, 2000) // ~80KB.
	reader := bytes.NewReader(data)
	_, err := LoadStreamConcurrent(reader)
	if err != nil {
		t.Fatal(err)
	}
}
