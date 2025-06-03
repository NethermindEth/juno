package p2p

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

type protoBroadcaster struct {
	log   utils.Logger
	topic *pubsub.Topic
	ch    chan proto.Message
}

func newProtoBroadcaster(log utils.Logger, topic *pubsub.Topic) protoBroadcaster {
	return protoBroadcaster{
		log:   log,
		topic: topic,
		ch:    make(chan proto.Message, 1024), // TODO: make configurable
	}
}

func (b protoBroadcaster) broadcast(msg proto.Message) {
	b.ch <- msg
}

func (b protoBroadcaster) Loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-b.ch:
			msgBytes, err := proto.Marshal(msg)
			if err != nil {
				b.log.Errorw("unable to marshal message", "error", err)
				continue
			}

			for {
				if err := b.topic.Publish(ctx, msgBytes, pubsub.WithReadiness(pubsub.MinTopicSize(1))); err != nil && !errors.Is(err, context.Canceled) {
					b.log.Errorw("unable to send message", "error", err)
					time.Sleep(1 * time.Second) // TODO: make configurable
					continue
				}
				break
			}
		}
	}
}
