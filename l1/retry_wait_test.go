package l1

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type retryTestSubscription struct {
	errChan chan error
}

func newRetryTestSubscription() *retryTestSubscription {
	return &retryTestSubscription{errChan: make(chan error)}
}

func (s *retryTestSubscription) Err() <-chan error {
	return s.errChan
}

func (s *retryTestSubscription) Unsubscribe() {
	close(s.errChan)
}

func TestSubscribeToUpdatesReturnsPromptlyOnContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		subscriber := mocks.NewMockSubscriber(ctrl)
		resubscribeAttempted := make(chan struct{}, 1)

		subscriber.
			EXPECT().
			WatchLogStateUpdate(gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
				select {
				case resubscribeAttempted <- struct{}{}:
				default:
				}
				return newRetryTestSubscription(), errors.New("boom")
			}).
			AnyTimes()

		client := &Client{
			l1:               subscriber,
			logger:           log.NewNopZapLogger(),
			resubscribeDelay: time.Hour,
		}

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() {
			_, err := client.subscribeToUpdates(ctx, make(chan *contract.StarknetLogStateUpdate))
			errCh <- err
		}()

		<-resubscribeAttempted
		cancel()

		err := <-errCh
		require.ErrorContains(t, err, "context canceled before resubscribe was successful")
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestFinalisedHeightRetryLoopReturnsPromptlyOnContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		subscriber := mocks.NewMockSubscriber(ctrl)
		retryWaitStarted := make(chan struct{}, 1)

		subscriber.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			DoAndReturn(func(ctx context.Context) (uint64, error) {
				select {
				case retryWaitStarted <- struct{}{}:
				default:
				}
				<-ctx.Done()
				return 0, ctx.Err()
			}).
			Times(1)

		client := &Client{
			l1:               subscriber,
			logger:           log.NewNopZapLogger(),
			resubscribeDelay: time.Hour,
		}

		ctx, cancel := context.WithCancel(t.Context())
		resultCh := make(chan uint64, 1)
		go func() {
			resultCh <- client.finalisedHeight(ctx)
		}()

		<-retryWaitStarted
		cancel()

		require.Zero(t, <-resultCh)
	})
}
