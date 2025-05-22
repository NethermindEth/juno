package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/adapters"
	"github.com/NethermindEth/juno/consensus/p2p/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/service"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

const proposalTopicName = "consensus_proposals"
const voteTopicName = "consensus_votes"

type P2P[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	service.Service
	Broadcasters() Broadcasters[V, H, A]
	Listeners() Listeners[V, H, A]
}

type p2p[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	host           host.Host
	proposalStream proposal.ProposalStream[V, H, A]
	voteAdapter    adapters.VoteAdapter[H, A]
	maxConcurrency int
	broadcasters   Broadcasters[V, H, A]
	listeners      Listeners[V, H, A]
	ready          chan struct{}
}

func New(host host.Host) P2P[starknet.Value, hash.Hash, address.Address] {
	return &p2p[starknet.Value, hash.Hash, address.Address]{
		host:           host,
		proposalStream: proposal.NewProposalStreamDemux(proposal.NewTransition()),
		voteAdapter:    adapters.StarknetVoteAdapter,
		ready:          make(chan struct{}),
	}
}

func (p *p2p[V, H, A]) Run(ctx context.Context) error {
	tracer, err := pubsub.NewJSONTracer("./trace.json")
	fmt.Println("err", err)
	if err != nil {
		return err
	}
	fmt.Println("tracer", tracer)

	gossipSub, err := pubsub.NewGossipSub(ctx, p.host, pubsub.WithEventTracer(tracer))
	if err != nil {
		return fmt.Errorf("unable to create gossipsub with error: %w", err)
	}

	proposalTopic, err := gossipSub.Join(proposalTopicName)
	if err != nil {
		return fmt.Errorf("unable to join proposal topic with error: %w", err)
	}

	voteTopic, err := gossipSub.Join(voteTopicName)
	if err != nil {
		return fmt.Errorf("unable to join vote topic with error: %w", err)
	}

	proposalSub, err := proposalTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("unable to subscribe to proposal topic with error: %w", err)
	}

	voteSub, err := voteTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("unable to subscribe to vote topic with error: %w", err)
	}

	broadcasterPool := pool.New().WithContext(ctx).WithMaxGoroutines(p.maxConcurrency)

	voteBroadcaster := voteBroadcaster[H, A]{
		topic:       voteTopic,
		pool:        broadcasterPool,
		voteAdapter: p.voteAdapter,
	}

	p.broadcasters = Broadcasters[V, H, A]{
		ProposalBroadcaster: &proposalBroadcaster[V, H, A]{
			topic:       proposalTopic,
			pool:        broadcasterPool,
			voteAdapter: p.voteAdapter,
		},
		PrevoteBroadcaster:   (*prevoteBroadcaster[H, A])(&voteBroadcaster),
		PrecommitBroadcaster: (*precommitBroadcaster[H, A])(&voteBroadcaster),
	}

	proposalListener := newListener[types.Proposal[V, H, A], V, H, A]()
	prevoteListener := newListener[types.Prevote[H, A], V, H, A]()
	precommitListener := newListener[types.Precommit[H, A], V, H, A]()

	p.listeners = Listeners[V, H, A]{
		ProposalListener:  proposalListener,
		PrevoteListener:   prevoteListener,
		PrecommitListener: precommitListener,
	}

	close(p.ready)

	listenerPool := conc.NewWaitGroup()

	listenerPool.Go(func() {
		p.proposalMessageLoop(ctx, proposalSub, proposalListener)
	})

	listenerPool.Go(func() {
		p.voteMessageLoop(ctx, voteSub, prevoteListener, precommitListener)
	})

	listenerPool.Wait()
	return broadcasterPool.Wait()
}

func (p *p2p[V, H, A]) proposalMessageLoop(
	ctx context.Context,
	proposalSub *pubsub.Subscription,
	proposalListener listener[types.Proposal[V, H, A], V, H, A],
) {
	for {
		msg, err := proposalSub.Next(ctx)
		if err != nil {
			continue
		}

		streamMessage := consensus.StreamMessage{}
		if err := proto.Unmarshal(msg.Data, &streamMessage); err != nil {
			continue
		}

		proposal, err := p.proposalStream.OnStreamMessage(&streamMessage)
		if err != nil {
			continue
		}

		if proposal != nil {
			proposalListener.Receive(ctx, *proposal)
		}
	}
}

func (p *p2p[V, H, A]) voteMessageLoop(
	ctx context.Context,
	voteSub *pubsub.Subscription,
	prevoteListener listener[types.Prevote[H, A], V, H, A],
	precommitListener listener[types.Precommit[H, A], V, H, A],
) {
	for {
		msg, err := voteSub.Next(ctx)
		if err != nil {
			continue
		}

		p2pVote := consensus.Vote{}
		if err := proto.Unmarshal(msg.Data, &p2pVote); err != nil {
			continue
		}

		vote, err := p.voteAdapter.ToVote(&p2pVote)
		if err != nil {
			continue
		}

		switch p2pVote.VoteType {
		case consensus.Vote_Prevote:
			prevoteListener.Receive(ctx, types.Prevote[H, A](*vote))
		case consensus.Vote_Precommit:
			precommitListener.Receive(ctx, types.Precommit[H, A](*vote))
		}
	}
}

func (p *p2p[V, H, A]) Broadcasters() Broadcasters[V, H, A] {
	<-p.ready
	return p.broadcasters
}

func (p *p2p[V, H, A]) Listeners() Listeners[V, H, A] {
	<-p.ready
	return p.listeners
}
