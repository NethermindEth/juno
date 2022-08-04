package sync

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/types"
)

// StateDiffCollector is a collection of StateDiff provided from the feeder gateway that can be iterated over.
type StateDiffCollector interface {
	// Run starts the collection of StateDiff.
	Run()
	// GetChannel returns the channel that will be used to collect the StateDiff.
	GetChannel() chan *starknetTypes.StateDiff
	// Close closes the collection of StateDiff.
	Close()
	// LatestBlock returns the last block of StarkNet
	LatestBlock() *feeder.StarknetBlock
	// PendingBlock returns the pending block of StarkNet
	PendingBlock() *feeder.StarknetBlock
}
