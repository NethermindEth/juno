package rpcv7_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpcv7.New(mockReader, mockSyncReader, nil, &utils.Mainnet, log)

	targetAddress := felt.FromUint64[felt.Felt](1234)
	targetSlot := felt.FromUint64[felt.Felt](5678)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Hash: &felt.Zero},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Number: 0},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(felt.Zero, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Latest: true},
		)
		require.Equal(t, storageValue, &felt.Zero)
		assert.Nil(t, rpcErr)
	})

	t.Run("internal error while retrieving key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(
			&targetAddress,
			&targetSlot,
		).Return(felt.Felt{}, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
			Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
			Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Hash: &felt.Zero},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
			Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Number: 0},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		pendingStateDiff := core.EmptyStateDiff()
		pendingStateDiff.
			StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: expectedStorage}
		pendingStateDiff.
			DeployedContracts[targetAddress] = felt.NewFromUint64[felt.Felt](123456789)

		pending := core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: felt.NewFromUint64[felt.Felt](2),
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &pendingStateDiff,
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockReader.EXPECT().StateAtBlockHash(pending.Block.ParentHash).
			Return(mockState, nopCloser, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpcv7.BlockID{Pending: true},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})
}
