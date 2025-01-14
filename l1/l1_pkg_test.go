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
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
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
			chain := blockchain.New(pebble.NewMemTest(t), &network, nil)

			client := NewClient(nil, chain, nopLog, nil).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

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
	network := utils.Mainnet
	chain := blockchain.New(pebble.NewMemTest(t), &network, nil)
	client := NewClient(nil, chain, nopLog, nil).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

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

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
			want := &core.L1Head{
				BlockNumber: block.expectedL2BlockHash.Uint64(),
				BlockHash:   block.expectedL2BlockHash,
				StateRoot:   block.expectedL2BlockHash,
			}
			assert.Equal(t, want, got)
		}
	}
}

func TestMakeSubscribtionsToBootnodes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nopLog := utils.NewNopZapLogger()
	network := utils.Mainnet
	address := common.HexToAddress("0x1234")
	network.BootnodeRegistry = address
	eventsChan := make(chan p2p.BootnodeRegistryEvent, 10)
	chain := blockchain.New(pebble.NewMemTest(t), &network, nil)
	client := NewClient(nil, chain, nopLog, eventsChan).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	storedAddresses := []string{"0x1", "0x2", "0x3"}
	addressesToAdd := []string{"0x4", "0x5", "0x6"}
	addressToRemove := []string{"0x2", "0x5"}

	subscriber := mocks.NewMockSubscriber(ctrl)
	client.l1 = subscriber

	subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(addressesToAdd[:2], nil).Times(1)
	subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sink chan<- *contract.BootnodeRegistryIPAdded) (*fakeSubscription, error) {
			go func() {
				for _, addr := range addressesToAdd {
					sink <- &contract.BootnodeRegistryIPAdded{IpAddress: addr}
				}
			}()
			return newFakeSubscription(), nil
		}).
		Times(1)

	subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sink chan<- *contract.BootnodeRegistryIPRemoved) (*fakeSubscription, error) {
			go func() {
				for _, addr := range addressToRemove {
					sink <- &contract.BootnodeRegistryIPRemoved{IpAddress: addr}
				}
			}()
			return newFakeSubscription(), nil
		}).
		Times(1)

	require.NoError(t, client.makeSubscribtionsToBootnodes(ctx, 1))

	expectedAddressesToAdd := make(map[string]struct{}, len(addressesToAdd)+len(storedAddresses))
	for _, addr := range addressesToAdd {
		expectedAddressesToAdd[addr] = struct{}{}
	}
	for _, addr := range storedAddresses {
		expectedAddressesToAdd[addr] = struct{}{}
	}
	expectedAddressesToRemove := make(map[string]struct{}, len(addressToRemove))
	for _, addr := range addressToRemove {
		expectedAddressesToRemove[addr] = struct{}{}
	}
	select {
	case event := <-eventsChan:
		switch event.EventType {
		case p2p.Add:
			assert.Contains(t, expectedAddressesToAdd, event.Address)
			delete(expectedAddressesToAdd, event.Address)
		case p2p.Remove:
			assert.Contains(t, expectedAddressesToRemove, event.Address)
			delete(expectedAddressesToRemove, event.Address)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected IP address addition event")
	}
}

func TestUnreliableSubscriptionToBootnodes(t *testing.T) {
	t.Parallel()
	address := common.HexToAddress("0x1234")
	network := utils.Mainnet
	network.BootnodeRegistry = address
	chain := blockchain.New(pebble.NewMemTest(t), &network, nil)
	nopLog := utils.NewNopZapLogger()
	err := errors.New("test err")

	testCases := []struct {
		name        string
		setupMock   func(subscriber *mocks.MockSubscriber)
		timeOut     time.Duration
		expectedErr error
	}{
		{
			name: "GetIPAddresses error",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, err).Times(1)
			},
			timeOut:     50 * time.Millisecond,
			expectedErr: err,
		},
		{
			name: "WatchIPAdded error",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(nil, err).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
			},
			timeOut:     time.Millisecond,
			expectedErr: context.DeadlineExceeded,
		},
		{
			name: "WatchIPRemoved error",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(nil, err).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
			},
			timeOut:     50 * time.Millisecond,
			expectedErr: context.DeadlineExceeded,
		},
		{
			name: "Addition subscription error",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(newFakeSubscription(err), nil).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
			},
			timeOut:     50 * time.Millisecond,
			expectedErr: context.DeadlineExceeded,
		},
		{
			name: "Removal subscription error",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(newFakeSubscription(err), nil).Times(1)
				subscriber.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(newFakeSubscription(), nil).Times(1)
			},
			timeOut:     50 * time.Millisecond,
			expectedErr: context.DeadlineExceeded,
		},
		{
			name: "Addition subscription expires",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).DoAndReturn(
					func(_ context.Context, _ common.Address) ([]string, error) {
						time.Sleep(100 * time.Millisecond)
						return nil, nil
					},
				).Times(1)
			},
			expectedErr: context.DeadlineExceeded,
			timeOut:     50 * time.Millisecond,
		},
		{
			name: "Removal subscription expires",
			setupMock: func(subscriber *mocks.MockSubscriber) {
				subscriber.EXPECT().GetIPAddresses(gomock.Any(), address).Return(nil, nil).Times(1)
				subscriber.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, sink chan<- *contract.BootnodeRegistryIPAdded) (*fakeSubscription, error) {
						time.Sleep(100 * time.Millisecond)
						return newFakeSubscription(), nil
					},
				).Times(1)
			},
			timeOut:     50 * time.Millisecond,
			expectedErr: context.DeadlineExceeded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			eventsChan := make(chan p2p.BootnodeRegistryEvent, 10)
			client := Client{
				l1:               nil,
				l2Chain:          chain,
				log:              nopLog,
				network:          chain.Network(),
				resubscribeDelay: 0,
				nonFinalisedLogs: make(map[uint64]*contract.StarknetLogStateUpdate, 0),
				listener:         SelectiveListener{},
				eventsToP2P:      eventsChan,
			}

			subscriber := mocks.NewMockSubscriber(ctrl)
			client.l1 = subscriber
			tc.setupMock(subscriber)

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeOut)
			defer cancel()
			err := client.makeSubscribtionsToBootnodes(ctx, 1)
			require.ErrorIs(t, err, tc.expectedErr)
		})
	}
}
