package starknet

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/core/felt"
)

type Value felt.Hash

func (v Value) Hash() felt.Hash {
	return felt.Hash(v)
}

type (
	Address = felt.Address
	Hash    = felt.Hash

	Message       = types.Message[Value, Hash, Address]
	Proposal      = types.Proposal[Value, Hash, Address]
	Prevote       = types.Prevote[Hash, Address]
	Precommit     = types.Precommit[Hash, Address]
	Vote          = types.Vote[Hash, Address]
	MessageHeader = types.MessageHeader[Address]

	Action             = actions.Action[Value, Hash, Address]
	WriteWAL           = actions.WriteWAL[Value, Hash, Address]
	BroadcastProposal  = actions.BroadcastProposal[Value, Hash, Address]
	BroadcastPrevote   = actions.BroadcastPrevote[Hash, Address]
	BroadcastPrecommit = actions.BroadcastPrecommit[Hash, Address]
	Commit             = actions.Commit[Value, Hash, Address]

	WALEntry     = wal.Entry[Value, Hash, Address]
	WALProposal  = wal.Proposal[Value, Hash, Address]
	WALPrevote   = wal.Prevote[Hash, Address]
	WALPrecommit = wal.Precommit[Hash, Address]
	WALTimeout   = wal.Timeout
)
