package services

import (
	"github.com/NethermindEth/juno/internal/db/sync"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
)

// APIIterator is an StateDiffIterator that can be used to iterate over the StateDiff provided by the feeder gateway.
type APIIterator struct {
	// manager represent the Sync manager
	manager *sync.Manager
	// currentBlockNumber is the current block number we are iterating over.
	currentBlockNumber int64
}

func (u *APIIterator) BlockNumberInTop() int64 {
	return u.currentBlockNumber
}

// HasNext returns true if there is another element in the collection.
func (u *APIIterator) HasNext() bool {
	latestBlockSynced := u.manager.GetLatestStateDiffSynced()
	return u.currentBlockNumber <= latestBlockSynced

}

// GetNext returns the next element in the collection.
func (u *APIIterator) GetNext() *starknetTypes.StateDiff {
	if u.HasNext() {
		stateDiff := u.manager.GetStateDiff(u.currentBlockNumber)
		u.currentBlockNumber++
		return stateDiff
	}
	return nil
}
