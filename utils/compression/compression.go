package compression

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
)

// Gzip compression levels, re-exported so callers don't depend on the backing gzip implementation.
const (
	HuffmanOnly        = gzip.HuffmanOnly
	NoCompression      = gzip.NoCompression
	BestSpeed          = gzip.BestSpeed
	BestCompression    = gzip.BestCompression
	DefaultCompression = gzip.DefaultCompression
)

// All gzip compression levels
const (
	minLevel   = HuffmanOnly
	maxLevel   = BestCompression
	levelCount = maxLevel - minLevel + 1
)

var ErrWriterNotAcquired = errors.New("using writer after release")

// gzipWriterPools holds one pool per compression level.
var gzipWriterPools *[levelCount]sync.Pool = func() *[levelCount]sync.Pool {
	pool := [levelCount]sync.Pool{}

	for i := range levelCount {
		pool[i].New = func() any { return newWriter(minLevel + i) }
	}

	return &pool
}()

// proxy exists to solve the problem of putting a [Writer] `w` back to the pool
// (via [Writer.Release]) while `w` still references live data that is not going to be used
// again. There is the option to drop reference with `w.gz.Reset(io.Discard)`
// but it's expensive (cost in the micro-seconds).
// With `proxy` we create a middle pointer to which `w.gz` will point.
// `proxy` in turn will point to the actual data.
// When releasing a [Writer] it is enough for the proxy to drop the reference.
// When acquired: [Writer.gz] ---> [Writer.proxy] ---> data
// After release: [Writer.gz] ---> [Writer.proxy] ---> nil
type proxy struct {
	dst io.Writer
}

func (d *proxy) Write(p []byte) (int, error) {
	if d.dst == nil {
		return 0, ErrWriterNotAcquired
	}
	return d.dst.Write(p)
}

// Writer is a pooled gzip writer. Its gzip writer is permanently wired to
// proxy, which forwards writes to the caller's dst while the writer is
// acquired. The indirection lets Release detach the caller's destination in
// O(1) instead of paying a second flate reset just to re-point the gzip
// writer at io.Discard.
type Writer struct {
	gz    *gzip.Writer
	proxy proxy      // dst of gz
	pool  *sync.Pool // pool this writer belongs to; nil once released
}

func newWriter(level int) *Writer {
	writer := &Writer{}
	gzipWriter, err := gzip.NewWriterLevel(&writer.proxy, level)
	if err != nil {
		panic(fmt.Sprintf("creating new gzip writer for level %d: %v", level, err))
	}
	writer.gz = gzipWriter
	return writer
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

// Release returns the writer to the pool
func (w *Writer) Release() {
	if !w.isAcquired() {
		panic("re-releasing writer")
	}
	pool := w.pool
	w.pool = nil
	w.proxy.dst = nil
	pool.Put(w)
}

func (w *Writer) isAcquired() bool {
	return w.pool != nil
}

// GzipWriter returns a gzip writer reset onto `dst`, compressing at the default
// level. Once used, it should be sent back to the pool via `Release`
func GzipWriter(dst io.Writer) *Writer {
	return GzipWriterLevel(dst, DefaultCompression)
}

// GzipWriterLevel returns a gzip writer reset onto `dst`, compressing at `level`.
// Once used, it should be sent back to the pool via `Release`
func GzipWriterLevel(dst io.Writer, level int) *Writer {
	pool := &gzipWriterPools[level-minLevel]
	writer := pool.Get().(*Writer)
	writer.pool = pool
	writer.proxy.dst = dst
	writer.gz.Reset(&writer.proxy)
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
