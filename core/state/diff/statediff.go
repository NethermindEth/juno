package core

import (
	"github.com/NethermindEth/juno/core"
)

// TODO
// See the following for an example of a state diff
// https://alpha-mainnet.starknet.io/feeder_gateway/get_state_update?blockNumber=latest
type StateDiff struct {
}

type StateDiffReader interface {
	StateDiff(blockNum uint64) (*core.StateDiff, error)
	// TODO: should we allow end > start here or add an AggregateReverseDiff function?
	AggregateDiff(start uint64, end uint64) (*core.StateDiff, error)
}

type StateDiffWriter interface {
	PutStateDiff(stateDiff *core.StateDiff) error
}
