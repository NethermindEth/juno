package pending_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/stretchr/testify/require"
)

func TestPreConfirmedTransactionByHash(t *testing.T) {
	preConfirmedTxHash := felt.FromUint64[felt.Felt](2)
	nonExistingTxHash := felt.FromUint64[felt.Felt](4)

	preConfirmedTx := &core.InvokeTransaction{
		TransactionHash: &preConfirmedTxHash,
	}

	preConfirmed := &pending.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 2,
			},
			Transactions: []core.Transaction{preConfirmedTx},
		},
	}

	t.Run("find transaction in pre_confirmed block", func(t *testing.T) {
		foundTx, idx, err := preConfirmed.TransactionByHash(&preConfirmedTxHash)
		require.NoError(t, err)
		require.Equal(t, preConfirmedTx, foundTx)
		require.Equal(t, uint(0), idx)
	})

	t.Run("transaction not found", func(t *testing.T) {
		_, _, err := preConfirmed.TransactionByHash(&nonExistingTxHash)
		require.Error(t, err)
		require.Equal(t, pending.ErrTransactionNotFound, err)
	})
}

func TestPreConfirmedReceiptByHash(t *testing.T) {
	preConfirmedReceiptHash := felt.FromUint64[felt.Felt](2)
	nonExistingReceiptHash := felt.FromUint64[felt.Felt](3)

	preConfirmedReceipt := core.TransactionReceipt{
		TransactionHash: &preConfirmedReceiptHash,
	}

	preConfirmedBlockNumber := uint64(2)
	preConfirmed := &pending.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: preConfirmedBlockNumber,
			},
			Receipts: []*core.TransactionReceipt{&preConfirmedReceipt},
		},
	}

	t.Run("find receipt in pre_confirmed block", func(t *testing.T) {
		foundReceipt, err := preConfirmed.ReceiptByHash(&preConfirmedReceiptHash)
		require.NoError(t, err)
		require.Equal(t, preConfirmedReceipt, *foundReceipt)
	})

	t.Run("receipt not found", func(t *testing.T) {
		_, err := preConfirmed.ReceiptByHash(&nonExistingReceiptHash)
		require.Error(t, err)
		require.Equal(t, pending.ErrTransactionReceiptNotFound, err)
	})
}
