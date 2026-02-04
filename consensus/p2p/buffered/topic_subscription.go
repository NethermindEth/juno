package buffered

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
)

type TopicSubscription struct {
	log        utils.Logger
	bufferSize int
	callback   func(context.Context, *pubsub.Message)
}

func NewTopicSubscription(log utils.Logger, bufferSize int, callback func(context.Context, *pubsub.Message)) TopicSubscription {
	return TopicSubscription{
		log:        log,
		bufferSize: bufferSize,
		callback:   callback,
	}
}

func (b TopicSubscription) Loop(ctx context.Context, topic *pubsub.Topic) {
	sub, err := topic.Subscribe(pubsub.WithBufferSize(b.bufferSize))
	if err != nil {
		b.log.Error("unable to subscribe to topic", zap.Error(err))
		return
	}
	defer sub.Cancel()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			b.log.Error("unable to receive message", zap.Error(err))
			continue
		}
		b.callback(ctx, msg)
	}
}
