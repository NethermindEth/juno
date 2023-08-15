package starknet_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
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

	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		testDB.Close()
	})
	bc := blockchain.New(testDB, utils.GOERLI2, utils.NewNopZapLogger())
	feederClient := feeder.NewTestClient(t, bc.Network())
	gw := adaptfeeder.New(feederClient)
	for i := 0; i < 7; i++ {
		b, err := gw.BlockByNumber(context.Background(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(context.Background(), uint64(i))
		require.NoError(t, err)
		commitment, err := bc.SanityCheckNewHeight(b, s, nil)
		require.NoError(t, err)
		require.NoError(t, bc.Store(b, commitment, s, nil))
	}

	handler := starknet.NewHandler(bc, log)

	handlerHost := mockNet.Host(handlerID)
	handlerHost.SetStreamHandler(testPID, handler.StreamHandler)

	clientHost := mockNet.Host(clientID)
	client := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return clientHost.NewStream(ctx, handlerID, pids...)
	}, testPID, log)

	t.Run("get blocks", func(t *testing.T) {
		assertIsBlockWithNumber := func(p2pHeader *spec.BlockHeader, expectedBlockNum uint64) {
			parentHash, err := p2p2core.AdaptHash(p2pHeader.ParentBlock.Hash)
			require.NoError(t, err)

			parentHeader, err := bc.BlockHeaderByHash(&parentHash)
			require.NoError(t, err)

			assert.Equal(t, expectedBlockNum, parentHeader.Number+1)
		}
		t.Run("forward", func(t *testing.T) {
			res, cErr := client.GetBlocks(testCtx, &spec.GetBlocks{
				Start: &spec.BlockID{
					Height: 0,
				},
				Direction: spec.GetBlocks_Forward,
				Limit:     3,
				Skip:      1,
				Step:      2,
			})
			require.NoError(t, cErr)

			block1, valid := res()
			require.True(t, valid)
			assertIsBlockWithNumber(block1, 1)
			block3, valid := res()
			require.True(t, valid)
			assertIsBlockWithNumber(block3, 3)
			block5, valid := res()
			require.True(t, valid)
			assertIsBlockWithNumber(block5, 5)
			_, valid = res()
			require.False(t, valid)
		})

		t.Run("backward", func(t *testing.T) {
			res, cErr := client.GetBlocks(testCtx, &spec.GetBlocks{
				Start: &spec.BlockID{
					Height: 5,
				},
				Direction: spec.GetBlocks_Backward,
				Limit:     2,
				Skip:      0,
				Step:      1,
			})
			require.NoError(t, cErr)

			block5, valid := res()
			require.True(t, valid)
			assertIsBlockWithNumber(block5, 5)
			block4, valid := res()
			require.True(t, valid)
			assertIsBlockWithNumber(block4, 4)
			_, valid = res()
			require.False(t, valid)

		})
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

	t.Run("get event", func(t *testing.T) {
		res, cErr := client.GetEvents(testCtx, &spec.GetEvents{})
		require.NoError(t, cErr)
		require.Len(t, res.GetEvents(), 44)
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
