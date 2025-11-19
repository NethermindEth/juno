package rpcv6_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
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

	targetAddress := felt.FromUint64[felt.Felt](1234)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Latest: true},
			targetAddress,
		)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Hash: &felt.Zero},
			targetAddress,
		)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Number: 0},
			targetAddress,
		)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockCommonState(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).
			Return(felt.Felt{}, errors.New("non-existent contract"))

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Latest: true},
			targetAddress,
		)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedNonce := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Latest: true},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Hash: &felt.Zero},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Number: 0},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})
	//nolint:dupl //  similar structure with nonce test, different endpoint.
	t.Run("blockID - pending", func(t *testing.T) {
		pendingStateDiff := core.EmptyStateDiff()
		pendingStateDiff.Nonces[targetAddress] = expectedNonce

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
		mockReader.EXPECT().StateAtBlockHash(pending.Block.ParentHash).Return(mockState, nopCloser, nil)

		nonce, rpcErr := handler.Nonce(
			rpc.BlockID{Pending: true},
			targetAddress,
		)
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

	targetAddress := felt.FromUint64[felt.Felt](1234)
	targetSlot := felt.FromUint64[felt.Felt](5678)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Hash: &felt.Zero},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Number: 0},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockCommonState(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("internal error while retrieving contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).
			Return(felt.Felt{}, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(felt.Zero, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Equal(t, storageValue, &felt.Zero)
		assert.Nil(t, rpcErr)
	})

	t.Run("internal error while retrieving key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
			Return(felt.Felt{}, errors.New("some internal error"))

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Nil(t, storageValue)
		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Latest: true},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Hash: &felt.Zero},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Number: 0},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		stateDiff := core.EmptyStateDiff()
		stateDiff.
			StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: expectedStorage}
		stateDiff.
			DeployedContracts[targetAddress] = felt.NewFromUint64[felt.Felt](123456789)

		pending := core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: felt.NewFromUint64[felt.Felt](2),
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &stateDiff,
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockReader.EXPECT().StateAtBlockHash(pending.Block.ParentHash).
			Return(mockState, nopCloser, nil)

		storageValue, rpcErr := handler.StorageAt(
			targetAddress,
			targetSlot,
			rpc.BlockID{Pending: true},
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storageValue)
	})
}
