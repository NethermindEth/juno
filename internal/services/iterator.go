package services

import starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"

// StateDiffIterator is a generic interface for iterating over a collection.
type StateDiffIterator interface {
	// HasNext returns true if there is another element in the collection.
	HasNext() bool
	// GetNext returns the next element in the collection.
	GetNext() *starknetTypes.StateDiff
	// BlockNumberInTop represent the block
	BlockNumberInTop() int64
}
