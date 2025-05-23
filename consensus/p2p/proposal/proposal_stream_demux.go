package proposal

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type starknetProposalStream = ProposalStream[starknet.Value, hash.Hash, address.Address]

type ProposalStream[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	OnStreamMessage(*consensus.StreamMessage) (*types.Proposal[V, H, A], error)
}

type proposalStreamDemux struct {
	transition Transition
	streams    map[string]starknetProposalStream
}

func NewProposalStreamDemux(transition Transition) starknetProposalStream {
	return proposalStreamDemux{
		transition: transition,
		streams:    make(map[string]starknetProposalStream),
	}
}

func (t proposalStreamDemux) OnStreamMessage(streamMessage *consensus.StreamMessage) (*starknet.Proposal, error) {
	id := string(streamMessage.StreamId)

	if _, exists := t.streams[id]; !exists {
		t.streams[id] = newSingleProposalStream(t.transition)
	}

	return t.streams[id].OnStreamMessage(streamMessage)
}
