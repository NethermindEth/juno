package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/proposer"
	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/sourcegraph/conc/iter"
)

const (
	proposalTopicName = "consensus_proposals"
	voteTopicName     = "consensus_votes"
)

type P2P[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	service.Service
	Broadcasters() Broadcasters[V, H, A]
	Listeners() Listeners[V, H, A]
	OnCommit(ctx context.Context, height types.Height)
}

type p2p[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	host               host.Host
	log                utils.Logger
	proposalStream     validator.ProposalStreamDemux[V, H, A]
	proposalDispatcher proposer.ProposerAdapter[V, H, A]
	voteAdapter        vote.VoteAdapter[H, A]
	bufferSizeConfig   *buffered.BufferSizeConfig
	broadcasters       Broadcasters[V, H, A]
	listeners          Listeners[V, H, A]
	ready              chan struct{}
}

type loop interface {
	Loop(ctx context.Context)
}

func New(host host.Host, log utils.Logger, currentHeight types.Height, bufferSizeConfig *buffered.BufferSizeConfig) P2P[starknet.Value, starknet.Hash, address.Address] {
	return &p2p[starknet.Value, starknet.Hash, address.Address]{
		host:               host,
		log:                log,
		proposalStream:     validator.NewProposalStreamDemux(log, validator.NewTransition(), currentHeight, bufferSizeConfig),
		proposalDispatcher: proposer.NewStarknetProposerAdapter(),
		voteAdapter:        vote.StarknetVoteAdapter,
		bufferSizeConfig:   bufferSizeConfig,
		ready:              make(chan struct{}),
	}
}

func (p *p2p[V, H, A]) Run(ctx context.Context) error {
	gossipSub, err := pubsub.NewGossipSub(ctx, p.host)
	if err != nil {
		return fmt.Errorf("unable to create gossipsub with error: %w", err)
	}

	proposalTopic, err := gossipSub.Join(proposalTopicName)
	if err != nil {
		return fmt.Errorf("unable to join proposal topic with error: %w", err)
	}
	defer proposalTopic.Close()

	voteTopic, err := gossipSub.Join(voteTopicName)
	if err != nil {
		return fmt.Errorf("unable to join vote topic with error: %w", err)
	}
	defer voteTopic.Close()

	voteBroadcaster := newVoteBroadcaster(p.log, p.voteAdapter, voteTopic)
	proposalBroadcaster := newProposalBroadcaster(p.log, p.proposalDispatcher, proposalTopic)

	p.broadcasters = Broadcasters[V, H, A]{
		ProposalBroadcaster:  &proposalBroadcaster,
		PrevoteBroadcaster:   (*prevoteBroadcaster[H, A])(&voteBroadcaster),
		PrecommitBroadcaster: (*precommitBroadcaster[H, A])(&voteBroadcaster),
	}

	voteStream := newVoteStream[V](p.log, p.voteAdapter, p.bufferSizeConfig)

	p.listeners = Listeners[V, H, A]{
		ProposalListener:  p.proposalStream,
		PrevoteListener:   voteStream.prevoteListener,
		PrecommitListener: voteStream.precommitListener,
	}

	close(p.ready)

	loops := [5]loop{
		buffered.New(p.log, proposalTopic, p.bufferSizeConfig.ProposalSubscription, p.proposalStream),
		buffered.New(p.log, voteTopic, p.bufferSizeConfig.VoteSubscription, voteStream),
		p.proposalStream,
		proposalBroadcaster,
		voteBroadcaster,
	}

	iter.Iterator[loop]{MaxGoroutines: len(loops)}.ForEach(loops[:], func(l *loop) {
		(*l).Loop(ctx)
	})
	return nil
}

func (p *p2p[V, H, A]) Broadcasters() Broadcasters[V, H, A] {
	<-p.ready
	return p.broadcasters
}

func (p *p2p[V, H, A]) Listeners() Listeners[V, H, A] {
	<-p.ready
	return p.listeners
}

func (p *p2p[V, H, A]) OnCommit(ctx context.Context, height types.Height) {
	p.proposalStream.OnCommit(ctx, height)
}
