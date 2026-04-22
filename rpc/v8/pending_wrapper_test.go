package rpcv8_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingWrapper_Pending(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := new(networks.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	logger := log.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, logger)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Returns empty pending placeholder based on latest header", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().BlockHeaderByNumber(
			latestBlock.Header.Number+1-sync.BlockHashLag,
		).Return(&core.Header{Hash: felt.NewFromUint64[felt.Felt](1234567)}, nil).Times(2)

		expectedPending, err := sync.MakeEmptyPendingForParent(
			mockReader,
			latestBlock.Header,
		)
		require.NoError(t, err)

		pending, err := handler.Pending()
		require.NoError(t, err)
		require.Equal(t, &expectedPending, pending)
	})
}
