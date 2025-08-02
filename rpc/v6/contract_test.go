package rpcv6_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, n, log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(nil, errors.New("non-existent contract"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedNonce := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		pending := sync.NewPending(nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Pending: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})
}

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, n, log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("internal error while retrieving contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(&felt.Zero, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Equal(t, storageValue, &felt.Zero)
		assert.Nil(t, rpcErr)
	})

	t.Run("internal error while retrieving key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(nil, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		pending := sync.NewPending(nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})
}
