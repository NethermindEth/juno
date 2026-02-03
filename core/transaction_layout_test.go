package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/require"
)

const (
	txCount     = 50
	chainHeight = 10
)

func layoutName(l core.TransactionLayout) string {
	switch l {
	case core.TransactionLayoutPerTx:
		return "PerTx"
	case core.TransactionLayoutCombined:
		return "Combined"
	default:
		return "Unknown"
	}
}

func assertByBlockIndex[T any](
	t *testing.T,
	database db.KeyValueReader,
	expectedErr bool,
	expected [][]T,
	get func(database db.KeyValueReader, blockNumber, index uint64) (T, error),
) {
	t.Helper()
	for blockNum := range chainHeight {
		for i, expected := range expected[blockNum] {
			actual, err := get(database, uint64(blockNum), uint64(i))
			if expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}
		}
	}
}

func assertByTransactionHash(
	t *testing.T,
	database db.KeyValueReader,
	expectedErr bool,
	expected [][]core.Transaction,
	layout core.TransactionLayout,
) {
	t.Helper()
	assertByBlockIndex(
		t,
		database,
		expectedErr,
		expected,
		func(database db.KeyValueReader, blockNumber, index uint64) (core.Transaction, error) {
			hash := (*felt.TransactionHash)(expected[blockNumber][index].Hash())
			return layout.TransactionByHash(database, hash)
		},
	)
}

func assertByBlockNumber[T any](
	t *testing.T,
	database db.KeyValueReader,
	expectedErr bool,
	expected [][]T,
	get func(database db.KeyValueReader, blockNumber uint64) ([]T, error),
) {
	t.Helper()
	for blockNum := range chainHeight {
		actual, err := get(database, uint64(blockNum))
		if expectedErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected[blockNum], actual)
		}
	}
}

func runTestTransactionLayout(
	t *testing.T,
	transactions [][]core.Transaction,
	receipts [][]*core.TransactionReceipt,
	headers []*core.Header,
	layout core.TransactionLayout,
) {
	t.Run(layoutName(layout), func(t *testing.T) {
		database, err := pebblev2.New(t.TempDir())
		require.NoError(t, err)
		defer database.Close()

		// WRITE
		t.Run("WriteTransactionsAndReceipts", func(t *testing.T) {
			for blockNum := range chainHeight {
				require.NoError(
					t,
					layout.WriteTransactionsAndReceipts(
						database,
						uint64(blockNum),
						transactions[blockNum],
						receipts[blockNum],
					),
				)
			}
		})

		// READ
		t.Run("TransactionByBlockAndIndex", func(t *testing.T) {
			assertByBlockIndex(t, database, false, transactions, layout.TransactionByBlockAndIndex)
		})

		t.Run("ReceiptByBlockAndIndex", func(t *testing.T) {
			assertByBlockIndex(t, database, false, receipts, layout.ReceiptByBlockAndIndex)
		})

		t.Run("TransactionsByBlockNumber", func(t *testing.T) {
			assertByBlockNumber(t, database, false, transactions, layout.TransactionsByBlockNumber)
		})

		t.Run("TransactionsByBlockNumberIter", func(t *testing.T) {
			for blockNum := range chainHeight {
				var actual []core.Transaction
				iter := layout.TransactionsByBlockNumberIter(database, uint64(blockNum))
				for tx, err := range iter {
					require.NoError(t, err)
					actual = append(actual, tx)
				}
				require.Equal(t, transactions[blockNum], actual)
			}
		})

		t.Run("ReceiptsByBlockNumber", func(t *testing.T) {
			assertByBlockNumber(t, database, false, receipts, layout.ReceiptsByBlockNumber)
		})

		t.Run("TransactionByHash", func(t *testing.T) {
			assertByTransactionHash(t, database, false, transactions, layout)
		})

		t.Run("BlockByNumber", func(t *testing.T) {
			// First, write all block headers
			for blockNum := range chainHeight {
				require.NoError(t, core.WriteBlockHeaderByNumber(database, headers[blockNum]))
			}

			// Now test BlockByNumber for all blocks
			for blockNum := range chainHeight {
				block, err := layout.BlockByNumber(database, uint64(blockNum))
				require.NoError(t, err)
				require.Equal(t, headers[blockNum], block.Header)
				require.Equal(t, transactions[blockNum], block.Transactions)
				require.Equal(t, receipts[blockNum], block.Receipts)
			}
		})

		// DELETE
		t.Run("DeleteTxsAndReceipts", func(t *testing.T) {
			// Delete all blocks
			for blockNum := range chainHeight {
				require.NoError(t, database.Update(func(batch db.IndexedBatch) error {
					return layout.DeleteTxsAndReceipts(batch, uint64(blockNum))
				}))
			}

			// Verify transactions are deleted
			assertByBlockIndex(t, database, true, transactions, layout.TransactionByBlockAndIndex)

			// Verify receipts are deleted
			assertByBlockIndex(t, database, true, receipts, layout.ReceiptByBlockAndIndex)

			// After all blocks deleted, verify all transaction hash lookups fail
			assertByTransactionHash(t, database, true, transactions, layout)
		})
	})
}

func TestTransactionLayout(t *testing.T) {
	// Generate fixtures for all blocks ONCE
	transactions := make([][]core.Transaction, chainHeight)
	receipts := make([][]*core.TransactionReceipt, chainHeight)
	headers := make([]*core.Header, chainHeight)
	for blockNum := range chainHeight {
		transactions[blockNum] = testutils.GetCoreTransactions(t, txCount)
		receipts[blockNum] = testutils.GetCoreReceipts(t, txCount)
		headers[blockNum] = &core.Header{
			Hash:   felt.NewRandom[felt.Felt](),
			Number: uint64(blockNum),
		}
	}

	runTestTransactionLayout(t, transactions, receipts, headers, core.TransactionLayoutPerTx)
	runTestTransactionLayout(t, transactions, receipts, headers, core.TransactionLayoutCombined)
}

func TestTransactionLayout_UnknownLayout(t *testing.T) {
	database, err := pebblev2.New(t.TempDir())
	require.NoError(t, err)
	defer database.Close()

	unknownLayout := core.TransactionLayout(99)

	t.Run("TransactionByBlockAndIndex", func(t *testing.T) {
		_, err := unknownLayout.TransactionByBlockAndIndex(database, 0, 0)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})

	t.Run("ReceiptByBlockAndIndex", func(t *testing.T) {
		_, err := unknownLayout.ReceiptByBlockAndIndex(database, 0, 0)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})

	t.Run("TransactionsByBlockNumber", func(t *testing.T) {
		_, err := unknownLayout.TransactionsByBlockNumber(database, 0)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})

	t.Run("TransactionsByBlockNumberIter", func(t *testing.T) {
		isFirstIteration := true
		for _, err := range unknownLayout.TransactionsByBlockNumberIter(database, 0) {
			require.True(t, isFirstIteration)
			isFirstIteration = false
			require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
		}
	})

	t.Run("ReceiptsByBlockNumber", func(t *testing.T) {
		_, err := unknownLayout.ReceiptsByBlockNumber(database, 0)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})

	t.Run("WriteTransactionsAndReceipts", func(t *testing.T) {
		err := unknownLayout.WriteTransactionsAndReceipts(database, 0, nil, nil)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})

	t.Run("DeleteTxsAndReceipts", func(t *testing.T) {
		batch := database.NewIndexedBatch()
		err := unknownLayout.DeleteTxsAndReceipts(batch, 0)
		require.ErrorIs(t, err, core.ErrUnknownTransactionLayout)
	})
}
