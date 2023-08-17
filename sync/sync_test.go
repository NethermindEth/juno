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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeout = time.Second

type fakeSubscription struct {
	c <-chan error
}

func (e *fakeSubscription) Err() <-chan error { return e.c }
func (e *fakeSubscription) Unsubscribe()      {}

func TestSyncBlocks(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	l1 := mocks.NewMockL1(mockCtrl)
	l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Return(&fakeSubscription{c: make(<-chan error)}, nil).Times(3)

	client := feeder.NewTestClient(t, utils.MAINNET)
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
		t.Parallel()
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET, log)
		synchronizer := sync.New(bc, l1, gw, log, time.Duration(0))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		t.Parallel()
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET, log)
		b0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, &core.BlockCommitments{}, s0, nil))

		synchronizer := sync.New(bc, l1, gw, log, time.Duration(0))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks, with an unreliable gw", func(t *testing.T) {
		t.Parallel()
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		mockSNData := mocks.NewMockStarknetData(mockCtrl)

		syncingHeight := uint64(0)

		mockSNData.EXPECT().BlockByNumber(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, height uint64) (*core.Block, error) {
			curHeight := atomic.LoadUint64(&syncingHeight)
			// reject any other requests
			if height != curHeight {
				return nil, errors.New("try again")
			}
			return gw.BlockByNumber(context.Background(), syncingHeight)
		}).AnyTimes()

		reqCount := 0
		mockSNData.EXPECT().StateUpdate(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, height uint64) (*core.StateUpdate, error) {
			curHeight := atomic.LoadUint64(&syncingHeight)
			// reject any other requests
			if height != curHeight {
				return nil, errors.New("try again")
			}

			reqCount++
			ret, err := gw.StateUpdate(context.Background(), curHeight)
			require.NoError(t, err)

			switch reqCount {
			case 1:
				return nil, errors.New("try again")
			case 2:
				ret.BlockHash = new(felt.Felt) // fail sanity checks
			case 3:
				ret.OldRoot = new(felt.Felt).SetUint64(1) // fail store
			default:
				reqCount = 0
				atomic.AddUint64(&syncingHeight, 1)
			}

			return ret, nil
		}).AnyTimes()
		mockSNData.EXPECT().Class(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash *felt.Felt) (core.Class, error) {
			return gw.Class(ctx, hash)
		}).AnyTimes()

		mockSNData.EXPECT().BlockLatest(gomock.Any()).DoAndReturn(func(ctx context.Context) (*core.Block, error) {
			return gw.BlockLatest(context.Background())
		}).AnyTimes()

		synchronizer := sync.New(bc, l1, mockSNData, log, time.Duration(0))
		ctx, cancel := context.WithTimeout(context.Background(), 2*timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}

func TestReorg(t *testing.T) {
	t.Parallel()
	mainClient := feeder.NewTestClient(t, utils.MAINNET)
	mainGw := adaptfeeder.New(mainClient)

	integClient := feeder.NewTestClient(t, utils.INTEGRATION)
	integGw := adaptfeeder.New(integClient)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	l1 := mocks.NewMockL1(mockCtrl)
	l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Return(&fakeSubscription{c: make(<-chan error)}, nil).Times(2)

	testDB := pebble.NewMemTest()

	// sync to integration for 2 blocks
	bc := blockchain.New(testDB, utils.INTEGRATION, utils.NewNopZapLogger())
	synchronizer := sync.New(bc, l1, integGw, utils.NewNopZapLogger(), time.Duration(0))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	t.Run("resync to mainnet with the same db", func(t *testing.T) {
		t.Parallel()
		bc = blockchain.New(testDB, utils.MAINNET, utils.NewNopZapLogger())

		// Ensure current head is Integration head
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, utils.HexToFelt(t, "0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86"), head.Hash)

		synchronizer = sync.New(bc, l1, mainGw, utils.NewNopZapLogger(), time.Duration(0))
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		// After syncing (and reorging) the current head should be at mainnet
		head, err = bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"), head.Hash)
	})
}

func TestPending(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	l1 := mocks.NewMockL1(mockCtrl)
	l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Return(&fakeSubscription{c: make(<-chan error)}, nil).Times(1)

	client := feeder.NewTestClient(t, utils.MAINNET)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest()
	log := utils.NewNopZapLogger()
	bc := blockchain.New(testDB, utils.MAINNET, log)
	synchronizer := sync.New(bc, l1, gw, log, time.Millisecond*100)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	head, err := bc.HeadsHeader()
	require.NoError(t, err)
	pending, err := bc.Pending()
	require.NoError(t, err)
	assert.Equal(t, head.Hash, pending.Block.ParentHash)
}

func TestL1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	testErr := errors.New("test err")

	t.Run("failed subscription", func(t *testing.T) {
		t.Parallel()
		client := feeder.NewTestClient(t, utils.INTEGRATION)
		gw := adaptfeeder.New(client)
		testDB := pebble.NewMemTest()
		log := utils.NewNopZapLogger()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		l1 := mocks.NewMockL1(mockCtrl)
		errChan := make(chan error, 1)
		t.Cleanup(func() { close(errChan) })
		errChan <- testErr
		l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Return(&fakeSubscription{c: errChan}, nil).Times(1)

		synchronizer := sync.New(bc, l1, gw, log, time.Millisecond*100)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		require.ErrorIs(t, synchronizer.Run(ctx), testErr)
		cancel()
	})

	t.Run("failed subscription attempt", func(t *testing.T) {
		t.Parallel()
		client := feeder.NewTestClient(t, utils.INTEGRATION)
		gw := adaptfeeder.New(client)
		testDB := pebble.NewMemTest()
		log := utils.NewNopZapLogger()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		l1 := mocks.NewMockL1(mockCtrl)
		l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Return(nil, testErr).Times(1)

		synchronizer := sync.New(bc, l1, gw, log, time.Millisecond*100)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)
		require.ErrorIs(t, synchronizer.Run(ctx), testErr)
	})

	t.Run("successful subscription", func(t *testing.T) {
		t.Parallel()
		client := feeder.NewTestClient(t, utils.INTEGRATION)
		gw := adaptfeeder.New(client)
		testDB := pebble.NewMemTest()
		log := utils.NewNopZapLogger()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		l1 := mocks.NewMockL1(mockCtrl)
		l1Head := &core.L1Head{
			BlockNumber: 0,
			BlockHash:   new(felt.Felt),
			StateRoot:   new(felt.Felt),
		}

		l1.EXPECT().WatchL1Heads(gomock.Any(), gomock.Any()).Do(func(_ context.Context, l1HeadsChan chan<- *core.L1Head) {
			l1HeadsChan <- l1Head
		}).Return(&fakeSubscription{c: make(<-chan error)}, nil).Times(1)

		synchronizer := sync.New(bc, l1, gw, log, time.Millisecond*100)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)
		require.NoError(t, synchronizer.Run(ctx))
		got, err := bc.L1Head()
		require.NoError(t, err)
		assert.Equal(t, l1Head, got)
	})
}
