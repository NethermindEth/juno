package sync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncBlocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
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
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET)
		synchronizer := New(bc, gw, log)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET)
		b0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)
		s0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b0, s0, nil))

		synchronizer := New(bc, gw, log)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})

	t.Run("sync multiple blocks, with an unreliable gw", func(t *testing.T) {
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET)

		mockSNData := mocks.NewMockStarknetData(mockCtrl)

		var rwMutex sync.RWMutex
		syncingHeight := uint64(0)

		mockSNData.EXPECT().BlockByNumber(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, height uint64) (*core.Block, error) {
			rwMutex.RLock()
			defer rwMutex.RUnlock()

			// reject any other requests
			if height != syncingHeight {
				return nil, errors.New("try again")
			}
			return gw.BlockByNumber(context.Background(), syncingHeight)
		}).AnyTimes()

		reqCount := 0
		mockSNData.EXPECT().StateUpdate(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, height uint64) (*core.StateUpdate, error) {
			rwMutex.RLock()

			// reject any other requests
			if height != syncingHeight {
				return nil, errors.New("try again")
			}

			reqCount++
			ret, err := gw.StateUpdate(context.Background(), syncingHeight)
			require.NoError(t, err)
			rwMutex.RUnlock()

			if reqCount == 1 {
				return nil, errors.New("try again")
			} else if reqCount == 2 {
				ret.BlockHash = new(felt.Felt) // fail sanity checks
			} else if reqCount == 3 {
				ret.OldRoot = new(felt.Felt).SetUint64(1) // fail store
			} else {
				reqCount = 0
				rwMutex.Lock()
				syncingHeight++
				rwMutex.Unlock()
			}

			return ret, nil
		}).AnyTimes()
		mockSNData.EXPECT().Class(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash *felt.Felt) (core.Class, error) {
			return gw.Class(ctx, hash)
		}).AnyTimes()

		synchronizer := New(bc, mockSNData, log)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}
