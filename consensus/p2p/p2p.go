package p2p

import (
	"context"
	"fmt"

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

const proposalTopicName = "consensus_proposals"
const voteTopicName = "consensus_votes"

type P2P[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	service.Service
	Broadcasters() Broadcasters[V, H, A]
	Listeners() Listeners[V, H, A]
}

type p2p[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	host               host.Host
	log                utils.Logger
	proposalStream     validator.ProposalStreamDemux[V, H, A]
	proposalDispatcher proposer.ProposerAdapter[V, H, A]
	voteAdapter        vote.VoteAdapter[H, A]
	maxConcurrency     int
	broadcasters       Broadcasters[V, H, A]
	listeners          Listeners[V, H, A]
	ready              chan struct{}
}

type loop interface {
	loop(ctx context.Context)
}

func New(host host.Host, log utils.Logger) P2P[starknet.Value, starknet.Hash, address.Address] {
	return &p2p[starknet.Value, starknet.Hash, address.Address]{
		host:               host,
		log:                log,
		proposalStream:     validator.NewProposalStreamDemux(validator.NewTransition()),
		proposalDispatcher: proposer.NewStarknetProposerAdapter(),
		voteAdapter:        vote.StarknetVoteAdapter,
		ready:              make(chan struct{}),
		maxConcurrency:     8, // TODO: make this configurable
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

	proposalSub, err := proposalTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("unable to subscribe to proposal topic with error: %w", err)
	}
	defer proposalSub.Cancel()

	voteSub, err := voteTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("unable to subscribe to vote topic with error: %w", err)
	}
	defer voteSub.Cancel()

	voteBroadcaster := newVoteBroadcaster(p.log, p.voteAdapter, voteTopic)
	proposalBroadcaster := newProposalBroadcaster(p.log, p.proposalDispatcher, proposalTopic)

	p.broadcasters = Broadcasters[V, H, A]{
		ProposalBroadcaster:  &proposalBroadcaster,
		PrevoteBroadcaster:   (*prevoteBroadcaster[H, A])(&voteBroadcaster),
		PrecommitBroadcaster: (*precommitBroadcaster[H, A])(&voteBroadcaster),
	}

	proposalListener := newProposalListener(p.log, proposalSub, p.proposalStream)
	voteListeners := newVoteListeners[V](p.log, voteSub, p.voteAdapter)

	p.listeners = Listeners[V, H, A]{
		ProposalListener:  proposalListener.listener,
		PrevoteListener:   voteListeners.prevoteListener,
		PrecommitListener: voteListeners.precommitListener,
	}

	close(p.ready)

	loops := [4]loop{
		proposalListener,
		voteListeners,
		proposalBroadcaster.protoBroadcaster,
		voteBroadcaster.protoBroadcaster,
	}

	iter.Iterator[loop]{MaxGoroutines: 4}.ForEach(loops[:], func(l *loop) {
		(*l).loop(ctx)
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
