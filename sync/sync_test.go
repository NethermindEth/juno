package sync_test

import (
	"context"
	"errors"
	"os"
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

func TestMain(m *testing.M) {
	statetestutils.Parse()
	os.Exit(m.Run())
}

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
		require.Equal(t, utils.HexToFelt(t, "0x5c627d4aeb51280058bed93c7889bce78114d63baad1be0f0aeb32496d5f19c"), head.Hash)
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
		require.Equal(t, utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"), head.Hash)

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
			_, err = synchronizer.PendingState()
			require.Error(t, err)
		})

		t.Run("cannot store unsupported pending block version", func(t *testing.T) {
			pending := &sync.Pending{Block: &core.Block{Header: &core.Header{ProtocolVersion: "1.9.0"}}}
			require.Error(t, synchronizer.StorePending(pending))
		})

		t.Run("store genesis as pending", func(t *testing.T) {
			pendingGenesis := &sync.Pending{
				Block:       b,
				StateUpdate: su,
			}

			require.NoError(t, synchronizer.StorePending(pendingGenesis))

			gotPending, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			expectedPending := sync.NewPending(pendingGenesis.Block, pendingGenesis.StateUpdate, nil)
			assert.Equal(t, &expectedPending, gotPending)
		})

		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))

		t.Run("storing a pending too far into the future should fail", func(t *testing.T) {
			b, err = gw.BlockByNumber(t.Context(), 2)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 2)
			require.NoError(t, err)

			notExpectedPending := sync.NewPending(b, su, nil)

			require.ErrorIs(t, synchronizer.StorePending(&notExpectedPending), blockchain.ErrParentDoesNotMatchHead)
		})

		t.Run("store expected pending block", func(t *testing.T) {
			b, err = gw.BlockByNumber(t.Context(), 1)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 1)
			require.NoError(t, err)

			expectedPending := &sync.Pending{
				Block:       b,
				StateUpdate: su,
			}

			require.NoError(t, synchronizer.StorePending(expectedPending))

			gotPending, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			assert.Equal(t, expectedPending, gotPending)
		})

		t.Run("get pending state", func(t *testing.T) {
			_, pErr := synchronizer.PendingState()
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
			_, err = synchronizer.PendingState()
			require.Error(t, err)
		})

		t.Run("cannot store unsupported pre_confirmed block version", func(t *testing.T) {
			preConfirmed := &core.PreConfirmed{Block: &core.Block{Header: &core.Header{ProtocolVersion: "1.9.0"}}}
			require.Error(t, synchronizer.StorePreConfirmed(preConfirmed))
		})

		t.Run("store genesis as pre_confirmed", func(t *testing.T) {
			preConfirmedGenesis := &core.PreConfirmed{
				Block: b,
				StateUpdate: &core.StateUpdate{
					OldRoot: &felt.Zero,
				},
			}

			require.NoError(t, synchronizer.StorePreConfirmed(preConfirmedGenesis))

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
			b, err = gw.BlockByNumber(t.Context(), 1)
			require.NoError(t, err)
			su, err = gw.StateUpdate(t.Context(), 1)
			require.NoError(t, err)
			head, err := chain.HeadsHeader()
			require.NoError(t, err)

			expectedPreConfirmed := &core.PreConfirmed{
				Block: b,
				StateUpdate: &core.StateUpdate{
					OldRoot: head.GlobalStateRoot,
				},
			}

			require.NoError(t, synchronizer.StorePreConfirmed(expectedPreConfirmed))

			gotPendingData, pErr := synchronizer.PendingData()
			require.NoError(t, pErr)
			assert.Equal(t, expectedPreConfirmed, gotPendingData)
		})

		t.Run("get pending state", func(t *testing.T) {
			_, pErr := synchronizer.PendingState()
			require.NoError(t, pErr)
		})

		t.Run("get pending state before index", func(t *testing.T) {
			var synchronizer *sync.Synchronizer
			testDB := memory.New()
			chain := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())
			dataSource := sync.NewFeederGatewayDataSource(chain, gw)
			synchronizer = sync.New(chain, dataSource, utils.NewNopZapLogger(), 0, 0, false, testDB)

			require.NoError(t, err)
			client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
			gw := adaptfeeder.New(client)
			preConfirmed, err := gw.PreConfirmedBlockByNumber(t.Context(), 1204672)
			preConfirmed.StateUpdate.OldRoot = &felt.Zero
			preConfirmed.Block.Number = 0
			require.NoError(t, err)
			require.NoError(t, synchronizer.StorePreConfirmed(&preConfirmed))
			txCount := len(preConfirmed.GetTransactions())

			pendingState, pErr := synchronizer.PendingStateBeforeIndex(txCount - 1)
			require.NoError(t, pErr)

			// Check storage value in two different index
			// See clients/feeder/testdata/sepolia-integration/pre_confirmed/1204672.json
			contractAddress, err := new(felt.Felt).SetString("0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
			require.NoError(t, err)
			key, err := new(felt.Felt).SetString("0x5496768776e3db30053404f18067d81a6e06f5a2b0de326e21298fd9d569a9a")
			require.NoError(t, err)
			val, err := pendingState.ContractStorage(contractAddress, key)
			require.NoError(t, err)
			expectedVal, err := new(felt.Felt).SetString("0x1d057bfbd3cadebffd74")
			require.NoError(t, err)
			require.Equal(t, *expectedVal, val)

			pendingState, pErr = synchronizer.PendingStateBeforeIndex(txCount)
			require.NoError(t, pErr)
			contractAddress, err = new(felt.Felt).SetString("0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
			require.NoError(t, err)
			key, err = new(felt.Felt).SetString("0x5496768776e3db30053404f18067d81a6e06f5a2b0de326e21298fd9d569a9a")
			require.NoError(t, err)
			val, err = pendingState.ContractStorage(contractAddress, key)
			require.NoError(t, err)
			expectedVal, err = new(felt.Felt).SetString("0x1d057bfbd3df63f5dd54")
			require.NoError(t, err)
			require.Equal(t, *expectedVal, val)
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
	pendingBlock, ok := <-sub.Recv()
	require.True(t, ok)
	require.Equal(t, pendingData, pendingBlock)
	sub.Unsubscribe()
}
