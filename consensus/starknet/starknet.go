package starknet

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/hash"
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

	Action             = types.Action[Value, Hash, Address]
	BroadcastProposal  = types.BroadcastProposal[Value, Hash, Address]
	BroadcastPrevote   = types.BroadcastPrevote[Hash, Address]
	BroadcastPrecommit = types.BroadcastPrecommit[Hash, Address]
	Commit             = types.Commit[Value, Hash, Address]

	WALEntry     = wal.Entry[Value, Hash, Address]
	WALProposal  = wal.WALProposal[Value, Hash, Address]
	WALPrevote   = wal.WALPrevote[Hash, Address]
	WALPrecommit = wal.WALPrecommit[Hash, Address]
	WALTimeout   = wal.WALTimeout
)
