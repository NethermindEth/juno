package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

// Inclusive range of levels gzip accepts: HuffmanOnly (-2) to BestCompression (9).
const (
	minLevel   = gzip.HuffmanOnly
	maxLevel   = gzip.BestCompression
	levelCount = maxLevel - minLevel + 1
)

// Writer is a pooled gzip writer. It carries its compression level because
// gzip.Writer does not expose it and PutGzipWriter needs it to refile the writer.
type Writer struct {
	*gzip.Writer
	level int
}

// gzipWriterPools holds one pool per compression level. Levels must not share a
// pool: Reset preserves the level a writer was built with, so a caller asking for
// one level could be handed another and silently compress at the wrong level.
// The pointer keeps the pools from being copied.
var gzipWriterPools = newGzipWriterPools()

func newGzipWriterPools() *[levelCount]sync.Pool {
	pools := new([levelCount]sync.Pool)
	for i := range pools {
		level := minLevel + i
		pools[i].New = func() any {
			// Only fails on an out-of-range level, and level comes from the index.
			gzipWriter, err := gzip.NewWriterLevel(io.Discard, level)
			if err != nil {
				panic(fmt.Sprintf("compression: gzip writer at level %d: %v", level, err))
			}
			return &Writer{Writer: gzipWriter, level: level}
		}
	}
	return pools
}

// poolIndex maps a level onto its pool. Callers pass constants, so a level gzip
// would reject is a programming error rather than a runtime condition.
func poolIndex(level int) int {
	if level < minLevel || level > maxLevel {
		panic(fmt.Sprintf("compression: gzip level %d outside [%d, %d]", level, minLevel, maxLevel))
	}
	return level - minLevel
}

// GzipWriter returns a gzip writer reset onto `dst`, compressing at the default
// level. Once used, it should be sent back to the pool via `PutGzipWriter`
func GzipWriter(dst io.Writer) *Writer {
	return GzipWriterLevel(dst, gzip.DefaultCompression)
}

// GzipWriterLevel returns a gzip writer reset onto `dst`, compressing at `level`.
// Once used, it should be sent back to the pool via `PutGzipWriter`
func GzipWriterLevel(dst io.Writer, level int) *Writer {
	writer := gzipWriterPools[poolIndex(level)].Get().(*Writer)
	writer.Reset(dst)
	return writer
}

// PutGzipWriter returns `writer` to the pool for its level. The writer need not be
// in a good state: Reset clears any sticky write error, so error paths should
// return it rather than drop it.
func PutGzipWriter(writer *Writer) {
	gzipWriterPools[poolIndex(writer.level)].Put(writer)
}

func Gzip64Encode(data []byte) (string, error) {
	var compressedBuffer bytes.Buffer
	gzipWriter := GzipWriter(&compressedBuffer)
	defer PutGzipWriter(gzipWriter)
	if _, err := gzipWriter.Write(data); err != nil {
		return "", fmt.Errorf("gzip data: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("close gzip writer: %v", err)
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
