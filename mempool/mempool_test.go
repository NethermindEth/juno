package mempool_test

import (
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mempool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDatabase(dltExisting bool) (*db.DB, func(), error) {
	dbPath := "testmempool"
	if _, err := os.Stat(dbPath); err == nil {
		if dltExisting {
			if err := os.RemoveAll(dbPath); err != nil {
				return nil, nil, err
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}
	db, err := pebble.New(dbPath)
	if err != nil {
		return nil, nil, err
	}
	closer := func() {
		// The db should be closed by the mempool closer function
		os.RemoveAll(dbPath)
	}
	return &db, closer, nil
}

func TestMempool(t *testing.T) {
	testDB, dbCloser, err := setupDatabase(true)
	require.NoError(t, err)
	defer dbCloser()
	pool, closer, err := mempool.New(*testDB, 5)
	defer closer()
	require.NoError(t, err)
	blockchain.RegisterCoreTypesToEncoder()

	l := pool.Len()
	assert.Equal(t, uint16(0), l)

	_, err = pool.Pop()
	require.Equal(t, err.Error(), "transaction pool is empty")

	// push multiple to empty (1,2,3)
	for i := uint64(1); i < 4; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l := pool.Len()
		assert.Equal(t, uint16(i), l)
	}

	// consume some (remove 1,2, keep 3)
	for i := uint64(1); i < 3; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())

		l := pool.Len()
		assert.Equal(t, uint16(3-i), l)
	}

	// push multiple to non empty (push 4,5. now have 3,4,5)
	for i := uint64(4); i < 6; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))

		l := pool.Len()
		assert.Equal(t, uint16(i-2), l)
	}

	// push more than max
	assert.ErrorIs(t, pool.Push(&mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(123),
		},
	}), mempool.ErrTxnPoolFull)

	// consume all (remove 3,4,5)
	for i := uint64(3); i < 6; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		assert.Equal(t, i, txn.Transaction.Hash().Uint64())
	}
	assert.Equal(t, uint16(0), l)

	_, err = pool.Pop()
	require.Equal(t, err.Error(), "transaction pool is empty")

}

func TestRestoreMempool(t *testing.T) {
	blockchain.RegisterCoreTypesToEncoder()

	testDB, _, err := setupDatabase(true)
	require.NoError(t, err)
	pool, closer, err := mempool.New(*testDB, 1024)
	require.NoError(t, err)

	// Check both pools are empty
	lenDB, err := pool.LenDB()
	require.NoError(t, err)
	assert.Equal(t, uint16(0), lenDB)
	assert.Equal(t, uint16(0), pool.Len())

	// push multiple transactions to empty mempool (1,2,3)
	for i := uint64(1); i < 4; i++ {
		assert.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
			},
		}))
		assert.Equal(t, uint16(i), pool.Len())
	}

	// check the db has stored the transactions
	time.Sleep(100 * time.Millisecond)
	lenDB, err = pool.LenDB()
	require.NoError(t, err)
	assert.Equal(t, uint16(3), lenDB)

	// Close the mempool
	require.NoError(t, closer())
	testDB, dbCloser, err := setupDatabase(false)
	require.NoError(t, err)
	defer dbCloser()

	poolRestored, closer2, err := mempool.New(*testDB, 1024)
	require.NoError(t, err)
	lenDB, err = poolRestored.LenDB()
	require.NoError(t, err)
	assert.Equal(t, uint16(3), lenDB)
	assert.Equal(t, uint16(3), poolRestored.Len())

	// Remove transactions
	_, err = poolRestored.Pop()
	require.NoError(t, err)
	_, err = poolRestored.Pop()
	require.NoError(t, err)
	lenDB, err = poolRestored.LenDB()
	assert.Equal(t, uint16(3), lenDB)
	assert.Equal(t, uint16(1), poolRestored.Len())

	closer2()
}

func TestWait(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	pool, _, err := mempool.New(testDB, 1024)
	require.NoError(t, err)
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
