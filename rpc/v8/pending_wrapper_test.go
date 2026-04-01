package rpcv8_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v8"
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
	handler := rpc.New(mockReader, mockSyncReader, nil, log)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Returns empty pending placeholder based on latest header", func(t *testing.T) {
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
		require.Equal(t, &expectedPending, pending)
	})
}

func TestPendingDataWrapper_PendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, nil)

	mockState := mocks.NewMockStateReader(mockCtrl)
	t.Run("Returns head state", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})
}
