package l1_test

import (
	"context"
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Aliases keep the diff against the original test small — only the
// boundary surface needed renaming, not every reference.
type (
	StateUpdate         = l1.StateUpdate
	MockSettlementLayer = mocks.MockSettlementLayer
)

var (
	NewClient                 = l1.NewClient
	WithResubscribeDelay      = l1.WithResubscribeDelay
	WithPollFinalisedInterval = l1.WithPollFinalisedInterval
	WithCatchUpChunkSize      = l1.WithCatchUpChunkSize
)

type fakeSubscription struct {
	errChan chan error
	closed  bool
}

func newFakeSubscription(errs ...error) *fakeSubscription {
	errChan := make(chan error, 1)
	if len(errs) >= 1 {
		errChan <- errs[0]
	}
	return &fakeSubscription{
		errChan: errChan,
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

type logStateUpdate struct {
	// The number of the L1 block in which the update was emitted.
	l1BlockNumber uint64
	// The L2 block to which the update corresponds.
	l2BlockNumber uint64
	// This update was previously emitted and has now been reorged.
	removed bool
}

func (logSU *logStateUpdate) ToStateUpdate() *StateUpdate {
	return &StateUpdate{
		L2BlockNumber: logSU.l2BlockNumber,
		L2BlockHash:   new(felt.Felt).SetUint64(logSU.l2BlockNumber),
		StateRoot:     new(felt.Felt).SetUint64(logSU.l2BlockNumber),
		L1RefHeight:   logSU.l1BlockNumber,
		Removed:       logSU.removed,
	}
}

type l1Block struct {
	finalisedHeight     uint64
	updates             []*logStateUpdate
	expectedL2BlockHash *felt.Felt
}

// requireL1Head polls chain.L1Head until it reports the expected L2 block
// number, or fails the test after a fixed budget. The expected hash and
// state root are derived from logStateUpdate.ToStateUpdate's convention
// (both equal to the L2 block number cast to a felt).
func requireL1Head(t *testing.T, chain *blockchain.Blockchain, wantL2Block uint64) {
	t.Helper()
	want := core.L1Head{
		BlockNumber: wantL2Block,
		BlockHash:   new(felt.Felt).SetUint64(wantL2Block),
		StateRoot:   new(felt.Felt).SetUint64(wantL2Block),
	}
	require.Eventually(t, func() bool {
		got, err := chain.L1Head()
		return err == nil && assert.ObjectsAreEqual(want, got)
	}, 2*time.Second, 5*time.Millisecond, "L1Head never advanced to l2=%d", wantL2Block)
}

// swappableSettlement is a test-only SettlementLayer that delegates to a
// runtime-swappable inner mock. It lets one Client stay alive across
// scenario iterations while the test rebinds expectations per iteration —
// previously this was achieved by exposing a SetSettlement hatch on
// production Client. The hatch is gone; the indirection lives here.
type swappableSettlement struct {
	inner atomic.Pointer[mocks.MockSettlementLayer]
}

func (s *swappableSettlement) set(m *mocks.MockSettlementLayer) { s.inner.Store(m) }

func (s *swappableSettlement) ChainID(ctx context.Context) (*big.Int, error) {
	return s.inner.Load().ChainID(ctx)
}

func (s *swappableSettlement) FinalisedHeight(ctx context.Context) (uint64, error) {
	return s.inner.Load().FinalisedHeight(ctx)
}

func (s *swappableSettlement) LatestHeight(ctx context.Context) (uint64, error) {
	return s.inner.Load().LatestHeight(ctx)
}

func (s *swappableSettlement) WatchStateUpdate(
	ctx context.Context,
	sink chan<- *StateUpdate,
) (eth.Subscription, error) {
	return s.inner.Load().WatchStateUpdate(ctx, sink)
}

func (s *swappableSettlement) FilterStateUpdate(
	ctx context.Context,
	from, to uint64,
) ([]*StateUpdate, error) {
	return s.inner.Load().FilterStateUpdate(ctx, from, to)
}

func (s *swappableSettlement) Close() { s.inner.Load().Close() }

var longSequenceOfBlocks = []*l1Block{
	{
		updates: []*logStateUpdate{
			{l1BlockNumber: 1, l2BlockNumber: 1},
			{l1BlockNumber: 1, l2BlockNumber: 2},
		},
	},
	{
		finalisedHeight: 1,
		updates: []*logStateUpdate{
			{l1BlockNumber: 2, l2BlockNumber: 3},
			{l1BlockNumber: 2, l2BlockNumber: 4},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(2),
	},
	{
		finalisedHeight: 1,
		updates: []*logStateUpdate{
			{l1BlockNumber: 3, l2BlockNumber: 5},
			{l1BlockNumber: 3, l2BlockNumber: 6},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(2),
	},
	{
		finalisedHeight: 2,
		updates: []*logStateUpdate{
			{l1BlockNumber: 4, l2BlockNumber: 7},
			{l1BlockNumber: 4, l2BlockNumber: 8},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(4),
	},
	{
		finalisedHeight: 5,
		updates: []*logStateUpdate{
			{l1BlockNumber: 5, l2BlockNumber: 9},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(9),
	},
}

func TestClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description string
		blocks      []*l1Block
	}{
		{
			description: "update L1 head",
			blocks: []*l1Block{
				{
					finalisedHeight: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(1),
				},
			},
		},
		{
			description: "ignore removed log",
			blocks: []*l1Block{
				{
					finalisedHeight: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3, removed: true},
					},
				},
			},
		},
		{
			description: "wait for log to be finalised",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
					},
				},
			},
		},
		{
			description: "do not update without logs",
			blocks: []*l1Block{
				{
					finalisedHeight: 1,
					updates:         []*logStateUpdate{},
				},
			},
		},
		{
			description: "handle updates that appear in the same l1 block",
			blocks: []*l1Block{
				{
					finalisedHeight: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
			},
		},
		{
			description: "multiple blocks and logs finalised every block",
			blocks: []*l1Block{
				{
					finalisedHeight: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
				{
					finalisedHeight: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(4),
				},
			},
		},
		{
			description: "multiple blocks and logs finalised irregularly",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
				},
				{
					finalisedHeight:     2,
					updates:             []*logStateUpdate{},
					expectedL2BlockHash: new(felt.Felt).SetUint64(4),
				},
			},
		},
		{
			description: "multiple blocks with removed log",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					finalisedHeight: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
				{
					// catchUp's setL1Head fires before the removed event in the
					// channel is drained, so the leftover {l1=2,l2=4} entry from
					// the previous block (now finalised at finalisedHeight=2)
					// gets committed as the L1 head. In production this stale-
					// state path can't happen because each process starts with
					// an empty nonFinalisedLogs map.
					finalisedHeight: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 4, removed: true},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(4),
				},
			},
		},
		{
			description: "reorg then finalise earlier block",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 2},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 2, removed: true},
					},
				},
				{
					finalisedHeight:     1,
					updates:             []*logStateUpdate{},
					expectedL2BlockHash: new(felt.Felt).SetUint64(1),
				},
			},
		},
		{
			description: "reorg then finalise later block",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 2},
						{l1BlockNumber: 2, l2BlockNumber: 3},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 3, l2BlockNumber: 4},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 2, removed: true},
					},
				},
				{
					finalisedHeight:     2,
					updates:             []*logStateUpdate{},
					expectedL2BlockHash: new(felt.Felt).SetUint64(1),
				},
			},
		},
		{
			description: "reorg affecting initial updates",
			blocks: []*l1Block{
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
				},
				{
					finalisedHeight: 0,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 2, removed: true},
					},
				},
			},
		},
		{
			description: "long sequence of blocks",
			blocks:      longSequenceOfBlocks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			nopLog := log.NewNopZapLogger()
			network := networks.Mainnet
			chain := blockchain.New(
				memory.New(),
				&network,
				blockchain.WithNewState(statetestutils.UseNewState()),
			)

			swap := &swappableSettlement{}
			client := NewClient(
				swap,
				chain,
				nopLog,
				WithResubscribeDelay(0),
				WithPollFinalisedInterval(time.Nanosecond),
			)

			// We loop over each block and check that the state agrees with our expectations.
			for _, block := range tt.blocks {
				subscriber := mocks.NewMockSettlementLayer(ctrl)
				subscriber.
					EXPECT().
					WatchStateUpdate(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, sink chan<- *StateUpdate) {
						for _, update := range block.updates {
							sink <- update.ToStateUpdate()
						}
					}).
					Return(newFakeSubscription(), nil).
					Times(1)

				subscriber.
					EXPECT().
					FinalisedHeight(gomock.Any()).
					Return(block.finalisedHeight, nil).
					AnyTimes()

				subscriber.
					EXPECT().
					LatestHeight(gomock.Any()).
					Return(block.finalisedHeight, nil).
					AnyTimes()

				subscriber.
					EXPECT().
					FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil).
					AnyTimes()

				subscriber.
					EXPECT().
					ChainID(gomock.Any()).
					Return(network.L1ChainID, nil).
					Times(1)

				subscriber.EXPECT().Close().Times(1)

				swap.set(subscriber)

				ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
				require.NoError(t, client.Run(ctx))
				cancel()

				got, err := chain.L1Head()
				if block.expectedL2BlockHash == nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					want := core.L1Head{
						BlockNumber: block.expectedL2BlockHash.Uint64(),
						BlockHash:   block.expectedL2BlockHash,
						StateRoot:   block.expectedL2BlockHash,
					}
					assert.Equal(t, want, got)
				}
			}
		})
	}
}

func TestUnreliableSubscription(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	swap := &swappableSettlement{}
	client := NewClient(
		swap,
		chain,
		nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Nanosecond),
	)

	err := errors.New("test err")
	for _, block := range longSequenceOfBlocks {
		subscriber := mocks.NewMockSettlementLayer(ctrl)

		// The subscription returns an error on each block.
		// Each time, a second subscription succeeds.

		failedUpdateSub := newFakeSubscription(err)
		failedUpdateCall := subscriber.
			EXPECT().
			WatchStateUpdate(gomock.Any(), gomock.Any()).
			Return(failedUpdateSub, nil).
			Times(1)

		successUpdateSub := newFakeSubscription()
		subscriber.
			EXPECT().
			WatchStateUpdate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, sink chan<- *StateUpdate) {
				for _, log := range block.updates {
					sink <- log.ToStateUpdate()
				}
			}).
			Return(successUpdateSub, nil).
			Times(1).
			After(failedUpdateCall)

		subscriber.
			EXPECT().
			ChainID(gomock.Any()).
			Return(network.L1ChainID, nil).
			Times(1)

		subscriber.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			Return(block.finalisedHeight, nil).
			AnyTimes()

		subscriber.
			EXPECT().
			LatestHeight(gomock.Any()).
			Return(block.finalisedHeight, nil).
			AnyTimes()

		subscriber.
			EXPECT().
			FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes()

		subscriber.EXPECT().Close().Times(1)

		// Replace the subscriber.
		swap.set(subscriber)

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		require.NoError(t, client.Run(ctx))
		cancel()

		// Subscription resources are released.
		assert.True(t, failedUpdateSub.closed)
		assert.True(t, successUpdateSub.closed)

		got, err := chain.L1Head()
		if block.expectedL2BlockHash == nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			want := core.L1Head{
				BlockNumber: block.expectedL2BlockHash.Uint64(),
				BlockHash:   block.expectedL2BlockHash,
				StateRoot:   block.expectedL2BlockHash,
			}
			assert.Equal(t, want, got)
		}
	}
}

// newCatchUpFixture builds the boilerplate every catch-up test repeats:
// a fresh chain, a mock subscriber wired with the chain-id check, an idle
// live subscription, and the final Close expectation. Per-test variation
// (heights, FilterStateUpdate calls, client options) stays in the test.
func newCatchUpFixture(t *testing.T) (*blockchain.Blockchain, *MockSettlementLayer) {
	t.Helper()
	ctrl := gomock.NewController(t)
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	subscriber := mocks.NewMockSettlementLayer(ctrl)
	subscriber.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
	subscriber.
		EXPECT().
		WatchStateUpdate(gomock.Any(), gomock.Any()).
		Return(newFakeSubscription(), nil).
		AnyTimes()
	subscriber.EXPECT().Close().Times(1)
	return chain, subscriber
}

func TestCatchUpSetsL1HeadOnStart(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize = 10. LatestHeight=10, FinalisedHeight=5 => one.
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).Times(1)
	subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(5), nil).AnyTimes()

	backfilled := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 7}).ToStateUpdate()
	subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(1), uint64(10)).
		Return([]*StateUpdate{backfilled}, nil).
		Times(1)

	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Hour),
		WithCatchUpChunkSize(10),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	got, err := chain.L1Head()
	require.NoError(t, err)
	assert.Equal(t, core.L1Head{
		BlockNumber: 7,
		BlockHash:   new(felt.Felt).SetUint64(7),
		StateRoot:   new(felt.Felt).SetUint64(7),
	}, got)
}

func TestCatchUpMultiChunk(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize = 10. LatestHeight=25, FinalisedHeight=5 forces three
	// filter calls:
	//   chunk 1: [16, 25]  -> from=16 > finalised=5, continue
	//   chunk 2: [6,  15]  -> from=6  > finalised=5, continue
	//   chunk 3: [0,   5]  -> from=0 <= finalised, stop
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(25), nil).Times(1)
	subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(5), nil).AnyTimes()

	firstEvent := (&logStateUpdate{l1BlockNumber: 20, l2BlockNumber: 50}).ToStateUpdate()
	thirdEvent := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 25}).ToStateUpdate()

	firstCall := subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(16), uint64(25)).
		Return([]*StateUpdate{firstEvent}, nil).
		Times(1)
	secondCall := subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(6), uint64(15)).
		Return(nil, nil).
		Times(1).
		After(firstCall)
	subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(0), uint64(5)).
		Return([]*StateUpdate{thirdEvent}, nil).
		Times(1).
		After(secondCall)

	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Hour),
		WithCatchUpChunkSize(10),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	// Only thirdEvent (l1=3) is at or below finalised=5, so it becomes L1Head.
	got, err := chain.L1Head()
	require.NoError(t, err)
	assert.Equal(t, core.L1Head{
		BlockNumber: 25,
		BlockHash:   new(felt.Felt).SetUint64(25),
		StateRoot:   new(felt.Felt).SetUint64(25),
	}, got)
}

func TestCatchUpFilterError(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(100), nil).Times(1)
	subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(80), nil).AnyTimes()

	rpcErr := errors.New("rpc broken")
	subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, rpcErr).
		Times(1)

	// Best-effort: catch-up error must NOT terminate Run. It logs and falls
	// through to the live subscription, which we let idle until ctx expires.
	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Hour),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	// Filter failed and the subscription delivered nothing → no L1 head written.
	_, err := chain.L1Head()
	require.Error(t, err)
}

// TestCatchUpHeadAndCachePartition feeds a single chunk with a mix of
// finalised and non-finalised events and verifies the post-setL1Head
// partition:
//   - finalised entries (l1=2,3,5) are promoted/evicted during catch-up, the
//     highest of them (l1=5) wins as the initial L1 head;
//   - non-finalised entries (l1=7,9) are buffered. We verify they survived
//     by walking the finalised height forward across the live-poll ticks and
//     observing L1Head promote each one in turn.
func TestCatchUpHeadAndCachePartition(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize default 1000. LatestHeight=10, initial FinalisedHeight=5
	// → single chunk [0, 10]. Five events span the finalised cutoff:
	//   l1=2,3,5  (<= finalised) → all deleted from cache, l1=5 wins as head
	//   l1=7,9    (>  finalised) → remain buffered for the live loop
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).Times(1)

	// Finalised height is dynamic across the test: starts at 5 (so catch-up
	// commits l1=5 as head and buffers l1=7,9), then the test bumps it to
	// promote each buffered entry via the live-poll tick.
	var finalised atomic.Uint64
	finalised.Store(5)
	subscriber.
		EXPECT().
		FinalisedHeight(gomock.Any()).
		DoAndReturn(func(_ context.Context) (uint64, error) {
			return finalised.Load(), nil
		}).
		AnyTimes()

	finalisedLow := (&logStateUpdate{l1BlockNumber: 2, l2BlockNumber: 20}).ToStateUpdate()
	finalisedMid := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 30}).ToStateUpdate()
	finalisedTop := (&logStateUpdate{l1BlockNumber: 5, l2BlockNumber: 50}).ToStateUpdate()
	pendingLow := (&logStateUpdate{l1BlockNumber: 7, l2BlockNumber: 70}).ToStateUpdate()
	pendingHigh := (&logStateUpdate{l1BlockNumber: 9, l2BlockNumber: 90}).ToStateUpdate()

	subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(0), uint64(10)).
		Return([]*StateUpdate{
			finalisedLow, finalisedMid, finalisedTop, pendingLow, pendingHigh,
		}, nil).
		Times(1)

	// Short poll interval so the live loop ticks setL1Head several times
	// during the test budget.
	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- client.Run(ctx) }()

	// Step 1: catch-up commits the highest finalised event (l1=5) as L1 head.
	requireL1Head(t, chain, 50)

	// Step 2: bump finalised so the live loop's setL1Head promotes l1=7.
	// If catch-up failed to buffer l1=7, L1Head can never advance past l2=50.
	finalised.Store(7)
	requireL1Head(t, chain, 70)

	// Step 3: same proof for l1=9 — must have been buffered by catch-up.
	finalised.Store(9)
	requireL1Head(t, chain, 90)

	cancel()
	require.NoError(t, <-runErr)
}

// TestCatchUpPartialProgressPreserved asserts the best-effort contract: when
// a backward chunk filter call errors mid-walk, entries already merged into
// nonFinalisedLogs by earlier successful chunks must remain available to the
// live subscription's setL1Head, instead of being rolled back. Observed via
// promotion: once the finalised height catches up to the buffered entry, the
// live-poll setL1Head commits it as L1 head.
func TestCatchUpPartialProgressPreserved(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize = 1000. LatestHeight=3000, initial FinalisedHeight=2000:
	//   chunk 1: [2001, 3000] -> succeeds with non-finalised event at l1=2500
	//                            (2500 > 2000 finalised, so foundFinalised=false,
	//                             loop continues to next chunk)
	//   chunk 2: [1001, 2000] -> errors, catch-up bails out
	// The chunk-1 event must still be sitting in nonFinalisedLogs after Run.
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(3000), nil).Times(1)

	// Finalised height starts at 2000 so the chunk-1 event (l1=2500) stays
	// buffered. The test later bumps it past 2500 so the live-poll setL1Head
	// can promote — that promotion only succeeds if catch-up preserved the
	// entry across the chunk-2 error.
	var finalised atomic.Uint64
	finalised.Store(2000)
	subscriber.
		EXPECT().
		FinalisedHeight(gomock.Any()).
		DoAndReturn(func(_ context.Context) (uint64, error) {
			return finalised.Load(), nil
		}).
		AnyTimes()

	chunkOneEvent := (&logStateUpdate{l1BlockNumber: 2500, l2BlockNumber: 42}).ToStateUpdate()
	rpcErr := errors.New("rpc broken")

	firstCall := subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(2001), uint64(3000)).
		Return([]*StateUpdate{chunkOneEvent}, nil).
		Times(1)
	subscriber.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(1001), uint64(2000)).
		Return(nil, rpcErr).
		Times(1).
		After(firstCall)

	// Short poll interval so the live loop ticks setL1Head shortly after Run
	// transitions out of catch-up.
	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- client.Run(ctx) }()

	// Give the live poll a few ticks before bumping finalised. With initial
	// finalised=2000 and the chunk-1 event at l1=2500, setL1Head finds nothing
	// to promote, so L1Head must remain unset.
	time.Sleep(50 * time.Millisecond)
	_, headErr := chain.L1Head()
	require.Error(t, headErr, "L1Head must remain unset while finalised < 2500")

	// Bump finalised past l1=2500. The live-poll setL1Head must find the
	// chunk-1 entry still buffered and promote it. If catch-up rolled state
	// back on chunk-2's error, L1Head never advances.
	finalised.Store(2500)
	requireL1Head(t, chain, 42)

	cancel()
	require.NoError(t, <-runErr)
}

// TestFinalisedHeightRetryReturnsPromptlyOnCancel drives Run and forces
// the inner finalisedHeight retry loop into its inter-attempt wait: ChainID
// and LatestHeight succeed (catch-up gets past its preamble) but every
// FinalisedHeight call errors. Catch-up bails best-effort; Run enters the
// live poll loop which ticks setL1Head → finalisedHeight retry → blocks on
// resubscribeDelay. Cancel ctx, assert Run returns without burning the full
// hour of virtual time.
func TestFinalisedHeightRetryReturnsPromptlyOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		network := networks.Mainnet
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		subscriber := mocks.NewMockSettlementLayer(ctrl)
		subscriber.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(0), nil).AnyTimes()
		subscriber.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			Return(uint64(0), errors.New("boom")).
			MinTimes(1)
		subscriber.
			EXPECT().
			WatchStateUpdate(gomock.Any(), gomock.Any()).
			Return(newFakeSubscription(), nil).
			AnyTimes()
		subscriber.EXPECT().Close().Times(1)

		client := NewClient(subscriber, chain, nopLog,
			WithResubscribeDelay(time.Hour),
			WithPollFinalisedInterval(time.Nanosecond),
		)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- client.Run(ctx) }()

		// Wait until Run is durably blocked inside finalisedHeight's
		// inter-attempt timer.
		synctest.Wait()
		cancel()

		require.NoError(t, <-done)
		require.Less(t, time.Since(start), time.Minute,
			"Run stalled in finalisedHeight retry after ctx cancel")
	})
}

// TestSubscribeRetryReturnsPromptlyOnCancel is the same check for the other
// retry loop: WatchStateUpdate fails repeatedly inside subscribeToUpdates,
// the loop enters its inter-attempt wait, ctx is cancelled, and Run must
// return without consuming resubscribeDelay.
func TestSubscribeRetryReturnsPromptlyOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		network := networks.Mainnet
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		subscriber := mocks.NewMockSettlementLayer(ctrl)
		subscriber.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(0), nil).AnyTimes()
		subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(0), nil).AnyTimes()
		subscriber.
			EXPECT().
			FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes()
		subscriber.
			EXPECT().
			WatchStateUpdate(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("boom")).
			MinTimes(1)
		subscriber.EXPECT().Close().Times(1)

		client := NewClient(subscriber, chain, nopLog, WithResubscribeDelay(time.Hour))

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- client.Run(ctx) }()

		// Wait until Run is durably blocked inside subscribeToUpdates'
		// inter-attempt timer.
		synctest.Wait()
		cancel()

		require.NoError(t, <-done)
		require.Less(t, time.Since(start), time.Minute,
			"Run stalled in subscribeToUpdates retry after ctx cancel")
	})
}
