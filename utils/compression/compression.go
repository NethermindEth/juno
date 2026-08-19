package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

// All gzip compression levels
const (
	minLevel   = gzip.HuffmanOnly
	maxLevel   = gzip.BestCompression
	levelCount = maxLevel - minLevel + 1
)

var ErrWriterNotAcquired = errors.New("using writer after release")

// gzipWriterPools holds one pool per compression level.
var gzipWriterPools *[levelCount]sync.Pool = func() *[levelCount]sync.Pool {
	pool := [levelCount]sync.Pool{}

	for i := range pool {
		level := minLevel + i
		pool[i].New = func() any {
			// Only fails on an out-of-range level, and level comes from the index.
			gzipWriter, err := gzip.NewWriterLevel(io.Discard, level)
			if err != nil {
				panic(fmt.Sprintf("creating new gzip writer for level %d: %v", level, err))
			}
			return &Writer{gz: gzipWriter}
		}
	}

	return &pool
}()

// Writer is a pooled gzip writer. It references the pool it belongs to so it can
// track its own state
type Writer struct {
	gz   *gzip.Writer
	pool *sync.Pool
}

func (w *Writer) Write(p []byte) (int, error) {
	if !w.isAcquired() {
		return 0, ErrWriterNotAcquired
	}
	return w.gz.Write(p)
}

func (w *Writer) Close() error {
	if !w.isAcquired() {
		return ErrWriterNotAcquired
	}
	return w.gz.Close()
}

func (w *Writer) Flush() error {
	if !w.isAcquired() {
		return ErrWriterNotAcquired
	}
	return w.gz.Flush()
}

func (w *Writer) isAcquired() bool {
	return w.pool != nil
}

// Release returns the writer to the pool
func (w *Writer) Release() {
	if w.pool == nil {
		panic("re-releasing writer")
	}
	pool := w.pool
	w.pool = nil
	// Intentionally added to break all the references this writer is pointing too before
	// putting it back to the pool.
	w.gz.Reset(io.Discard)
	pool.Put(w)
}

// GzipWriter returns a gzip writer reset onto `dst`, compressing at the default
// level. Once used, it should be sent back to the pool via `Release`
func GzipWriter(dst io.Writer) *Writer {
	return GzipWriterLevel(dst, gzip.DefaultCompression)
}

// GzipWriterLevel returns a gzip writer reset onto `dst`, compressing at `level`.
// Once used, it should be sent back to the pool via `Release`
func GzipWriterLevel(dst io.Writer, level int) *Writer {
	pool := &gzipWriterPools[level-minLevel]
	writer := pool.Get().(*Writer)
	writer.pool = pool
	writer.gz.Reset(dst)
	return writer
}

func Gzip64Encode(data []byte) (string, error) {
	var compressedBuffer bytes.Buffer
	gzipWriter := GzipWriter(&compressedBuffer)
	defer gzipWriter.Release()
	if _, err := gzipWriter.Write(data); err != nil {
		return "", fmt.Errorf("writing data with gzip: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("closing gzip writer: %w", err)
	}
	return base64.StdEncoding.EncodeToString(compressedBuffer.Bytes()), nil
}

func Gzip64Decode(data string) ([]byte, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedBytes))
	if err != nil {
		return nil, err
	}
	decompressedBytes, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}
	err = gzipReader.Close()
	if err != nil {
		return nil, err
	}
	return decompressedBytes, nil
}
