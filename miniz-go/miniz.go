package minizgo

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// FileEntry represents a file to be added to a ZIP archive.
type FileEntry struct {
	Name string
	Data []byte
}

// ZipFile represents an extracted file from a ZIP archive.
type ZipFile struct {
	Name string
	Data []byte
	Size int64
}

// CreateArchive creates a ZIP archive from a list of files.
func CreateArchive(files []FileEntry) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("no files provided")
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, file := range files {
		if file.Name == "" {
			return nil, errors.New("file name cannot be empty")
		}

		fw, err := w.Create(file.Name)
		if err != nil {
			w.Close()
			return nil, fmt.Errorf("failed to create file %s: %w", file.Name, err)
		}

		if _, err := fw.Write(file.Data); err != nil {
			w.Close()
			return nil, fmt.Errorf("failed to write file %s: %w", file.Name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close archive: %w", err)
	}

	return buf.Bytes(), nil
}

// ExtractArchive extracts all files from a ZIP archive.
func ExtractArchive(data []byte) ([]ZipFile, error) {
	if len(data) == 0 {
		return nil, errors.New("empty archive data")
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}

	files := make([]ZipFile, 0, len(r.File))

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", f.Name, err)
		}

		files = append(files, ZipFile{
			Name: f.Name,
			Data: data,
			Size: int64(len(data)),
		})
	}

	return files, nil
}

// CreateArchiveConcurrent creates a ZIP archive from files using parallel compression.
func CreateArchiveConcurrent(ctx context.Context, files []FileEntry) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("no files provided")
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}

	// Compress files in parallel
	type compressedFile struct {
		name       string
		compressed []byte
		err        error
		index      int
	}

	results := make([]compressedFile, len(files))
	fileChan := make(chan struct {
		entry FileEntry
		index int
	}, len(files))

	var wg sync.WaitGroup
	resultChan := make(chan compressedFile, len(files))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case work, ok := <-fileChan:
					if !ok {
						return
					}

					var buf bytes.Buffer
					w, err := flate.NewWriter(&buf, flate.BestCompression)
					if err != nil {
						resultChan <- compressedFile{err: err, index: work.index}
						continue
					}

					_, err = w.Write(work.entry.Data)
					w.Close()
					if err != nil {
						resultChan <- compressedFile{err: err, index: work.index}
						continue
					}

					resultChan <- compressedFile{
						name:       work.entry.Name,
						compressed: buf.Bytes(),
						index:      work.index,
					}
				}
			}
		}()
	}

	// Send work
	go func() {
		for i, file := range files {
			select {
			case <-ctx.Done():
				close(fileChan)
				return
			case fileChan <- struct {
				entry FileEntry
				index int
			}{file, i}:
			}
		}
		close(fileChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if result.err != nil {
			return nil, fmt.Errorf("failed to compress file: %w", result.err)
		}
		results[result.index] = result
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Build ZIP archive with pre-compressed data
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, result := range results {
		fw, err := w.CreateHeader(&zip.FileHeader{
			Name:   result.name,
			Method: zip.Deflate,
		})
		if err != nil {
			w.Close()
			return nil, fmt.Errorf("failed to create file header: %w", err)
		}

		if _, err := fw.Write(result.compressed); err != nil {
			w.Close()
			return nil, fmt.Errorf("failed to write compressed data: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close archive: %w", err)
	}

	return buf.Bytes(), nil
}

// ListArchive returns the names and sizes of files in a ZIP archive.
func ListArchive(data []byte) ([]ZipFile, error) {
	if len(data) == 0 {
		return nil, errors.New("empty archive data")
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}

	files := make([]ZipFile, len(r.File))
	for i, f := range r.File {
		files[i] = ZipFile{
			Name: f.Name,
			Size: int64(f.UncompressedSize64),
		}
	}

	return files, nil
}

// CompressData compresses data using DEFLATE algorithm.
func CompressData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, fmt.Errorf("compression error: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compressor: %w", err)
	}

	return buf.Bytes(), nil
}

// DecompressData decompresses DEFLATE-compressed data.
func DecompressData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("decompression error: %w", err)
	}

	return result, nil
}
