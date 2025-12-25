package rpcv9_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	targetAddress := felt.FromUint64[felt.Felt](1234)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		latest := blockIDLatest(t)
		nonce, rpcErr := handler.Nonce(&latest, &targetAddress)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		hash := blockIDHash(t, &felt.Zero)
		nonce, rpcErr := handler.Nonce(&hash, &targetAddress)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		number := blockIDNumber(t, 0)
		nonce, rpcErr := handler.Nonce(&number, &targetAddress)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).
			Return(felt.Felt{}, errors.New("non-existent contract"))

		latest := blockIDLatest(t)
		nonce, rpcErr := handler.Nonce(&latest, &targetAddress)
		require.Nil(t, nonce)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedNonce := felt.NewFromUint64[felt.Felt](1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		latest := blockIDLatest(t)
		nonce, rpcErr := handler.Nonce(&latest, &targetAddress)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		hash := blockIDHash(t, &felt.Zero)
		nonce, rpcErr := handler.Nonce(&hash, &targetAddress)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		number := blockIDNumber(t, 0)
		nonce, rpcErr := handler.Nonce(&number, &targetAddress)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	//nolint:dupl //  similar structure with class test, different endpoint.
	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		stateDiff := core.EmptyStateDiff()
		stateDiff.Nonces[targetAddress] = expectedNonce

		preConfirmed := core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 2,
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &stateDiff,
			},
		}

		mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
		mockReader.EXPECT().StateAtBlockNumber(preConfirmed.Block.Number-1).
			Return(mockState, nopCloser, nil)

		preConfirmedBlockID := blockIDPreConfirmed(t)
		nonce, rpcErr := handler.Nonce(&preConfirmedBlockID, &targetAddress)
		require.Nil(t, rpcErr)
		require.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		l1AcceptedBlockNumber := uint64(10)

		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: l1AcceptedBlockNumber,
				BlockHash:   &felt.One,
				StateRoot:   &felt.One,
			},
			nil,
		)
		mockReader.EXPECT().StateAtBlockNumber(l1AcceptedBlockNumber).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&targetAddress).Return(*expectedNonce, nil)

		l1AcceptedID := blockIDL1Accepted(t)
		nonce, rpcErr := handler.Nonce(&l1AcceptedID, &targetAddress)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})
}
