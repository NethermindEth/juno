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
	"github.com/NethermindEth/juno/l1/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/core/types"
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

func TestFailedSubscription(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")

	tests := []struct {
		description     string
		watchUpdateRets []any
		watchHeaderRets []any
	}{
		{
			description:     "fail to create updates subscription",
			watchUpdateRets: []any{newFakeSubscription(), err},
			watchHeaderRets: []any{newFakeSubscription(), nil},
		},
		{
			description:     "fail to create header subscription",
			watchUpdateRets: []any{newFakeSubscription(), nil},
			watchHeaderRets: []any{newFakeSubscription(), err},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			network := utils.MAINNET
			ctrl := gomock.NewController(t)
			nopLog := utils.NewNopZapLogger()
			chain := blockchain.New(pebble.NewMemTest(), network, nopLog)

			subscriber := mocks.NewMockSubscriber(ctrl)

			subscriber.
				EXPECT().
				WatchLogStateUpdate(gomock.Any(), gomock.Any()).
				Return(tt.watchUpdateRets...).
				AnyTimes()

			subscriber.
				EXPECT().
				WatchHeader(gomock.Any(), gomock.Any()).
				Do(func(_ context.Context, sink chan<- *types.Header) {
					sink <- &types.Header{
						Number: new(big.Int),
					}
				}).
				Return(tt.watchHeaderRets...).
				AnyTimes()

			subscriber.
				EXPECT().
				ChainID(gomock.Any()).
				Return(network.DefaultL1ChainID(), nil).
				Times(1)

			client := l1.NewClient(subscriber, chain, 0, nopLog)

			require.ErrorIs(t, client.Run(context.Background()), err)
		})
	}
}

func TestChainID(t *testing.T) {
	t.Parallel()

	helper := func(t *testing.T, matching bool) error {
		t.Helper()

		network := utils.MAINNET
		ctrl := gomock.NewController(t)
		nopLog := utils.NewNopZapLogger()
		chain := blockchain.New(pebble.NewMemTest(), network, nopLog)

		subscriber := mocks.NewMockSubscriber(ctrl)

		subscriber.
			EXPECT().
			WatchLogStateUpdate(gomock.Any(), gomock.Any()).
			Return(newFakeSubscription(), nil).
			AnyTimes()

		subscriber.
			EXPECT().
			WatchHeader(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, sink chan<- *types.Header) {
				sink <- &types.Header{
					Number: new(big.Int),
				}
			}).
			Return(newFakeSubscription(), nil).
			AnyTimes()

		l1ChainID := new(big.Int)
		if matching {
			l1ChainID.Set(network.DefaultL1ChainID())
		}

		subscriber.
			EXPECT().
			ChainID(gomock.Any()).
			Return(l1ChainID, nil).
			Times(1)

		client := l1.NewClient(subscriber, chain, 0, nopLog)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return client.Run(ctx)
	}

	t.Run("matching chain IDs", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, helper(t, true))
	})

	t.Run("mismatched chain IDs", func(t *testing.T) {
		t.Parallel()
		require.Error(t, helper(t, false))
	})
}
