package rpcv6_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v6"
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
	handler := rpc.New(mockReader, mockSyncReader, nil, n, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Always returns empty Pending placeholder", func(t *testing.T) {
		blockToRegisterHash := core.Header{
			Number: latestBlock.Header.Number + 1 - pendingdata.BlockHashLag,
			Hash:   felt.NewFromUint64[felt.Felt](1234567),
		}

		latestHeader := latestBlock.Header
		latestHeader.ProtocolVersion = "0.13.1"
		mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)
		mockReader.EXPECT().BlockHeaderByNumber(
			latestBlock.Header.Number+1-pendingdata.BlockHashLag,
		).Return(&blockToRegisterHash, nil)

		pending, err := handler.PendingData()
		require.NoError(t, err)
		_, isPending := pending.(*core.Pending)
		require.True(t, isPending)
	})

	t.Run("Returns empty Pending for latest block", func(t *testing.T) {
		blockToRegisterHash := core.Header{
			Number: latestBlock.Header.Number + 1 - pendingdata.BlockHashLag,
			Hash:   felt.NewFromUint64[felt.Felt](1234567),
		}

		latestHeader := latestBlock.Header
		latestHeader.ProtocolVersion = "0.13.1"
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
		require.Equal(t, &expectedPending, pending)
	})
}

func TestPendingDataWrapper_PendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, &utils.Sepolia, nil)

	mockState := mocks.NewMockStateReader(mockCtrl)
	t.Run("Returns latest state", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state for PreConfirmed with non-nil block", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state when pending data is not valid", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})
}
