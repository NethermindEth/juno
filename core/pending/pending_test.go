package pending_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/stretchr/testify/require"
)

func TestPreConfirmedValidate(t *testing.T) {
	t.Run("without pre-latest", func(t *testing.T) {
		// Genesis case with nil parent
		preConfirmed0 := &pending.PreConfirmed{
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
		preConfirmed1 := &pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 1,
				},
			},
		}

		require.True(t, preConfirmed1.Validate(head0.Header))
	})

	t.Run("with pre-latest", func(t *testing.T) {
		head0 := &core.Block{
			Header: &core.Header{
				Number:     0,
				ParentHash: &felt.Zero,
				Hash:       &felt.One,
			},
		}

		preLatest1 := pending.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: &felt.One,
					Number:     1,
				},
			},
		}
		// Genesis case with nil parent
		preConfirmed2 := &pending.PreConfirmed{
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

func TestPreConfirmedTransactionByHash(t *testing.T) {
	preLatestTxHash := felt.FromUint64[felt.Felt](1)
	preConfirmedTxHash := felt.FromUint64[felt.Felt](2)
	candidateTxHash := felt.FromUint64[felt.Felt](3)
	nonExistingTxHash := felt.FromUint64[felt.Felt](4)

	preLatestTx := &core.InvokeTransaction{
		TransactionHash: &preLatestTxHash,
	}
	preConfirmedTx := &core.InvokeTransaction{
		TransactionHash: &preConfirmedTxHash,
	}
	candidateTx := &core.InvokeTransaction{
		TransactionHash: &candidateTxHash,
	}

	preLatest := &pending.PreLatest{
		Block: &core.Block{
			Header: &core.Header{
				Number:     1,
				ParentHash: &felt.Zero,
			},
			Transactions: []core.Transaction{preLatestTx},
		},
	}

	preConfirmed := &pending.PreConfirmed{
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
		require.Equal(t, pending.ErrTransactionNotFound, err)
	})
}

func TestPreConfirmedReceiptByHash(t *testing.T) {
	preLatestReceiptHash := felt.FromUint64[felt.Felt](1)
	preConfirmedReceiptHash := felt.FromUint64[felt.Felt](2)
	nonExistingReceiptHash := felt.FromUint64[felt.Felt](3)

	preLatestReceipt := core.TransactionReceipt{
		TransactionHash: &preLatestReceiptHash,
	}
	preConfirmedReceipt := core.TransactionReceipt{
		TransactionHash: &preConfirmedReceiptHash,
	}

	preLatestParentHash := felt.FromUint64[felt.Felt](100)
	preLatestBlockNumber := uint64(1)
	preLatest := &pending.PreLatest{
		Block: &core.Block{
			Header: &core.Header{
				Number:     preLatestBlockNumber,
				ParentHash: &preLatestParentHash,
			},
			Receipts: []*core.TransactionReceipt{&preLatestReceipt},
		},
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

	t.Run("find receipt in pre_latest", func(t *testing.T) {
		preConfirmed.WithPreLatest(preLatest)
		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err := preConfirmed.ReceiptByHash(&preLatestReceiptHash)
		require.NoError(t, err)
		require.Equal(t, preLatestReceipt, *foundReceipt)
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
		require.Equal(t, preConfirmedReceipt, *foundReceipt)
		require.Equal(t, preConfirmedBlockNumber, foundBlockNumber)
	})

	t.Run("receipt not found", func(t *testing.T) {
		_, _, _, err := preConfirmed.ReceiptByHash(&nonExistingReceiptHash)
		require.Error(t, err)
		require.Equal(t, pending.ErrTransactionReceiptNotFound, err)
	})
}

// Helper function to create state diffs with incrementing counter values
func createStateDiffWithIncrementingCounter(
	t *testing.T,
	contractAddress *felt.Felt,
	storageKey *felt.Felt,
	numTxs int,
) ([]*core.StateDiff, *core.StateDiff) {
	t.Helper()

	transactionStateDiffs := make([]*core.StateDiff, numTxs)
	contractAddr := *contractAddress
	storageKeyVal := *storageKey

	for i := range numTxs {
		counterValue := felt.FromUint64[felt.Felt](uint64(i + 1))
		nonceValue := felt.FromUint64[felt.Felt](uint64(i + 1))

		stateDiff := &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				contractAddr: {
					storageKeyVal: &counterValue,
				},
			},
			Nonces: map[felt.Felt]*felt.Felt{
				contractAddr: &nonceValue,
			},
			DeployedContracts: make(map[felt.Felt]*felt.Felt, 0),
			DeclaredV0Classes: make([]*felt.Felt, 0),
			DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, 0),
			ReplacedClasses:   make(map[felt.Felt]*felt.Felt, 0),
		}
		transactionStateDiffs[i] = stateDiff
	}

	// Create aggregated state diff
	aggregatedStateDiff := core.EmptyStateDiff()
	for _, stateDiff := range transactionStateDiffs {
		aggregatedStateDiff.Merge(stateDiff)
	}

	return transactionStateDiffs, &aggregatedStateDiff
}

func TestPreConfirmed_PendingState(t *testing.T) {
	// Create a state diff with storage update
	contractAddress := felt.FromUint64[felt.Felt](0x10000)
	storageKey := felt.FromUint64[felt.Felt](0x10)
	storageValue := felt.FromUint64[felt.Felt](0x100)

	stateDiff := core.EmptyStateDiff()
	stateDiff.StorageDiffs[contractAddress] = map[felt.Felt]*felt.Felt{
		storageKey: &storageValue,
	}
	emptyBlock := core.Block{
		Header: &core.Header{
			Number: 0,
		},
	}

	preLatestContractAddress := felt.FromUint64[felt.Felt](0x20000)
	preLatestStorageKey := felt.FromUint64[felt.Felt](0x20)
	preLatestStorageValue := felt.FromUint64[felt.Felt](0x200)

	preLatestStateDiff := core.EmptyStateDiff()
	preLatestStateDiff.StorageDiffs[preLatestContractAddress] = map[felt.Felt]*felt.Felt{
		preLatestStorageKey: &preLatestStorageValue,
	}
	// Set a different storage value in pre_latest for storageKey to
	// verify that it is overwritten by the pre_confirmed state diff
	preLatestStateDiff.StorageDiffs[storageKey] = map[felt.Felt]*felt.Felt{
		storageKey: felt.NewFromUint64[felt.Felt](0xFFFFFFFFFFFFFFFF),
	}

	preConfirmed := pending.PreConfirmed{
		StateUpdate: &core.StateUpdate{
			StateDiff: &stateDiff,
		},
	}
	preConfirmed.Block = &emptyBlock

	preLatest := &pending.PreLatest{
		StateUpdate: &core.StateUpdate{
			StateDiff: &preLatestStateDiff,
		},
	}

	t.Run("pending state without pre-latest", func(t *testing.T) {
		// Test that we can get pending state
		state := preConfirmed.PendingState(nil)
		require.NotNil(t, state)

		// Test that we can query the updated storage value
		retrievedValue, err := state.ContractStorage(&contractAddress, &storageKey)
		require.NoError(t, err)
		require.Equal(t, storageValue, retrievedValue)
	})

	t.Run("pending state with pre-latest", func(t *testing.T) {
		// Create a state diff with storage update
		preConfirmedWithPreLatest := preConfirmed.Copy().WithPreLatest(preLatest)

		// Test that we can get pending state
		state := preConfirmedWithPreLatest.PendingState(nil)
		require.NotNil(t, state)

		// Test that we can query the storage value in pre_confirmed
		retrievedValue, err := state.ContractStorage(&contractAddress, &storageKey)
		require.NoError(t, err)
		require.Equal(t, storageValue, retrievedValue)

		// Test that we can query the storage value in pre_latest
		retrievedValue, err = state.ContractStorage(&preLatestContractAddress, &preLatestStorageKey)
		require.NoError(t, err)
		require.Equal(t, preLatestStorageValue, retrievedValue)
	})
}

func TestPending_PendingStateBeforeIndex(t *testing.T) {
	// Create a state diff with storage update
	preConfirmedContractAddress := felt.FromUint64[felt.Felt](0x10000)
	preConfirmedStorageKey := felt.FromUint64[felt.Felt](0x10)

	preLatestContractAddress := felt.FromUint64[felt.Felt](0x20000)
	preLatestStorageKey := felt.FromUint64[felt.Felt](0x20)
	preLatestStorageValue := felt.FromUint64[felt.Felt](0x200)

	preLatestStateDiff := core.EmptyStateDiff()
	preLatestStateDiff.StorageDiffs[preLatestContractAddress] = map[felt.Felt]*felt.Felt{
		preLatestStorageKey: &preLatestStorageValue,
	}
	// Set a different storage value in pre_latest for preConfirmedStorageKey to
	// verify that it is overwritten by the pre_confirmed state diff
	preLatestStateDiff.StorageDiffs[preConfirmedContractAddress] = map[felt.Felt]*felt.Felt{
		preConfirmedStorageKey: felt.NewFromUint64[felt.Felt](0xFFFFFFFFFFFFFFFF),
	}

	preLatest := &pending.PreLatest{
		StateUpdate: &core.StateUpdate{
			StateDiff: &preLatestStateDiff,
		},
	}

	// Create transaction state diffs that increment a counter
	numTxs := 10
	transactionStateDiffs, aggregatedStateDiff := createStateDiffWithIncrementingCounter(
		t,
		&preConfirmedContractAddress,
		&preConfirmedStorageKey,
		numTxs,
	)

	assertPendingStateAtIndex := func(t *testing.T, preConfirmed *pending.PreConfirmed, idx int) {
		state, err := preConfirmed.PendingStateBeforeIndex(nil, uint(idx+1))
		require.NoError(t, err)
		require.NotNil(t, state)

		// Check that storage value reflects the state up to transaction i
		retrievedValue, err := state.ContractStorage(
			&preConfirmedContractAddress,
			&preConfirmedStorageKey,
		)
		require.NoError(t, err)
		expectedValue := felt.FromUint64[felt.Felt](uint64(idx + 1))
		require.Equal(t, expectedValue, retrievedValue)

		// Check that nonce value reflects the state up to transaction i
		retrievedNonce, err := state.ContractNonce(&preConfirmedContractAddress)
		require.NoError(t, err)
		expectedNonce := felt.FromUint64[felt.Felt](uint64(idx + 1))
		require.Equal(t, expectedNonce, retrievedNonce)
	}

	preConfirmed := pending.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 0,
			},
			Transactions: make([]core.Transaction, numTxs),
		},
		StateUpdate: &core.StateUpdate{
			StateDiff: aggregatedStateDiff,
		},
		TransactionStateDiffs: transactionStateDiffs,
	}

	t.Run("out of bound index returns error", func(t *testing.T) {
		_, err := preConfirmed.PendingStateBeforeIndex(
			nil,
			uint(len(preConfirmed.Block.Transactions)+1),
		)
		require.ErrorIs(t, err, pending.ErrTransactionIndexOutOfBounds)
	})

	t.Run("without pre-latest", func(t *testing.T) {
		// Test PendingStateBeforeIndex for different transaction indices
		for i := range numTxs {
			assertPendingStateAtIndex(t, &preConfirmed, i)
		}
	})

	t.Run("with pre-latest", func(t *testing.T) {
		preConfirmed.WithPreLatest(preLatest)

		state, err := preConfirmed.PendingStateBeforeIndex(nil, uint(0))
		require.NoError(t, err)
		require.NotNil(t, state)

		// Test that we can query the storage value in pre_latest
		retrievedValue, err := state.ContractStorage(&preLatestContractAddress, &preLatestStorageKey)
		require.NoError(t, err)
		require.Equal(t, preLatestStorageValue, retrievedValue)

		for i := range numTxs {
			assertPendingStateAtIndex(t, &preConfirmed, i)
		}
	})
}
