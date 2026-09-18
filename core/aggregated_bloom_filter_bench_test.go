package core_test

import (
	"crypto/rand"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

type blocksForKeysQuerier interface {
	BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error
}

func benchBlocksForKeysSetup(b *testing.B) (core.AggregatedBloomFilter, [][]byte) {
	b.Helper()
	filter := core.NewAggregatedFilter(0)
	keys := make([][]byte, 4)
	for i := range keys {
		keys[i] = make([]byte, 32)
		_, err := rand.Read(keys[i])
		require.NoError(b, err)

		bf := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bf.Add(keys[i])
		require.NoError(b, filter.Insert(bf, uint64(i)*17))
	}
	return filter, keys
}

func benchBlocksForKeysInto(b *testing.B, querier blocksForKeysQuerier, keys [][]byte) {
	b.Helper()
	out := bitset.New(uint(core.NumBlocksPerFilter))
	b.ReportAllocs()
	for b.Loop() {
		if err := querier.BlocksForKeysInto(keys, out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAggregatedBloomFilterBlocksForKeysInto(b *testing.B) {
	filter, keys := benchBlocksForKeysSetup(b)
	benchBlocksForKeysInto(b, &filter, keys)
}

func BenchmarkAggregatedBloomFilterViewBlocksForKeysInto(b *testing.B) {
	filter, keys := benchBlocksForKeysSetup(b)
	data, err := filter.MarshalBinary()
	require.NoError(b, err)

	var view core.AggregatedBloomFilterView
	require.NoError(b, view.UnmarshalBinary(data))
	benchBlocksForKeysInto(b, &view, keys)
}

func BenchmarkAggregatedBloomFilterUnmarshal(b *testing.B) {
	filter := core.NewAggregatedFilter(0)
	bf := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	bf.Add([]byte{0x01})
	require.NoError(b, filter.Insert(bf, 0))

	data, err := filter.MarshalBinary()
	require.NoError(b, err)

	b.ReportAllocs()
	for b.Loop() {
		var decoded core.AggregatedBloomFilter
		if err := decoded.UnmarshalBinary(data); err != nil {
			b.Fatal(err)
		}
	}
}
