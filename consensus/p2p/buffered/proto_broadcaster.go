package buffered

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type ProtoBroadcaster[M proto.Message] struct {
	log                 utils.Logger
	ch                  chan M
	retryInterval       time.Duration
	rebroadcastStrategy RebroadcastStrategy[M]
}

func NewProtoBroadcaster[M proto.Message](
	log utils.Logger,
	bufferSize int,
	retryInterval time.Duration,
	rebroadcastStrategy RebroadcastStrategy[M],
) ProtoBroadcaster[M] {
	return ProtoBroadcaster[M]{
		log:                 log,
		ch:                  make(chan M, bufferSize),
		retryInterval:       retryInterval,
		rebroadcastStrategy: rebroadcastStrategy,
	}
}

func (b ProtoBroadcaster[M]) Broadcast(ctx context.Context, msg M) {
	select {
	case <-ctx.Done():
		return
	case b.ch <- msg:
	}
}

func (b ProtoBroadcaster[M]) Loop(ctx context.Context, topic *pubsub.Topic) {
	readinessOpt := pubsub.WithReadiness(pubsub.MinTopicSize(1))
	var rebroadcasted rebroadcastMessages

	time.Sleep(pubsub.GossipSubHeartbeatInitialDelay * 2)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-b.ch:
			msgBytes, err := proto.Marshal(msg)
			if err != nil {
				b.log.Error("unable to marshal message", zap.Error(err))
				continue
			}

			for {
				if err := topic.Publish(ctx, msgBytes, readinessOpt); err != nil && !errors.Is(err, context.Canceled) {
					b.log.Error("unable to send message", zap.Error(err))
					time.Sleep(b.retryInterval)
					continue
				}
				break
			}

			if b.rebroadcastStrategy != nil {
				rebroadcasted = b.rebroadcastStrategy.Receive(msg, msgBytes)
			}
		case <-rebroadcasted.trigger:
			for msgBytes := range rebroadcasted.messages {
				if err := topic.Publish(ctx, msgBytes, readinessOpt); err != nil && !errors.Is(err, context.Canceled) {
					b.log.Error("unable to rebroadcast message", zap.Error(err))
				}
			}
		}
	}
}
