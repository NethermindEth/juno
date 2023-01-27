package datasource

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestFetcher(t *testing.T) {
	fromHeight := uint64(44)
	ds := &FakeDataSource{}
	fetcher := NewFetcher(ds, fromHeight, 3)
	assert.NoError(t, fetcher.Run())

	for i := 0; i < 100; i++ {
		next, err := fetcher.GetNext()
		assert.NoError(t, err)
		assert.Equal(t, fromHeight, next.Block.Number)
		assert.Equal(t, new(felt.Felt).SetUint64(fromHeight), next.Update.BlockHash)
		fromHeight++
	}

	assert.NoError(t, fetcher.Shutdown())

	var err error
	for i := 0; i < 10; i++ {
		_, err = fetcher.GetNext() // deplete the data in the channels
		if err != nil {
			break
		}
	}
	assert.EqualError(t, err, "fetcher is shutdown")
}
