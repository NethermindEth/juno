package pubsub_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/pubsub"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/require"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	subscribed := false
	t.Cleanup(func() {
		require.True(t, subscribed)
	})
	sub := event.NewSubscription(func(quit <-chan struct{}) error {
		subscribed = true
		<-quit
		return nil
	})

	log := utils.NewNopZapLogger()
	registry := pubsub.New(log)
	id := registry.Add(context.Background(), sub)
	err := registry.Delete(id)
	require.NoError(t, err)
}
