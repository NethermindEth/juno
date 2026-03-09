package rpcv10_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v10"
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
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	targetAddress := felt.FromUint64[felt.Felt](1234)
	targetSlot := felt.FromUint64[felt.Felt](5678)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		blockID := blockIDLatest(t)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		blockID := blockIDHash(t, &felt.Zero)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		blockID := blockIDNumber(t, 0)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

		blockID := blockIDLatest(t)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(felt.Zero, nil)

		blockID := blockIDLatest(t)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		assert.Equal(t, &felt.Zero, storageValue)
		require.Nil(t, rpcErr)
	})

	t.Run("internal error while retrieving key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
			Return(felt.Felt{}, errors.New("some internal error"))

		blockID := blockIDLatest(t)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		assert.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	expectedStorage := felt.NewFromUint64[felt.Felt](1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		blockID := blockIDLatest(t)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		blockID := blockIDHash(t, &felt.Zero)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		blockID := blockIDNumber(t, 0)
		storageValue, rpcErr := handler.StorageAt(
			&targetAddress,
			&targetSlot,
			&blockID,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		preConfirmedStateDiff := core.EmptyStateDiff()
		preConfirmedStateDiff.
			StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: expectedStorage}
		preConfirmedStateDiff.
			DeployedContracts[targetAddress] = felt.NewFromUint64[felt.Felt](123456789)

		preConfirmed := core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 2,
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &preConfirmedStateDiff,
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
		mockReader.EXPECT().StateAtBlockNumber(preConfirmed.Block.Number-1).
			Return(mockState, nopCloser, nil)
		preConfirmedID := blockIDPreConfirmed(t)
		storageValue, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &preConfirmedID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		l1HeadBlockNumber := uint64(10)
		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: l1HeadBlockNumber,
				BlockHash:   &felt.Zero,
				StateRoot:   &felt.Zero,
			},
			nil,
		)
		mockReader.EXPECT().StateAtBlockNumber(l1HeadBlockNumber).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedStorage, nil)

		blockID := blockIDL1Accepted(t)
		storageValue, rpcErr := handler.StorageAt(&felt.Zero, &felt.Zero, &blockID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})
}
