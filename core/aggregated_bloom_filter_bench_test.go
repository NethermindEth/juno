package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

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
