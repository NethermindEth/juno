package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const timeout = 10 * time.Second

func TestSyncBlocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()

	testBlockchain := func(t *testing.T, bc *blockchain.Blockchain) {
		t.Helper()
		assert.NoError(t, func() error {
			for blockNumber := 0; blockNumber <= 2; blockNumber++ {
				b, err := gw.BlockByNumber(context.Background(), uint64(blockNumber))
				if err != nil {
					return err
				}

				block, err := bc.BlockByNumber(uint64(blockNumber))
				require.NoError(t, err)

				assert.Equal(t, b.Header, block.Header)
				assert.Equal(t, b.Transactions, block.Transactions)
			}
			return nil
		}())
	}

	t.Run("sync multiple blocks in an empty db", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		bc := blockchain.New(testDB, &utils.Mainnet)
		synchronizer := sync.New(bc, gw, log, time.Duration(0), false)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		testBlockchain(t, bc)
	})
}
