package rpcv9_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingDataWrapper(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Returns pending block data when starknet version < 0.14.0", func(t *testing.T) {
		expectedPending := sync.NewPending(latestBlock, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&expectedPending,
			nil,
		)
		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, core.PendingBlockVariant, pending.Variant())
		require.Equal(t, expectedPending, sync.Pending{
			Block:       pending.GetBlock(),
			StateUpdate: pending.GetStateUpdate(),
			NewClasses:  pending.GetNewClasses(),
		})
	})

	t.Run("Returns pre_confirmed block data when starknet version >= 0.14.0", func(t *testing.T) {
		expectedPending := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&expectedPending,
			nil,
		)
		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, core.PreConfirmedBlockVariant, pending.Variant())
		require.Equal(t, expectedPending, core.PreConfirmed{
			Block:                 pending.GetBlock(),
			StateUpdate:           pending.GetStateUpdate(),
			NewClasses:            pending.GetNewClasses(),
			CandidateTxs:          pending.GetCandidateTransaction(),
			TransactionStateDiffs: nil,
		})
	})

	t.Run("Finality status is ACCEPTED_ON_L2 when pending block", func(t *testing.T) {
		expectedPending := sync.NewPending(latestBlock, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&expectedPending,
			nil,
		).Times(2)
		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, core.PendingBlockVariant, pending.Variant())
		require.Equal(t, rpc.TxnAcceptedOnL2, handler.PendingBlockFinalityStatus())
	})

	t.Run("Finality status is PRE_CONFIRMED when pre_confirmed block", func(t *testing.T) {
		expectedPending := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&expectedPending,
			nil,
		).Times(2)
		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, core.PreConfirmedBlockVariant, pending.Variant())
		require.Equal(t, rpc.TxnPreConfirmed, handler.PendingBlockFinalityStatus())
	})
}

func TestPendingDataWrapper_PendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, nil)

	mockState := mocks.NewMockStateReader(mockCtrl)
	t.Run("Returns pending state", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		pendingState, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pendingState)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state when pending data is nil", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingState().Return(
			nil,
			nil,
			sync.ErrPendingBlockNotFound,
		)

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})
}
