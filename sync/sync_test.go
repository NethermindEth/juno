package sync_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const timeout = time.Second

func TestSyncBlocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	testBlockchain := func(t *testing.T, bc *blockchain.Blockchain) {
		t.Helper()
		assert.NoError(t, func() error {
			headBlock, err := bc.Head()
			require.NoError(t, err)

			height := int(headBlock.Number)
			assert.Equal(t, 2, height)
			for height >= 0 {
				b, err := gw.BlockByNumber(t.Context(), uint64(height))
				if err != nil {
					return err
				}

				block, err := bc.BlockByNumber(uint64(height))
				require.NoError(t, err)

				assert.Equal(t, b, block)
				height--
			}
			return nil
		}())
	}
	log := utils.NewNopZapLogger()
	t.Run("sync multiple blocks in an empty db", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
		dataSource := sync.NewFeederGatewayDataSource(bc, gw)
		synchronizer := sync.New(bc, dataSource, log, time.Duration(0), time.Duration(0), false, testDB)
		ctx, cancel := context.WithTimeout(t.Context(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
		b0, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, &core.BlockCommitments{}, s0, nil))

		dataSource := sync.NewFeederGatewayDataSource(bc, gw)
		synchronizer := sync.New(bc, dataSource, log, time.Duration(0), time.Duration(0), false, testDB)
		ctx, cancel := context.WithTimeout(t.Context(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks, with an unreliable gw", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())

		mockSNData := mocks.NewMockStarknetData(mockCtrl)

		syncingHeight := uint64(0)
		reqCount := 0
		mockSNData.EXPECT().StateUpdateWithBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, height uint64) (*core.StateUpdate, *core.Block, error) {
			curHeight := atomic.LoadUint64(&syncingHeight)
			// reject any other requests
			if height != curHeight {
				return nil, nil, errors.New("try again")
			}

			reqCount++
			state, block, err := gw.StateUpdateWithBlock(t.Context(), curHeight)
			if err != nil {
				return nil, nil, err
			}

			switch reqCount {
			case 1:
				return nil, nil, errors.New("try again")
			case 2:
				state.BlockHash = new(felt.Felt) // fail sanity checks
			case 3:
				state.OldRoot = new(felt.Felt).SetUint64(1) // fail store
			default:
				reqCount = 0
				atomic.AddUint64(&syncingHeight, 1)
			}

			return state, block, nil
		}).AnyTimes()
		mockSNData.EXPECT().Class(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash *felt.Felt) (core.Class, error) {
			return gw.Class(ctx, hash)
		}).AnyTimes()

		mockSNData.EXPECT().BlockLatest(gomock.Any()).DoAndReturn(func(ctx context.Context) (*core.Block, error) {
			return gw.BlockLatest(t.Context())
		}).AnyTimes()

		dataSource := sync.NewFeederGatewayDataSource(bc, mockSNData)
		synchronizer := sync.New(bc, dataSource, log, time.Duration(0), time.Duration(0), false, testDB)
		ctx, cancel := context.WithTimeout(t.Context(), 2*timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}

func TestReorg(t *testing.T) {
	mainClient := feeder.NewTestClient(t, &utils.Mainnet)
	mainGw := adaptfeeder.New(mainClient)

	sepoliaClient := feeder.NewTestClient(t, &utils.Sepolia)
	sepoliaGw := adaptfeeder.New(sepoliaClient)

	testDB := memory.New()

	// sync to Sepolia for 2 blocks
	bc := blockchain.New(testDB, &utils.Sepolia, statetestutils.UseNewState())
	dataSource := sync.NewFeederGatewayDataSource(bc, sepoliaGw)
	synchronizer := sync.New(bc, dataSource, utils.NewNopZapLogger(), 0, 0, false, testDB)

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	require.NoError(t, bc.Stop())

	t.Run("resync to mainnet with the same db", func(t *testing.T) {
		bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())

		// Ensure current head is Sepolia head
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x5c627d4aeb51280058bed93c7889bce78114d63baad1be0f0aeb32496d5f19c"), head.Hash)
		sepoliaEnd := head
		sepoliaStart, err := bc.BlockHeaderByNumber(0)
		require.NoError(t, err)

		dataSource := sync.NewFeederGatewayDataSource(bc, mainGw)
		synchronizer = sync.New(bc, dataSource, utils.NewNopZapLogger(), 0, 0, false, testDB)
		sub := synchronizer.SubscribeReorg()
		ctx, cancel = context.WithTimeout(t.Context(), timeout)
		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		// After syncing (and reorging) the current head should be at mainnet
		head, err = bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"), head.Hash)

		// Validate reorg event
		got, ok := <-sub.Recv()
		require.True(t, ok)
		assert.Equal(t, sepoliaEnd.Hash, got.EndBlockHash)
		assert.Equal(t, sepoliaEnd.Number, got.EndBlockNum)
		assert.Equal(t, sepoliaStart.Hash, got.StartBlockHash)
		assert.Equal(t, sepoliaStart.Number, got.StartBlockNum)
	})
}

func TestPendingData(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	t.Run("starknet version <= 0.14.0", func(t *testing.T) {
		var synchronizer *sync.Synchronizer
		testDB := memory.New()
		chain := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
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
			assert.Equal(t, &expectedPending, gotPending)
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
			assert.Equal(t, expectedPending, gotPending)
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
		chain := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
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
			preConfirmed := core.PreConfirmed{Block: &core.Block{Header: &core.Header{ProtocolVersion: "1.9.0"}}}
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

			assert.Equal(t, expectedPreConfirmed, gotPendingData)
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
			assert.Equal(t, expectedPreConfirmed, gotPendingData)
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
				isWritten, err := synchronizer.StorePreConfirmed(preConfirmed)
				require.NoError(t, err)
				require.True(t, isWritten)

				for i := range numTxs {
					pendingState, pendingStateCloser, pErr := synchronizer.PendingStateBeforeIndex(i + 1)
					require.NoError(t, pErr)
					val, err := pendingState.ContractStorage(contractAddress, storageKey)
					require.NoError(t, err)
					expected := new(felt.Felt).SetUint64(uint64(i) + 1)
					require.Equal(t, *expected, val)
					require.NoError(t, pendingStateCloser())
				}
			})

			t.Run("With Prelatest", func(t *testing.T) {
				storageKey2 := new(felt.Felt).SetUint64(11)
				val2 := new(felt.Felt).SetUint64(15)
				preLatestStateDiff := core.EmptyStateDiff()

				preLatestStateDiff.StorageDiffs[*contractAddress] = map[felt.Felt]*felt.Felt{
					*storageKey2: val2,
				}

				preLatest := core.PreLatest{
					Block: &core.Block{
						Header: &core.Header{
							Number:     b.Number + 1,
							ParentHash: b.Hash,
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
				preConfirmed.WithPreLatest(&preLatest)
				isWritten, err := synchronizer.StorePreConfirmed(preConfirmed)
				require.NoError(t, err)
				require.True(t, isWritten)
				for i := range numTxs {
					pendingState, pendingStateCloser, pErr := synchronizer.PendingStateBeforeIndex(i + 1)
					require.NoError(t, pErr)
					val, err := pendingState.ContractStorage(contractAddress, storageKey)
					require.NoError(t, err)
					expected := new(felt.Felt).SetUint64(uint64(i) + 1)
					require.Equal(t, *expected, val)

					val, err = pendingState.ContractStorage(contractAddress, storageKey2)
					require.NoError(t, err)
					require.Equal(t, *val2, val)
					require.NoError(t, pendingStateCloser())
				}
			})
		})
	})
}

func TestSubscribeNewHeads(t *testing.T) {
	t.Parallel()
	testDB := memory.New()
	log := utils.NewNopZapLogger()
	network := utils.Mainnet
	chain := blockchain.New(testDB, &network, statetestutils.UseNewState())
	feeder := feeder.NewTestClient(t, &network)
	gw := adaptfeeder.New(feeder)
	dataSource := sync.NewFeederGatewayDataSource(chain, gw)
	syncer := sync.New(chain, dataSource, log, 0, 0, false, testDB)

	sub := syncer.SubscribeNewHeads()

	// Receive on new block.
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	require.NoError(t, syncer.Run(ctx))
	cancel()
	got, ok := <-sub.Recv()
	require.True(t, ok)
	want, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	require.Equal(t, want, got)
	sub.Unsubscribe()
}

func TestSubscribePending(t *testing.T) {
	t.Parallel()

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := memory.New()
	log := utils.NewNopZapLogger()
	bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
	dataSource := sync.NewFeederGatewayDataSource(bc, gw)
	synchronizer := sync.New(
		bc,
		dataSource,
		log,
		time.Millisecond*100,
		time.Millisecond*100,
		false,
		testDB,
	)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)

	sub := synchronizer.SubscribePendingData()

	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	pendingData, err := synchronizer.PendingData()
	require.NoError(t, err)
	select {
	case pendingBlock, ok := <-sub.Recv():
		require.True(t, ok)
		require.Equal(t, pendingData, pendingBlock)
	case <-time.After(time.Second):
		t.Fatal("expected pending data")
	}

	sub.Unsubscribe()
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
