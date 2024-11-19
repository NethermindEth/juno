package rpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/NethermindEth/juno/core"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubscribeEventsAndUnsubscribe(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	mockSyncer := mocks.NewMockSyncReader(mockCtrl)

	log := utils.NewNopZapLogger()
	handler := rpc.New(mockChain, mockSyncer, nil, "", log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("Too many keys in filter", func(t *testing.T) {
		keys := make([][]felt.Felt, 1024+1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, nil)
		assert.Zero(t, id)
		assert.Equal(t, rpc.ErrTooManyKeysInFilter, rpcErr)
	})

	t.Run("Too many blocks back", func(t *testing.T) {
		keys := make([][]felt.Felt, 1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))
		blockID := &rpc.BlockID{Number: 0}

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		// Note the end of the window doesn't need to be tested because if a requested number is more than the
		// head a block not found error will be returned. This behaviour has been tested in various other test, and we
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
}
