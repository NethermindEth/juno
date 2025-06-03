package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

func TestAggregatedBloomFilter(t *testing.T) {
	key0, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	key1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	t.Run("Add bloom and check single block found", func(t *testing.T) {
		fromBlock := 0
		aggFilter := core.NewAggregatedFilter(uint64(fromBlock))

		bloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())
		bloom.Add(key1.Marshal())

		require.NoError(t, aggFilter.Insert(bloom, uint64(fromBlock)))

		matches := aggFilter.BlocksForKeys([][]byte{key0.Marshal(), key1.Marshal()})

		relativeBlockNumbers := make([]uint, matches.Count())
		_, relativeBlockNumbers = matches.NextSetMany(0, relativeBlockNumbers)
		expected := []uint{0}
		require.Equal(t, expected, relativeBlockNumbers)
	})

	t.Run("Add blooms and check multiple blocks found", func(t *testing.T) {
		fromBlock := 0
		aggFilter := core.NewAggregatedFilter(uint64(fromBlock))

		bloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())
		bloom.Add(key1.Marshal())

		require.NoError(t, aggFilter.Insert(bloom, uint64(fromBlock)))
		require.NoError(t, aggFilter.Insert(bloom, uint64(fromBlock+1)))

		matches := aggFilter.BlocksForKeys([][]byte{key0.Marshal()})

		relativeBlockNumbers := make([]uint, matches.Count())
		_, relativeBlockNumbers = matches.NextSetMany(0, relativeBlockNumbers)
		expected := []uint{0, 1}
		require.Equal(t, expected, relativeBlockNumbers)
	})

	t.Run("No match should returns empty bitmap", func(t *testing.T) {
		fromBlock := 0
		aggFilter := core.NewAggregatedFilter(uint64(fromBlock))

		bloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())

		require.NoError(t, aggFilter.Insert(bloom, uint64(fromBlock)))

		matches := aggFilter.BlocksForKeys([][]byte{key1.Marshal()})

		relativeBlockNumbers := make([]uint, matches.Count())
		_, relativeBlockNumbers = matches.NextSetMany(0, relativeBlockNumbers)
		expected := []uint{}
		require.Equal(t, expected, relativeBlockNumbers)
	})

	t.Run("Insert out of range block number", func(t *testing.T) {
		var fromBlock uint64 = 100
		aggFilter := core.NewAggregatedFilter(fromBlock)

		bloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())

		// block number > upper bound
		err := aggFilter.Insert(bloom, fromBlock+core.AggregateBloomBlockRangeLen)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrAggregatedBloomFilterBlockOutOfRange)

		// block number < upper bound
		err = aggFilter.Insert(bloom, fromBlock-1)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrAggregatedBloomFilterBlockOutOfRange)
	})

	t.Run("Insert bloom filter of different size", func(t *testing.T) {
		var fromBlock uint64 = 100
		aggFilter := core.NewAggregatedFilter(fromBlock)

		bloom := bloom.New(core.EventsBloomLength-1, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())

		// block number > upper bound
		err := aggFilter.Insert(bloom, fromBlock)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrBloomFilterSizeMismatch)
	})

	t.Run("Encode decode", func(t *testing.T) {
		fromBlock := 0
		aggFilter := core.NewAggregatedFilter(uint64(fromBlock))

		bloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		bloom.Add(key0.Marshal())

		require.NoError(t, aggFilter.Insert(bloom, uint64(fromBlock)))
		marshalledFilter, err := encoder.Marshal(aggFilter)
		require.NoError(t, err)
		var unMarshalled core.AggregatedBloomFilter
		require.NoError(t, encoder.Unmarshal(marshalledFilter, &unMarshalled))
		require.Equal(t, aggFilter, &unMarshalled)
	})
}
