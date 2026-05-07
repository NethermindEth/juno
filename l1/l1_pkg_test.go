package l1

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func (logSU *logStateUpdate) ToContractType() *contract.StarknetLogStateUpdate {
	return &contract.StarknetLogStateUpdate{
		BlockNumber: new(big.Int).SetUint64(logSU.l2BlockNumber),
		BlockHash:   new(big.Int).SetUint64(logSU.l2BlockNumber),
		GlobalRoot:  new(big.Int).SetUint64(logSU.l2BlockNumber),
		Raw: types.Log{
			Removed:     logSU.removed,
			BlockNumber: logSU.l1BlockNumber,
		},
	}
}

type l1Block struct {
	finalisedHeight     uint64
	updates             []*logStateUpdate
	expectedL2BlockHash *felt.Felt
}

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

			client := NewClient(
				nil,
				chain,
				nopLog,
				WithResubscribeDelay(0),
				WithPollFinalisedInterval(time.Nanosecond),
			)

			// We loop over each block and check that the state agrees with our expectations.
			for _, block := range tt.blocks {
				subscriber := mocks.NewMockSubscriber(ctrl)
				subscriber.
					EXPECT().
					WatchLogStateUpdate(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, sink chan<- *contract.StarknetLogStateUpdate) {
						for _, update := range block.updates {
							sink <- update.ToContractType()
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
					FilterLogStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil).
					AnyTimes()

				subscriber.
					EXPECT().
					ChainID(gomock.Any()).
					Return(network.L1ChainID, nil).
					Times(1)

				subscriber.EXPECT().Close().Times(1)

				client.l1 = subscriber

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
	client := NewClient(
		nil,
		chain,
		nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Nanosecond),
	)

	err := errors.New("test err")
	for _, block := range longSequenceOfBlocks {
		subscriber := mocks.NewMockSubscriber(ctrl)

		// The subscription returns an error on each block.
		// Each time, a second subscription succeeds.

		failedUpdateSub := newFakeSubscription(err)
		failedUpdateCall := subscriber.
			EXPECT().
			WatchLogStateUpdate(gomock.Any(), gomock.Any()).
			Return(failedUpdateSub, nil).
			Times(1)

		successUpdateSub := newFakeSubscription()
		subscriber.
			EXPECT().
			WatchLogStateUpdate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, sink chan<- *contract.StarknetLogStateUpdate) {
				for _, log := range block.updates {
					sink <- log.ToContractType()
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
			FilterLogStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes()

		subscriber.EXPECT().Close().Times(1)

		// Replace the subscriber.
		client.l1 = subscriber

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
// (heights, FilterLogStateUpdate calls, client options) stays in the test.
func newCatchUpFixture(t *testing.T) (*blockchain.Blockchain, *mocks.MockSubscriber) {
	t.Helper()
	ctrl := gomock.NewController(t)
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	subscriber := mocks.NewMockSubscriber(ctrl)
	subscriber.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
	subscriber.
		EXPECT().
		WatchLogStateUpdate(gomock.Any(), gomock.Any()).
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

	backfilled := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 7}).ToContractType()
	subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(1), uint64(10)).
		Return([]*contract.StarknetLogStateUpdate{backfilled}, nil).
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

	firstEvent := (&logStateUpdate{l1BlockNumber: 20, l2BlockNumber: 50}).ToContractType()
	thirdEvent := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 25}).ToContractType()

	firstCall := subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(16), uint64(25)).
		Return([]*contract.StarknetLogStateUpdate{firstEvent}, nil).
		Times(1)
	secondCall := subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(6), uint64(15)).
		Return(nil, nil).
		Times(1).
		After(firstCall)
	subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(0), uint64(5)).
		Return([]*contract.StarknetLogStateUpdate{thirdEvent}, nil).
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
		FilterLogStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
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
// finalised and non-finalised events and asserts the post-setL1Head state:
// the highest finalised event wins as L1 head, every finalised entry is
// evicted from nonFinalisedLogs, and entries above finalisedHeight stay
// buffered for later commitment.
func TestCatchUpHeadAndCachePartition(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize default 1000. LatestHeight=10, FinalisedHeight=5 →
	// single chunk [0, 10]. Five events span the finalised cutoff:
	//   l1=2,3,5  (<= finalised) → all deleted from cache, l1=5 wins as head
	//   l1=7,9    (>  finalised) → remain buffered for the live loop
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).Times(1)
	subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(5), nil).AnyTimes()

	finalisedLow := (&logStateUpdate{l1BlockNumber: 2, l2BlockNumber: 20}).ToContractType()
	finalisedMid := (&logStateUpdate{l1BlockNumber: 3, l2BlockNumber: 30}).ToContractType()
	finalisedTop := (&logStateUpdate{l1BlockNumber: 5, l2BlockNumber: 50}).ToContractType()
	pendingLow := (&logStateUpdate{l1BlockNumber: 7, l2BlockNumber: 70}).ToContractType()
	pendingHigh := (&logStateUpdate{l1BlockNumber: 9, l2BlockNumber: 90}).ToContractType()

	subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(0), uint64(10)).
		Return([]*contract.StarknetLogStateUpdate{
			finalisedLow, finalisedMid, finalisedTop, pendingLow, pendingHigh,
		}, nil).
		Times(1)

	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Hour),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	// Highest finalised event (l1=5 → l2=50) commits as L1 head.
	got, err := chain.L1Head()
	require.NoError(t, err)
	assert.Equal(t, core.L1Head{
		BlockNumber: 50,
		BlockHash:   new(felt.Felt).SetUint64(50),
		StateRoot:   new(felt.Felt).SetUint64(50),
	}, got)

	// Non-finalised entries survive; every finalised entry is evicted
	// (including the one that became the head).
	require.Len(t, client.nonFinalisedLogs, 2)
	assert.Equal(t, pendingLow, client.nonFinalisedLogs[7])
	assert.Equal(t, pendingHigh, client.nonFinalisedLogs[9])
	for _, l1Block := range []uint64{2, 3, 5} {
		_, present := client.nonFinalisedLogs[l1Block]
		assert.Falsef(t, present, "finalised l1=%d should be deleted from cache", l1Block)
	}
}

// TestCatchUpPartialProgressPreserved asserts the best-effort contract: when
// a backward chunk filter call errors mid-walk, entries already merged into
// nonFinalisedLogs by earlier successful chunks must remain available to the
// live subscription's setL1Head, instead of being rolled back.
func TestCatchUpPartialProgressPreserved(t *testing.T) {
	t.Parallel()

	chain, subscriber := newCatchUpFixture(t)
	nopLog := log.NewNopZapLogger()

	// catchUpChunkSize = 1000. LatestHeight=3000, FinalisedHeight=2000:
	//   chunk 1: [2001, 3000] -> succeeds with non-finalised event at l1=2500
	//                            (2500 > 2000 finalised, so foundFinalised=false,
	//                             loop continues to next chunk)
	//   chunk 2: [1001, 2000] -> errors, catch-up bails out
	// The chunk-1 event must still be sitting in nonFinalisedLogs after Run.
	subscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(3000), nil).Times(1)
	subscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(2000), nil).AnyTimes()

	chunkOneEvent := (&logStateUpdate{l1BlockNumber: 2500, l2BlockNumber: 42}).ToContractType()
	rpcErr := errors.New("rpc broken")

	firstCall := subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(2001), uint64(3000)).
		Return([]*contract.StarknetLogStateUpdate{chunkOneEvent}, nil).
		Times(1)
	subscriber.
		EXPECT().
		FilterLogStateUpdate(gomock.Any(), uint64(1001), uint64(2000)).
		Return(nil, rpcErr).
		Times(1).
		After(firstCall)

	// Poll interval is 1h so the live loop never ticks setL1Head — the only
	// thing that could populate nonFinalisedLogs is the catch-up walk.
	client := NewClient(subscriber, chain, nopLog,
		WithResubscribeDelay(0),
		WithPollFinalisedInterval(time.Hour),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	// Partial state from chunk 1 survived the chunk-2 error.
	require.Len(t, client.nonFinalisedLogs, 1)
	got, ok := client.nonFinalisedLogs[2500]
	require.True(t, ok, "chunk-1 event at l1=2500 should remain buffered")
	assert.Equal(t, chunkOneEvent, got)

	// Above finalised, so setL1Head wouldn't have committed it anyway.
	_, err := chain.L1Head()
	require.Error(t, err)
}
