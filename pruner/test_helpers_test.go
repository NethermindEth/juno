package pruner

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB returns an isolated pebble-backed KeyValueStore for the test.
// We use pebble (not the in-memory db) because pebble's batch DeleteRange
// performs a real range delete, while the memory implementation only
// scans keys with the start byte sequence as a literal prefix.
func newTestDB(t *testing.T) db.KeyValueStore {
	t.Helper()
	database, err := pebblev2.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close())
	})
	return database
}

// storedBlock holds all data written for a block, so assertions can verify
// every pruned bucket.
type storedBlock struct {
	Header   *core.Header
	TxHashes []*felt.Felt
	// L1HandlerMsgHashes are the message hashes for any L1 handler txs in this block.
	L1HandlerMsgHashes [][]byte
	// StateUpdate is the state update written for this block.
	StateUpdate *core.StateUpdate
}

// Two fixed addresses that appear across multiple blocks to simulate
// realistic state history where the same contract is touched repeatedly.
var (
	sharedAddr1 = new(felt.Felt).SetUint64(0xA001)
	sharedAddr2 = new(felt.Felt).SetUint64(0xA002)
	sharedSlot  = new(felt.Felt).SetUint64(0x5001)
)

// storeBlock writes a complete block into the database covering all pruned buckets:
//   - Bucket 7:  BlockHeaderNumbersByHash
//   - Bucket 8:  BlockHeadersByNumber
//   - Bucket 9:  TransactionBlockNumbersAndIndicesByHash
//   - Bucket 12: StateUpdatesByBlockNumber
//   - Bucket 21: BlockCommitments
//   - Bucket 24: L1HandlerTxnHashByMsgHash (one L1 handler tx per block)
//   - Bucket 40: BlockTransactions
//   - Bucket 14: ContractStorageHistory
//   - Bucket 15: ContractNonceHistory
//   - Bucket 16: ContractClassHashHistory
func storeBlock(t *testing.T, database db.KeyValueStore, blockNum uint64) *storedBlock {
	t.Helper()

	header := &core.Header{
		Number:           blockNum,
		Hash:             felt.NewRandom[felt.Felt](),
		ParentHash:       felt.NewRandom[felt.Felt](),
		GlobalStateRoot:  felt.NewRandom[felt.Felt](),
		SequencerAddress: felt.NewRandom[felt.Felt](),
		TransactionCount: 3,
	}

	// Create transactions: 2 invoke + 1 L1 handler.
	invokeTx1 := &core.InvokeTransaction{
		TransactionHash: felt.NewRandom[felt.Felt](),
		Version:         new(core.TransactionVersion).SetUint64(1),
	}
	invokeTx2 := &core.InvokeTransaction{
		TransactionHash: felt.NewRandom[felt.Felt](),
		Version:         new(core.TransactionVersion).SetUint64(1),
	}
	l1HandlerTx := &core.L1HandlerTransaction{
		TransactionHash:    felt.NewRandom[felt.Felt](),
		ContractAddress:    felt.NewRandom[felt.Felt](),
		EntryPointSelector: felt.NewRandom[felt.Felt](),
		Nonce:              new(felt.Felt).SetUint64(blockNum),
		CallData:           []*felt.Felt{felt.NewRandom[felt.Felt](), felt.NewRandom[felt.Felt]()},
		Version:            new(core.TransactionVersion).SetUint64(0),
	}

	txs := []core.Transaction{invokeTx1, invokeTx2, l1HandlerTx}
	txHashes := []*felt.Felt{invokeTx1.Hash(), invokeTx2.Hash(), l1HandlerTx.Hash()}

	receipts := make([]*core.TransactionReceipt, 3)
	for i, tx := range txs {
		receipts[i] = &core.TransactionReceipt{
			TransactionHash: tx.Hash(),
			Fee:             new(felt.Felt).SetUint64(100),
		}
	}

	commitments := &core.BlockCommitments{
		TransactionCommitment: felt.NewRandom[felt.Felt](),
		EventCommitment:       felt.NewRandom[felt.Felt](),
		ReceiptCommitment:     felt.NewRandom[felt.Felt](),
		StateDiffCommitment:   felt.NewRandom[felt.Felt](),
	}

	// Build state diff with overlapping addresses across blocks.
	// sharedAddr1: storage + nonce change in every block
	// sharedAddr2: class hash change in every block
	// Plus a unique address per block for variety.
	uniqueAddr := new(felt.Felt).SetUint64(0xB000 + blockNum)
	uniqueSlot := new(felt.Felt).SetUint64(0xC000 + blockNum)
	oldValue := new(felt.Felt).SetUint64(blockNum * 100)

	storageDiffs := map[felt.Felt]map[felt.Felt]*felt.Felt{
		*sharedAddr1: {
			*sharedSlot: oldValue,
		},
		*uniqueAddr: {
			*uniqueSlot: oldValue,
		},
	}

	nonces := map[felt.Felt]*felt.Felt{
		*sharedAddr1: oldValue,
	}

	replacedClasses := map[felt.Felt]*felt.Felt{
		*sharedAddr2: oldValue,
	}

	stateUpdate := &core.StateUpdate{
		BlockHash: header.Hash,
		NewRoot:   felt.NewRandom[felt.Felt](),
		OldRoot:   felt.NewRandom[felt.Felt](),
		StateDiff: &core.StateDiff{
			StorageDiffs:      storageDiffs,
			Nonces:            nonces,
			ReplacedClasses:   replacedClasses,
			DeployedContracts: make(map[felt.Felt]*felt.Felt),
		},
	}

	// Write block data.
	require.NoError(t, core.WriteBlockHeaderByNumber(database, header))
	require.NoError(t, core.WriteBlockHeaderNumberByHash(database, header.Hash, blockNum))
	require.NoError(t, core.WriteBlockCommitment(database, blockNum, commitments))
	require.NoError(t, core.WriteStateUpdateByBlockNum(database, blockNum, stateUpdate))
	require.NoError(t, core.WriteTransactionsAndReceipts(database, blockNum, txs, receipts))

	// Write L1 handler msg hash → tx hash (bucket 24).
	msgHash := l1HandlerTx.MessageHash()
	require.NoError(
		t,
		core.WriteL1HandlerTxnHashByMsgHash(database, msgHash, l1HandlerTx.TransactionHash),
	)

	// Write state history entries (buckets 14, 15, 16).
	for addr, slots := range storageDiffs {
		for slot := range slots {
			require.NoError(
				t,
				core.WriteContractStorageHistory(database, &addr, &slot, oldValue, blockNum),
			)
		}
	}
	for addr := range nonces {
		require.NoError(t, core.WriteContractNonceHistory(database, &addr, oldValue, blockNum))
	}
	for addr := range replacedClasses {
		require.NoError(t, core.WriteContractClassHashHistory(database, &addr, oldValue, blockNum))
	}

	return &storedBlock{
		Header:             header,
		TxHashes:           txHashes,
		L1HandlerMsgHashes: [][]byte{msgHash},
		StateUpdate:        stateUpdate,
	}
}

func withBatch(t *testing.T, database db.KeyValueStore, fn func(db.Batch) error) {
	t.Helper()
	batch := database.NewBatch()
	require.NoError(t, fn(batch))
	require.NoError(t, batch.Write())
}

// assertBlockExists verifies all data for a block is present.
//
//nolint:dupl // symmetric with assertBlockPruned.
func assertBlockExists(t *testing.T, database db.KeyValueReader, block *storedBlock) {
	t.Helper()
	blockNum := block.Header.Number

	assert.True(t, blockHeaderExists(database, blockNum))
	assert.True(t, blockHeaderHashExists(database, block.Header.Hash))
	assert.True(t, blockCommitmentsExist(database, blockNum))
	assert.True(t, stateUpdateExists(database, blockNum))
	assert.True(t, transactionsExist(database, blockNum))

	for i, txHash := range block.TxHashes {
		_, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(
			database, (*felt.TransactionHash)(txHash))
		assert.NoError(t, err, "block %d tx %d hash lookup should exist", blockNum, i)
	}

	for _, msgHash := range block.L1HandlerMsgHashes {
		_, err := core.GetL1HandlerTxnHashByMsgHash(database, msgHash)
		assert.NoError(t, err, "block %d L1 handler msg hash should exist", blockNum)
	}

	assertStateHistoryExists(t, database, blockNum, block.StateUpdate)
}

// assertBlockPruned verifies all data for a block is gone.
//
//nolint:dupl // symmetric with assertBlockExists.
func assertBlockPruned(t *testing.T, database db.KeyValueReader, block *storedBlock) {
	t.Helper()
	blockNum := block.Header.Number

	// Bucket 8: header by number
	assert.False(t, blockHeaderExists(database, blockNum))
	// Bucket 7: hash → number reverse lookup
	assert.False(t, blockHeaderHashExists(database, block.Header.Hash))
	// Bucket 21: commitments
	assert.False(t, blockCommitmentsExist(database, blockNum))
	// Bucket 12: state update
	assert.False(t, stateUpdateExists(database, blockNum))
	// Bucket 40: transactions
	assert.False(t, transactionsExist(database, blockNum))

	// Bucket 9: tx hash reverse lookups
	for i, txHash := range block.TxHashes {
		_, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(
			database,
			(*felt.TransactionHash)(txHash),
		)
		assert.Error(t, err, "block %d tx %d hash lookup should be deleted", blockNum, i)
	}

	// Bucket 24: L1 handler msg hash → tx hash
	for _, msgHash := range block.L1HandlerMsgHashes {
		_, err := core.GetL1HandlerTxnHashByMsgHash(database, msgHash)
		assert.Error(t, err, "block %d L1 handler msg hash should be deleted", blockNum)
	}

	// Buckets 14, 15, 16: state history
	assertStateHistoryPruned(t, database, blockNum, block.StateUpdate)
}

//nolint:dupl // symmetric with assertStateHistoryPruned.
func assertStateHistoryExists(
	t *testing.T,
	r db.KeyValueReader,
	blockNum uint64,
	su *core.StateUpdate,
) {
	t.Helper()
	for addr, slots := range su.StateDiff.StorageDiffs {
		for slot := range slots {
			key := db.ContractStorageHistoryAtBlockKey(&addr, &slot, blockNum)
			assert.NoError(t, r.Get(key, func([]byte) error { return nil }))
		}
	}
	for addr := range su.StateDiff.Nonces {
		key := db.ContractNonceHistoryAtBlockKey(&addr, blockNum)
		assert.NoError(t, r.Get(key, func([]byte) error { return nil }))
	}
	for addr := range su.StateDiff.ReplacedClasses {
		key := db.ContractClassHashHistoryAtBlockKey(&addr, blockNum)
		assert.NoError(t, r.Get(key, func([]byte) error { return nil }))
	}
}

//nolint:dupl // symmetric with assertStateHistoryExists; see note there.
func assertStateHistoryPruned(
	t *testing.T,
	r db.KeyValueReader,
	blockNum uint64,
	su *core.StateUpdate,
) {
	t.Helper()
	for addr, slots := range su.StateDiff.StorageDiffs {
		for slot := range slots {
			key := db.ContractStorageHistoryAtBlockKey(&addr, &slot, blockNum)
			assert.Error(t, r.Get(key, func([]byte) error { return nil }))
		}
	}
	for addr := range su.StateDiff.Nonces {
		key := db.ContractNonceHistoryAtBlockKey(&addr, blockNum)
		assert.Error(t, r.Get(key, func([]byte) error { return nil }))
	}
	for addr := range su.StateDiff.ReplacedClasses {
		key := db.ContractClassHashHistoryAtBlockKey(&addr, blockNum)
		assert.Error(t, r.Get(key, func([]byte) error { return nil }))
	}
}

// Helpers for individual bucket checks.
func blockHeaderExists(database db.KeyValueReader, blockNum uint64) bool {
	_, err := core.GetBlockHeaderByNumber(database, blockNum)
	return err == nil
}

func blockHeaderHashExists(database db.KeyValueReader, hash *felt.Felt) bool {
	_, err := core.BlockHeaderNumbersByHashBucket.Get(database, hash)
	return err == nil
}

func blockCommitmentsExist(database db.KeyValueReader, blockNum uint64) bool {
	_, err := core.GetBlockCommitmentByBlockNum(database, blockNum)
	return err == nil
}

func stateUpdateExists(database db.KeyValueReader, blockNum uint64) bool {
	_, err := core.GetStateUpdateByBlockNum(database, blockNum)
	return err == nil
}

func transactionsExist(database db.KeyValueReader, blockNum uint64) bool {
	_, err := core.GetTransactionByBlockAndIndex(database, blockNum, 0)
	return err == nil
}
