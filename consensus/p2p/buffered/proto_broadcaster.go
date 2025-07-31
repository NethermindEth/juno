package buffered

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
)

type ProtoBroadcaster struct {
	log           utils.Logger
	ch            chan proto.Message
	retryInterval time.Duration
}

func NewProtoBroadcaster(log utils.Logger, bufferSize int, retryInterval time.Duration) ProtoBroadcaster {
	return ProtoBroadcaster{
		log:           log,
		ch:            make(chan proto.Message, bufferSize),
		retryInterval: retryInterval,
	}
}

func (b ProtoBroadcaster) Broadcast(msg proto.Message) {
	b.ch <- msg
}

func (b ProtoBroadcaster) Loop(ctx context.Context, topic *pubsub.Topic) {
	readinessOpt := pubsub.WithReadiness(pubsub.MinTopicSize(1))
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
				if err := topic.Publish(ctx, msgBytes, readinessOpt); err != nil && !errors.Is(err, context.Canceled) {
					b.log.Errorw("unable to send message", "error", err)
					time.Sleep(b.retryInterval)
					continue
				}
				break
			}
		}
	}
}
