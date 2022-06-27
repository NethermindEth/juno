package services

import starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"

// StateDiffCollector is a collection of StateDiff provided from the feeder gateway that can be iterated over.
type StateDiffCollector interface {
	// GetStateForBlock returns the StateDiff for the given block number.
	GetStateForBlock(blockNumber int64) *starknetTypes.StateDiff
}
