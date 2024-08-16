package junopluginsync

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type BlockAndStateUpdate struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
}

type JunoPlugin interface {
	NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error
	// The state is reverted by applying a write operation with the reverseStateDiff's StorageDiffs, Nonces, and ReplacedClasses,
	// and a delete option with its DeclaredV0Classes, DeclaredV1Classes, and ReplacedClasses.
	RevertBlock(from, to *BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error
}
