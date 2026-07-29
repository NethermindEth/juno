package sync_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const timeout = time.Second

func TestSyncBlocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, &networks.Mainnet)
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
	logger := log.NewNopZapLogger()
	t.Run("sync multiple blocks in an empty db", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(
			testDB,
			&networks.Mainnet,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)
		dataSource := sync.NewFeederGatewayDataSource(bc, gw)
		synchronizer := sync.New(
			bc,
			dataSource,
			logger,
			time.Duration(0),
			false,
			testDB,
		)
		ctx, cancel := context.WithTimeout(t.Context(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(
			testDB,
			&networks.Mainnet,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)
		b0, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, &core.BlockCommitments{}, s0, nil))

		dataSource := sync.NewFeederGatewayDataSource(bc, gw)
		synchronizer := sync.New(
			bc,
			dataSource,
			logger,
			time.Duration(0),
			false,
			testDB,
		)
		ctx, cancel := context.WithTimeout(t.Context(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks, with an unreliable gw", func(t *testing.T) {
		testDB := memory.New()
		bc := blockchain.New(
			testDB,
			&networks.Mainnet,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		mockSNData := mocks.NewMockStarknetData(mockCtrl)

		syncingHeight := uint64(0)
		reqCount := 0
		mockSNData.EXPECT().StateUpdateWithBlock(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, height uint64) (*core.StateUpdate, *core.Block, error) {
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
		mockSNData.EXPECT().Class(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, hash *felt.Felt) (core.ClassDefinition, error) {
				return gw.Class(ctx, hash)
			}).AnyTimes()

		mockSNData.EXPECT().BlockHeaderLatest(gomock.Any()).DoAndReturn(
			func(ctx context.Context) (core.Header, error) {
				block, err := gw.BlockLatest(t.Context())
				if err != nil {
					return core.Header{}, err
				}
				return *block.Header, nil
			}).AnyTimes()

		dataSource := sync.NewFeederGatewayDataSource(bc, mockSNData)
		synchronizer := sync.New(
			bc,
			dataSource,
			logger,
			time.Duration(0),
			false,
			testDB,
		)
		ctx, cancel := context.WithTimeout(t.Context(), 2*timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}

func TestStartingBlockHeaderFallsBackToBlockchain(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	startingHeader := &core.Header{Number: 1, Hash: felt.NewFromUint64[felt.Felt](1)}
	require.NoError(t, core.WriteChainHeight(testDB, 0))
	require.NoError(t, core.WriteBlockHeaderByNumber(testDB, startingHeader))

	dataSource := newTestBlockDataSource()
	dataSource.setBlocks([]sync.CommittedBlock{
		{Block: &core.Block{Header: &core.Header{Number: 0}}},
		{Block: &core.Block{Header: startingHeader}},
		{Block: &core.Block{Header: &core.Header{Number: 2, Hash: felt.NewFromUint64[felt.Felt](2)}}},
	})

	synchronizer := sync.New(
		bc,
		dataSource,
		log.NewNopZapLogger(),
		time.Duration(0),
		true,
		testDB,
	)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- synchronizer.Run(ctx)
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		header, err := synchronizer.StartingBlockHeader()
		assert.NoError(c, err)
		assert.Equal(c, startingHeader, header)
	}, timeout, 10*time.Millisecond)

	require.NoError(t, core.DeleteBlockHeaderByNumber(testDB, startingHeader.Number))
	header, err := synchronizer.StartingBlockHeader()
	require.NoError(t, err)
	require.Equal(t, startingHeader, header)

	cancel()
	require.NoError(t, <-done)
}

func TestStartingBlockHeaderCachesStoredHeader(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)
	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	dataSource := sync.NewFeederGatewayDataSource(bc, gw)
	synchronizer := sync.New(
		bc,
		dataSource,
		log.NewNopZapLogger(),
		time.Duration(0),
		false,
		testDB,
	)

	storedStartingBlock := make(chan struct{}, 1)
	synchronizer.WithListener(&sync.SelectiveListener{
		OnSyncStepDoneCb: func(op string, blockNum uint64, took time.Duration) {
			if op == sync.OpStore && blockNum == 0 {
				storedStartingBlock <- struct{}{}
			}
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- synchronizer.Run(ctx)
	}()

	select {
	case <-storedStartingBlock:
	case <-time.After(timeout):
		t.Fatal("starting block was not stored")
	}

	require.NoError(t, core.DeleteBlockHeaderByNumber(testDB, block0.Number))
	header, err := synchronizer.StartingBlockHeader()
	require.NoError(t, err)
	require.Equal(t, block0.Header, header)

	cancel()
	require.NoError(t, <-done)
}

func TestStartingBlockHeaderNotRunning(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	synchronizer := sync.New(
		bc,
		newTestBlockDataSource(),
		log.NewNopZapLogger(),
		time.Duration(0),
		true,
		testDB,
	)

	header, err := synchronizer.StartingBlockHeader()
	require.Error(t, err)
	require.Nil(t, header)
}

func TestStartingBlockHeaderFallbackUnavailable(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	dataSource := newTestBlockDataSource()
	dataSource.setBlocks([]sync.CommittedBlock{
		{Block: &core.Block{Header: &core.Header{Number: 0, Hash: felt.NewFromUint64[felt.Felt](0)}}},
	})
	synchronizer := sync.New(
		bc,
		dataSource,
		log.NewNopZapLogger(),
		time.Duration(0),
		true,
		testDB,
	)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- synchronizer.Run(ctx)
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotNil(c, synchronizer.HighestBlockHeader())
	}, timeout, 10*time.Millisecond)

	header, err := synchronizer.StartingBlockHeader()
	require.Error(t, err)
	require.Nil(t, header)

	cancel()
	require.NoError(t, <-done)
}

func TestReorg(t *testing.T) {
	mainClient := feeder.NewTestClient(t, &networks.Mainnet)
	mainGw := adaptfeeder.New(mainClient)

	sepoliaClient := feeder.NewTestClient(t, &networks.Sepolia)
	sepoliaGw := adaptfeeder.New(sepoliaClient)

	testDB := memory.New()

	// sync to Sepolia for 2 blocks
	bc := blockchain.New(
		testDB,
		&networks.Sepolia,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	dataSource := sync.NewFeederGatewayDataSource(bc, sepoliaGw)
	synchronizer := sync.New(bc, dataSource, log.NewNopZapLogger(), 0, false, testDB)

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	t.Run("resync to mainnet with the same db", func(t *testing.T) {
		bc := blockchain.New(
			testDB,
			&networks.Mainnet,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		// Ensure current head is Sepolia head
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x5c627d4aeb51280058bed93c7889bce78114d63baad1be0f0aeb32496d5f19c"), head.Hash)
		sepoliaEnd := head
		sepoliaStart, err := bc.BlockHeaderByNumber(0)
		require.NoError(t, err)

		dataSource := sync.NewFeederGatewayDataSource(bc, mainGw)
		synchronizer = sync.New(bc, dataSource, log.NewNopZapLogger(), 0, false, testDB)
		sub := synchronizer.SubscribeReorg()
		// Use a generous timeout with early cancellation once the expected block is stored.
		// The reorg flow (detect mismatch → revert → re-sync 3 blocks) needs more than 1s on slow CI.
		ctx, cancel = context.WithTimeout(t.Context(), 30*time.Second)
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if h, err := bc.Height(); err == nil && h >= 2 {
						cancel()
						return
					}
				}
			}
		}()
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

func TestSubscribeNewHeads(t *testing.T) {
	t.Parallel()
	testDB := memory.New()
	logger := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		testDB,
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	feeder := feeder.NewTestClient(t, &network)
	gw := adaptfeeder.New(feeder)
	dataSource := sync.NewFeederGatewayDataSource(chain, gw)
	syncer := sync.New(chain, dataSource, logger, 0, false, testDB)

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

func TestPreConfirmed(t *testing.T) {
	t.Parallel()
	logger := log.NewNopZapLogger()
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)

	// The stored-snapshot fast path is covered by
	// TestPreConfirmedChainReturnsStoredSnapshot (preconfirmed_chain_test.go),
	// which fills the storage through Run's pre-confirmed poller.

	t.Run("Returns empty pre_confirmed when nothing stored", func(t *testing.T) {
		t.Parallel()
		testDB := memory.New()
		bc := blockchain.New(
			testDB,
			&networks.Mainnet,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)
		b0, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, &core.BlockCommitments{}, s0, nil))

		synchronizer := sync.New(bc, nil, logger, 0, false, testDB)
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		result, err := synchronizer.PreConfirmedChain()
		require.NoError(t, err)
		require.Equal(t, 1, result.Length())
		tip := result.Head()
		require.Equal(t, head.Number+1, tip.Block.Number)
		require.Empty(t, tip.Block.Transactions)
	})
}
