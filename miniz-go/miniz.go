// Package minizgo provides ZIP archive creation and extraction plus raw DEFLATE
// compression helpers, with support for compressing archive entries in parallel.
// It is inspired by the miniz C library (github.com/richgel999/miniz).
package minizgo

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
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

// MaxDecompressedSize caps how many bytes ExtractArchive and DecompressData will
// produce, guarding against decompression bombs (a small input that inflates to
// gigabytes). For ExtractArchive the cap is on the ARCHIVE TOTAL across all
// entries, not per entry. The default is 256 MiB. Set it to 0 to disable.
//
// It must be set before any concurrent decompression begins; it is read without
// synchronization, so mutating it while a decompress is in flight is a data race.
var MaxDecompressedSize int64 = 256 << 20

// readAllLimited reads all of r, but errors instead of allocating without bound
// once the output would exceed limit. A limit <= 0 means unlimited.
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	src := r
	if limit > 0 {
		// +1 so we can distinguish "exactly at the limit" from "over it".
		src = io.LimitReader(r, limit+1)
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if limit > 0 && int64(len(data)) > limit {
		return nil, fmt.Errorf("decompressed size exceeds %d-byte limit (adjust MaxDecompressedSize)", limit)
	}
	return data, nil
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

	var total int64 // aggregate decompressed bytes across all entries
	for _, f := range r.File {
		// Bound each entry by the budget remaining for the whole archive, so a
		// many-entry zip bomb can't blow past MaxDecompressedSize in aggregate.
		var perEntryLimit int64
		if MaxDecompressedSize > 0 {
			if perEntryLimit = MaxDecompressedSize - total; perEntryLimit <= 0 {
				return nil, fmt.Errorf("archive exceeds the %d-byte limit (adjust MaxDecompressedSize)", MaxDecompressedSize)
			}
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", f.Name, err)
		}

		data, err := readAllLimited(rc, perEntryLimit)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", f.Name, err)
		}
		total += int64(len(data))

		files = append(files, ZipFile{
			Name: f.Name,
			Data: data,
			Size: int64(len(data)),
		})
	}

	return files, nil
}

// compressedFile holds one file's raw DEFLATE stream plus the metadata needed
// to assemble it into a ZIP entry without re-compressing it.
type compressedFile struct {
	name       string
	compressed []byte
	crc        uint32
	rawSize    uint64
	index      int
	err        error
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

	type job struct {
		entry FileEntry
		index int
	}
	jobCh := make(chan job, len(files))
	resultCh := make(chan compressedFile, len(files))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-jobCh:
					if !ok {
						return
					}
					cf := compressEntry(j.entry)
					cf.index = j.index
					resultCh <- cf
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for i, file := range files {
			select {
			case <-ctx.Done():
				return
			case jobCh <- job{entry: file, index: i}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]compressedFile, len(files))
	for r := range resultCh {
		if r.err != nil {
			return nil, fmt.Errorf("failed to compress %q: %w", r.name, r.err)
		}
		results[r.index] = r
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return buildRawZip(results)
}

// compressEntry produces the raw DEFLATE stream for one file along with the CRC
// and uncompressed size that a ZIP entry needs. Errors are carried on the
// returned value so workers stay branch-light.
func compressEntry(entry FileEntry) compressedFile {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return compressedFile{name: entry.Name, err: err}
	}
	if _, err := w.Write(entry.Data); err != nil {
		_ = w.Close()
		return compressedFile{name: entry.Name, err: err}
	}
	if err := w.Close(); err != nil {
		return compressedFile{name: entry.Name, err: err}
	}
	return compressedFile{
		name:       entry.Name,
		compressed: buf.Bytes(),
		crc:        crc32.ChecksumIEEE(entry.Data),
		rawSize:    uint64(len(entry.Data)),
	}
}

// buildRawZip assembles already-compressed entries into a ZIP using CreateRaw,
// so the parallel compression work is preserved and the archive round-trips
// correctly (writing pre-deflated bytes into a Deflate entry would double-
// compress them).
func buildRawZip(results []compressedFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, r := range results {
		fh := &zip.FileHeader{
			Name:               r.name,
			Method:             zip.Deflate,
			CRC32:              r.crc,
			CompressedSize64:   uint64(len(r.compressed)),
			UncompressedSize64: r.rawSize,
		}
		w, err := zw.CreateRaw(fh)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("failed to create zip entry %q: %w", r.name, err)
		}
		if _, err := w.Write(r.compressed); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("failed to write zip entry %q: %w", r.name, err)
		}
	}
	if err := zw.Close(); err != nil {
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
		size := f.UncompressedSize64
		if size > math.MaxInt64 {
			size = math.MaxInt64 // clamp absurd/hostile sizes rather than overflow
		}
		files[i] = ZipFile{
			Name: f.Name,
			Size: int64(size), //nolint:gosec // G115: clamped to MaxInt64 above
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

	result, err := readAllLimited(r, MaxDecompressedSize)
	if err != nil {
		return nil, fmt.Errorf("decompression error: %w", err)
	}

	return result, nil
}

// CompressStream DEFLATE-compresses everything read from src and writes it to
// dst, without buffering the whole input or output in memory. Use it for large
// files where DecompressData/CompressData's all-in-memory model is too costly.
func CompressStream(dst io.Writer, src io.Reader) error {
	if dst == nil || src == nil {
		return errors.New("nil reader or writer")
	}
	w, err := flate.NewWriter(dst, flate.BestCompression)
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}
	if _, err := io.Copy(w, src); err != nil {
		_ = w.Close()
		return fmt.Errorf("compress stream: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finalize compression: %w", err)
	}
	return nil
}

// DecompressStream inflates DEFLATE data from src into dst as a stream. The
// output is capped at MaxDecompressedSize (set 0 to disable) so a decompression
// bomb cannot exhaust memory or fill the destination unbounded.
func DecompressStream(dst io.Writer, src io.Reader) error {
	if dst == nil || src == nil {
		return errors.New("nil reader or writer")
	}
	r := flate.NewReader(src)
	defer r.Close()

	var reader io.Reader = r
	if MaxDecompressedSize > 0 {
		reader = io.LimitReader(r, MaxDecompressedSize+1)
	}
	n, err := io.Copy(dst, reader)
	if err != nil {
		return fmt.Errorf("decompress stream: %w", err)
	}
	if MaxDecompressedSize > 0 && n > MaxDecompressedSize {
		return fmt.Errorf("decompressed size exceeds %d-byte limit (adjust MaxDecompressedSize)", MaxDecompressedSize)
	}
	return nil
}
