package sync

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestSyncBlocks(t *testing.T) {
	testBlockchain := func(t *testing.T, bc *blockchain.Blockchain, fakeData *fakeStarkNetData) bool {
		return assert.NoError(t, func() error {
			headBlock, err := bc.Head()
			assert.NoError(t, err)

			height := int(headBlock.Number)
			for height >= 0 {
				b, err := fakeData.BlockByNumber(uint64(height))
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
		fakeData := newFakeStarkNetData()
		synchronizer := NewSynchronizer(bc, fakeData)
		assert.Error(t, synchronizer.SyncBlocks())

		testBlockchain(t, bc, fakeData)
	})
	t.Run("sync multiple blocks in a non-empty db", func(t *testing.T) {
		testDB := db.NewTestDb()
		bc := blockchain.NewBlockchain(testDB, utils.MAINNET)
		fakeData := newFakeStarkNetData()
		b0, err := fakeData.BlockByNumber(0)
		assert.NoError(t, err)
		s0, err := fakeData.StateUpdate(0)
		assert.NoError(t, err)
		assert.NoError(t, bc.Store(b0, s0))

		synchronizer := NewSynchronizer(bc, fakeData)
		assert.Error(t, synchronizer.SyncBlocks())

		testBlockchain(t, bc, fakeData)
	})
}

type fakeStarkNetData struct {
	blocks      map[uint64]*core.Block
	stateUpdate map[uint64]*core.StateUpdate
}

func newFakeStarkNetData() *fakeStarkNetData {
	blocksM, stateUpdateM := populateBlocksAndStateUpdate()
	return &fakeStarkNetData{blocksM, stateUpdateM}
}

// As mentioned here: https://dariodip.medium.com/go-embed-unleashed-1eab8b4b1ba6.
// It is possible that file are excluded from when go build is run. This needs to be investigated.
var (
	//go:embed testdata/mainnet_block_0.json
	mainnetBlock0 []byte
	//go:embed testdata/mainnet_block_1.json
	mainnetBlock1 []byte
	//go:embed testdata/mainnet_block_2.json
	mainnetBlock2 []byte
	//go:embed testdata/mainnet_state_update_0.json
	mainnetStateUpdate0 []byte
	//go:embed testdata/mainnet_state_update_1.json
	mainnetStateUpdate1 []byte
	//go:embed testdata/mainnet_state_update_2.json
	mainnetStateUpdate2 []byte
)

func populateBlocksAndStateUpdate() (map[uint64]*core.Block, map[uint64]*core.StateUpdate) {
	numOfBlocks := 3
	bm := make(map[uint64]*core.Block, 3)
	sm := make(map[uint64]*core.StateUpdate, 3)
	rawBlocks := [][]byte{mainnetBlock0, mainnetBlock1, mainnetBlock2}
	rawStateUpdate := [][]byte{mainnetStateUpdate0, mainnetStateUpdate1, mainnetStateUpdate2}

	for i := 0; i < numOfBlocks; i++ {
		clientBlock, clientStateUpdate := new(clients.Block), new(clients.StateUpdate)
		if err := json.Unmarshal(rawBlocks[i], clientBlock); err != nil {
			panic(err)
		}
		if err := json.Unmarshal(rawStateUpdate[i], clientStateUpdate); err != nil {
			panic(err)
		}
		b, err := gateway.AdaptBlock(clientBlock)
		if err != nil {
			panic(err)
		}
		bm[uint64(i)] = b
		su, err := gateway.AdaptStateUpdate(clientStateUpdate)
		if err != nil {
			panic(err)
		}
		sm[uint64(i)] = su
	}
	return bm, sm
}

func (f *fakeStarkNetData) BlockByNumber(blockNumber uint64) (*core.Block, error) {
	b := f.blocks[blockNumber]
	if b == nil {
		return nil, errors.New("unknown block")
	}
	return b, nil
}

func (f *fakeStarkNetData) StateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
	u := f.stateUpdate[blockNumber]
	if u == nil {
		return nil, errors.New("unknown state update")
	}
	return u, nil
}

func (f *fakeStarkNetData) Transaction(_ *felt.Felt) (core.Transaction, error) {
	return nil, nil
}

func (f *fakeStarkNetData) Class(_ *felt.Felt) (*core.Class, error) {
	return nil, nil
}
