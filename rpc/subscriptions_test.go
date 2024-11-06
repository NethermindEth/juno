package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Due to the difference in how some test files in rpc use "package rpc" vs "package rpc_test" it was easiest to copy
// the fakeConn here.
// Todo: move all the subscription related test here
type fakeConn struct {
	w io.Writer
}

func (fc *fakeConn) Write(p []byte) (int, error) {
	return fc.w.Write(p)
}

func (fc *fakeConn) Equal(other jsonrpc.Conn) bool {
	fc2, ok := other.(*fakeConn)
	if !ok {
		return false
	}
	return fc.w == fc2.w
}

func TestSubscribeEvents(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("Return error if too many keys in filter", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

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
		assert.Equal(t, ErrTooManyKeysInFilter, rpcErr)
	})

	t.Run("Return error if called on pending block", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		keys := make([][]felt.Felt, 1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))
		blockID := &BlockID{Pending: true}

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)

		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, blockID)
		assert.Zero(t, id)
		assert.Equal(t, ErrCallOnPending, rpcErr)
	})

	t.Run("Return error if block is too far back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		keys := make([][]felt.Felt, 1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))
		blockID := &BlockID{Number: 0}

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		// Note the end of the window doesn't need to be tested because if requested block number is more than the
		// head, a block not found error will be returned. This behaviour has been tested in various other tests, and we
		// don't need to test it here again.
		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})
	})

	n := utils.Ptr(utils.Sepolia)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	b1, err := gw.BlockByNumber(context.Background(), 56377)
	require.NoError(t, err)

	fromAddr := new(felt.Felt).SetBytes([]byte("some address"))
	keys := [][]felt.Felt{{*new(felt.Felt).SetBytes([]byte("key1"))}}

	filteredEvents := []*blockchain.FilteredEvent{
		{
			Event:           b1.Receipts[0].Events[0],
			BlockNumber:     b1.Number,
			BlockHash:       new(felt.Felt).SetBytes([]byte("b1")),
			TransactionHash: b1.Transactions[0].Hash(),
		},
		{
			Event:           b1.Receipts[1].Events[0],
			BlockNumber:     b1.Number + 1,
			BlockHash:       new(felt.Felt).SetBytes([]byte("b2")),
			TransactionHash: b1.Transactions[1].Hash(),
		},
	}

	var emittedEvents []*EmittedEvent
	for _, e := range filteredEvents {
		emittedEvents = append(emittedEvents, &EmittedEvent{
			Event: &Event{
				From: e.From,
				Keys: e.Keys,
				Data: e.Data,
			},
			BlockHash:       e.BlockHash,
			BlockNumber:     &e.BlockNumber,
			TransactionHash: e.TransactionHash,
		})
	}

	t.Run("Events from old blocks", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: b1.Number}, nil)
		mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
		mockChain.EXPECT().EventFilter(fromAddr, keys).Return(mockEventFilterer, nil)

		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(filteredEvents, nil, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		ctx, cancel := context.WithCancel(context.Background())
		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &BlockID{Number: b1.Number})
		require.Nil(t, rpcErr)

		var marshalledResponses [][]byte
		for _, e := range emittedEvents {
			resp, err := marshalSubscriptionResponse(e, id.ID)
			require.NoError(t, err)
			marshalledResponses = append(marshalledResponses, resp)
		}

		for _, m := range marshalledResponses {
			got := make([]byte, len(m))
			_, err := clientConn.Read(got)
			require.NoError(t, err)
			assert.Equal(t, string(m), string(got))
		}
		cancel()
	})

	t.Run("Events when continuation token is not nil", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: b1.Number}, nil)
		mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
		mockChain.EXPECT().EventFilter(fromAddr, keys).Return(mockEventFilterer, nil)

		cToken := new(blockchain.ContinuationToken)
		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(
			[]*blockchain.FilteredEvent{filteredEvents[0]}, cToken, nil)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(
			[]*blockchain.FilteredEvent{filteredEvents[1]}, nil, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		ctx, cancel := context.WithCancel(context.Background())
		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &BlockID{Number: b1.Number})
		require.Nil(t, rpcErr)

		var marshalledResponses [][]byte
		for _, e := range emittedEvents {
			resp, err := marshalSubscriptionResponse(e, id.ID)
			require.NoError(t, err)
			marshalledResponses = append(marshalledResponses, resp)
		}

		for _, m := range marshalledResponses {
			got := make([]byte, len(m))
			_, err := clientConn.Read(got)
			require.NoError(t, err)
			assert.Equal(t, string(m), string(got))
		}
		cancel()
	})

	t.Run("Events from new blocks", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)

		handler := New(mockChain, mockSyncer, nil, "", log)
		headerFeed := feed.New[*core.Header]()
		handler.newHeads = headerFeed

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: b1.Number}, nil)
		mockChain.EXPECT().EventFilter(fromAddr, keys).Return(mockEventFilterer, nil)

		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return([]*blockchain.FilteredEvent{filteredEvents[0]}, nil, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		ctx, cancel := context.WithCancel(context.Background())
		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, nil)
		require.Nil(t, rpcErr)

		resp, err := marshalSubscriptionResponse(emittedEvents[0], id.ID)
		require.NoError(t, err)

		got := make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, string(resp), string(got))

		mockChain.EXPECT().EventFilter(fromAddr, keys).Return(mockEventFilterer, nil)

		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return([]*blockchain.FilteredEvent{filteredEvents[1]}, nil, nil)

		headerFeed.Send(&core.Header{Number: b1.Number + 1})

		resp, err = marshalSubscriptionResponse(emittedEvents[1], id.ID)
		require.NoError(t, err)

		got = make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, string(resp), string(got))

		cancel()
		time.Sleep(100 * time.Millisecond)
	})
}

func marshalSubscriptionResponse(e *EmittedEvent, id uint64) ([]byte, error) {
	return json.Marshal(jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionEvents",
		Params: map[string]any{
			"subscription_id": id,
			"result":          e,
		},
	})
}
