package application

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

type pending[H types.Hash] struct {
	pending sync.Pending
	network *utils.Network
}

func NewPending[H types.Hash](network *utils.Network) pending[H] {
	return pending[H]{network: network}
}

var _ types.Hashable[felt.Felt] = (*pending[felt.Felt])(nil)

func (p pending[H]) Hash() felt.Felt {
	hash, _, err := core.BlockHash(p.pending.Block, p.pending.StateUpdate.StateDiff, p.network, nil)
	if err != nil {
		panic("Todo")
	}
	return *hash
}
