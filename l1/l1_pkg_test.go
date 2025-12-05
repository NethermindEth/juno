package l1

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
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

func (log *logStateUpdate) ToContractType() *contract.StarknetLogStateUpdate {
	return &contract.StarknetLogStateUpdate{
		BlockNumber: new(big.Int).SetUint64(log.l2BlockNumber),
		BlockHash:   new(big.Int).SetUint64(log.l2BlockNumber),
		GlobalRoot:  new(big.Int).SetUint64(log.l2BlockNumber),
		Raw: types.Log{
			Removed:     log.removed,
			BlockNumber: log.l1BlockNumber,
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
					finalisedHeight: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 4, removed: true},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
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
			nopLog := utils.NewNopZapLogger()
			network := utils.Mainnet
			chain := blockchain.New(memory.New(), &network, statetestutils.UseNewState())

			client := NewClient(nil, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

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
	nopLog := utils.NewNopZapLogger()
	network := utils.Mainnet
	chain := blockchain.New(memory.New(), &network, statetestutils.UseNewState())
	client := NewClient(nil, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

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
