package sync

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

/*

Todo:
- Get the fake StarkNetData to serve one block
	- For the above you need to have a head of the block in blockchain
	- You need to make the blockchain accept a db/transaction
	- Then get it to store in the fake db
	- Write todos for the verify and Store function
	- Decide what to do for
*/

func TestSyncBlocks(t *testing.T) {
	bc := blockchain.NewBlockchain()
	fakeData := newFakeStarkNetData()
	synchronizer := NewSynchronizer(bc, fakeData)
	err := synchronizer.SyncBlocks()
	assert.NoError(t, err)
}

type fakeStarkNetData struct {
	blocks      map[uint64]*core.Block
	stateUpdate map[uint64]*core.StateUpdate
}

func newFakeStarkNetData() *fakeStarkNetData {
	blocksM, stateUpdateM := populateBlocksAndStateUpdate()
	return &fakeStarkNetData{blocksM, stateUpdateM}
}

func populateBlocksAndStateUpdate() (map[uint64]*core.Block, map[uint64]*core.StateUpdate) {
	// go
	bm := make(map[uint64]*core.Block)
	sm := make(map[uint64]*core.StateUpdate)

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
