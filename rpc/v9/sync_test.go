package rpcv9_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSyncing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, synchronizer, nil, nil)
	defaultSyncState := false

	startingBlockHeader := &core.Header{Number: 0, Hash: &felt.Zero}
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
	synchronizer.EXPECT().HighestBlockHeader().Return(
		&core.Header{Number: 2, Hash: new(felt.Felt).SetUint64(2)},
	)
	synchronizer.EXPECT().StartingBlockHeader().Return(nil, errors.New("nope"))
	t.Run("undefined starting block header", func(t *testing.T) {
		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
	synchronizer.EXPECT().HighestBlockHeader().Return(
		&core.Header{Number: 2, Hash: new(felt.Felt).SetUint64(2)},
	)
	synchronizer.EXPECT().StartingBlockHeader().Return(nil, nil)
	t.Run("nil starting block header", func(t *testing.T) {
		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	synchronizer.EXPECT().StartingBlockHeader().Return(startingBlockHeader, nil).AnyTimes()
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	synchronizer.EXPECT().HighestBlockHeader().Return(nil).Times(2)
	t.Run("undefined highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("block height is greater than highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	synchronizer.EXPECT().HighestBlockHeader().Return(
		&core.Header{
			Number: 2,
			Hash:   new(felt.Felt).SetUint64(2),
		},
	).Times(2)
	t.Run("block height is equal to highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 2}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("syncing", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(
			&core.Header{
				Number: 1,
				Hash:   new(felt.Felt).SetUint64(1),
			},
			nil,
		)

		currentBlockNumber := uint64(1)
		highestBlockNumber := uint64(2)
		expectedSyncing := &rpc.Sync{
			StartingBlockHash:   &felt.Zero,
			StartingBlockNumber: &startingBlockHeader.Number,
			CurrentBlockHash:    new(felt.Felt).SetUint64(1),
			CurrentBlockNumber:  &currentBlockNumber,
			HighestBlockHash:    new(felt.Felt).SetUint64(2),
			HighestBlockNumber:  &highestBlockNumber,
		}
		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, expectedSyncing, syncing)
	})
}
