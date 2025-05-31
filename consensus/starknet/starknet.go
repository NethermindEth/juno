package starknet

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
)

// TODO: Replace with the actual value type.
type Value uint64

func (v Value) Hash() types.Hash {
	return types.Hash(*new(felt.Felt).SetUint64(uint64(v)))
}

type (
	Address = address.Address
	Hash    = types.Hash

	Message  = types.Message[Value]
	Proposal = types.Proposal[Value]
	Messages = types.Messages[Value]

	Action            = types.Action[Value]
	BroadcastProposal = types.BroadcastProposal[Value]
)
