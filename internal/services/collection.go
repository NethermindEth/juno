package services

import (
	"context"

	starknetTypes "github.com/NethermindEth/juno/pkg/types"
)

// StateDiffCollector is a collection of StateDiff provided from the feeder gateway that can be iterated over.
type StateDiffCollector interface {
	// Run starts the collection of StateDiff.
	Run() error
	// GetChannel returns the channel that will be used to collect the StateDiff.
	GetChannel() chan *starknetTypes.StateDiff
	// Close closes the collection of StateDiff.
	Close(ctx context.Context)
	// GetLatestBlockOnChain returns the last block to be collected by the StateDiffCollector.
	GetLatestBlockOnChain() int64
	// IsSynced returns true if we are Synced
	IsSynced() bool
}
