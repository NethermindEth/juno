package starknet

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
)

// TODO: Replace with the actual value type.
type Value uint64

func (v Value) Hash() hash.Hash {
	return hash.Hash(*new(felt.Felt).SetUint64(uint64(v)))
}

type (
	Address = address.Address
	Hash    = hash.Hash

	Message       = types.Message[Value, Hash, Address]
	Proposal      = types.Proposal[Value, Hash, Address]
	Prevote       = types.Prevote[Hash, Address]
	Precommit     = types.Precommit[Hash, Address]
	Vote          = types.Vote[Hash, Address]
	MessageHeader = types.MessageHeader[Address]
	Messages      = types.Messages[Value, Hash, Address]

	Action             = types.Action[Value, Hash, Address]
	BroadcastProposal  = types.BroadcastProposal[Value, Hash, Address]
	BroadcastPrevote   = types.BroadcastPrevote[Hash, Address]
	BroadcastPrecommit = types.BroadcastPrecommit[Hash, Address]
)
