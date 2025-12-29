package rpcv7_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v7"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync/pendingdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingDataWrapper_PendingData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, n, log)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Returns pending data when valid", func(t *testing.T) {
		t.Run("when starknet version < 0.14.0", func(t *testing.T) {
			expectedPending := core.NewPending(latestBlock, nil, nil)
			mockSyncReader.EXPECT().PendingData().Return(
				&expectedPending,
				nil,
			)
			pending, err := handler.PendingData()
			require.NoError(t, err)
			require.Equal(t, core.PendingBlockVariant, pending.Variant())
			require.Equal(t, &expectedPending, pending)
		})

		t.Run("when starknet version >= 0.14.0 - returns placeholder pending block", func(t *testing.T) {
			preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
			mockSyncReader.EXPECT().PendingData().Return(
				&preConfirmed,
				nil,
			)
			mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
			mockReader.EXPECT().BlockHeaderByNumber(
				latestBlock.Header.Number+1-pendingdata.BlockHashLag,
			).Return(&core.Header{Hash: felt.NewFromUint64[felt.Felt](1234567)}, nil).Times(2)

			expectedPending, err := pendingdata.MakeEmptyPendingForParent(
				mockReader,
				latestBlock.Header,
			)
			require.NoError(t, err)

			pending, err := handler.PendingData()
			require.NoError(t, err)
			require.Equal(t, core.PendingBlockVariant, pending.Variant())
			require.Equal(t, &expectedPending, pending)
		})
	})

	t.Run("Returns placeholder pending data when pending data is not valid", func(t *testing.T) {
		blockToRegisterHash := core.Header{
			Number: latestBlock.Header.Number + 1 - pendingdata.BlockHashLag,
			Hash:   felt.NewFromUint64[felt.Felt](1234567),
		}

		t.Run("when starknet version < 0.14.0", func(t *testing.T) {
			mockSyncReader.EXPECT().PendingData().Return(
				nil,
				core.ErrPendingDataNotFound,
			)

			mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
			mockReader.EXPECT().BlockHeaderByNumber(
				latestBlock.Header.Number+1-pendingdata.BlockHashLag,
			).Return(&blockToRegisterHash, nil).Times(2)

			expectedPending, err := pendingdata.MakeEmptyPendingForParent(
				mockReader,
				latestBlock.Header,
			)
			require.NoError(t, err)
			pending, err := handler.PendingData()
			require.NoError(t, err)
			require.Equal(t, core.PendingBlockVariant, pending.Variant())
			require.Equal(t, &expectedPending, pending)
		})

		t.Run("when starknet version >= 0.14.0", func(t *testing.T) {
			mockSyncReader.EXPECT().PendingData().Return(
				nil,
				core.ErrPendingDataNotFound,
			)
			latestHeader := latestBlock.Header
			latestHeader.ProtocolVersion = "0.14.0"
			mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)
			mockReader.EXPECT().BlockHeaderByNumber(
				latestBlock.Header.Number+1-pendingdata.BlockHashLag,
			).Return(&blockToRegisterHash, nil).Times(2)

			expectedPending, err := pendingdata.MakeEmptyPendingForParent(
				mockReader,
				latestHeader,
			)
			require.NoError(t, err)

			pending, err := handler.PendingData()
			require.NoError(t, err)
			require.Equal(t, core.PendingBlockVariant, pending.Variant())
			require.Equal(t, &expectedPending, pending)
		})
	})
}

func TestPendingDataWrapper_PendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, &utils.Sepolia, nil)

	mockState := mocks.NewMockStateReader(mockCtrl)
	t.Run("Returns pending state when starknet version < 0.14.0", func(t *testing.T) {
		stateDiff := core.EmptyStateDiff()
		pendingData := core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					Number: 1,
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &stateDiff,
			},
			NewClasses: map[felt.Felt]core.ClassDefinition{},
		}
		mockSyncReader.EXPECT().PendingData().Return(&pendingData, nil)
		mockReader.EXPECT().StateAtBlockHash(
			pendingData.Block.ParentHash,
		).Return(mockState, nopCloser, nil)
		pendingState, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pendingState)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state when starknet version >= 0.14.0", func(t *testing.T) {
		// v8 only allows PendingBlockVariant, so PreConfirmed gets converted to head state
		preConfirmed := core.NewPreConfirmed(nil, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state when pending data is not valid", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})
}
