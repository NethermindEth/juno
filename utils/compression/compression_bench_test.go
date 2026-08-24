package compression_test

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/utils/compression"
)

var benchSizes = []int{16 << 10, 256 << 10, 2 << 20}

func programLike(size int) []byte {
	tokens := []string{
		`{"prime":"0x800000000000011000000000000000000000000000000000000000000000001",`,
		`"data":["0x480680017fff8000","0x1","0x48127ffe7fff8000","0x208b7fff7fff7ffe"],`,
		`"attributes":[],"debug_info":null,"builtins":["range_check","pedersen"],`,
		`"hints":{"0":[{"accessible_scopes":["starkware.cairo.common.math"]}]},`,
	}

	var buf bytes.Buffer
	buf.Grow(size)
	for i := 0; buf.Len() < size; i++ {
		buf.WriteString(tokens[i%len(tokens)])
	}
	return buf.Bytes()[:size]
}

func BenchmarkGzip64Encode(b *testing.B) {
	for _, size := range benchSizes {
		data := programLike(size)
		b.Run(strconv.Itoa(size>>10)+"KiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				if _, err := compression.Gzip64Encode(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGzip64Decode(b *testing.B) {
	for _, size := range benchSizes {
		encoded, err := compression.Gzip64Encode(programLike(size))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(strconv.Itoa(size>>10)+"KiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				if _, err := compression.Gzip64Decode(encoded, math.MaxInt64); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// GzipWriter is also used directly by callers that stream into a destination
// rather than building a base64 string, e.g. the JSON-RPC HTTP handler.
func BenchmarkGzipWriter(b *testing.B) {
	data := programLike(256 << 10)
	var sink bytes.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		sink.Reset()
		gzipWriter := compression.GzipWriter(&sink)
		if _, err := gzipWriter.Write(data); err != nil {
			b.Fatal(err)
		}
		if err := gzipWriter.Close(); err != nil {
			b.Fatal(err)
		}
		gzipWriter.Release()
	}
}
