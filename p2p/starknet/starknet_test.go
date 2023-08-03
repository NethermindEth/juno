package starknet_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestClientHandler(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	testPID := protocol.ID("testProtocol")
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockNet, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)

	peers := mockNet.Peers()
	require.Len(t, peers, 2)
	handlerID := peers[0]
	clientID := peers[1]

	log := utils.NewNopZapLogger()
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := starknet.NewHandler(mockReader, log)

	handlerHost := mockNet.Host(handlerID)
	handlerHost.SetStreamHandler(testPID, handler.StreamHandler)

	clientHost := mockNet.Host(clientID)
	client := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return clientHost.NewStream(ctx, handlerID, pids...)
	}, testPID, log)

	t.Run("get blocks", func(t *testing.T) {
		res, cErr := client.GetBlocks(testCtx, &spec.GetBlocks{})
		require.NoError(t, cErr)
		require.Len(t, res.GetBlocks(), 1)
		require.Equal(t, uint32(251), res.GetBlocks()[0].Header.State.NLeaves)
	})
}
