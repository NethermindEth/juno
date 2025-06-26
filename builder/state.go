package builder

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
)

type BuildState struct {
	Pending           *sync.Pending
	L2GasConsumed     uint64
	RevealedBlockHash *felt.Felt
}

func (b *BuildState) PendingBlock() *core.Block {
	if b.Pending == nil {
		return nil
	}
	return b.Pending.Block
}

func (b *BuildState) ClearPending() error {
	b.L2GasConsumed = 0
	b.Pending = &sync.Pending{}
	b.RevealedBlockHash = nil

	return nil
}
