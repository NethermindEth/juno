package mempool_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mempool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMempool(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	pool := mempool.New(testDB)
	blockchain.RegisterCoreTypesToEncoder()

	t.Run("empty pool", func(t *testing.T) {
		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, uint64(0), l)

		_, err = pool.Pop()
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	// push multiple to empty
	for i := uint64(0); i < 3; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, i+1, l)
	}

	// consume some
	for i := uint64(0); i < 2; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, 3-i-1, l)
	}

	// push multiple to non empty
	for i := uint64(3); i < 5; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, i-1, l)
	}

	// consume all
	for i := uint64(2); i < 5; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())
	}

	l, err := pool.Len()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), l)

	_, err = pool.Pop()
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	// validation error
	pool = pool.WithValidator(func(bt *mempool.BroadcastedTransaction) error {
		return errors.New("some error")
	})
	require.EqualError(t, pool.Push(&mempool.BroadcastedTransaction{}), "some error")
}
