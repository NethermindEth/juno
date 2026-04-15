package core_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionsAndReceipts(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	// write txs and receipts (testing `WriteTransactionsAndReceipts` function)
	require.NoError(
		t,
		core.WriteTransactionsAndReceipts(memDB, block.Number, block.Transactions, block.Receipts),
	)

	clearEmptyProofFacts(block.Transactions)

	// read txs (testing `GetTransactionsByBlockNumber` function)
	txs, err := core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block.Transactions, txs)

	// read txs from iterator (testing `GetTransactionsByBlockNumberIter` function)
	iterTxs := make([]core.Transaction, 0)
	iter := core.GetTransactionsByBlockNumberIter(memDB, block.Number)
	for tx, err := range iter {
		require.NoError(t, err)
		iterTxs = append(iterTxs, tx)
	}
	assert.Equal(t, txs, iterTxs)

	// read txs by block and index (testing `GetTransactionByBlockAndIndex` function)
	indexedTxs := make([]core.Transaction, len(txs))
	// we range through len(txs) + 1 to test we are not getting more txs than expected
	for i := range len(txs) + 1 {
		tx, err := core.GetTransactionByBlockAndIndex(memDB, block.Number, uint64(i))
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) && i == len(txs) {
				continue
			}
			require.NoError(t, err)
		}
		indexedTxs[i] = tx
	}
	assert.Equal(t, txs, indexedTxs)

	// read tx by hash (testing `GetTransactionByHash` function)
	for _, tx := range txs {
		tx, err := core.GetTransactionByHash(memDB, (*felt.TransactionHash)(tx.Hash()))
		require.NoError(t, err)
		assert.Equal(t, tx, tx)
	}

	// read receipts by block number (testing `GetReceiptsByBlockNumber` function)
	receipts, err := core.GetReceiptsByBlockNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block.Receipts, receipts)

	// read receipts by block and index (testing `GetReceiptByBlockAndIndex` function)
	indexedReceipts := make([]*core.TransactionReceipt, len(receipts))
	// we range through len(receipts) + 1 to test we are not getting more receipts than expected
	for i := range len(receipts) + 1 {
		receipt, err := core.GetReceiptByBlockAndIndex(memDB, block.Number, uint64(i))
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) && i == len(receipts) {
				continue
			}
			require.NoError(t, err)
		}
		indexedReceipts[i] = receipt
	}
	assert.Equal(t, receipts, indexedReceipts)

	// read block by number (testing `GetBlockByNumber` function)
	require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))
	blockFromDB, err := core.GetBlockByNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block, blockFromDB)

	// delete txs and receipts (testing `DeleteTransactionsAndReceipts` function)
	batch := memDB.NewBatch()
	require.NoError(t, core.DeleteTransactionsAndReceipts(memDB, batch, block.Number))
	require.NoError(t, batch.Write())

	txs, err = core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	assert.Empty(t, txs)
	receipts, err = core.GetReceiptsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	assert.Empty(t, receipts)
}

// clearEmptyProofFacts fills empty proof facts of the transactions with nil proof facts.
// This is necessary because feeder returns empty proof facts ([]felt.Felt{}),
// but when storing the txs in the db, due to the `cbor:",omitempty"` tag, the proof facts
// are omitted, making the `assert.ElementsMatch` or any other comparison fail ([] vs nil).
func clearEmptyProofFacts(txs []core.Transaction) {
	for i := range txs {
		switch tx := txs[i].(type) {
		case *core.InvokeTransaction:
			if len(tx.ProofFacts) == 0 {
				tx.ProofFacts = nil
			}
		default:
		}
	}
}
