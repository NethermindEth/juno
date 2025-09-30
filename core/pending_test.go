package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
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

func TestPendingTransactionByHash(t *testing.T) {
	existingTxHash := felt.FromUint64[felt.Felt](1)

	txn := &core.InvokeTransaction{
		TransactionHash: &existingTxHash,
	}

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

	receipt1 := core.TransactionReceipt{
		TransactionHash: &receiptHash1,
	}
	receipt2 := core.TransactionReceipt{
		TransactionHash: &receiptHash2,
	}

	parentHash := felt.FromUint64[felt.Felt](999)
	blockNumber := uint64(1)

	pending := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				Number:     blockNumber,
				ParentHash: &parentHash,
			},
			Receipts: []*core.TransactionReceipt{&receipt1, &receipt2},
		},
	}

	t.Run("find existing receipt", func(t *testing.T) {
		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err := pending.ReceiptByHash(&receiptHash1)
		require.NoError(t, err)
		require.Equal(t, receipt1, *foundReceipt)
		require.Equal(t, parentHash, *foundParentHash)
		require.Equal(t, blockNumber, foundBlockNumber)

		foundReceipt,
			foundParentHash,
			foundBlockNumber,
			err = pending.ReceiptByHash(&receiptHash2)
		require.NoError(t, err)
		require.Equal(t, receipt2, *foundReceipt)
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

	preLatestTx := &core.InvokeTransaction{
		TransactionHash: &preLatestTxHash,
	}
	preConfirmedTx := &core.InvokeTransaction{
		TransactionHash: &preConfirmedTxHash,
	}
	candidateTx := &core.InvokeTransaction{
		TransactionHash: &candidateTxHash,
	}

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

	preLatestReceipt := core.TransactionReceipt{
		TransactionHash: &preLatestReceiptHash,
	}
	preConfirmedReceipt := core.TransactionReceipt{
		TransactionHash: &preConfirmedReceiptHash,
	}

	preLatestParentHash := felt.FromUint64[felt.Felt](100)
	preLatestBlockNumber := uint64(1)
	preLatest := &core.PreLatest{
		Block: &core.Block{
			Header: &core.Header{
				Number:     preLatestBlockNumber,
				ParentHash: &preLatestParentHash,
			},
			Receipts: []*core.TransactionReceipt{&preLatestReceipt},
		},
	}

	preConfirmedBlockNumber := uint64(2)
	preConfirmed := &core.PreConfirmed{
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
		require.Equal(t, core.ErrTransactionReceiptNotFound, err)
	})
}

func TestPendingStateAccess(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	t.Run("starknet version <= 0.14.0", func(t *testing.T) {
		var synchronizer *sync.Synchronizer
		testDB := memory.New()
		chain := blockchain.New(testDB, &utils.Mainnet)
		dataSource := sync.NewFeederGatewayDataSource(chain, gw)
		synchronizer = sync.New(chain, dataSource, utils.NewNopZapLogger(), 0, 0, false, testDB)

		b, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		t.Run("pending state shouldnt exist if no pending block", func(t *testing.T) {
			_, _, err = synchronizer.PendingState()
			require.Error(t, err)
		})

		t.Run("cannot store unsupported pending block version", func(t *testing.T) {
			pending := &core.Pending{Block: &core.Block{Header: &core.Header{ProtocolVersion: "1.9.0"}}}
			changed, err := synchronizer.StorePending(pending)
			require.Error(t, err)
			require.False(t, changed)
		})

		t.Run("store genesis as pending", func(t *testing.T) {
			pendingGenesis := &core.Pending{
				Block:       b,
				StateUpdate: su,
			}

			changed, err := synchronizer.StorePending(pendingGenesis)
			require.NoError(t, err)
			require.True(t, changed)

			gotPending, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			expectedPending := core.NewPending(pendingGenesis.Block, pendingGenesis.StateUpdate, nil)
			require.Equal(t, &expectedPending, gotPending)
		})

		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))

		t.Run("storing a pending too far into the future should fail", func(t *testing.T) {
			b, err = gw.BlockByNumber(t.Context(), 2)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 2)
			require.NoError(t, err)

			notExpectedPending := core.NewPending(b, su, nil)
			changed, err := synchronizer.StorePending(&notExpectedPending)
			require.ErrorIs(t, err, blockchain.ErrParentDoesNotMatchHead)
			require.False(t, changed)
		})

		t.Run("store expected pending block", func(t *testing.T) {
			b, err = gw.BlockByNumber(t.Context(), 1)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 1)
			require.NoError(t, err)

			expectedPending := &core.Pending{
				Block:       b,
				StateUpdate: su,
			}
			changed, err := synchronizer.StorePending(expectedPending)
			require.NoError(t, err)
			require.True(t, changed)

			gotPending, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			require.Equal(t, expectedPending, gotPending)
		})

		t.Run("get pending state", func(t *testing.T) {
			_, pendingStateCloser, pErr := synchronizer.PendingState()
			t.Cleanup(func() {
				require.NoError(t, pendingStateCloser())
			})
			require.NoError(t, pErr)
		})
	})

	t.Run("starknet version > 0.14.0", func(t *testing.T) {
		var synchronizer *sync.Synchronizer
		testDB := memory.New()
		chain := blockchain.New(testDB, &utils.Mainnet)
		dataSource := sync.NewFeederGatewayDataSource(chain, gw)
		synchronizer = sync.New(chain, dataSource, utils.NewNopZapLogger(), 0, 0, false, testDB)

		b, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		t.Run("pending state shouldnt exist if no pre_confirmed block", func(t *testing.T) {
			_, _, err = synchronizer.PendingState()
			require.Error(t, err)
		})

		t.Run("cannot store unsupported pre_confirmed block version", func(t *testing.T) {
			preConfirmed := core.PreConfirmed{
				Block: &core.Block{
					Header: &core.Header{ProtocolVersion: "1.9.0"},
				},
			}
			isWritten, err := synchronizer.StorePreConfirmed(&preConfirmed)
			require.Error(t, err)
			require.False(t, isWritten)
		})

		t.Run("store genesis as pre_confirmed", func(t *testing.T) {
			preConfirmedGenesis := core.PreConfirmed{
				Block: b,
				StateUpdate: &core.StateUpdate{
					OldRoot: &felt.Zero,
				},
			}

			isWritten, err := synchronizer.StorePreConfirmed(&preConfirmedGenesis)
			require.NoError(t, err)
			require.True(t, isWritten)

			gotPendingData, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			expectedPreConfirmed := &core.PreConfirmed{
				Block:       preConfirmedGenesis.Block,
				StateUpdate: preConfirmedGenesis.StateUpdate,
			}

			require.Equal(t, expectedPreConfirmed, gotPendingData)
		})

		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))

		t.Run("store expected pre_confirmed block", func(t *testing.T) {
			preConfirmedB, err := gw.BlockByNumber(t.Context(), 1)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 1)
			require.NoError(t, err)

			emptyStateDiff := core.EmptyStateDiff()
			expectedPreConfirmed := &core.PreConfirmed{
				Block: preConfirmedB,
				StateUpdate: &core.StateUpdate{
					StateDiff: &emptyStateDiff,
				},
			}

			emptyStateDiff = core.EmptyStateDiff()
			isWritten, err := synchronizer.StorePreConfirmed(expectedPreConfirmed)
			require.NoError(t, err)
			require.True(t, isWritten)
			gotPendingData, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			require.Equal(t, expectedPreConfirmed, gotPendingData)
		})

		t.Run("get pending state", func(t *testing.T) {
			_, pendingStateCloser, pErr := synchronizer.PendingState()
			require.NoError(t, pErr)
			t.Cleanup(func() {
				require.NoError(t, pendingStateCloser())
			})
		})

		t.Run("get pending state before index", func(t *testing.T) {
			contractAddress, err := new(felt.Felt).SetString("0xFFFFF")
			require.NoError(t, err)
			storageKey := &felt.One

			t.Run("Without Prelatest", func(t *testing.T) {
				numTxs := 10
				preConfirmed := makePreConfirmedWithIncrementingCounter(
					1,
					numTxs,
					contractAddress,
					storageKey,
					0,
				)
				// Set ParentHash to the head block hash
				head, err := chain.HeadsHeader()
				require.NoError(t, err)
				preConfirmed.Block.ParentHash = head.Hash

				isWritten, err := synchronizer.StorePreConfirmed(preConfirmed)
				require.NoError(t, err)
				require.True(t, isWritten)

				for i := range numTxs {
					pendingData, pErr := synchronizer.PendingData()
					require.NoError(t, pErr)
					baseState, stateCloser, err := chain.StateAtBlockHash(preConfirmed.Block.ParentHash)
					require.NoError(t, err)
					pendingState, err := pendingData.PendingStateBeforeIndex(i+1, baseState)
					require.NoError(t, err)
					val, err := pendingState.ContractStorage(contractAddress, storageKey)
					require.NoError(t, err)
					expected := new(felt.Felt).SetUint64(uint64(i) + 1)
					require.Equal(t, expected, val)
					require.NoError(t, stateCloser())
				}
			})

			t.Run("With Prelatest", func(t *testing.T) {
				storageKey2 := new(felt.Felt).SetUint64(11)
				val2 := new(felt.Felt).SetUint64(15)
				preLatestStateDiff := core.EmptyStateDiff()

				preLatestStateDiff.StorageDiffs[*contractAddress] = map[felt.Felt]*felt.Felt{
					*storageKey2: val2,
				}

				preLatestHash := new(felt.Felt).SetUint64(999) // Mock hash for preLatest
				preLatest := core.PreLatest{
					Block: &core.Block{
						Header: &core.Header{
							Number:     b.Number + 1,
							ParentHash: b.Hash,
							Hash:       preLatestHash,
						},
					},
					StateUpdate: &core.StateUpdate{
						StateDiff: &preLatestStateDiff,
					},
				}

				numTxs := 11
				preConfirmed := makePreConfirmedWithIncrementingCounter(
					preLatest.Block.Number+1,
					numTxs,
					contractAddress,
					storageKey,
					0,
				)
				// Set ParentHash to the preLatest block hash
				preConfirmed.Block.ParentHash = preLatest.Block.Hash
				preConfirmed.WithPreLatest(&preLatest)
				isWritten, err := synchronizer.StorePreConfirmed(preConfirmed)
				require.NoError(t, err)
				require.True(t, isWritten)
				for i := range numTxs {
					pendingData, pErr := synchronizer.PendingData()
					require.NoError(t, pErr)
					baseState, stateCloser, err := chain.StateAtBlockHash(preLatest.Block.ParentHash)
					require.NoError(t, err)
					pendingState, err := pendingData.PendingStateBeforeIndex(i+1, baseState)
					require.NoError(t, err)
					val, err := pendingState.ContractStorage(contractAddress, storageKey)
					require.NoError(t, err)
					expected := new(felt.Felt).SetUint64(uint64(i) + 1)
					require.Equal(t, expected, val)

					val, err = pendingState.ContractStorage(contractAddress, storageKey2)
					require.NoError(t, err)
					require.Equal(t, val2, val)
					require.NoError(t, stateCloser())
				}
			})
		})
	})
}

func makePreConfirmedWithIncrementingCounter(
	blockNumber uint64,
	numTxs int,
	contractAddr *felt.Felt,
	storageKey *felt.Felt,
	startingNonce uint64,
) *core.PreConfirmed {
	transactions := make([]core.Transaction, numTxs)
	receipts := make([]*core.TransactionReceipt, numTxs)
	stateDiffs := make([]*core.StateDiff, numTxs)

	for i := range numTxs {
		transactions[i] = &core.InvokeTransaction{}
		receipts[i] = &core.TransactionReceipt{}

		// Increment counter value: i+1 (1 for first tx, 2 for second, etc)
		counterVal := new(felt.Felt).SetUint64(uint64(i + 1))

		// Increment nonce: startingNonce + i + 1
		nonceVal := new(felt.Felt).SetUint64(startingNonce + uint64(i) + 1)

		// Compose storage diffs map for this tx
		storageDiffForContract := map[felt.Felt]*felt.Felt{
			*storageKey: counterVal,
		}

		stateDiff := &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*contractAddr: storageDiffForContract,
			},
			Nonces: map[felt.Felt]*felt.Felt{
				*contractAddr: nonceVal,
			},
			DeployedContracts: make(map[felt.Felt]*felt.Felt, 0),
			DeclaredV0Classes: make([]*felt.Felt, 0),
			DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, 0),
			ReplacedClasses:   make(map[felt.Felt]*felt.Felt, 0),
		}

		stateDiffs[i] = stateDiff
	}

	block := &core.Block{
		Header: &core.Header{
			Number:           blockNumber,
			TransactionCount: uint64(numTxs),
		},
		Transactions: transactions,
		Receipts:     receipts,
	}

	aggregatedStateDiff := core.EmptyStateDiff()
	for _, stateDiff := range stateDiffs {
		aggregatedStateDiff.Merge(stateDiff)
	}

	return &core.PreConfirmed{
		Block:                 block,
		TransactionStateDiffs: stateDiffs,
		StateUpdate: &core.StateUpdate{
			StateDiff: &aggregatedStateDiff,
		},
	}
}
