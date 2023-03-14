package sync

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncBlocks(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()
	gw := adaptfeeder.New(client)
	testBlockchain := func(t *testing.T, bc *blockchain.Blockchain) bool {
		return assert.NoError(t, func() error {
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
		ctx, _ := context.WithTimeout(context.Background(), time.Second)

		require.NoError(t, synchronizer.Run(ctx))

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
		ctx, _ := context.WithTimeout(context.Background(), time.Second)

		require.NoError(t, synchronizer.Run(ctx))

		testBlockchain(t, bc)
	})
}
