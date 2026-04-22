package buffered

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type ProtoBroadcaster[M proto.Message] struct {
	logger              log.Logger
	ch                  chan M
	retryInterval       time.Duration
	rebroadcastStrategy RebroadcastStrategy[M]
}

func NewProtoBroadcaster[M proto.Message](
	logger log.Logger,
	bufferSize int,
	retryInterval time.Duration,
	rebroadcastStrategy RebroadcastStrategy[M],
) ProtoBroadcaster[M] {
	return ProtoBroadcaster[M]{
		logger:              logger,
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
	var rebroadcasted rebroadcastMessages

	time.Sleep(pubsub.GossipSubHeartbeatInitialDelay * 2)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-b.ch:
			msgBytes, err := proto.Marshal(msg)
			if err != nil {
				b.logger.Error("unable to marshal message", zap.Error(err))
				continue
			}

			for {
				if err := topic.Publish(ctx, msgBytes); err != nil && !errors.Is(err, context.Canceled) {
					b.logger.Error("unable to send message", zap.Error(err))
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
				if err := topic.Publish(ctx, msgBytes); err != nil && !errors.Is(err, context.Canceled) {
					b.logger.Error("unable to rebroadcast message", zap.Error(err))
				}
			}
		}
	}
}
