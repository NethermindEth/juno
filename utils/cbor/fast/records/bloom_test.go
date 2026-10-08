package records

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

func bloomBytes(tb testing.TB, filter *bloom.BloomFilter) []byte {
	tb.Helper()
	data, err := filter.MarshalBinary()
	require.NoError(tb, err)
	return data
}

// rawBloom is a binary form with the given m, k and bit count, followed by words.
func rawBloom(m, k, bits uint64, words ...uint64) []byte {
	data := binary.BigEndian.AppendUint64(nil, m)
	data = binary.BigEndian.AppendUint64(data, k)
	data = binary.BigEndian.AppendUint64(data, bits)
	for _, w := range words {
		data = binary.BigEndian.AppendUint64(data, w)
	}
	return data
}

// requireSameBloom decodes data with decodeBloom and with the bloom package and requires
// the same outcome: both fail, or both build equal filters that answer Test alike.
func requireSameBloom(t *testing.T, data []byte) {
	t.Helper()
	got, gotErr := decodeBloom(data)
	want := new(bloom.BloomFilter)
	wantErr := want.UnmarshalBinary(data)
	if wantErr != nil {
		require.Error(t, gotErr)
		return
	}
	require.NoError(t, gotErr)
	// require.Equal would print a diff of every word, which takes minutes on a large set.
	require.True(t, reflect.DeepEqual(want, got), "m=%d k=%d bits=%d", want.Cap(), want.K(), want.BitSet().Len())
	require.True(t, want.Equal(got))
	require.Equal(t, bloomBytes(t, want), bloomBytes(t, got))
	// Test divides by m and hashes k times.
	if want.Cap() == 0 || want.K() > 64 {
		return
	}
	for i := range 64 {
		key := fmt.Appendf(nil, "key %d", i)
		require.Equal(t, want.Test(key), got.Test(key), "key %q", key)
	}
}

func TestDecodeBloomMatchesTheBloomPackage(t *testing.T) {
	full := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	for i := range 300 {
		full.Add(fmt.Appendf(nil, "event %d", i))
	}
	oneWord := bloom.New(64, 1)
	oneWord.Add([]byte("a"))
	elevenWords := bloom.New(11*64, 3) // a round of eight words and three more
	for i := range 40 {
		elevenWords.Add(fmt.Appendf(nil, "event %d", i))
	}
	odd := bloom.New(100, 3) // 100 bits, not a whole number of words
	odd.Add([]byte("a"))

	cases := map[string][]byte{
		"events":              bloomBytes(t, full),
		"empty events":        bloomBytes(t, bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)),
		"one word":            bloomBytes(t, oneWord),
		"eleven words":        bloomBytes(t, elevenWords),
		"odd bit count":       bloomBytes(t, odd),
		"no bits":             bloomBytes(t, bloom.New(0, 0)),
		"zero m and k":        rawBloom(0, 0, 0),
		"m differs from bits": rawBloom(7, 2, 128, 1, 1<<63),
		"huge m and k":        rawBloom(1<<63, 1<<40, 64, 0xff),
		"trailing bytes":      append(rawBloom(64, 2, 64, 5), 1, 2, 3),
		"odd count short":     rawBloom(64, 2, 65, 1, 0)[:3*8+9],
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) { requireSameBloom(t, data) })
	}
}

func TestDecodeBloomRejectsCountsLongerThanTheBytes(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":          nil,
		"no bit count":   rawBloom(64, 1, 0)[:2*8],
		"one bit over":   rawBloom(64, 1, 65, 1),
		"one word short": rawBloom(64, 1, 128, 1),
		"2^58 bits":      rawBloom(64, 1, 1<<58, 1),
		"max count":      rawBloom(64, 1, ^uint64(0), 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeBloom(data)
			require.ErrorIs(t, err, ErrBloomSize)
		})
	}
}

// The bloom package is only safe on inputs whose count fits the bytes, so the fuzz target
// compares it on those and requires ErrBloomSize on the others.
func FuzzDecodeBloom(f *testing.F) {
	f.Add(bloomBytes(f, bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)))
	f.Add(bloomBytes(f, bloom.New(100, 3)))
	f.Add(rawBloom(0, 0, 0))
	f.Add(rawBloom(64, 2, 64, 5))
	f.Fuzz(func(t *testing.T, data []byte) {
		if !bloomSizeMatches(data) {
			_, err := decodeBloom(data)
			require.ErrorIs(t, err, ErrBloomSize)
			return
		}
		requireSameBloom(t, data)
	})
}

func BenchmarkDecodeBloom(b *testing.B) {
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	filter.Add([]byte("event"))
	data := bloomBytes(b, filter)
	b.Run("decodeBloom", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := decodeBloom(data); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("UnmarshalBinary", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if err := new(bloom.BloomFilter).UnmarshalBinary(data); err != nil {
				b.Fatal(err)
			}
		}
	})
}
