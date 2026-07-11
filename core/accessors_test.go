package core_test

import (
	"math"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const nonexistentBlockNumber = math.MaxUint64

func setupForTxsAndReceiptsTests(t *testing.T) (db.KeyValueStore, *core.Block) {
	t.Helper()
	memDB := memory.New()
	client := feeder.NewTestClient(t, &networks.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

	require.NoError(t, core.WriteTransactionsAndReceipts(
		memDB,
		block.Number,
		block.Transactions,
		block.Receipts,
	))
	clearEmptyProofFacts(block.Transactions)

	return memDB, block
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

func TestWriteTransactionsAndReceipts(t *testing.T) {
	t.Parallel()
	memDB := memory.New()
	client := feeder.NewTestClient(t, &networks.Sepolia)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)

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

//nolint:dupl // Similar to TestGetReceiptsByBlockNumber, but they're different methods
func TestGetTransactionsByBlockNumber(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		txs, err := core.GetTransactionsByBlockNumber(memDB, block.Number)
		require.NoError(t, err)
		assert.Equal(t, block.Transactions, txs)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetTransactionsByBlockNumber(memDB, nonexistentBlockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestGetTransactionsByBlockNumberIter(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		iterTxs := make([]core.Transaction, 0)
		for tx, err := range core.GetTransactionsByBlockNumberIter(memDB, block.Number) {
			require.NoError(t, err)
			iterTxs = append(iterTxs, tx)
		}
		assert.Equal(t, block.Transactions, iterTxs)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		for _, err := range core.GetTransactionsByBlockNumberIter(memDB, nonexistentBlockNumber) {
			require.ErrorIs(t, err, db.ErrKeyNotFound)
		}
	})
}

//nolint:dupl // Similar to TestGetReceiptByBlockAndIndex, but they're different methods
func TestGetTransactionByBlockAndIndex(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		for i, expectedTx := range block.Transactions {
			tx, err := core.GetTransactionByBlockAndIndex(memDB, block.Number, uint64(i))
			require.NoError(t, err)
			assert.Equal(t, expectedTx, tx)
		}

		// one past the last index should return ErrKeyNotFound
		_, err := core.GetTransactionByBlockAndIndex(memDB, block.Number, uint64(len(block.Transactions)))
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetTransactionByBlockAndIndex(memDB, nonexistentBlockNumber, 0)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestGetTransactionByHash(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid transaction", func(t *testing.T) {
		t.Parallel()
		for _, expectedTx := range block.Transactions {
			tx, err := core.GetTransactionByHash(memDB, (*felt.TransactionHash)(expectedTx.Hash()))
			require.NoError(t, err)
			assert.Equal(t, expectedTx, tx)
		}
	})

	t.Run("non-existent transaction", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetTransactionByHash(memDB, new(felt.TransactionHash))
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

//nolint:dupl // Similar to TestGetTransactionsByBlockNumber, but they're different methods
func TestGetReceiptsByBlockNumber(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		receipts, err := core.GetReceiptsByBlockNumber(memDB, block.Number)
		require.NoError(t, err)
		assert.Equal(t, block.Receipts, receipts)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetReceiptsByBlockNumber(memDB, nonexistentBlockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

//nolint:dupl // Similar to TestGetTransactionByBlockAndIndex, but they're different methods
func TestGetReceiptByBlockAndIndex(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		for i, expectedReceipt := range block.Receipts {
			receipt, err := core.GetReceiptByBlockAndIndex(memDB, block.Number, uint64(i))
			require.NoError(t, err)
			assert.Equal(t, expectedReceipt, receipt)
		}

		// one past the last index should return ErrKeyNotFound
		_, err := core.GetReceiptByBlockAndIndex(memDB, block.Number, uint64(len(block.Receipts)))
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetReceiptByBlockAndIndex(memDB, nonexistentBlockNumber, 0)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestGetBlockByNumber(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))

		blockFromDB, err := core.GetBlockByNumber(memDB, block.Number)
		require.NoError(t, err)
		assert.Equal(t, block, blockFromDB)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		_, err := core.GetBlockByNumber(memDB, nonexistentBlockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestDeleteTransactionsAndReceipts(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)

	t.Run("valid block", func(t *testing.T) {
		t.Parallel()
		batch := memDB.NewBatch()
		require.NoError(t, core.DeleteTransactionsAndReceipts(memDB, batch, block.Number))
		require.NoError(t, batch.Write())

		txs, err := core.GetTransactionsByBlockNumber(memDB, block.Number)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Empty(t, txs)

		receipts, err := core.GetReceiptsByBlockNumber(memDB, block.Number)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Empty(t, receipts)
	})

	t.Run("non-existent block", func(t *testing.T) {
		t.Parallel()
		batch := memDB.NewBatch()
		err := core.DeleteTransactionsAndReceipts(memDB, batch, nonexistentBlockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestPartialBlockHeaderAccessorsByNumber(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)
	require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))

	tests := []struct {
		name               string
		readPartial        func(db.KeyValueReader, uint64) (*felt.Felt, error)
		getExpected        func(*core.Header) *felt.Felt
		headerWithoutField any
	}{
		{
			name:        "global state root",
			readPartial: core.GetGlobalStateRootByBlockNumber,
			getExpected: func(header *core.Header) *felt.Felt {
				return header.GlobalStateRoot
			},
			headerWithoutField: struct {
				Hash *felt.Felt
			}{Hash: block.Hash},
		},
		{
			name:        "block hash",
			readPartial: core.GetBlockHeaderHashByNumber,
			getExpected: func(header *core.Header) *felt.Felt {
				return header.Hash
			},
			headerWithoutField: struct {
				GlobalStateRoot *felt.Felt
			}{GlobalStateRoot: block.GlobalStateRoot},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("matches full header decode", func(t *testing.T) {
				t.Parallel()
				header, err := core.GetBlockHeaderByNumber(memDB, block.Number)
				require.NoError(t, err)
				got, err := tt.readPartial(memDB, block.Number)
				require.NoError(t, err)
				assert.Equal(t, tt.getExpected(header), got)
			})

			t.Run("missing block returns ErrKeyNotFound", func(t *testing.T) {
				t.Parallel()
				_, err := tt.readPartial(memDB, nonexistentBlockNumber)
				require.ErrorIs(t, err, db.ErrKeyNotFound)
			})

			t.Run("missing field returns error", func(t *testing.T) {
				t.Parallel()
				partialHeaderDB := memory.New()
				data, err := encoder.Marshal(tt.headerWithoutField)
				require.NoError(t, err)
				require.NoError(t, partialHeaderDB.Put(db.BlockHeaderByNumberKey(block.Number), data))

				_, err = tt.readPartial(partialHeaderDB, block.Number)
				require.Error(t, err)
			})
		})
	}
}

func TestGetStateRootAndHashByBlockNumber(t *testing.T) {
	t.Parallel()
	memDB, block := setupForTxsAndReceiptsTests(t)
	require.NoError(t, core.WriteBlockHeaderByNumber(memDB, block.Header))

	t.Run("matches full header decode", func(t *testing.T) {
		t.Parallel()
		header, err := core.GetBlockHeaderByNumber(memDB, block.Number)
		require.NoError(t, err)

		gotHash, gotStateRoot, err := core.GetStateRootAndHashByBlockNumber(memDB, block.Number)
		require.NoError(t, err)

		assert.Equal(t, header.Hash, gotHash)
		assert.Equal(t, header.GlobalStateRoot, gotStateRoot)
	})

	t.Run("missing block returns ErrKeyNotFound", func(t *testing.T) {
		t.Parallel()
		_, _, err := core.GetStateRootAndHashByBlockNumber(memDB, nonexistentBlockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("missing Hash returns error", func(t *testing.T) {
		t.Parallel()
		partialHeaderDB := memory.New()
		headerWithoutHash := struct {
			GlobalStateRoot *felt.Felt
		}{GlobalStateRoot: block.GlobalStateRoot}

		data, err := encoder.Marshal(headerWithoutHash)
		require.NoError(t, err)
		require.NoError(t, partialHeaderDB.Put(db.BlockHeaderByNumberKey(block.Number), data))

		_, _, err = core.GetStateRootAndHashByBlockNumber(partialHeaderDB, block.Number)
		require.ErrorContains(t, err, "missing Hash in block header")
	})

	t.Run("missing GlobalStateRoot returns error", func(t *testing.T) {
		t.Parallel()
		partialHeaderDB := memory.New()
		headerWithoutStateRoot := struct {
			Hash *felt.Felt
		}{Hash: block.Hash}

		data, err := encoder.Marshal(headerWithoutStateRoot)
		require.NoError(t, err)
		require.NoError(t, partialHeaderDB.Put(db.BlockHeaderByNumberKey(block.Number), data))

		_, _, err = core.GetStateRootAndHashByBlockNumber(partialHeaderDB, block.Number)
		require.ErrorContains(t, err, "missing GlobalStateRoot in block header")
	})
}
