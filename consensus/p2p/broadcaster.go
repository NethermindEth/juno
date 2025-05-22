package p2p

import (
	"context"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/adapters"
	"github.com/NethermindEth/juno/consensus/types"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sourcegraph/conc/pool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type Broadcaster[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)
}

type Broadcasters[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	ProposalBroadcaster  Broadcaster[types.Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[types.Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[types.Precommit[H, A], V, H, A]
}

type voteBroadcaster[H types.Hash, A types.Addr] struct {
	topic       *pubsub.Topic
	pool        *pool.ContextPool
	voteAdapter adapters.VoteAdapter[H, A]
}

func (b *voteBroadcaster[H, A]) broadcast(message *types.Vote[H, A], voteType consensus.Vote_VoteType) {
	msg, err := b.voteAdapter.FromVote(message, voteType)
	if err != nil {
		return // TODO: log error
	}

	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return // TODO: log error
	}

	b.pool.Go(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				err = b.topic.Publish(ctx, msgBytes)
				if err != nil {
					time.Sleep(1 * time.Second) // TODO: log error and use proper backoff
					continue
				}
				return nil
			}
		}
	})
}

type prevoteBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *prevoteBroadcaster[H, A]) Broadcast(message types.Prevote[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast((*types.Vote[H, A])(&message), consensus.Vote_Prevote)
}

type precommitBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *precommitBroadcaster[H, A]) Broadcast(message types.Precommit[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast((*types.Vote[H, A])(&message), consensus.Vote_Precommit)
}

type proposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	topic       *pubsub.Topic
	pool        *pool.ContextPool
	voteAdapter adapters.VoteAdapter[H, A]
}

func (b *proposalBroadcaster[V, H, A]) Broadcast(message types.Proposal[V, H, A]) {
	// TODO: Implement this
}
