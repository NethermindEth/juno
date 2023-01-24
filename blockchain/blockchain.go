package blockchain

import (
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
)

// Blockchain is responsible for keeping track of all things related to the StarkNet blockchain
type Blockchain struct {
	sync.RWMutex

	head     *core.Block
	database db.DB
	state    *state.State
	// todo: much more
}

func NewBlockchain(database db.DB) *Blockchain {
	// Todo: get the latest block from db using the prefix created in db/buckets.go
	return &Blockchain{
		RWMutex:  sync.RWMutex{},
		database: database,
		state:    state.NewState(database),
	}
}

// Height returns the latest block height
func (b *Blockchain) Height() uint64 {
	b.RLock()
	defer b.RUnlock()
	return b.head.Number
}

// NextHeight returns the current height plus 1
func (b *Blockchain) NextHeight() uint64 {
	b.RLock()
	defer b.RUnlock()

	if b.head == nil {
		return 0
	}
	return b.head.Number + 1
}

func (b *Blockchain) Store(block *core.Block, stateUpdate *core.StateUpdate) error {
	b.Lock()
	defer b.Unlock()

	/*
		Todo (tentative plan):
		- Call verifyBlockAndStateUpdate()
		- Apply the StateDiff and if the State.Update
		- Store the block using either Cbor
			- prefix for block hash has been created in bucket.go
		- Update the head
	*/

	b.head = block
	return nil
}

func (b *Blockchain) verifyBlockAndStateUpdate(block *core.Block,
	stateUpdate *core.StateUpdate,
) error {
	/*
			Todo (tentative plan):
			- Sanity checks:
				- Check parent hash matches head hash
				- Check the block hash in block matches the block hash in state update
				- Check the GlobalStateRoot matches the the NewRoot in StateUpdate
			- Block Checks:
				- Check block hash contained in the Block matches the block hash when it is computed
		          manually
					- Currently, There is no Hash field on the block this needs to be added.
					- BlockHash function should be made Static
						- These static functions which includes TransactionCommitment,
						  EventCommitment and BlockHash should be moved to utils package
			- Transaction and TransactionReceipts:
				- When Block is changed to include a list of Transaction and TransactionReceipts
					- Further checks would need to be added to ensure Transaction Hash has been
					  computed properly.
					- Sanity check would need to include checks which ensure there is same number
					  of Transactions and TransactionReceipts.

	*/
	return nil
}
