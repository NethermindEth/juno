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
	"github.com/stretchr/testify/assert"
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

		count := uint32(0)
		for header, valid := res(); valid; header, valid = res() {
			assert.Equal(t, count, header.State.NLeaves)
			count++
		}
		require.Equal(t, uint32(4), count)
	})

	t.Run("get signatures", func(t *testing.T) {
		res, cErr := client.GetSignatures(testCtx, &spec.GetSignatures{
			Id: &spec.BlockID{
				Height: 44,
			},
		})
		require.NoError(t, cErr)
		require.Equal(t, res.Id.Height, uint64(44))
	})

	t.Run("get receipts", func(t *testing.T) {
		res, cErr := client.GetReceipts(testCtx, &spec.GetReceipts{})
		require.NoError(t, cErr)
		require.Len(t, res.GetReceipts(), 37)
	})

	t.Run("get txns", func(t *testing.T) {
		res, cErr := client.GetTransactions(testCtx, &spec.GetTransactions{})
		require.NoError(t, cErr)
		require.Len(t, res.GetTransactions(), 1337)
	})
}
