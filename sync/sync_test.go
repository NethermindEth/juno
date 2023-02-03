package sync

import (
	_ "embed"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestSyncBlocks(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()
	testBlockchain := func(t *testing.T, bc *blockchain.Blockchain) bool {
		return assert.NoError(t, func() error {
			headBlock, err := bc.Head()
			assert.NoError(t, err)

			height := int(headBlock.Number)
			for height >= 0 {
				b, err := gw.BlockByNumber(uint64(height))
				if err != nil {
					return err
				}

				block, err := bc.GetBlockByNumber(uint64(height))
				assert.NoError(t, err)

				assert.Equal(t, b, block)
				height--
			}
			return nil
		}())
	}
	t.Run("sync multiple blocks in an empty db", func(t *testing.T) {
		testDB := db.NewTestDb()
		bc := blockchain.NewBlockchain(testDB, utils.MAINNET)
		synchronizer := NewSynchronizer(bc, gw)
		assert.Error(t, synchronizer.SyncBlocks())

		testBlockchain(t, bc)
	})
	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := db.NewTestDb()
		bc := blockchain.NewBlockchain(testDB, utils.MAINNET)
		b0, err := gw.BlockByNumber(0)
		assert.NoError(t, err)
		s0, err := gw.StateUpdate(0)
		assert.NoError(t, err)
		assert.NoError(t, bc.Store(b0, s0))

		synchronizer := NewSynchronizer(bc, gw)
		assert.Error(t, synchronizer.SyncBlocks())

		testBlockchain(t, bc)
	})
}
