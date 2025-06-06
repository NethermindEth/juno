package buffered

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type SubscriptionCallback interface {
	OnMessage(context.Context, *pubsub.Message)
}

type TopicSubscription struct {
	log        utils.Logger
	topic      *pubsub.Topic
	bufferSize int
	callback   SubscriptionCallback
}

func New(log utils.Logger, topic *pubsub.Topic, bufferSize int, callback SubscriptionCallback) TopicSubscription {
	return TopicSubscription{
		log:        log,
		topic:      topic,
		bufferSize: bufferSize,
		callback:   callback,
	}
}

func (b TopicSubscription) Loop(ctx context.Context) {
	sub, err := b.topic.Subscribe(pubsub.WithBufferSize(b.bufferSize))
	if err != nil {
		b.log.Errorw("unable to subscribe to topic with error: %w", err)
		return
	}
	defer sub.Cancel()

	for {
		msg, err := sub.Next(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			b.log.Errorw("unable to receive message", "error", err)
			continue
		}
		b.callback.OnMessage(ctx, msg)
	}
}
