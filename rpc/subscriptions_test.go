package rpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubscribeEventsAndUnsubscribe(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("Too many keys in filter", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := rpc.New(mockChain, mockSyncer, nil, "", log)

		keys := make([][]felt.Felt, 1024+1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, nil)
		assert.Zero(t, id)
		assert.Equal(t, rpc.ErrTooManyKeysInFilter, rpcErr)
	})

	t.Run("Too many blocks back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := rpc.New(mockChain, mockSyncer, nil, "", log)

		keys := make([][]felt.Felt, 1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))
		blockID := &rpc.BlockID{Number: 0}

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		// Note the end of the window doesn't need to be tested because if requested block number is more than the
		// head, a block not found error will be returned. This behaviour has been tested in various other test, and we
		// don't need to test it here again.
		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpc.ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpc.ErrTooManyBlocksBack, rpcErr)
		})
	})

	t.Run("Events from old blocks and new", func(t *testing.T) {
		n := utils.Ptr(utils.Sepolia)
		client := feeder.NewTestClient(t, n)
		gw := adaptfeeder.New(client)

		b1, err := gw.BlockByNumber(context.Background(), 56377)
		require.NoError(t, err)

		// Make a shallow copy of b1 into b2 and b3. Then modify them accordingly.
		b2, b3 := new(core.Block), new(core.Block)
		b2.Header, b3.Header = new(core.Header), new(core.Header)
		*b2.Header, *b3.Header = *b1.Header, *b1.Header
		b2.Number = b1.Number + 1
		b3.Number = b2.Number + 1
		fmt.Println(b1.Number, b2.Number, b3.Number)

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		fromAddr := b1.Receipts[0].Events[0].From
		keys := make([][]felt.Felt, 1)
		for _, k := range b1.Receipts[0].Events[0].Keys {
			keys[0] = append(keys[0], *k)
		}

		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := rpc.New(mockChain, mockSyncer, nil, "", log)

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: b2.Number}, nil)
		mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)

		_, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &rpc.BlockID{Number: b1.Number})
		require.Nil(t, rpcErr)

		// Check from the conn that the correct id has been passed
	})
}
