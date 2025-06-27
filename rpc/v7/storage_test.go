package rpcv7_test

import (
	"errors"
	"testing"

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
	log := utils.NewNopZapLogger()
	handler := rpcv7.New(mockReader, nil, nil, "", &utils.Mainnet, log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Hash: &felt.Zero})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Number: 0})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(felt.Zero, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(felt.Zero, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Latest: true})
		require.Equal(t, storageValue, &felt.Zero)
		assert.Nil(t, rpcErr)
	})

	t.Run("internal error while retrieving key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(felt.Zero, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpcv7.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})
}
