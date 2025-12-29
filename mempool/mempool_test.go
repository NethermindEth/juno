package mempool_test

import (
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupDatabase(dbPath string, dltExisting bool) (db.KeyValueStore, func(), error) {
	if _, err := os.Stat(dbPath); err == nil {
		if dltExisting {
			if err := os.RemoveAll(dbPath); err != nil {
				return nil, nil, err
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}
	persistentPool, err := pebble.New(dbPath)
	if err != nil {
		return nil, nil, err
	}
	closer := func() {
		os.RemoveAll(dbPath)
	}
	return persistentPool, closer, nil
}

func TestMempool(t *testing.T) {
	testDB, dbCloser, err := setupDatabase("testmempool", true)
	log := utils.NewNopZapLogger()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	chain := mocks.NewMockReader(mockCtrl)
	state := mocks.NewMockStateHistoryReader(mockCtrl)

	require.NoError(t, err)
	defer dbCloser()
	pool := mempool.New(testDB, chain, 4, log)
	require.NoError(t, pool.LoadFromDB())

	require.Equal(t, 0, pool.Len())

	_, err = pool.Pop()
	require.Equal(t, err.Error(), "transaction pool is empty")

	// push multiple to empty (1,2,3)
	for i := uint64(1); i < 4; i++ {
		senderAddress := new(felt.Felt).SetUint64(i)
		chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
		state.EXPECT().ContractNonce(senderAddress).Return(felt.Zero, nil)
		require.NoError(t, pool.Push(t.Context(), &mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
				Nonce:           new(felt.Felt).SetUint64(1),
				SenderAddress:   senderAddress,
				Version:         new(core.TransactionVersion).SetUint64(1),
			},
		}))
		require.Equal(t, int(i), pool.Len())
	}
	// consume some (remove 1,2, keep 3)
	for i := uint64(1); i < 3; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		require.Equal(t, i, txn.Transaction.Hash().Uint64())
		require.Equal(t, int(3-i), pool.Len())
	}

	// push multiple to non empty (push 4,5. now have 3,4,5)
	for i := uint64(4); i < 6; i++ {
		senderAddress := new(felt.Felt).SetUint64(i)
		chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
		state.EXPECT().ContractNonce(senderAddress).Return(felt.Zero, nil)
		require.NoError(t, pool.Push(t.Context(), &mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
				Nonce:           new(felt.Felt).SetUint64(1),
				SenderAddress:   senderAddress,
				Version:         new(core.TransactionVersion).SetUint64(1),
			},
		}))
		require.Equal(t, int(i-2), pool.Len())
	}

	// push more than max
	require.ErrorIs(t, pool.Push(t.Context(), &mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(123),
		},
	}), mempool.ErrTxnPoolFull)

	// consume all (remove 3,4,5)
	for i := uint64(3); i < 6; i++ {
		txn, err := pool.Pop()
		require.NoError(t, err)
		require.Equal(t, i, txn.Transaction.Hash().Uint64())
	}
	require.Equal(t, 0, pool.Len())

	_, err = pool.Pop()
	require.Equal(t, err.Error(), "transaction pool is empty")
	pool.Close()
}

func TestRestoreMempool(t *testing.T) {
	log := utils.NewNopZapLogger()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	state := mocks.NewMockStateHistoryReader(mockCtrl)
	chain := mocks.NewMockReader(mockCtrl)
	testDB, dbDeleter, err := setupDatabase("testrestoremempool", true)
	require.NoError(t, err)
	defer dbDeleter()
	pool := mempool.New(testDB, chain, 1024, log)
	require.NoError(t, pool.LoadFromDB())
	// Check both pools are empty
	lenDB, err := pool.LenDB()
	require.NoError(t, err)
	require.Equal(t, 0, lenDB)
	require.Equal(t, 0, pool.Len())

	// push multiple transactions to empty mempool (1,2,3)
	expectedTxs := [3]mempool.BroadcastedTransaction{}
	for i := uint64(1); i < 4; i++ {
		senderAddress := new(felt.Felt).SetUint64(i)
		chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
		state.EXPECT().ContractNonce(senderAddress).Return(felt.Zero, nil)
		tx := mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
				Version:         new(core.TransactionVersion).SetUint64(1),
				SenderAddress:   senderAddress,
				Nonce:           new(felt.Felt).SetUint64(0),
			},
		}
		require.NoError(t, pool.Push(t.Context(), &tx))
		expectedTxs[i-1] = tx
		require.Equal(t, int(i), pool.Len())
	}
	// check the db has stored the transactions
	time.Sleep(100 * time.Millisecond)
	lenDB, err = pool.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	// Close the mempool
	pool.Close()
	require.NoError(t, testDB.Close())
	testDB, _, err = setupDatabase("testrestoremempool", false)
	require.NoError(t, err)

	poolRestored := mempool.New(testDB, chain, 1024, log)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, poolRestored.LoadFromDB())
	lenDB, err = poolRestored.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	lenRestored := poolRestored.Len()
	require.Equal(t, len(expectedTxs), lenRestored)

	// Pop transactions
	restoredTransactions, err := poolRestored.PopBatch(lenRestored)
	require.NoError(t, err)
	for i, txn := range restoredTransactions {
		require.Equal(t, expectedTxs[i].Transaction.Hash(), txn.Transaction.Hash())
	}
	lenDB, err = poolRestored.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	require.Equal(t, 0, poolRestored.Len())
	poolRestored.Close()
}

func TestWait(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	testDB, dbCloser, err := setupDatabase("testwait", true)
	require.NoError(t, err)
	defer dbCloser()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	bc := blockchain.New(testDB, &utils.Sepolia, statetestutils.UseNewState())
	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	var address felt.Felt
	for k := range stateUpdate0.StateDiff.Nonces {
		address = k
	}
	pool := mempool.New(testDB, bc, 1024, log)
	require.NoError(t, pool.LoadFromDB())

	select {
	case <-pool.Wait():
		require.Fail(t, "wait channel should not be signalled on empty mempool")
	default:
	}

	// One transaction.
	require.NoError(t, bc.Store(block0, &core.BlockCommitments{}, stateUpdate0, nil))
	require.NoError(t, pool.Push(t.Context(), &mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(1),
			Nonce:           new(felt.Felt).SetUint64(5),
			SenderAddress:   &address,
			Version:         new(core.TransactionVersion).SetUint64(1),
		},
	}))
	<-pool.Wait()
	pool.Close()
}

func TestPopBatch(t *testing.T) {
	testDB, dbCloser, err := setupDatabase("testpopbatch", true)
	log := utils.NewNopZapLogger()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	chain := mocks.NewMockReader(mockCtrl)
	state := mocks.NewMockStateHistoryReader(mockCtrl)

	require.NoError(t, err)
	defer dbCloser()
	pool := mempool.New(testDB, chain, 10, log)
	require.NoError(t, pool.LoadFromDB())

	require.Equal(t, 0, pool.Len())

	// Test PopBatch on empty pool
	txns, err := pool.PopBatch(3)
	require.ErrorIs(t, err, mempool.ErrTxnPoolEmpty)
	require.Nil(t, txns)

	// Test with zero count
	txns, err = pool.PopBatch(0)
	require.NoError(t, err)
	require.Empty(t, txns)

	// Test with negative count
	txns, err = pool.PopBatch(-5)
	require.NoError(t, err)
	require.Empty(t, txns)

	// Helper function to add transactions to the pool
	addTransactions := func(start, end uint64) {
		for i := start; i <= end; i++ {
			senderAddress := new(felt.Felt).SetUint64(i)
			chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
			state.EXPECT().ContractNonce(senderAddress).Return(felt.Zero, nil)
			require.NoError(t, pool.Push(t.Context(), &mempool.BroadcastedTransaction{
				Transaction: &core.InvokeTransaction{
					TransactionHash: new(felt.Felt).SetUint64(i),
					Nonce:           new(felt.Felt).SetUint64(1),
					SenderAddress:   senderAddress,
					Version:         new(core.TransactionVersion).SetUint64(1),
				},
			}))
		}
	}

	// Push 5 transactions to the pool
	addTransactions(1, 5)
	require.Equal(t, 5, pool.Len())

	// Test PopBatch with count less than pool size
	txns, err = pool.PopBatch(3)
	require.NoError(t, err)
	require.Len(t, txns, 3)
	for i, txn := range txns {
		require.Equal(t, uint64(i+1), txn.Transaction.Hash().Uint64())
	}
	require.Equal(t, 2, pool.Len())

	// Test PopBatch with count equal to pool size
	txns, err = pool.PopBatch(2)
	require.NoError(t, err)
	require.Len(t, txns, 2)
	for i, txn := range txns {
		require.Equal(t, uint64(i+4), txn.Transaction.Hash().Uint64())
	}
	require.Equal(t, 0, pool.Len())

	// Test PopBatch with count greater than pool size
	// First, add 2 more transactions
	addTransactions(6, 7)
	require.Equal(t, 2, pool.Len())

	// Try to pop more than available
	txns, err = pool.PopBatch(5)
	require.NoError(t, err)
	require.Len(t, txns, 2)
	for i, txn := range txns {
		require.Equal(t, uint64(i+6), txn.Transaction.Hash().Uint64())
	}
	require.Equal(t, 0, pool.Len())

	pool.Close()
}
