package core_test

import (
	"encoding/binary"
	"io"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

func TestAggregatedBloomFilter_Insert(t *testing.T) {
	filter := core.NewAggregatedFilter(200)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0x01, 0x02}
	b.Add(key)

	t.Run("Insert at valid block in range", func(t *testing.T) {
		tests := []struct {
			description string
			blockNumber uint64
		}{
			{
				description: "first block in range",
				blockNumber: 200,
			},
			{
				description: "between (first, last) block in range",
				blockNumber: 201,
			},
			{
				description: "last block in range",
				blockNumber: 200 + core.MaxBlockOffsetPerFilter,
			},
		}

		for _, test := range tests {
			require.NoError(t, filter.Insert(b, test.blockNumber))
			matches := filter.BlocksForKeys([][]byte{key})
			relativeBlockNumber := test.blockNumber - filter.FromBlock()
			require.True(t, matches.Test(uint(relativeBlockNumber)))
		}
	})

	t.Run("Insert at out-of-range block", func(t *testing.T) {
		require.ErrorIs(t, filter.Insert(b, filter.FromBlock()-1), core.ErrAggregatedBloomFilterBlockOutOfRange)
		require.ErrorIs(t, filter.Insert(b, filter.ToBlock()+1), core.ErrAggregatedBloomFilterBlockOutOfRange)
	})

	t.Run("Insert with wrong-sized bloom", func(t *testing.T) {
		differentSizeBloom := bloom.New(core.EventsBloomLength-1, core.EventsBloomHashFuncs)

		err := filter.Insert(differentSizeBloom, 201)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrBloomFilterSizeMismatch)
	})
}

func TestAggregatedBloomFilter_BlocksForKeys(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0xab}
	b.Add(key)
	require.NoError(t, filter.Insert(b, 0))

	t.Run("No keys: all bits set", func(t *testing.T) {
		matches := filter.BlocksForKeys(nil)
		require.True(t, matches.All())
	})

	t.Run("Unmatched key: returns none", func(t *testing.T) {
		matches := filter.BlocksForKeys([][]byte{{0xff}})
		require.False(t, matches.Any())
	})

	t.Run("Known key: block 0 is set", func(t *testing.T) {
		matches := filter.BlocksForKeys([][]byte{key})
		require.True(t, matches.Test(0))
	})
}

func TestAggregatedBloomFilter_BlocksForKeysInto(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0xab}
	b.Add(key)
	require.NoError(t, filter.Insert(b, 0))
	matchesBuf := bitset.New(uint(core.NumBlocksPerFilter))
	t.Run("No keys: all bits set", func(t *testing.T) {
		require.NoError(t, filter.BlocksForKeysInto(nil, matchesBuf))
		require.True(t, matchesBuf.All())
	})

	t.Run("Unmatched key: returns none", func(t *testing.T) {
		require.NoError(t, filter.BlocksForKeysInto([][]byte{{0xff}}, matchesBuf))
		require.False(t, matchesBuf.Any())
	})

	t.Run("Known key: block 0 is set", func(t *testing.T) {
		require.NoError(t, filter.BlocksForKeysInto([][]byte{key}, matchesBuf))
		require.True(t, matchesBuf.Test(0))
	})

	t.Run("Buffer size mismatch", func(t *testing.T) {
		differentSizeBuf := bitset.New(uint(core.NumBlocksPerFilter - 1))
		require.ErrorIs(
			t,
			filter.BlocksForKeysInto([][]byte{key}, differentSizeBuf),
			core.ErrMatchesBufferSizeMismatch,
		)
	})

	t.Run("Buffer is nil", func(t *testing.T) {
		require.ErrorIs(
			t,
			filter.BlocksForKeysInto([][]byte{key}, nil),
			core.ErrMatchesBufferNil,
		)
	})
}

func TestAggregatedBloomFilter_Clone(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0x77}
	b.Add(key)
	require.NoError(t, filter.Insert(b, 0))
	cp := filter.Clone()
	require.NotSame(t, &filter, &cp)
	require.Equal(t, filter, cp)
	// Mutate copy: shouldn't change origin
	require.NoError(t, cp.Insert(b, 1))
	require.True(t, cp.BlocksForKeys([][]byte{key}).Test(1))
	require.False(t, filter.BlocksForKeys([][]byte{key}).Test(1))
}

func TestAggregatedBloomFilter_Serialise(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0x33}
	b.Add(key)
	require.NoError(t, filter.Insert(b, 0))

	data, err := filter.MarshalBinary()
	require.NoError(t, err)

	filter2 := &core.AggregatedBloomFilter{}
	require.NoError(t, filter2.UnmarshalBinary(data))
	require.Equal(t, filter, *filter2)
}

func TestAggregatedBloomFilter_UnmarshalBinary_Compat(t *testing.T) {
	// Build a filter with a couple of keys at known blocks.
	filter := core.NewAggregatedFilter(0)
	keyA := []byte{0x33}
	keyB := []byte{0xAB, 0xCD}

	bloomA := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	bloomA.Add(keyA)
	require.NoError(t, filter.Insert(bloomA, 0))

	bloomB := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	bloomB.Add(keyB)
	require.NoError(t, filter.Insert(bloomB, 5))

	data, err := filter.MarshalBinary()
	require.NoError(t, err)

	// Round-trips into an equal value.
	var decoded core.AggregatedBloomFilter
	require.NoError(t, decoded.UnmarshalBinary(data))
	require.Equal(t, filter, decoded)

	// Matching behavior is preserved after decode.
	require.True(t, decoded.BlocksForKeys([][]byte{keyA}).Test(0))
	require.True(t, decoded.BlocksForKeys([][]byte{keyB}).Test(5))
	require.False(t, decoded.BlocksForKeys([][]byte{keyB}).Test(0))

	// Short/corrupt input returns a non-nil, non-panicking result.
	require.Error(t, decoded.UnmarshalBinary(data[:10]))
	require.Error(t, decoded.UnmarshalBinary(nil))
}

// Round-trips a filter whose range does not start at block 0, so a byte-offset
// bug in the header parse cannot hide behind fromBlock == 0.
func TestAggregatedBloomFilter_UnmarshalBinary_NonZeroRange(t *testing.T) {
	const from = 3 * core.NumBlocksPerFilter // distinctive, non-zero start
	filter := core.NewAggregatedFilter(from)
	key := []byte{0x5e}
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	b.Add(key)
	require.NoError(t, filter.Insert(b, from+7))

	data, err := filter.MarshalBinary()
	require.NoError(t, err)

	var decoded core.AggregatedBloomFilter
	require.NoError(t, decoded.UnmarshalBinary(data))
	require.Equal(t, filter, decoded)
	require.Equal(t, from, decoded.FromBlock())
	require.Equal(t, from+core.MaxBlockOffsetPerFilter, decoded.ToBlock())
	require.True(t, decoded.BlocksForKeys([][]byte{key}).Test(7))
}

// Pins the on-disk header layout (fromBlock:uint64, toBlock:uint64,
// count:uint32, all big-endian) that UnmarshalBinary relies on, guarding
// against a future change to MarshalBinary silently breaking the wire format.
func TestAggregatedBloomFilter_MarshalBinary_HeaderLayout(t *testing.T) {
	const from = 0x0102030405060708
	filter := core.NewAggregatedFilter(from)

	data, err := filter.MarshalBinary()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 20)

	require.Equal(t, uint64(from), binary.BigEndian.Uint64(data[0:8]))
	require.Equal(t, from+core.MaxBlockOffsetPerFilter, binary.BigEndian.Uint64(data[8:16]))
	require.Equal(t, uint32(core.EventsBloomLength), binary.BigEndian.Uint32(data[16:20]))
}

// Exercises the decode validation and bounds-checking branches: a row with a
// wrong bit-length header is rejected, and truncation mid-row does not panic.
func TestAggregatedBloomFilter_UnmarshalBinary_Corrupt(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	b.Add([]byte{0x01})
	require.NoError(t, filter.Insert(b, 0))

	data, err := filter.MarshalBinary()
	require.NoError(t, err)

	t.Run("wrong row bit-length", func(t *testing.T) {
		corrupt := make([]byte, len(data))
		copy(corrupt, data)
		// Row 0 blob starts after the 20-byte header + 4-byte blob length;
		// its first 8 bytes are the bitset bit-length header.
		binary.BigEndian.PutUint64(corrupt[24:], core.NumBlocksPerFilter+1)

		var decoded core.AggregatedBloomFilter
		require.ErrorIs(t, decoded.UnmarshalBinary(corrupt), core.ErrBloomFilterSizeMismatch)
	})

	t.Run("truncated mid-row", func(t *testing.T) {
		var decoded core.AggregatedBloomFilter
		require.ErrorIs(t, decoded.UnmarshalBinary(data[:30]), io.ErrUnexpectedEOF)
	})

	t.Run("wrong row count", func(t *testing.T) {
		corrupt := make([]byte, len(data))
		copy(corrupt, data)
		binary.BigEndian.PutUint32(corrupt[16:20], core.EventsBloomLength-1)

		var decoded core.AggregatedBloomFilter
		require.ErrorIs(t, decoded.UnmarshalBinary(corrupt), core.ErrBloomFilterSizeMismatch)
	})

	t.Run("toBlock not matching range", func(t *testing.T) {
		corrupt := make([]byte, len(data))
		copy(corrupt, data)
		binary.BigEndian.PutUint64(corrupt[8:16], 1<<40)

		var decoded core.AggregatedBloomFilter
		require.ErrorIs(t, decoded.UnmarshalBinary(corrupt), core.ErrBloomFilterSizeMismatch)
	})

	t.Run("trailing bytes", func(t *testing.T) {
		withJunk := append([]byte{}, data...)
		withJunk = append(withJunk, 0x00, 0x01, 0x02, 0x03)
		var decoded core.AggregatedBloomFilter
		require.ErrorIs(t, decoded.UnmarshalBinary(withJunk), io.ErrUnexpectedEOF)
	})
}

// UnmarshalBinary decodes untrusted DB bytes and must never panic on arbitrary
// input, only return an error.
func FuzzAggregatedBloomFilterUnmarshal(f *testing.F) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	b.Add([]byte{0x01})
	require.NoError(f, filter.Insert(b, 0))
	valid, err := filter.MarshalBinary()
	require.NoError(f, err)

	f.Add([]byte(nil))
	f.Add([]byte{0x00})
	f.Add(valid)
	f.Add(valid[:25])
	// Header claiming a huge row count must not drive an unbounded allocation.
	hugeCount := make([]byte, 20)
	binary.BigEndian.PutUint32(hugeCount[16:], 0xFFFFFFFF)
	f.Add(hugeCount)

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded core.AggregatedBloomFilter
		_ = decoded.UnmarshalBinary(data) // must not panic
	})
}

func TestAggregatedBloomFilter_UnmarshalBinary_Allocs(t *testing.T) {
	// A fully populated filter is the worst case: all EventsBloomLength rows present.
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	b.Add([]byte{0x01})
	require.NoError(t, filter.Insert(b, 0))

	data, err := filter.MarshalBinary()
	require.NoError(t, err)

	var decoded core.AggregatedBloomFilter
	allocs := testing.AllocsPerRun(20, func() {
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatal(err)
		}
	})
	// One backing []uint64 + one []bitset.BitSet header. Allow slack for
	// interface boxing in AllocsPerRun itself; current code is ~2.
	require.LessOrEqual(t, allocs, float64(8),
		"decode should allocate O(1) buffers, got %.0f", allocs)
}
