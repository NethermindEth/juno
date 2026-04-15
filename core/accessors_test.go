package core_test

import (
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

func TestWriteTransactionsAndReceipts(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetBlockByNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	err = core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	)
	require.NoError(t, err)

	clearEmptyProofFacts(block.Transactions)

	// required for GetBlockByNumber
	require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))

	blockFromDB, err := core.GetBlockByNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block, blockFromDB)
}

func TestGetTransactionsByBlockNumber(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)

	txs, err := core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block.Transactions, txs)
}

func TestGetTransactionsByBlockNumberIter(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	for _, err := range core.GetTransactionsByBlockNumberIter(memDB, block.Number) {
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	}

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)

	iterTxs := make([]core.Transaction, 0)
	iter := core.GetTransactionsByBlockNumberIter(memDB, block.Number)
	for tx, err := range iter {
		require.NoError(t, err)
		iterTxs = append(iterTxs, tx)
	}
	assert.Equal(t, block.Transactions, iterTxs)
}

func TestGetTransactionByBlockAndIndex(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetTransactionByBlockAndIndex(memDB, block.Number, 0)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)

	for i, expectedTx := range block.Transactions {
		tx, err := core.GetTransactionByBlockAndIndex(memDB, block.Number, uint64(i))
		require.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
	}

	// one past the last index should return ErrKeyNotFound
	_, err = core.GetTransactionByBlockAndIndex(memDB, block.Number, uint64(len(block.Transactions)))
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}

func TestGetTransactionByHash(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetTransactionByHash(memDB, (*felt.TransactionHash)(block.Transactions[0].Hash()))
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)

	for _, expectedTx := range block.Transactions {
		tx, err := core.GetTransactionByHash(memDB, (*felt.TransactionHash)(expectedTx.Hash()))
		require.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
	}
}

func TestGetReceiptsByBlockNumber(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetReceiptsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))

	receipts, err := core.GetReceiptsByBlockNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block.Receipts, receipts)
}

func TestGetReceiptByBlockAndIndex(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetReceiptByBlockAndIndex(memDB, block.Number, 0)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))

	for i, expectedReceipt := range block.Receipts {
		receipt, err := core.GetReceiptByBlockAndIndex(memDB, block.Number, uint64(i))
		require.NoError(t, err)
		assert.Equal(t, expectedReceipt, receipt)
	}

	// one past the last index should return ErrKeyNotFound
	_, err = core.GetReceiptByBlockAndIndex(memDB, block.Number, uint64(len(block.Receipts)))
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}

func TestGetBlockByNumber(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetBlockByNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)
	require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))

	blockFromDB, err := core.GetBlockByNumber(memDB, block.Number)
	require.NoError(t, err)
	assert.Equal(t, block, blockFromDB)
}

func TestDeleteTransactionsAndReceipts(t *testing.T) {
	memDB := memory.New()
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	_, err = core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	_, err = core.GetReceiptsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))

	batch := memDB.NewBatch()
	require.NoError(t, core.DeleteTransactionsAndReceipts(memDB, batch, block.Number))
	require.NoError(t, batch.Write())

	txs, err := core.GetTransactionsByBlockNumber(memDB, block.Number)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	assert.Empty(t, txs)

	receipts, err := core.GetReceiptsByBlockNumber(memDB, block.Number)
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
