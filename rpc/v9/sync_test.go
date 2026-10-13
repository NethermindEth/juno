package rpcv9_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Run("undefined starting block header", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(
			&core.Header{Number: 2, Hash: new(felt.Felt).SetUint64(2)},
		)
		synchronizer.EXPECT().StartingBlockHeader().Return(nil, errors.New("nope"))

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	t.Run("undefined highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("block height is greater than highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	t.Run("block height is equal to highest block", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 2}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(
			&core.Header{
				Number: 2,
				Hash:   new(felt.Felt).SetUint64(2),
			},
		)

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
		synchronizer.EXPECT().HighestBlockHeader().Return(
			&core.Header{
				Number: 2,
				Hash:   new(felt.Felt).SetUint64(2),
			},
		)
		synchronizer.EXPECT().StartingBlockHeader().Return(startingBlockHeader, nil)

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

func TestSyncMarshalJSON(t *testing.T) {
	t.Run("not syncing", func(t *testing.T) {
		notSyncing := false
		data, err := json.Marshal(rpc.Sync{Syncing: &notSyncing})
		require.NoError(t, err)
		assert.JSONEq(t, `false`, string(data))
	})

	t.Run("syncing", func(t *testing.T) {
		start, current, highest := uint64(1), uint64(2), uint64(3)
		data, err := json.Marshal(rpc.Sync{
			StartingBlockHash:   felt.NewFromUint64[felt.Felt](0xa),
			StartingBlockNumber: &start,
			CurrentBlockHash:    felt.NewFromUint64[felt.Felt](0xb),
			CurrentBlockNumber:  &current,
			HighestBlockHash:    felt.NewFromUint64[felt.Felt](0xc),
			HighestBlockNumber:  &highest,
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"starting_block_hash":"0xa","starting_block_num":1,
			"current_block_hash":"0xb","current_block_num":2,
			"highest_block_hash":"0xc","highest_block_num":3
		}`, string(data))
	})
}
