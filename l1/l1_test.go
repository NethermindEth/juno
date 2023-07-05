package l1_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

	network := utils.MAINNET
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(), network, nopLog)

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

	client := l1.NewClient(subscriber, chain, nopLog)

	require.ErrorIs(t, client.Run(context.Background()), err)
}

func TestMismatchedChainID(t *testing.T) {
	t.Parallel()

	network := utils.MAINNET
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(), network, nopLog)

	subscriber := mocks.NewMockSubscriber(ctrl)

	subscriber.EXPECT().Close().Times(1)
	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(new(big.Int), nil).
		Times(1)

	client := l1.NewClient(subscriber, chain, nopLog)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	err := client.Run(ctx)
	require.ErrorContains(t, err, "mismatched L1 and L2 networks")
}
