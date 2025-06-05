package core_test

import (
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
				blockNumber: 200 + core.AggregateBloomBlockRangeLen - 1,
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
	matchesBuf := bitset.New(uint(core.AggregateBloomBlockRangeLen))
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
		differentSizeBuf := bitset.New(uint(core.AggregateBloomBlockRangeLen - 1))
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

func TestAggregatedBloomFilter_Copy(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0x77}
	b.Add(key)
	require.NoError(t, filter.Insert(b, 0))
	cp := filter.Copy()
	require.NotSame(t, filter, cp)
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
	require.Equal(t, filter, filter2)
}
