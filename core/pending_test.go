package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestPendingValidate(t *testing.T) {
	pending0 := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				ParentHash: &felt.Zero,
				Number:     0,
			},
		},
	}

	require.True(t, pending0.Validate(nil))

	// Pending becomes head
	head0 := &core.Block{
		Header: &core.Header{
			ParentHash: &felt.Zero,
			Hash:       &felt.One,
			Number:     0,
		},
	}

	require.False(t, pending0.Validate(head0.Header))

	// Pending for head
	pending1 := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				ParentHash: &felt.One,
				Number:     1,
			},
		},
	}

	require.True(t, pending1.Validate(head0.Header))
}

func TestPreConfirmedValidate(t *testing.T) {
	t.Run("Without PreLatest", func(t *testing.T) {
		// Genesis case with nil parent
		preConfirmed0 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 0,
				},
			},
		}

		require.True(t, preConfirmed0.Validate(nil))

		// PreConfirmed becomes head
		head0 := &core.Block{
			Header: &core.Header{
				Number:     0,
				ParentHash: &felt.Zero,
				Hash:       &felt.One,
			},
		}

		require.False(t, preConfirmed0.Validate(head0.Header))

		// PreConfirmed for head
		preConfirmed1 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 1,
				},
			},
		}

		require.True(t, preConfirmed1.Validate(head0.Header))
	})

	t.Run("With Prelatest", func(t *testing.T) {
		head0 := &core.Block{
			Header: &core.Header{
				Number:     0,
				ParentHash: &felt.Zero,
				Hash:       &felt.One,
			},
		}

		preLatest1 := core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: &felt.One,
					Number:     1,
				},
			},
		}
		// Genesis case with nil parent
		preConfirmed2 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 2,
				},
			},
		}

		preConfirmed2.WithPreLatest(&preLatest1)
		require.True(t, preConfirmed2.Validate(head0.Header))

		// Prelatest becomes latest preLatest nullified
		preConfirmed2.WithPreLatest(nil)
		head1 := preLatest1.Block
		require.True(t, preConfirmed2.Validate(head1.Header))

		// PreConfirmed becomes head, preconfirmed not upto date
		head2 := preConfirmed2.Block
		require.False(t, preConfirmed2.Validate(head2.Header))
	})
}

// Helper function to create test transactions
func createTestTransaction(hash *felt.Felt) core.Transaction {
	return &core.InvokeTransaction{
		TransactionHash: hash,
	}
}

// Helper function to create test receipts
func createTestReceipt(txHash *felt.Felt) *core.TransactionReceipt {
	return &core.TransactionReceipt{
		TransactionHash: txHash,
	}
}

func TestPendingTransactionByHash(t *testing.T) {
	existingTxHash := felt.FromUint64[felt.Felt](1)

	txn := createTestTransaction(&existingTxHash)

	pending := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				Number:     1,
				ParentHash: &felt.Zero,
			},
			Transactions: []core.Transaction{txn},
		},
	}

	t.Run("find existing transaction", func(t *testing.T) {
		foundTxn, err := pending.TransactionByHash(&existingTxHash)
		require.NoError(t, err)
		require.Equal(t, txn, foundTxn)
	})

	t.Run("transaction not found", func(t *testing.T) {
		nonExistingHash := felt.FromUint64[felt.Felt](999)
		_, err := pending.TransactionByHash(&nonExistingHash)
		require.Error(t, err)
		require.Equal(t, core.ErrTransactionNotFound, err)
	})
}

func TestPendingReceiptByHash(t *testing.T) {
	receiptHash1 := felt.FromUint64[felt.Felt](1)
	receiptHash2 := felt.FromUint64[felt.Felt](2)

	receipt1 := createTestReceipt(&receiptHash1)
	receipt2 := createTestReceipt(&receiptHash2)

	parentHash := felt.FromUint64[felt.Felt](999)
	blockNumber := uint64(1)

	pending := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				Number:     blockNumber,
				ParentHash: &parentHash,
			},
			Receipts: []*core.TransactionReceipt{receipt1, receipt2},
		},
	}

	t.Run("find existing receipt", func(t *testing.T) {
		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err := pending.ReceiptByHash(&receiptHash1)
		require.NoError(t, err)
		require.Equal(t, receipt1, foundReceipt)
		require.Equal(t, parentHash, *foundParentHash)
		require.Equal(t, blockNumber, foundBlockNumber)

		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err = pending.ReceiptByHash(&receiptHash2)
		require.NoError(t, err)
		require.Equal(t, receipt2, foundReceipt)
		require.Equal(t, parentHash, *foundParentHash)
		require.Equal(t, blockNumber, foundBlockNumber)
	})

	t.Run("receipt not found", func(t *testing.T) {
		nonExistingReceiptHash := new(felt.Felt).SetUint64(3)
		_, _, _, err := pending.ReceiptByHash(nonExistingReceiptHash)
		require.Error(t, err)
		require.Equal(t, core.ErrTransactionReceiptNotFound, err)
	})
}

func TestPreConfirmedTransactionByHash(t *testing.T) {
	preLatestTxHash := felt.FromUint64[felt.Felt](1)
	preConfirmedTxHash := felt.FromUint64[felt.Felt](2)
	candidateTxHash := felt.FromUint64[felt.Felt](3)
	nonExistingTxHash := felt.FromUint64[felt.Felt](4)

	preLatestTx := createTestTransaction(&preLatestTxHash)
	preConfirmedTx := createTestTransaction(&preConfirmedTxHash)
	candidateTx := createTestTransaction(&candidateTxHash)

	preLatest := &core.PreLatest{
		Block: &core.Block{
			Header: &core.Header{
				Number:     1,
				ParentHash: &felt.Zero,
			},
			Transactions: []core.Transaction{preLatestTx},
		},
	}

	preConfirmed := &core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 2,
			},
			Transactions: []core.Transaction{preConfirmedTx},
		},
		CandidateTxs: []core.Transaction{candidateTx},
	}

	t.Run("find transaction in pre_latest", func(t *testing.T) {
		preConfirmed.WithPreLatest(preLatest)
		foundTx, err := preConfirmed.TransactionByHash(&preLatestTxHash)
		require.NoError(t, err)
		require.Equal(t, preLatestTx, foundTx)
	})

	t.Run("find transaction in candidate transactions", func(t *testing.T) {
		foundTx, err := preConfirmed.TransactionByHash(&candidateTxHash)
		require.NoError(t, err)
		require.Equal(t, candidateTx, foundTx)
	})

	t.Run("find transaction in pre_confirmed block", func(t *testing.T) {
		foundTx, err := preConfirmed.TransactionByHash(&preConfirmedTxHash)
		require.NoError(t, err)
		require.Equal(t, preConfirmedTx, foundTx)
	})

	t.Run("transaction not found", func(t *testing.T) {
		_, err := preConfirmed.TransactionByHash(&nonExistingTxHash)
		require.Error(t, err)
		require.Equal(t, core.ErrTransactionNotFound, err)
	})
}

func TestPreConfirmedReceiptByHash(t *testing.T) {
	preLatestReceiptHash := felt.FromUint64[felt.Felt](1)
	preConfirmedReceiptHash := felt.FromUint64[felt.Felt](2)
	nonExistingReceiptHash := felt.FromUint64[felt.Felt](3)

	preLatestReceipt := createTestReceipt(&preLatestReceiptHash)
	preConfirmedReceipt := createTestReceipt(&preConfirmedReceiptHash)

	preLatestParentHash := felt.FromUint64[felt.Felt](100)
	preLatestBlockNumber := uint64(1)
	preLatest := &core.PreLatest{
		Block: &core.Block{
			Header: &core.Header{
				Number:     preLatestBlockNumber,
				ParentHash: &preLatestParentHash,
			},
			Receipts: []*core.TransactionReceipt{preLatestReceipt},
		},
	}

	preConfirmedBlockNumber := uint64(2)
	preConfirmed := &core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: preConfirmedBlockNumber,
			},
			Receipts: []*core.TransactionReceipt{preConfirmedReceipt},
		},
	}

	t.Run("find receipt in pre_latest", func(t *testing.T) {
		preConfirmed.WithPreLatest(preLatest)
		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err := preConfirmed.ReceiptByHash(&preLatestReceiptHash)
		require.NoError(t, err)
		require.Equal(t, preLatestReceipt, foundReceipt)
		require.Equal(t, preLatestParentHash, *foundParentHash)
		require.Equal(t, preLatestBlockNumber, foundBlockNumber)
	})

	t.Run("find receipt in pre_confirmed block", func(t *testing.T) {
		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err := preConfirmed.ReceiptByHash(&preConfirmedReceiptHash)
		require.NoError(t, err)
		require.Nil(t, foundParentHash)
		require.Equal(t, preConfirmedReceipt, foundReceipt)
		require.Equal(t, preConfirmedBlockNumber, foundBlockNumber)
	})

	t.Run("receipt not found", func(t *testing.T) {
		_, _, _, err := preConfirmed.ReceiptByHash(&nonExistingReceiptHash)
		require.Error(t, err)
		require.Equal(t, core.ErrTransactionReceiptNotFound, err)
	})
}
