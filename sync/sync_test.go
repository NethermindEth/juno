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
	"github.com/NethermindEth/juno/db/pebble"
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
				b, err := gw.BlockByNumber(context.Background(), uint64(height))
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
		testDB := pebble.NewMemTest(t)
		bc := blockchain.New(testDB, &utils.Mainnet)
		synchronizer := sync.New(bc, gw, log, time.Duration(0), false)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		bc := blockchain.New(testDB, &utils.Mainnet)
		b0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, &core.BlockCommitments{}, s0, nil))

		synchronizer := sync.New(bc, gw, log, time.Duration(0), false)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks, with an unreliable gw", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		bc := blockchain.New(testDB, &utils.Mainnet)

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
			state, block, err := gw.StateUpdateWithBlock(context.Background(), curHeight)
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
			return gw.BlockLatest(context.Background())
		}).AnyTimes()

		synchronizer := sync.New(bc, mockSNData, log, time.Duration(0), false)
		ctx, cancel := context.WithTimeout(context.Background(), 2*timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}

func TestReorg(t *testing.T) {
	mainClient := feeder.NewTestClient(t, &utils.Mainnet)
	mainGw := adaptfeeder.New(mainClient)

	integClient := feeder.NewTestClient(t, &utils.Integration)
	integGw := adaptfeeder.New(integClient)

	testDB := pebble.NewMemTest(t)

	// sync to integration for 2 blocks
	bc := blockchain.New(testDB, &utils.Integration)
	synchronizer := sync.New(bc, integGw, utils.NewNopZapLogger(), 0, false)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	t.Run("resync to mainnet with the same db", func(t *testing.T) {
		bc := blockchain.New(testDB, &utils.Mainnet)

		// Ensure current head is Integration head
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, utils.HexToFelt(t, "0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86"), head.Hash)
		integEnd := head
		integStart, err := bc.BlockHeaderByNumber(0)
		require.NoError(t, err)

		synchronizer = sync.New(bc, mainGw, utils.NewNopZapLogger(), 0, false)
		sub := synchronizer.SubscribeReorg()
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		// After syncing (and reorging) the current head should be at mainnet
		head, err = bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"), head.Hash)

		// Validate reorg event
		got, ok := <-sub.Recv()
		require.True(t, ok)
		assert.Equal(t, integEnd.Hash, got.EndBlockHash)
		assert.Equal(t, integEnd.Number, got.EndBlockNum)
		assert.Equal(t, integStart.Hash, got.StartBlockHash)
		assert.Equal(t, integStart.Number, got.StartBlockNum)
	})
}

func TestPending(t *testing.T) {
	t.Parallel()

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest(t)
	log := utils.NewNopZapLogger()
	bc := blockchain.New(testDB, &utils.Mainnet)
	synchronizer := sync.New(bc, gw, log, time.Millisecond*100, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	head, err := bc.HeadsHeader()
	require.NoError(t, err)
	pending, err := bc.Pending()
	require.NoError(t, err)
	assert.Equal(t, head.Hash, pending.Block.ParentHash)
}

func TestSubscribeNewHeads(t *testing.T) {
	t.Parallel()
	testDB := pebble.NewMemTest(t)
	log := utils.NewNopZapLogger()
	integration := utils.Integration
	chain := blockchain.New(testDB, &integration)
	integrationClient := feeder.NewTestClient(t, &integration)
	gw := adaptfeeder.New(integrationClient)
	syncer := sync.New(chain, gw, log, 0, false)

	sub := syncer.SubscribeNewHeads()

	// Receive on new block.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	require.NoError(t, syncer.Run(ctx))
	cancel()
	got, ok := <-sub.Recv()
	require.True(t, ok)
	want, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, want.Header, got)
	sub.Unsubscribe()
}
