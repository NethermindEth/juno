package pending_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
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

func TestPreConfirmedApplyDelta(t *testing.T) {
	n := &networks.SepoliaIntegration
	client := feeder.NewTestClient(t, n)
	adapter := adaptfeeder.New(client)
	emptyPreConfirmedBlock := uint64(1201960)
	blockWithCandidates := uint64(1204672)
	blockWithNoCandidates := uint64(1204673)
	blockFullOfCandidates := uint64(1204674)
	blocksWithRandomCandidateOrder := uint64(1204675)

	type preConfirmedCase struct {
		description string
		blockNumber uint64
	}

	pcCases := []preConfirmedCase{
		{
			description: "PreConfirmedBlock with no candidates",
			blockNumber: blockWithNoCandidates,
		},
		{
			description: "PreConfirmedBlock with candidates",
			blockNumber: blockWithCandidates,
		},
		{
			description: "PreConfirmedBlock full of candidates",
			blockNumber: blockFullOfCandidates,
		},
		{
			description: "PreConfirmedBlock empty",
			blockNumber: emptyPreConfirmedBlock,
		},
		{
			description: "PreConfirmedBlock with candidate in between preconfirmed txns",
			blockNumber: blocksWithRandomCandidateOrder,
		},
	}

	for _, pcCase := range pcCases {
		t.Run(pcCase.description, func(t *testing.T) {
			// early fetch the pre-confirmed block; we need it to generate the delta test cases
			resp, err := adapter.PreConfirmedBlockByNumber(t.Context(), pcCase.blockNumber, "", 0)
			require.NoError(t, err)
			require.NotNil(t, resp.FullBlock)
			deltaCases := makeDeltaCases(t, resp.FullBlock)

			for _, deltaCase := range deltaCases {
				t.Run(deltaCase.description, func(t *testing.T) {
					t.Parallel()
					response, err := adapter.PreConfirmedBlockByNumber(t.Context(), pcCase.blockNumber, "", 0)
					require.NoError(t, err)
					require.NotNil(t, response.FullBlock)
					preConfirmed := response.FullBlock

					new := preConfirmed.ApplyDelta(
						deltaCase.update.AppendTransactions,
						deltaCase.update.AppendReceipts,
						deltaCase.update.AppendStateDiffs,
						deltaCase.update.AppendCandidateTxs,
						response.BlockIdentifier,
					)

					// get again a brand new preConfirmed from the feeder to use in the assertions
					response, err = adapter.PreConfirmedBlockByNumber(t.Context(), pcCase.blockNumber, "", 0)
					require.NoError(t, err)
					require.NotNil(t, response.FullBlock)
					originalPreConfirmed := response.FullBlock

					assert.Equal(
						t,
						originalPreConfirmed,
						preConfirmed,
						"ApplyDelta should not modify the original block",
					)

					t.Run("assert changes related to transactions delta", func(t *testing.T) {
						t.Parallel()
						original := *originalPreConfirmed
						txsLength := len(deltaCase.update.AppendTransactions)

						assert.Equal(t,
							original.Block.Transactions,
							new.Block.Transactions[:len(new.Block.Transactions)-txsLength],
						)
						assert.Equal(t,
							original.Block.TransactionCount+uint64(txsLength),
							new.Block.TransactionCount,
						)

						if len(deltaCase.update.AppendTransactions) > 0 {
							assert.Equal(t,
								deltaCase.update.AppendTransactions,
								new.Block.Transactions[len(original.Block.Transactions):],
							)
						}
					})

					t.Run("assert changes related to receipts delta", func(t *testing.T) {
						t.Parallel()
						original := *originalPreConfirmed
						receiptsLength := len(deltaCase.update.AppendReceipts)

						assert.Equal(t,
							original.Block.Receipts,
							new.Block.Receipts[:len(new.Block.Receipts)-receiptsLength],
						)

						if len(deltaCase.update.AppendReceipts) > 0 {
							assert.Equal(t,
								deltaCase.update.AppendReceipts,
								new.Block.Receipts[len(original.Block.Receipts):],
							)
						}

						var eventCount uint64
						for _, r := range deltaCase.update.AppendReceipts {
							eventCount += uint64(len(r.Events))
						}
						assert.Equal(t,
							original.Block.EventCount+eventCount,
							new.Block.EventCount,
						)

						if eventCount > 0 {
							deltaAndOriginalBloom := core.EventsBloom(deltaCase.update.AppendReceipts)
							require.NoError(t, deltaAndOriginalBloom.Merge(original.Block.Header.EventsBloom))
							assert.True(t, deltaAndOriginalBloom.Equal(new.Block.Header.EventsBloom))
							return
						}
						assert.True(t, original.Block.Header.EventsBloom.Equal(new.Block.Header.EventsBloom))
					})

					t.Run("assert changes related to state diff delta", func(t *testing.T) {
						t.Parallel()
						original := *originalPreConfirmed
						originalDiff := original.StateUpdate.StateDiff
						deltaDiffs := deltaCase.update.AppendStateDiffs

						assert.Equal(t,
							original.TransactionStateDiffs,
							new.TransactionStateDiffs[:len(new.TransactionStateDiffs)-len(deltaDiffs)],
						)

						if len(deltaDiffs) > 0 {
							assert.Equal(t,
								deltaDiffs,
								new.TransactionStateDiffs[len(original.TransactionStateDiffs):],
							)
						}

						deltaAndOriginalDiff := core.EmptyStateDiff()
						deltaAndOriginalDiff.Merge(originalDiff)
						for _, sd := range deltaDiffs {
							deltaAndOriginalDiff.Merge(sd)
						}
						assert.Equal(t,
							deltaAndOriginalDiff.Hash(),
							new.StateUpdate.StateDiff.Hash(),
						)

						assert.Equal(t, original.StateUpdate.BlockHash, new.StateUpdate.BlockHash)
						assert.Equal(t, original.StateUpdate.NewRoot, new.StateUpdate.NewRoot)
						assert.Equal(t, original.StateUpdate.OldRoot, new.StateUpdate.OldRoot)
					})

					t.Run("assert changes related to candidate transactions delta", func(t *testing.T) {
						t.Parallel()
						original := *originalPreConfirmed
						candidateTxsLength := len(deltaCase.update.AppendCandidateTxs)

						assert.Equal(t,
							original.CandidateTxs,
							new.CandidateTxs[:len(new.CandidateTxs)-candidateTxsLength],
						)

						if len(deltaCase.update.AppendCandidateTxs) > 0 {
							assert.Equal(t,
								deltaCase.update.AppendCandidateTxs,
								new.CandidateTxs[len(original.CandidateTxs):],
							)
						}
					})

					t.Run("assert immutability of remaining fields", func(t *testing.T) {
						t.Parallel()
						localNew := copyPreConfirmed(t, new)
						require.Equal(t, new, localNew)

						//revert all the changes made by ApplyDelta
						localNew.Block.Header.EventsBloom = originalPreConfirmed.Block.Header.EventsBloom
						localNew.Block.Header.TransactionCount = originalPreConfirmed.Block.Header.TransactionCount
						localNew.Block.Header.EventCount = originalPreConfirmed.Block.Header.EventCount
						localNew.Block.Header.TransactionCount = originalPreConfirmed.Block.Header.TransactionCount
						localNew.Block.Transactions = originalPreConfirmed.Block.Transactions
						localNew.Block.Receipts = originalPreConfirmed.Block.Receipts
						localNew.StateUpdate.StateDiff = originalPreConfirmed.StateUpdate.StateDiff
						localNew.TransactionStateDiffs = originalPreConfirmed.TransactionStateDiffs
						localNew.CandidateTxs = originalPreConfirmed.CandidateTxs

						assert.Equal(t, originalPreConfirmed, localNew)
					})
				})
			}
		})
	}
}

type deltaCase struct {
	description string
	update      pending.PreConfirmedUpdate
}

func makeDeltaCases(t *testing.T, prec *pending.PreConfirmed) []deltaCase {
	t.Helper()

	randFelt := func() *felt.Felt { return felt.NewRandom[felt.Felt]() }

	makeTxsAndReceipts := func(count int) ([]core.Transaction, []*core.TransactionReceipt) {
		txs := make([]core.Transaction, count)
		receipts := make([]*core.TransactionReceipt, count)

		for i := range count {
			hash := randFelt()
			txs[i] = &core.InvokeTransaction{
				TransactionHash: hash,
			}
			receipts[i] = &core.TransactionReceipt{
				TransactionHash: hash,
			}
		}

		return txs, receipts
	}

	makeStateDiffs := func(count int) []*core.StateDiff {
		stateDiffs := make([]*core.StateDiff, count)
		for i := range count {
			stateDiff := core.EmptyStateDiff()

			// if there are state diffs available in the pre-confirmed block that we are going to
			// apply the delta to, we use their data as a base in order to
			// simulate a tx overwriting storage values already changed
			// by a previous transaction.
			if len(prec.TransactionStateDiffs) > i {
				stateDiff.Merge(prec.TransactionStateDiffs[i])

				// add a random new entry
				stateDiff.StorageDiffs[*randFelt()] = map[felt.Felt]*felt.Felt{
					*randFelt(): randFelt(),
				}

				// overwrite all existing entries
				for addr, storageMap := range stateDiff.StorageDiffs {
					for key := range storageMap {
						stateDiff.StorageDiffs[addr][key] = randFelt()
					}
				}
			} else {
				// otherwise, a new state diff with random values
				stateDiff.StorageDiffs[*randFelt()] = map[felt.Felt]*felt.Felt{
					*randFelt(): randFelt(),
					*randFelt(): randFelt(),
				}
				stateDiff.StorageDiffs[*randFelt()] = map[felt.Felt]*felt.Felt{
					*randFelt(): randFelt(),
				}
			}

			stateDiffs[i] = &stateDiff
		}
		return stateDiffs
	}

	deltaCases := []deltaCase{}

	txs, _ := makeTxsAndReceipts(1)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with one candidate",
			update: pending.PreConfirmedUpdate{
				AppendCandidateTxs: txs,
			},
		},
	)

	txs, _ = makeTxsAndReceipts(5)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with multiple candidates",
			update: pending.PreConfirmedUpdate{
				AppendCandidateTxs: txs,
			},
		},
	)

	txs, receipts := makeTxsAndReceipts(1)
	stateDiffs := makeStateDiffs(1)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with one tx, receipt and state diff",
			update: pending.PreConfirmedUpdate{
				AppendTransactions: txs,
				AppendReceipts:     receipts,
				AppendStateDiffs:   stateDiffs,
			},
		},
	)

	txs, receipts = makeTxsAndReceipts(1)
	candidateTxs, _ := makeTxsAndReceipts(1)
	stateDiffs = makeStateDiffs(1)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with one tx, receipt, state diff and candidate tx",
			update: pending.PreConfirmedUpdate{
				AppendTransactions: txs,
				AppendReceipts:     receipts,
				AppendStateDiffs:   stateDiffs,
				AppendCandidateTxs: candidateTxs,
			},
		},
	)

	txs, receipts = makeTxsAndReceipts(10)
	stateDiffs = makeStateDiffs(10)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with multiple txs, receipts and state diffs",
			update: pending.PreConfirmedUpdate{
				AppendTransactions: txs,
				AppendReceipts:     receipts,
				AppendStateDiffs:   stateDiffs,
			},
		},
	)

	txs, receipts = makeTxsAndReceipts(10)
	candidateTxs, _ = makeTxsAndReceipts(10)
	stateDiffs = makeStateDiffs(10)
	deltaCases = append(
		deltaCases,
		deltaCase{
			description: "Delta with multiple txs, receipts, state diffs and candidate txs",
			update: pending.PreConfirmedUpdate{
				AppendTransactions: txs,
				AppendReceipts:     receipts,
				AppendStateDiffs:   stateDiffs,
				AppendCandidateTxs: candidateTxs,
			},
		},
	)

	return deltaCases
}

func copyPreConfirmed(t *testing.T, pc *pending.PreConfirmed) *pending.PreConfirmed {
	t.Helper()

	copiedPreConfirmed := *pc

	header := *pc.Block.Header
	block := core.Block{
		Header:       &header,
		Transactions: pc.Block.Transactions,
		Receipts:     pc.Block.Receipts,
	}
	copiedPreConfirmed.Block = &block

	copiedPreConfirmed.StateUpdate = &core.StateUpdate{
		BlockHash: pc.StateUpdate.BlockHash,
		NewRoot:   pc.StateUpdate.NewRoot,
		OldRoot:   pc.StateUpdate.OldRoot,
		StateDiff: pc.StateUpdate.StateDiff,
	}

	return &copiedPreConfirmed
}
