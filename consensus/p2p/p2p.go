package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/p2p/proposer"
	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/p2p/pubsub"
	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	libp2p "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sourcegraph/conc"
)

type topicName string

const (
	chainID                     = "1" // TODO: Make this configurable
	proposalTopicName topicName = "consensus_proposals"
	voteTopicName     topicName = "consensus_votes"
)

type P2P[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	service.Service
	Broadcasters() Broadcasters[V, H, A]
	Listeners() Listeners[V, H, A]
	OnCommit(ctx context.Context, height types.Height, value V)
}

type p2p[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	host             host.Host
	log              utils.Logger
	network          *utils.Network
	commitNotifier   chan types.Height
	broadcasters     Broadcasters[V, H, A]
	listeners        Listeners[V, H, A]
	topicAttachment  map[topicName][]attachedToTopic
	pubSubQueueSize  int
	bootstrapPeersFn func() []peer.AddrInfo
}

type attachedToTopic interface {
	Loop(ctx context.Context, topic *libp2p.Topic)
}

func New(
	host host.Host,
	log utils.Logger,
	builder *builder.Builder,
	proposalStore *proposal.ProposalStore[starknet.Hash],
	currentHeight types.Height,
	bufferSizeConfig *config.BufferSizes,
	bootstrapPeersFn func() []peer.AddrInfo,
) P2P[starknet.Value, starknet.Hash, starknet.Address] {
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
		bufferSizeConfig,
	)
	broadcasters := Broadcasters[starknet.Value, starknet.Hash, starknet.Address]{
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
	listeners := Listeners[starknet.Value, starknet.Hash, starknet.Address]{
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

	return &p2p[starknet.Value, starknet.Hash, starknet.Address]{
		host:             host,
		log:              log,
		network:          builder.Network(),
		commitNotifier:   commitNotifier,
		broadcasters:     broadcasters,
		listeners:        listeners,
		topicAttachment:  topicAttachment,
		pubSubQueueSize:  bufferSizeConfig.PubSubQueueSize,
		bootstrapPeersFn: bootstrapPeersFn,
	}
}

func (p *p2p[V, H, A]) Run(ctx context.Context) (returnedError error) {
	gossipSub, closer, err := pubsub.Run(
		ctx,
		p.host,
		p.network,
		starknetp2p.ConsensusProtocolID,
		p.bootstrapPeersFn,
		p.pubSubQueueSize,
	)
	if err != nil {
		return fmt.Errorf("unable to create gossipsub with error: %w", err)
	}
	defer func() {
		returnedError = errors.Join(returnedError, closer())
	}()

	topics := make([]*libp2p.Topic, 0, len(p.topicAttachment))
	relayCancels := make([]func(), 0, len(p.topicAttachment))
	defer func() {
		for _, cancel := range relayCancels {
			cancel()
		}
		for _, topic := range topics {
			topic.Close()
		}
	}()

	wg := conc.NewWaitGroup()
	defer wg.Wait()

	for topicName, services := range p.topicAttachment {
		topic, relayCancel, err := pubsub.JoinTopic(gossipSub, string(topicName))
		if err != nil {
			return fmt.Errorf("unable to join topic %s with error: %w", topicName, err)
		}

		topics = append(topics, topic)
		relayCancels = append(relayCancels, relayCancel)

		for _, service := range services {
			wg.Go(func() {
				service.Loop(ctx, topic)
			})
		}
	}

	<-ctx.Done()
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
