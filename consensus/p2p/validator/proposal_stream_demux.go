package validator

import (
	"context"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type ProposalStreamDemux[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	OnStreamMessage(context.Context, *consensus.StreamMessage) (*types.Proposal[V, H, A], error)
}

type proposalStreamDemux struct {
	transition Transition
	streams    map[string]*proposalStream
}

func NewProposalStreamDemux(transition Transition) ProposalStreamDemux[starknet.Value, starknet.Hash, starknet.Address] {
	return proposalStreamDemux{
		transition: transition,
		streams:    make(map[string]*proposalStream),
	}
}

func (t proposalStreamDemux) OnStreamMessage(
	ctx context.Context,
	streamMessage *consensus.StreamMessage,
) (*starknet.Proposal, error) {
	id := string(streamMessage.StreamId)

	if _, exists := t.streams[id]; !exists {
		t.streams[id] = newSingleProposalStream(t.transition)
	}

	return t.streams[id].OnStreamMessage(ctx, streamMessage)
}
