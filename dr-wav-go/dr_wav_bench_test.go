package drwavgo

import (
	"context"
	"encoding/binary"
	"testing"
)

func createBenchmarkWAV() []byte {
	// Create a simple 16-bit stereo WAV
	data := make([]byte, 44+1000000) // Header + 1MB of audio
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:24], 2) // stereo
	binary.LittleEndian.PutUint32(data[24:28], 44100)
	binary.LittleEndian.PutUint32(data[28:32], 44100*2*2)
	binary.LittleEndian.PutUint16(data[32:34], 4)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], 1000000)
	return data
}

func BenchmarkParse(b *testing.B) {
	data := createBenchmarkWAV()
	for i := 0; i < b.N; i++ {
		Parse(data)
	}
}

func BenchmarkParseBatch(b *testing.B) {
	dataList := make([][]byte, 10)
	wavData := createBenchmarkWAV()
	for i := range dataList {
		dataList[i] = wavData
	}
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		ParseBatch(ctx, dataList)
	}
}
