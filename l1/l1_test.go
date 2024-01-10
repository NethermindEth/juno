package l1_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeSubscription struct {
	errChan chan error
	closed  bool
}

func newFakeSubscription() *fakeSubscription {
	return &fakeSubscription{
		errChan: make(chan error),
	}
}

func (s *fakeSubscription) Err() <-chan error {
	return s.errChan
}

func (s *fakeSubscription) Unsubscribe() {
	if !s.closed {
		close(s.errChan)
		s.closed = true
	}
}

func TestFailToCreateSubscription(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")

	network := utils.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), network)

	subscriber := mocks.NewMockSubscriber(ctrl)

	subscriber.
		EXPECT().
		WatchLogStateUpdate(gomock.Any(), gomock.Any()).
		Return(newFakeSubscription(), err).
		AnyTimes()

	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.DefaultL1ChainID(), nil).
		Times(1)

	subscriber.EXPECT().Close().Times(1)

	client := l1.NewClient(subscriber, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	require.ErrorContains(t, client.Run(ctx), "context canceled before resubscribe was successful")
	cancel()
}

func TestMismatchedChainID(t *testing.T) {
	t.Parallel()

	network := utils.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), network)

	subscriber := mocks.NewMockSubscriber(ctrl)

	subscriber.EXPECT().Close().Times(1)
	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(new(big.Int), nil).
		Times(1)

	client := l1.NewClient(subscriber, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	err := client.Run(ctx)
	require.ErrorContains(t, err, "mismatched L1 and L2 networks")
}

func TestEventListener(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	network := utils.Mainnet
	chain := blockchain.New(pebble.NewMemTest(t), network)

	subscriber := mocks.NewMockSubscriber(ctrl)
	subscriber.
		EXPECT().
		WatchLogStateUpdate(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, sink chan<- *contract.StarknetLogStateUpdate) {
			sink <- &contract.StarknetLogStateUpdate{
				GlobalRoot:  new(big.Int),
				BlockNumber: new(big.Int),
				BlockHash:   new(big.Int),
			}
		}).
		Return(newFakeSubscription(), nil).
		Times(1)

	subscriber.
		EXPECT().
		FinalisedHeight(gomock.Any()).
		Return(uint64(0), nil).
		AnyTimes()

	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.DefaultL1ChainID(), nil).
		Times(1)

	subscriber.EXPECT().Close().Times(1)

	var got *core.L1Head
	client := l1.NewClient(subscriber, chain, nopLog).
		WithResubscribeDelay(0).
		WithPollFinalisedInterval(time.Nanosecond).
		WithEventListener(l1.SelectiveListener{
			OnNewL1HeadCb: func(head *core.L1Head) {
				got = head
			},
		})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	require.Equal(t, &core.L1Head{
		BlockHash: new(felt.Felt),
		StateRoot: new(felt.Felt),
	}, got)
}
