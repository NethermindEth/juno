package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/p2p/proposer"
	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/sourcegraph/conc"
)

type topicName string

const (
	proposalTopicName topicName = "consensus_proposals"
	voteTopicName     topicName = "consensus_votes"
	gossipSubHistory            = 60
)

type P2P[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	service.Service
	Broadcasters() Broadcasters[V, H, A]
	Listeners() Listeners[V, H, A]
	OnCommit(ctx context.Context, height types.Height, value V)
}

type p2p[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	host            host.Host
	log             utils.Logger
	commitNotifier  chan types.Height
	broadcasters    Broadcasters[V, H, A]
	listeners       Listeners[V, H, A]
	topicAttachment map[topicName][]attachedToTopic
}

type attachedToTopic interface {
	Loop(ctx context.Context, topic *pubsub.Topic)
}

func New(
	host host.Host,
	log utils.Logger,
	builder *builder.Builder,
	proposalStore *proposal.ProposalStore[starknet.Hash],
	currentHeight types.Height,
	bufferSizeConfig *config.BufferSizes,
) P2P[starknet.Value, starknet.Hash, address.Address] {
	commitNotifier := make(chan types.Height, bufferSizeConfig.ProposalCommitNotifier)

	proposalBroadcaster := proposer.NewProposalBroadcaster(
		log,
		proposer.NewStarknetProposerAdapter(),
		proposalStore,
		bufferSizeConfig.ProposalProtoBroadcaster,
		bufferSizeConfig.RetryInterval,
	)
	voteBroadcaster := vote.NewVoteBroadcaster(
		log,
		vote.StarknetVoteAdapter,
		bufferSizeConfig.VoteProtoBroadcaster,
		bufferSizeConfig.RetryInterval,
	)
	broadcasters := Broadcasters[starknet.Value, starknet.Hash, address.Address]{
		ProposalBroadcaster:  &proposalBroadcaster,
		PrevoteBroadcaster:   voteBroadcaster.AsPrevoteBroadcaster(),
		PrecommitBroadcaster: voteBroadcaster.AsPrecommitBroadcaster(),
	}

	proposalStream := validator.NewProposalStreamDemux(
		log,
		proposalStore,
		validator.NewTransition(builder),
		bufferSizeConfig,
		commitNotifier,
		currentHeight,
	)
	voteStream := vote.NewVoteListeners[starknet.Value](log, vote.StarknetVoteAdapter, bufferSizeConfig)
	listeners := Listeners[starknet.Value, starknet.Hash, address.Address]{
		ProposalListener:  proposalStream,
		PrevoteListener:   voteStream.PrevoteListener,
		PrecommitListener: voteStream.PrecommitListener,
	}

	topicAttachment := map[topicName][]attachedToTopic{
		proposalTopicName: {
			proposalStream,
			&proposalBroadcaster,
		},
		voteTopicName: {
			voteStream,
			voteBroadcaster,
		},
	}

	return &p2p[starknet.Value, starknet.Hash, address.Address]{
		host:            host,
		log:             log,
		commitNotifier:  commitNotifier,
		broadcasters:    broadcasters,
		listeners:       listeners,
		topicAttachment: topicAttachment,
	}
}

func (p *p2p[V, H, A]) getGossipSubOptions() pubsub.Option {
	params := pubsub.DefaultGossipSubParams()
	params.HistoryLength = gossipSubHistory
	params.HistoryGossip = gossipSubHistory

	return pubsub.WithGossipSubParams(params)
}

func (p *p2p[V, H, A]) Run(ctx context.Context) error {
	gossipSub, err := pubsub.NewGossipSub(ctx, p.host, p.getGossipSubOptions())
	if err != nil {
		return fmt.Errorf("unable to create gossipsub with error: %w", err)
	}

	topics := make(map[topicName]*pubsub.Topic)
	defer func() {
		for _, topic := range topics {
			topic.Close()
		}
	}()

	wg := conc.NewWaitGroup()

	for topicName := range p.topicAttachment {
		if topics[topicName], err = gossipSub.Join(string(topicName)); err != nil {
			return fmt.Errorf("unable to join topic %s with error: %w", topicName, err)
		}
	}

	for topicName, services := range p.topicAttachment {
		for _, service := range services {
			wg.Go(func() {
				service.Loop(ctx, topics[topicName])
			})
		}
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

func (p *p2p[V, H, A]) OnCommit(ctx context.Context, height types.Height, value V) {
	select {
	case <-ctx.Done():
		return
	case p.commitNotifier <- height:
	}
}
