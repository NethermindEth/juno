package l1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/require"
)

type mockL1 struct {
	head *core.L1Head
}

var _ l1.L1 = (*mockL1)(nil)

func (m *mockL1) WatchL1Heads(ctx context.Context, heads chan<- *core.L1Head) event.Subscription {
	return event.NewSubscription(func(unsubscribe <-chan struct{}) error {
		heads <- m.head
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-unsubscribe:
			return nil
		}
	})
}

func (m *mockL1) Close() {}

type errMockL1 struct {
	err error
}

func (m *errMockL1) WatchL1Heads(ctx context.Context, heads chan<- *core.L1Head) event.Subscription {
	return event.NewSubscription(func(unsubscribe <-chan struct{}) error {
		return m.err
	})
}

func (m *errMockL1) Close() {}

func TestL1(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("unreliable subscription", func(t *testing.T) {
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		testErr := errors.New("test err")
		ml1 := &errMockL1{err: testErr}
		client := l1.NewClient(ml1, bc, log)
		require.ErrorIs(t, client.Run(context.Background()), testErr)
	})

	t.Run("reliable subscription", func(t *testing.T) {
		testDB := pebble.NewMemTest()
		bc := blockchain.New(testDB, utils.MAINNET, log)

		l1Head := &core.L1Head{
			BlockHash: new(felt.Felt),
			StateRoot: new(felt.Felt),
		}
		ml1 := &mockL1{head: l1Head}
		client := l1.NewClient(ml1, bc, log)
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		t.Cleanup(cancel)
		require.NoError(t, client.Run(ctx))
		got, err := bc.L1Head()
		require.NoError(t, err)
		require.Equal(t, l1Head, got)
	})
}
