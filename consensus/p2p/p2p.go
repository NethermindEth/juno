package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/config"
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
	"github.com/sourcegraph/conc"
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
	commitNotifier     chan types.Height
	attachedToProposal []attachedToTopic
	attachedToVote     []attachedToTopic
	broadcasters       Broadcasters[V, H, A]
	listeners          Listeners[V, H, A]
}

type attachedToTopic interface {
	Loop(ctx context.Context, topic *pubsub.Topic)
}

func New(
	host host.Host,
	log utils.Logger,
	currentHeight types.Height,
	bufferSizeConfig *config.BufferSizes,
) P2P[starknet.Value, starknet.Hash, address.Address] {
	commitNotifier := make(chan types.Height, bufferSizeConfig.ProposalCommitNotifier)

	proposalBroadcaster := newProposalBroadcaster(log, proposer.NewStarknetProposerAdapter(), bufferSizeConfig.ProposalProtoBroadcaster)
	voteBroadcaster := vote.NewVoteBroadcaster(log, vote.StarknetVoteAdapter, bufferSizeConfig.VoteProtoBroadcaster)
	broadcasters := Broadcasters[starknet.Value, starknet.Hash, address.Address]{
		ProposalBroadcaster:  &proposalBroadcaster,
		PrevoteBroadcaster:   voteBroadcaster.AsPrevoteBroadcaster(),
		PrecommitBroadcaster: voteBroadcaster.AsPrecommitBroadcaster(),
	}

	proposalStream := validator.NewProposalStreamDemux(log, validator.NewTransition(), bufferSizeConfig, commitNotifier, currentHeight)
	voteStream := vote.NewVoteListeners[starknet.Value](log, vote.StarknetVoteAdapter, bufferSizeConfig)
	listeners := Listeners[starknet.Value, starknet.Hash, address.Address]{
		ProposalListener:  proposalStream,
		PrevoteListener:   voteStream.PrevoteListener,
		PrecommitListener: voteStream.PrecommitListener,
	}

	attachedToProposal := []attachedToTopic{
		proposalStream,
		proposalBroadcaster,
	}
	attachedToVote := []attachedToTopic{
		voteStream,
		voteBroadcaster,
	}

	return &p2p[starknet.Value, starknet.Hash, address.Address]{
		host:               host,
		log:                log,
		commitNotifier:     commitNotifier,
		broadcasters:       broadcasters,
		listeners:          listeners,
		attachedToProposal: attachedToProposal,
		attachedToVote:     attachedToVote,
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

	wg := conc.NewWaitGroup()

	for _, service := range p.attachedToProposal {
		wg.Go(func() {
			service.Loop(ctx, proposalTopic)
		})
	}

	for _, service := range p.attachedToVote {
		wg.Go(func() {
			service.Loop(ctx, voteTopic)
		})
	}

	wg.Wait()
	return nil
}

func (p *p2p[V, H, A]) Broadcasters() Broadcasters[V, H, A] {
	return p.broadcasters
}

func (p *p2p[V, H, A]) Listeners() Listeners[V, H, A] {
	return p.listeners
}

func (p *p2p[V, H, A]) OnCommit(ctx context.Context, height types.Height) {
	select {
	case <-ctx.Done():
		return
	case p.commitNotifier <- height:
	}
}
