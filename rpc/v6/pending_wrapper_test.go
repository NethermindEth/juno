package rpcv6_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingWrapper_Pending(t *testing.T) {
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
			Number: latestBlock.Header.Number + 1 - sync.BlockHashLag,
			Hash:   felt.NewFromUint64[felt.Felt](1234567),
		}

		latestHeader := latestBlock.Header
		latestHeader.ProtocolVersion = "0.13.1"
		mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)
		mockReader.EXPECT().BlockHeaderByNumber(
			latestBlock.Header.Number+1-sync.BlockHashLag,
		).Return(&blockToRegisterHash, nil).Times(2)

		expectedPending, err := sync.MakeEmptyPendingForParent(
			mockReader,
			latestHeader,
		)
		require.NoError(t, err)

		pending, err := handler.Pending()
		require.NoError(t, err)
		require.NotNil(t, pending)
		require.Equal(t, &expectedPending, pending)
	})
}
