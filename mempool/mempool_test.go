package mempool_test

import (
	"errors"
	"fmt"
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

	// push multiple to empty (1,2,3)
	for i := uint64(1); i < 4; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, i, l)
	}

	// consume some (remove 1,2, keep 3)
	for i := uint64(1); i < 3; i++ {
		txn, err := pool.Pop()
		fmt.Println("txn", txn.Transaction.Hash().String())
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, 3-i, l)
	}

	// push multiple to non empty (push 4,5. now have 3,4,5)
	for i := uint64(4); i < 6; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l, err := pool.Len()
		require.NoError(t, err)
		assert.Equal(t, i-2, l)
	}

	// consume all (remove 3,4,5)
	for i := uint64(3); i < 6; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())
	}

	l, err := pool.Len()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), l)

	_, err = pool.Pop()
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	// reject duplicate txn
	txn := mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(2),
		},
	}
	require.NoError(t, pool.Push(&txn))
	require.Error(t, pool.Push(&txn))

	// validation error
	pool = pool.WithValidator(func(bt *mempool.BroadcastedTransaction) error {
		return errors.New("some error")
	})
	require.EqualError(t, pool.Push(&mempool.BroadcastedTransaction{}), "some error")
}

func TestWait(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	pool := mempool.New(testDB)
	blockchain.RegisterCoreTypesToEncoder()

	select {
	case <-pool.Wait():
		require.Fail(t, "wait channel should not be signalled on empty mempool")
	default:
	}

	// One transaction.
	require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(1),
		},
	}))
	<-pool.Wait()

	// Two transactions.
	require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(2),
		},
	}))
	require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(3),
		},
	}))
	<-pool.Wait()
}
