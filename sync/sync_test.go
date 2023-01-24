package sync

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata/gateway"
)

/*

Todo:
- Get the fake StarkNetData to serve one block
	- For the above you need to have a head of the block in blockchain
	- You need to make the blockchain accept a db/transaction
	- Then get it to store in the fake db
	- Write todos for the verify and Store function
Test Cases to enumerate:
1. Sync from couple of blocks
2. Start from non empty db and sync couple of blocks
3. Stop syncing when error occurs
4. Consider testing some new block which involve changes to the block structure
*/

func testSyncBlocks(t *testing.T) {
	bc := blockchain.NewBlockchain(db.NewTestDb())
	fakeData := newFakeStarkNetData()
	synchronizer := NewSynchronizer(bc, fakeData)
	synchronizer.SyncBlocks()
	// assert.NoError(t, err)
}

type fakeStarkNetData struct {
	blocks      map[uint64]*core.Block
	stateUpdate map[uint64]*core.StateUpdate
}

func newFakeStarkNetData() *fakeStarkNetData {
	blocksM, stateUpdateM := populateBlocksAndStateUpdate()
	return &fakeStarkNetData{blocksM, stateUpdateM}
}

// Todo: consider not using embed since it drastically increase compiled binary size.
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

func (f *fakeStarkNetData) Transaction(_ *felt.Felt) (*core.Transaction, error) {
	return nil, nil
}

func (f *fakeStarkNetData) Class(_ *felt.Felt) (*core.Class, error) {
	return nil, nil
}
