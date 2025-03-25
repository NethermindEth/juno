package mempool_test

import (
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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

func setupDatabase(dbPath string, dltExisting bool) (db.DB, func(), error) {
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
	pool, closer := mempool.New(testDB, chain, 4, log)
	require.NoError(t, pool.LoadFromDB())

	require.Equal(t, 0, pool.Len())

	_, err = pool.Pop()
	require.Equal(t, err.Error(), "transaction pool is empty")

	// push multiple to empty (1,2,3)
	for i := uint64(1); i < 4; i++ {
		senderAddress := new(felt.Felt).SetUint64(i)
		chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
		state.EXPECT().ContractNonce(senderAddress).Return(&felt.Zero, nil)
		require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
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
		state.EXPECT().ContractNonce(senderAddress).Return(&felt.Zero, nil)
		require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
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
	require.ErrorIs(t, pool.Push(&mempool.BroadcastedTransaction{
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
	require.NoError(t, closer())
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
	pool, mempoolCloser := mempool.New(testDB, chain, 1024, log)
	require.NoError(t, pool.LoadFromDB())
	// Check both pools are empty
	lenDB, err := pool.LenDB()
	require.NoError(t, err)
	require.Equal(t, 0, lenDB)
	require.Equal(t, 0, pool.Len())

	// push multiple transactions to empty mempool (1,2,3)
	for i := uint64(1); i < 4; i++ {
		senderAddress := new(felt.Felt).SetUint64(i)
		chain.EXPECT().HeadState().Return(state, func() error { return nil }, nil)
		state.EXPECT().ContractNonce(senderAddress).Return(new(felt.Felt).SetUint64(0), nil)
		require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
			Transaction: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(i),
				Version:         new(core.TransactionVersion).SetUint64(1),
				SenderAddress:   senderAddress,
				Nonce:           new(felt.Felt).SetUint64(0),
			},
		}))
		require.Equal(t, int(i), pool.Len())
	}
	// check the db has stored the transactions
	time.Sleep(100 * time.Millisecond)
	lenDB, err = pool.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	// Close the mempool
	require.NoError(t, mempoolCloser())
	require.NoError(t, testDB.Close())
	testDB, _, err = setupDatabase("testrestoremempool", false)
	require.NoError(t, err)

	poolRestored, mempoolCloser2 := mempool.New(testDB, chain, 1024, log)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, poolRestored.LoadFromDB())
	lenDB, err = poolRestored.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	require.Equal(t, 3, poolRestored.Len())

	// Remove transactions
	_, err = poolRestored.Pop()
	require.NoError(t, err)
	_, err = poolRestored.Pop()
	require.NoError(t, err)
	lenDB, err = poolRestored.LenDB()
	require.NoError(t, err)
	require.Equal(t, 3, lenDB)
	require.Equal(t, 1, poolRestored.Len())
	require.NoError(t, mempoolCloser2())
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
	bc := blockchain.New(testDB, &utils.Sepolia)
	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	var address felt.Felt
	for k := range stateUpdate0.StateDiff.Nonces {
		address = k
	}
	pool, mempoolCloser := mempool.New(testDB, bc, 1024, log)
	require.NoError(t, pool.LoadFromDB())

	select {
	case <-pool.Wait():
		require.Fail(t, "wait channel should not be signalled on empty mempool")
	default:
	}

	// One transaction.
	require.NoError(t, bc.Store(block0, &core.BlockCommitments{}, stateUpdate0, nil))
	require.NoError(t, pool.Push(&mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(1),
			Nonce:           new(felt.Felt).SetUint64(5),
			SenderAddress:   &address,
			Version:         new(core.TransactionVersion).SetUint64(1),
		},
	}))
	<-pool.Wait()

	require.NoError(t, mempoolCloser())
}
