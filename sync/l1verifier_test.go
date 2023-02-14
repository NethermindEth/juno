package sync

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestL1Verifier(t *testing.T) {
	testDB := pebble.NewMemTest()
	bc := blockchain.New(testDB, utils.MAINNET)

	gw, gCloseFn := testsource.NewTestGateway(utils.MAINNET)
	for i := 0; i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(context.Background(), uint64(i))
		require.NoError(t, err)

		require.NoError(t, bc.Store(b, s, nil))
	}
	gCloseFn()

	t.Run("should not verify blocks that are not yet accepted on L1", func(t *testing.T) {
		client, closeFn := testsource.NewTestClient(utils.MAINNET)
		defer closeFn()

		log := utils.NewNopZapLogger()
		l1Verifier := NewL1Verifier(bc, client, log)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()

		assert.NoError(t, l1Verifier.Run(ctx))

		l1ChainHeight, err := bc.L1ChainHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(1), l1ChainHeight)
	})
}
