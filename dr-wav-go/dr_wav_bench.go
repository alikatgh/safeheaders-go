package drwavgo

import (
	"bytes"
	"testing"
)

func BenchmarkLoadStreamConcurrent(b *testing.B) {
	data := make([]byte, 1<<20) // 1MB dummy WAV.
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		LoadStreamConcurrent(reader)
	}
}
