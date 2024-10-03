package rpc_test

import (
	"errors"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(nil, errors.New("non-existent contract"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(&felt.Zero, errors.New("non-existent key"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})
}

func TestStorageProof(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	blockLatest := rpc.BlockID{Latest: true}

	t.Run("empty blockchain", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("class trie hash does not exist in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("class trie hash exists in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("storage trie address does not exist in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("storage trie address exists in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("contract storage trie address does not exist in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("contract storage trie key slot does not exist in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("contract storage trie address/key exists in a trie", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
	t.Run("class & storage tries proofs requested", func(t *testing.T) {
		//mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, proof)

		assert.Equal(t, rpc.ErrUnexpectedError, rpcErr)
	})
}
