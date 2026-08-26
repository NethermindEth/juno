package compression_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"weak"

	"github.com/NethermindEth/juno/utils/compression"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzip64(t *testing.T) {
	bytes := []byte{0}
	// klauspost writes uint32(zero time.Time) as MTIME, a fixed constant rather than 0.
	expectedComBytes := "H4sIAAAJbogA/wABAP7/AAMAje8C0gEAAAA="
	comBytes, err := compression.Gzip64Encode(bytes)
	require.NoError(t, err)
	assert.Equal(t, expectedComBytes, comBytes)

	decompBytes, err := compression.Gzip64Decode(comBytes, compression.NoLimit)
	require.NoError(t, err)
	assert.Equal(t, bytes, decompBytes)
}

func TestGzip64Decode(t *testing.T) {
	const limit = 1024
	tests := []struct {
		name            string
		payload         []byte
		limit           int64
		wantErrContains string // empty means the payload is expected to round-trip
	}{
		{
			name:    "within limit",
			payload: bytes.Repeat([]byte("a"), limit/2),
			limit:   limit,
		},
		{
			name:    "unbounded limit",
			payload: []byte{0},
			limit:   compression.NoLimit,
		},
		{
			name:    "empty payload",
			payload: []byte{},
			limit:   limit,
		},
		{
			// The budget is inclusive: a payload that exactly fills it is valid.
			name:    "exactly at limit",
			payload: bytes.Repeat([]byte("a"), limit),
			limit:   limit,
		},
		{
			// One byte over is the smallest overflow the +1 read must catch.
			name:            "one byte over limit",
			payload:         bytes.Repeat([]byte("a"), limit+1),
			limit:           limit,
			wantErrContains: "decompressed data exceeded the maximum byte size:",
		},
		{
			name:            "zero limit rejects any content",
			payload:         []byte("x"),
			limit:           0,
			wantErrContains: "decompressed data exceeded the maximum byte size:",
		},
		{
			name:    "zero limit accepts empty payload",
			payload: []byte{},
			limit:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := compression.Gzip64Encode(test.payload)
			require.NoError(t, err)

			decoded, err := compression.Gzip64Decode(encoded, test.limit)
			if test.wantErrContains != "" {
				assert.ErrorContains(t, err, test.wantErrContains)
				assert.Nil(t, decoded)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.payload, decoded)
		})
	}
}

// A compression bomb — a few KiB of input inflating to 64 MiB — must be rejected
// cheaply.
func TestGzip64DecodeCompressionBomb(t *testing.T) {
	const limit = 1024
	bomb := make([]byte, 64*1024*1024) // zeros compress ~1000:1
	encoded, err := compression.Gzip64Encode(bomb)
	require.NoError(t, err)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	decoded, err := compression.Gzip64Decode(encoded, limit)
	runtime.ReadMemStats(&after)

	assert.ErrorContains(t, err, "decompressed data exceeded the maximum byte size:")
	assert.Nil(t, decoded)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(8*1024*1024),
		"rejecting an over-limit stream must not materialise the decompressed payload")
}

// Check stream corruption is properly shown.
func TestGzip64DecodeCorruptStream(t *testing.T) {
	const limit = 1024
	payload := bytes.Repeat([]byte("a"), limit)
	encoded, err := compression.Gzip64Encode(payload)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	// Corrupt the gzip footer
	t.Run("corrupt checksum at exactly the limit", func(t *testing.T) {
		corrupted := bytes.Clone(raw)
		corrupted[len(corrupted)-1] ^= 0xff // corrupt the footer

		decoded, err := compression.Gzip64Decode(
			base64.StdEncoding.EncodeToString(corrupted), limit,
		)
		assert.ErrorIs(t, err, gzip.ErrChecksum)
		assert.Nil(t, decoded)
	})

	t.Run("truncated stream", func(t *testing.T) {
		decoded, err := compression.Gzip64Decode(
			// remove the footer and part of the data.
			base64.StdEncoding.EncodeToString(raw[:len(raw)/2]),
			limit,
		)

		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.Nil(t, decoded)
	})
}

func FuzzGzip64(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		compressed, err := compression.Gzip64Encode(data)
		require.NoError(t, err)
		decompressed, err := compression.Gzip64Decode(compressed, compression.NoLimit)
		require.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})
}

// Successive encodes draw the same writer back out of the pool. The sizes below
// are deliberately not monotonic: a writer that carried state over from the
// previous encode would trail bytes of a longer payload into a shorter one, which
// a run of same-or-growing sizes would hide.
func TestGzip64EncodeAcrossSuccessiveCalls(t *testing.T) {
	for _, size := range []int{4096, 2048, 64, 8192, 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte("a"), size)

			encoded, err := compression.Gzip64Encode(payload)
			require.NoError(t, err)
			decoded, err := compression.Gzip64Decode(encoded, compression.NoLimit)
			require.NoError(t, err)
			assert.Equal(t, payload, decoded)
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("destination unavailable")
}

// A writer whose destination failed is still safe to hand back: the next caller
// gets output identical to what an untouched writer produces. The expected value
// comes from gzipAtLevel rather than Gzip64Encode so the oracle cannot itself be
// tainted by the pool under test.
func TestGzipWriterAfterFailedDestination(t *testing.T) {
	const repeats = 4096
	payload := bytes.Repeat([]byte("compress me "), repeats)

	poisoned := compression.GzipWriter(failingWriter{})
	_, writeErr := poisoned.Write(payload)
	closeErr := poisoned.Close()
	require.Error(t, errors.Join(writeErr, closeErr), "failing destination must fault the writer")
	poisoned.Release()

	var buf bytes.Buffer
	reused := compression.GzipWriter(&buf)
	_, err := reused.Write(payload)
	require.NoError(t, err)
	require.NoError(t, reused.Close())
	reused.Release()

	assert.Equal(t, gzipAtLevel(t, payload, gzip.DefaultCompression), buf.Bytes())
}

// After Release the pooled writer must hold no reference to the caller's
// destination: the destination has to be collectable while the writer lives on
// in the pool. Holding gzipWriter across the GC is deliberate — it pins the
// writer so the assertion is about the dst reference, not the writer itself
// being collected.
func TestGzipWriterReleaseDropsDestination(t *testing.T) {
	dst := &bytes.Buffer{}
	gzipWriter := compression.GzipWriter(dst)
	_, err := gzipWriter.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	weakDst := weak.Make(dst)
	gzipWriter.Release()
	dst = nil //nolint:ineffassign // drop the last strong reference so GC can collect the buffer
	runtime.GC()
	assert.Nil(t, weakDst.Value(), "released writer still references its destination")
	runtime.KeepAlive(gzipWriter)
}

// Releasing twice would hand the same writer to two callers at once, so the
// second Release must fail loudly rather than corrupt a concurrent caller.
func TestGzipWriterDoubleReleasePanics(t *testing.T) {
	gzipWriter := compression.GzipWriter(io.Discard)
	gzipWriter.Release()
	assert.Panics(t, func() { gzipWriter.Release() })
}

// A released writer may already be owned by another goroutine, so using it must
// be refused rather than silently corrupt the new owner's output.
func TestGzipWriterUseAfterReleaseErrors(t *testing.T) {
	gzipWriter := compression.GzipWriter(io.Discard)
	gzipWriter.Release()

	_, err := gzipWriter.Write([]byte("stale"))
	assert.ErrorIs(t, err, compression.ErrWriterNotAcquired)
	assert.ErrorIs(t, gzipWriter.Close(), compression.ErrWriterNotAcquired)
	assert.ErrorIs(t, gzipWriter.Flush(), compression.ErrWriterNotAcquired)
}

// gzipAtLevel is the reference encoding: a writer built at `level` and used once,
// so no pool can influence the result.
func gzipAtLevel(t *testing.T, data []byte, level int) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buf, level)
	require.NoError(t, err)
	_, err = gzipWriter.Write(data)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())
	return buf.Bytes()
}

// Each level keeps its own writers. Returning a writer at one level must never
// let it come back out at another, which would silently compress at the wrong
// level: the only symptom is a differently sized output, never an error.
func TestGzipWriterLevelsDoNotMix(t *testing.T) {
	const repeats = 8192
	payload := bytes.Repeat([]byte("mixed levels must not share writers "), repeats)

	// HuffmanOnly and NoCompression are included because they take a different
	// reset path inside flate than the levels that build a match chain.
	levels := []int{
		gzip.HuffmanOnly,
		gzip.NoCompression,
		gzip.BestSpeed,
		gzip.DefaultCompression,
		gzip.BestCompression,
	}

	// Cycle the levels so every pool has been drawn from and returned to before the
	// assertions below, giving a misfiled writer the chance to surface.
	for range 2 {
		for _, level := range levels {
			compression.GzipWriterLevel(io.Discard, level).Release()
		}
	}

	for _, level := range levels {
		t.Run(strconv.Itoa(level), func(t *testing.T) {
			var buf bytes.Buffer
			gzipWriter := compression.GzipWriterLevel(&buf, level)
			_, err := gzipWriter.Write(payload)
			require.NoError(t, err)
			require.NoError(t, gzipWriter.Close())
			gzipWriter.Release()

			assert.Equal(t, gzipAtLevel(t, payload, level), buf.Bytes())
		})
	}

	// Guard the premise: these levels really do produce different output, so the
	// assertions above could actually fail if writers were shared.
	fastest := gzipAtLevel(t, payload, gzip.BestSpeed)
	smallest := gzipAtLevel(t, payload, gzip.BestCompression)
	require.NotEqual(t, len(fastest), len(smallest), "levels must be distinguishable")
}

func TestGzipWriterLevelRejectsOutOfRange(t *testing.T) {
	assert.Panics(t, func() { compression.GzipWriterLevel(io.Discard, gzip.BestCompression+1) })
	assert.Panics(t, func() { compression.GzipWriterLevel(io.Discard, gzip.HuffmanOnly-1) })
}

// Concurrent callers must not end up sharing a writer. Run under -race.
func TestGzip64EncodeConcurrent(t *testing.T) {
	const (
		goroutines = 16
		chunk      = 512
	)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			payload := bytes.Repeat([]byte{byte('a' + i)}, chunk*(i+1))
			encoded, err := compression.Gzip64Encode(payload)
			assert.NoError(t, err)
			decoded, err := compression.Gzip64Decode(encoded, compression.NoLimit)
			assert.NoError(t, err)
			assert.Equal(t, payload, decoded)
		})
	}
	wg.Wait()
}
