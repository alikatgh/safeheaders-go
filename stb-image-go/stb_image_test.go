package stbimagego

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: uint8(x % 256), B: uint8(y % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func createTestGIF(width, height int) []byte {
	img := image.NewPaletted(image.Rect(0, 0, width, height), []color.Color{
		color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255},
		color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255},
	})
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetColorIndex(x, y, uint8((x+y)%4))
		}
	}
	var buf bytes.Buffer
	gif.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		width  int
		height int
	}{
		{"PNG 10x10", createTestPNG(10, 10), 10, 10},
		{"JPEG 20x20", createTestJPEG(20, 20), 20, 20},
		{"GIF 15x15", createTestGIF(15, 15), 15, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := Load(tt.data)
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			bounds := img.Bounds()
			if bounds.Dx() != tt.width || bounds.Dy() != tt.height {
				t.Errorf("Expected %dx%d, got %dx%d", tt.width, tt.height, bounds.Dx(), bounds.Dy())
			}
		})
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty", []byte{}},
		{"Invalid", []byte{0x00, 0x01}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.data)
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestGetInfo(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		width  int
		height int
		format string
	}{
		{"PNG", createTestPNG(10, 20), 10, 20, "png"},
		{"JPEG", createTestJPEG(30, 40), 30, 40, "jpeg"},
		{"GIF", createTestGIF(15, 25), 15, 25, "gif"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetInfo(tt.data)
			if err != nil {
				t.Fatalf("GetInfo() failed: %v", err)
			}
			if info.Width != tt.width || info.Height != tt.height {
				t.Errorf("Expected %dx%d, got %dx%d", tt.width, tt.height, info.Width, info.Height)
			}
			if info.Format != tt.format {
				t.Errorf("Expected format %s, got %s", tt.format, info.Format)
			}
		})
	}
}

func TestLoadStream(t *testing.T) {
	pngData := createTestPNG(10, 10)
	img, err := LoadStream(bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("LoadStream() failed: %v", err)
	}
	if img == nil {
		t.Fatal("LoadStream() returned nil")
	}
}

func TestLoadBatchConcurrent(t *testing.T) {
	datas := [][]byte{createTestPNG(10, 10), createTestJPEG(20, 20), createTestGIF(15, 15)}
	images, err := LoadBatchConcurrent(context.Background(), datas)
	if err != nil {
		t.Fatalf("LoadBatchConcurrent() failed: %v", err)
	}
	if len(images) != len(datas) {
		t.Fatalf("Expected %d images, got %d", len(datas), len(images))
	}
}

func TestLoadBatchConcurrent_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := LoadBatchConcurrent(ctx, [][]byte{createTestPNG(10, 10)})
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}
