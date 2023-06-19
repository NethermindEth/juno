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
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/l1/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	l1BlockNumber       uint64
	updates             []*logStateUpdate
	expectedL2BlockHash *felt.Felt
}

func (b *l1Block) Header() *types.Header {
	return &types.Header{
		Number: new(big.Int).SetUint64(b.l1BlockNumber),
	}
}

var longSequenceOfBlocks = []*l1Block{
	{
		l1BlockNumber: 1,
		updates: []*logStateUpdate{
			{l1BlockNumber: 1, l2BlockNumber: 1},
			{l1BlockNumber: 1, l2BlockNumber: 2},
		},
	},
	{
		l1BlockNumber: 2,
		updates: []*logStateUpdate{
			{l1BlockNumber: 2, l2BlockNumber: 3},
			{l1BlockNumber: 2, l2BlockNumber: 4},
		},
	},
	{
		l1BlockNumber: 3,
		updates: []*logStateUpdate{
			{l1BlockNumber: 3, l2BlockNumber: 5},
			{l1BlockNumber: 3, l2BlockNumber: 6},
		},
	},
	{
		l1BlockNumber: 4,
		updates: []*logStateUpdate{
			{l1BlockNumber: 4, l2BlockNumber: 7},
			{l1BlockNumber: 4, l2BlockNumber: 8},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(2),
	},
	{
		l1BlockNumber: 5,
		updates: []*logStateUpdate{
			{l1BlockNumber: 5, l2BlockNumber: 9},
		},
		expectedL2BlockHash: new(felt.Felt).SetUint64(4),
	},
}

func TestClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description        string
		confirmationPeriod uint64
		blocks             []*l1Block
	}{
		{
			description: "update L1 head",
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
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
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1, removed: true},
					},
				},
			},
		},
		{
			description:        "wait for confirmation period",
			confirmationPeriod: 1,
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
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
					l1BlockNumber: 1,
					updates:       []*logStateUpdate{},
				},
			},
		},
		{
			description: "handle updates that appear in the same l1 block",
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
			},
		},
		{
			description: "multiple blocks and logs without confirmation period",
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
				{
					l1BlockNumber: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(4),
				},
			},
		},
		{
			description:        "multiple blocks and logs with confirmation period",
			confirmationPeriod: 1,
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					l1BlockNumber: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
				{
					l1BlockNumber:       3,
					updates:             []*logStateUpdate{},
					expectedL2BlockHash: new(felt.Felt).SetUint64(4),
				},
			},
		},
		{
			description:        "multiple blocks with confirmation period and removed log",
			confirmationPeriod: 1,
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					l1BlockNumber: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
				{
					l1BlockNumber: 3,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 4, removed: true},
					},
					expectedL2BlockHash: new(felt.Felt).SetUint64(2),
				},
			},
		},
		{
			description:        "reorg affecting initial updates",
			confirmationPeriod: 2,
			blocks: []*l1Block{
				{
					l1BlockNumber: 1,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 1},
						{l1BlockNumber: 1, l2BlockNumber: 2},
					},
				},
				{
					l1BlockNumber: 2,
					updates: []*logStateUpdate{
						{l1BlockNumber: 2, l2BlockNumber: 3},
						{l1BlockNumber: 2, l2BlockNumber: 4},
					},
				},
				{
					l1BlockNumber: 3,
					updates: []*logStateUpdate{
						{l1BlockNumber: 1, l2BlockNumber: 2, removed: true},
					},
				},
				// Adding another block ensures no previous logs are carried forward.
				// Among possibly other things, this tests that reorg handling removes
				// logs occurring after the reorged log, not before.
				{
					l1BlockNumber: 4,
					updates:       []*logStateUpdate{},
				},
			},
		},
		{
			description:        "long confirmation period",
			confirmationPeriod: 3,
			blocks:             longSequenceOfBlocks,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			nopLog := utils.NewNopZapLogger()
			network := utils.MAINNET
			chain := blockchain.New(pebble.NewMemTest(), network, nopLog)
			client := NewClient(nil, chain, tt.confirmationPeriod, nopLog)

			// We loop over each block and check that the state of the chain aligns with our expectations.
			for _, block := range tt.blocks {
				subscriber := mocks.NewMockSubscriber(ctrl)
				subscriber.
					EXPECT().
					WatchLogStateUpdate(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, sink chan<- *contract.StarknetLogStateUpdate) {
						for _, log := range block.updates {
							sink <- log.ToContractType()
						}
					}).
					Return(newFakeSubscription(), nil).
					Times(1)

				subscriber.
					EXPECT().
					WatchHeader(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, sink chan<- *types.Header) {
						sink <- block.Header()
					}).
					Return(newFakeSubscription(), nil).
					Times(1)

				subscriber.
					EXPECT().
					ChainID(gomock.Any()).
					Return(network.DefaultL1ChainID(), nil).
					Times(1)

				// Replace the subscriber.
				client.l1 = subscriber

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				require.NoError(t, client.Run(ctx))
				cancel()

				got, err := chain.L1Head()
				if block.expectedL2BlockHash == nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					want := &core.L1Head{
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
	network := utils.MAINNET
	chain := blockchain.New(pebble.NewMemTest(), network, nopLog)
	client := NewClient(nil, chain, 3, nopLog)

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
			Return(network.DefaultL1ChainID(), nil).
			Times(1)

		failedHeaderSub := newFakeSubscription(err)
		failedHeaderCall := subscriber.
			EXPECT().
			WatchHeader(gomock.Any(), gomock.Any()).
			Return(failedHeaderSub, nil).
			Times(1)

		successHeaderSub := newFakeSubscription()
		subscriber.
			EXPECT().
			WatchHeader(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, sink chan<- *types.Header) {
				sink <- block.Header()
			}).
			Return(successHeaderSub, nil).
			Times(1).
			After(failedHeaderCall)

		// Replace the subscriber.
		client.l1 = subscriber

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		require.NoError(t, client.Run(ctx))
		cancel()

		// Subscription resources are released.
		assert.True(t, failedUpdateSub.closed)
		assert.True(t, successUpdateSub.closed)
		assert.True(t, failedHeaderSub.closed)
		assert.True(t, successHeaderSub.closed)

		got, err := chain.L1Head()
		if block.expectedL2BlockHash == nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			want := &core.L1Head{
				BlockNumber: block.expectedL2BlockHash.Uint64(),
				BlockHash:   block.expectedL2BlockHash,
				StateRoot:   block.expectedL2BlockHash,
			}
			assert.Equal(t, want, got)
		}
	}
}
