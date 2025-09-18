package starknet

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/core/types/address"
	"github.com/NethermindEth/juno/core/types/hash"
)

type Value hash.Hash

func (v Value) Hash() hash.Hash {
	return hash.Hash(v)
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

	Action             = actions.Action[Value, Hash, Address]
	BroadcastProposal  = actions.BroadcastProposal[Value, Hash, Address]
	BroadcastPrevote   = actions.BroadcastPrevote[Hash, Address]
	BroadcastPrecommit = actions.BroadcastPrecommit[Hash, Address]
	Commit             = actions.Commit[Value, Hash, Address]

	WALEntry     = wal.Entry[Value, Hash, Address]
	WALProposal  = wal.WALProposal[Value, Hash, Address]
	WALPrevote   = wal.WALPrevote[Hash, Address]
	WALPrecommit = wal.WALPrecommit[Hash, Address]
	WALTimeout   = wal.WALTimeout
)
