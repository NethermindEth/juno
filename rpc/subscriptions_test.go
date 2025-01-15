package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var emptyCommitments = core.BlockCommitments{}

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

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
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

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

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

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
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
			resp, err := marshalSubEventsResp(e, id.ID)
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
			resp, err := marshalSubEventsResp(e, id.ID)
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

		resp, err := marshalSubEventsResp(emittedEvents[0], id.ID)
		require.NoError(t, err)

		got := make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, string(resp), string(got))

		mockChain.EXPECT().EventFilter(fromAddr, keys).Return(mockEventFilterer, nil)

		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return([]*blockchain.FilteredEvent{filteredEvents[1]}, nil, nil)

		headerFeed.Send(&core.Header{Number: b1.Number + 1})

		resp, err = marshalSubEventsResp(emittedEvents[1], id.ID)
		require.NoError(t, err)

		got = make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, string(resp), string(got))

		cancel()
		time.Sleep(100 * time.Millisecond)
	})
}

func TestSubscribeTxnStatus(t *testing.T) {
	log := utils.NewNopZapLogger()
	txHash := new(felt.Felt).SetUint64(1)

	t.Run("Return error when transaction not found after timeout expiry", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		subscribeTxStatusTimeout = 50 * time.Millisecond
		subscribeTxStatusTickerDuration = 10 * time.Millisecond

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound).AnyTimes()
		mockSyncer.EXPECT().PendingBlock().Return(nil).AnyTimes()

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, nil)
		assert.Nil(t, id)
		assert.Equal(t, ErrTxnHashNotFound, rpcErr)
	})

	t.Run("Return error if block is too far back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		blockID := &BlockID{Number: 0}

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		// Note the end of the window doesn't need to be tested because if requested block number is more than the
		// head, a block not found error will be returned. This behaviour has been tested in various other tests, and we
		// don't need to test it here again.
		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})
	})

	t.Run("Transaction status is final", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)
		handler.WithFeeder(feeder.NewTestClient(t, &utils.SepoliaIntegration))

		t.Run("rejected", func(t *testing.T) { //nolint:dupl
			serverConn, clientConn := net.Pipe()
			t.Cleanup(func() {
				require.NoError(t, serverConn.Close())
				require.NoError(t, clientConn.Close())
			})

			respStr := `{"jsonrpc":"2.0","method":"starknet_subscriptionTransactionsStatus","params":{"result":{"transaction_hash":"%v","status":{"finality_status":"%s","failure_reason":"some error"}},"subscription_id":%v}}`
			txHash, err := new(felt.Felt).SetString("0x1111")
			require.NoError(t, err)

			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
			mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
			mockSyncer.EXPECT().PendingBlock().Return(nil)

			ctx, cancel := context.WithCancel(context.Background())
			subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
			id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, nil)
			require.Nil(t, rpcErr)

			b, err := TxnStatusRejected.MarshalText()
			require.NoError(t, err)

			resp := fmt.Sprintf(respStr, txHash, b, id.ID)
			got := make([]byte, len(resp))
			_, err = clientConn.Read(got)
			require.NoError(t, err)
			assert.Equal(t, resp, string(got))
			cancel()
		})

		t.Run("accepted on L1", func(t *testing.T) { //nolint:dupl
			serverConn, clientConn := net.Pipe()
			t.Cleanup(func() {
				require.NoError(t, serverConn.Close())
				require.NoError(t, clientConn.Close())
			})

			respStr := `{"jsonrpc":"2.0","method":"starknet_subscriptionTransactionsStatus","params":{"result":{"transaction_hash":"%v","status":{"finality_status":"%s","execution_status":"SUCCEEDED"}},"subscription_id":%v}}`
			txHash, err := new(felt.Felt).SetString("0x1010")
			require.NoError(t, err)

			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
			mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
			mockSyncer.EXPECT().PendingBlock().Return(nil)

			ctx, cancel := context.WithCancel(context.Background())
			subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
			id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, nil)
			require.Nil(t, rpcErr)

			b, err := TxnStatusAcceptedOnL1.MarshalText()
			require.NoError(t, err)

			resp := fmt.Sprintf(respStr, txHash, b, id.ID)
			got := make([]byte, len(resp))
			_, err = clientConn.Read(got)
			require.NoError(t, err)
			assert.Equal(t, resp, string(got))
			cancel()
		})
	})

	t.Run("Multiple transaction status update", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
		gw := adaptfeeder.New(client)
		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)
		handler.WithFeeder(client)
		l2Feed := feed.New[*core.Header]()
		l1Feed := feed.New[*core.L1Head]()
		handler.newHeads = l2Feed
		handler.l1Heads = l1Feed

		block, err := gw.BlockByNumber(context.Background(), 38748)
		require.NoError(t, err)

		serverConn, clientConn := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
			require.NoError(t, clientConn.Close())
		})

		receivedRespStr := `{"jsonrpc":"2.0","method":"starknet_subscriptionTransactionsStatus","params":{"result":{"transaction_hash":"%v","status":{"finality_status":"%s"}},"subscription_id":%v}}`
		txHash, err := new(felt.Felt).SetString("0x1001")
		require.NoError(t, err)

		mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: block.Number}, nil)
		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingBlock().Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		id, rpcErr := handler.SubscribeTransactionStatus(subCtx, *txHash, nil)
		require.Nil(t, rpcErr)

		b, err := TxnStatusReceived.MarshalText()
		require.NoError(t, err)

		resp := fmt.Sprintf(receivedRespStr, txHash, b, id.ID)
		got := make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, resp, string(got))

		mockChain.EXPECT().TransactionByHash(txHash).Return(block.Transactions[0], nil)
		mockChain.EXPECT().Receipt(txHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
		mockChain.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		l2Feed.Send(&core.Header{Number: block.Number + 1})

		b, err = TxnStatusAcceptedOnL2.MarshalText()
		require.NoError(t, err)

		l1AndL2RespStr := `{"jsonrpc":"2.0","method":"starknet_subscriptionTransactionsStatus","params":{"result":{"transaction_hash":"%v","status":{"finality_status":"%s","execution_status":"SUCCEEDED"}},"subscription_id":%v}}`
		resp = fmt.Sprintf(l1AndL2RespStr, txHash, b, id.ID)
		got = make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, resp, string(got))

		l1Head := &core.L1Head{BlockNumber: block.Number}
		mockChain.EXPECT().TransactionByHash(txHash).Return(block.Transactions[0], nil)
		mockChain.EXPECT().Receipt(txHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
		mockChain.EXPECT().L1Head().Return(l1Head, nil)

		l1Feed.Send(l1Head)

		b, err = TxnStatusAcceptedOnL1.MarshalText()
		require.NoError(t, err)

		resp = fmt.Sprintf(l1AndL2RespStr, txHash, b, id.ID)
		got = make([]byte, len(resp))
		_, err = clientConn.Read(got)
		require.NoError(t, err)
		assert.Equal(t, resp, string(got))
		cancel()
	})
}

type fakeSyncer struct {
	newHeads   *feed.Feed[*core.Header]
	reorgs     *feed.Feed[*sync.ReorgBlockRange]
	pendingTxs *feed.Feed[[]core.Transaction]
}

func newFakeSyncer() *fakeSyncer {
	return &fakeSyncer{
		newHeads:   feed.New[*core.Header](),
		reorgs:     feed.New[*sync.ReorgBlockRange](),
		pendingTxs: feed.New[[]core.Transaction](),
	}
}

func (fs *fakeSyncer) SubscribeNewHeads() sync.HeaderSubscription {
	return sync.HeaderSubscription{Subscription: fs.newHeads.Subscribe()}
}

func (fs *fakeSyncer) SubscribeReorg() sync.ReorgSubscription {
	return sync.ReorgSubscription{Subscription: fs.reorgs.Subscribe()}
}

func (fs *fakeSyncer) SubscribePendingTxs() sync.PendingTxSubscription {
	return sync.PendingTxSubscription{Subscription: fs.pendingTxs.Subscribe()}
}

func (fs *fakeSyncer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (fs *fakeSyncer) HighestBlockHeader() *core.Header {
	return nil
}

func (fs *fakeSyncer) Pending() (*sync.Pending, error)                       { return nil, nil }
func (fs *fakeSyncer) PendingBlock() *core.Block                             { return nil }
func (fs *fakeSyncer) PendingState() (core.StateReader, func() error, error) { return nil, nil, nil }

func TestSubscribeNewHeads(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("Return error if called on pending block", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeNewHeads(subCtx, &BlockID{Pending: true})
		assert.Zero(t, id)
		assert.Equal(t, ErrCallOnPending, rpcErr)
	})

	t.Run("Return error if block is too far back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, "", log)

		blockID := &BlockID{Number: 0}

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeNewHeads(subCtx, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number).Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeNewHeads(subCtx, blockID)
			assert.Zero(t, id)
			assert.Equal(t, ErrTooManyBlocksBack, rpcErr)
		})
	})

	t.Run("new block is received", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		mockChain := mocks.NewMockReader(mockCtrl)
		syncer := newFakeSyncer()

		l1Feed := feed.New[*core.L1Head]()
		mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})

		handler, server := setupRPC(t, ctx, mockChain, syncer)
		conn := createWsConn(t, ctx, server)

		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })

		got := sendWsMessage(t, ctx, conn, subMsg("starknet_subscribeNewHeads"))
		require.Equal(t, subResp(id), got)

		// Ignore the first mock header
		_, _, err := conn.Read(ctx)
		require.NoError(t, err)

		// Simulate a new block
		syncer.newHeads.Send(testHeader(t))

		// Receive a block header.
		_, headerGot, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, newHeadsResponse(id), string(headerGot))
	})
}

func TestSubscribeNewHeadsHistorical(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	block0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Mainnet, nil)
	assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

	chain = blockchain.New(testDB, &utils.Mainnet, nil)
	syncer := newFakeSyncer()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler, server := setupRPC(t, ctx, chain, syncer)

	conn := createWsConn(t, ctx, server)

	id := uint64(1)
	handler.WithIDGen(func() uint64 { return id })

	subMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribeNewHeads", "params":{"block":{"block_number":0}}}`
	got := sendWsMessage(t, ctx, conn, subMsg)
	require.Equal(t, subResp(id), got)

	// Check block 0 content
	want := `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943","parent_hash":"0x0","block_number":0,"new_root":"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6","timestamp":1637069048,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`
	want = fmt.Sprintf(want, id)
	_, block0Got, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(block0Got))

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Check new block content
	_, newBlockGot, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(id), string(newBlockGot))
}

func TestMultipleSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockChain := mocks.NewMockReader(mockCtrl)
	syncer := newFakeSyncer()

	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})

	handler, server := setupRPC(t, ctx, mockChain, syncer)

	mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil).Times(2)

	ws := jsonrpc.NewWebsocket(server, nil, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn1.Close(websocket.StatusNormalClosure, ""))
	})

	conn2, _, err := websocket.Dial(ctx, httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn2.Close(websocket.StatusNormalClosure, ""))
	})

	firstID := uint64(1)
	secondID := uint64(2)

	handler.WithIDGen(func() uint64 { return firstID })
	firstGot := sendWsMessage(t, ctx, conn1, subMsg("starknet_subscribeNewHeads"))
	require.NoError(t, err)
	require.Equal(t, subResp(firstID), firstGot)

	handler.WithIDGen(func() uint64 { return secondID })
	secondGot := sendWsMessage(t, ctx, conn2, subMsg("starknet_subscribeNewHeads"))
	require.NoError(t, err)
	require.Equal(t, subResp(secondID), secondGot)

	// Ignore the first mock header
	_, _, err = conn1.Read(ctx)
	require.NoError(t, err)
	_, _, err = conn2.Read(ctx)
	require.NoError(t, err)

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Receive a block header.
	_, firstHeaderGot, err := conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(firstID), string(firstHeaderGot))

	_, secondHeaderGot, err := conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(secondID), string(secondHeaderGot))

	// Unsubscribe
	unsubMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_unsubscribe","params":[%d]}`
	require.NoError(t, conn1.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, firstID))))
	require.NoError(t, conn2.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, secondID))))
}

func TestSubscriptionReorg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})

	syncer := newFakeSyncer()
	handler, server := setupRPC(t, ctx, mockChain, syncer)

	testCases := []struct {
		name            string
		subscribeMethod string
		ignoreFirst     bool
	}{
		{
			name:            "reorg event in starknet_subscribeNewHeads",
			subscribeMethod: "starknet_subscribeNewHeads",
			ignoreFirst:     true,
		},
		{
			name:            "reorg event in starknet_subscribeEvents",
			subscribeMethod: "starknet_subscribeEvents",
			ignoreFirst:     false,
		},
		// TODO: test reorg event in TransactionStatus
	}

	mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil).Times(len(testCases))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := createWsConn(t, ctx, server)

			id := uint64(1)
			handler.WithIDGen(func() uint64 { return id })

			got := sendWsMessage(t, ctx, conn, subMsg(tc.subscribeMethod))
			require.Equal(t, subResp(id), got)

			if tc.ignoreFirst {
				_, _, err := conn.Read(ctx)
				require.NoError(t, err)
			}

			// Simulate a reorg
			syncer.reorgs.Send(&sync.ReorgBlockRange{
				StartBlockHash: utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"),
				StartBlockNum:  0,
				EndBlockHash:   utils.HexToFelt(t, "0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86"),
				EndBlockNum:    2,
			})

			// Receive reorg event
			expectedRes := `{"jsonrpc":"2.0","method":"starknet_subscriptionReorg","params":{"result":{"starting_block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","starting_block_number":0,"ending_block_hash":"0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86","ending_block_number":2},"subscription_id":%d}}`
			want := fmt.Sprintf(expectedRes, id)
			_, reorgGot, err := conn.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, want, string(reorgGot))
		})
	}
}

func TestSubscribePendingTxs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})

	syncer := newFakeSyncer()
	handler, server := setupRPC(t, ctx, mockChain, syncer)

	t.Run("Basic subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions"}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		hash1 := new(felt.Felt).SetUint64(1)
		addr1 := new(felt.Felt).SetUint64(11)

		hash2 := new(felt.Felt).SetUint64(2)
		addr2 := new(felt.Felt).SetUint64(22)

		hash3 := new(felt.Felt).SetUint64(3)
		hash4 := new(felt.Felt).SetUint64(4)
		hash5 := new(felt.Felt).SetUint64(5)

		syncer.pendingTxs.Send([]core.Transaction{
			&core.InvokeTransaction{TransactionHash: hash1, SenderAddress: addr1},
			&core.DeclareTransaction{TransactionHash: hash2, SenderAddress: addr2},
			&core.DeployTransaction{TransactionHash: hash3},
			&core.DeployAccountTransaction{DeployTransaction: core.DeployTransaction{TransactionHash: hash4}},
			&core.L1HandlerTransaction{TransactionHash: hash5},
		})

		want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":["0x1","0x2","0x3","0x4","0x5"],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
	})

	t.Run("Filtered subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions", "params":{"sender_address":["0xb", "0x16"]}}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		hash1 := new(felt.Felt).SetUint64(1)
		addr1 := new(felt.Felt).SetUint64(11)

		hash2 := new(felt.Felt).SetUint64(2)
		addr2 := new(felt.Felt).SetUint64(22)

		hash3 := new(felt.Felt).SetUint64(3)
		hash4 := new(felt.Felt).SetUint64(4)
		hash5 := new(felt.Felt).SetUint64(5)

		hash6 := new(felt.Felt).SetUint64(6)
		addr6 := new(felt.Felt).SetUint64(66)

		hash7 := new(felt.Felt).SetUint64(7)
		addr7 := new(felt.Felt).SetUint64(77)

		syncer.pendingTxs.Send([]core.Transaction{
			&core.InvokeTransaction{TransactionHash: hash1, SenderAddress: addr1},
			&core.DeclareTransaction{TransactionHash: hash2, SenderAddress: addr2},
			&core.DeployTransaction{TransactionHash: hash3},
			&core.DeployAccountTransaction{DeployTransaction: core.DeployTransaction{TransactionHash: hash4}},
			&core.L1HandlerTransaction{TransactionHash: hash5},
			&core.InvokeTransaction{TransactionHash: hash6, SenderAddress: addr6},
			&core.DeclareTransaction{TransactionHash: hash7, SenderAddress: addr7},
		})

		want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":["0x1","0x2"],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
	})

	t.Run("Full details subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions", "params":{"transaction_details": true}}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		syncer.pendingTxs.Send([]core.Transaction{
			&core.InvokeTransaction{
				TransactionHash:       new(felt.Felt).SetUint64(1),
				CallData:              []*felt.Felt{new(felt.Felt).SetUint64(2)},
				TransactionSignature:  []*felt.Felt{new(felt.Felt).SetUint64(3)},
				MaxFee:                new(felt.Felt).SetUint64(4),
				ContractAddress:       new(felt.Felt).SetUint64(5),
				Version:               new(core.TransactionVersion).SetUint64(3),
				EntryPointSelector:    new(felt.Felt).SetUint64(6),
				Nonce:                 new(felt.Felt).SetUint64(7),
				SenderAddress:         new(felt.Felt).SetUint64(8),
				ResourceBounds:        map[core.Resource]core.ResourceBounds{},
				Tip:                   9,
				PaymasterData:         []*felt.Felt{new(felt.Felt).SetUint64(10)},
				AccountDeploymentData: []*felt.Felt{new(felt.Felt).SetUint64(11)},
			},
		})

		want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":[{"transaction_hash":"0x1","type":"INVOKE","version":"0x3","nonce":"0x7","max_fee":"0x4","contract_address":"0x5","sender_address":"0x8","signature":["0x3"],"calldata":["0x2"],"entry_point_selector":"0x6","resource_bounds":{},"tip":"0x9","paymaster_data":["0xa"],"account_deployment_data":["0xb"],"nonce_data_availability_mode":"L1","fee_data_availability_mode":"L1"}],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
	})

	t.Run("Return error if too many addresses in filter", func(t *testing.T) {
		addresses := make([]felt.Felt, 1024+1)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribePendingTxs(subCtx, nil, addresses)
		assert.Zero(t, id)
		assert.Equal(t, ErrTooManyAddressesInFilter, rpcErr)
	})
}

func createWsConn(t *testing.T, ctx context.Context, server *jsonrpc.Server) *websocket.Conn {
	ws := jsonrpc.NewWebsocket(server, nil, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	conn, _, err := websocket.Dial(ctx, httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
	})

	return conn
}

func subResp(id uint64) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","result":{"subscription_id":%d},"id":1}`, id)
}

func subMsg(method string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q}`, method)
}

func newHeadsResponse(id uint64) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`, id)
}

// setupRPC creates a RPC handler that runs in a goroutine and a JSONRPC server that can be used to test subscriptions
func setupRPC(t *testing.T, ctx context.Context, chain blockchain.Reader, syncer sync.Reader) (*Handler, *jsonrpc.Server) {
	t.Helper()

	log := utils.NewNopZapLogger()
	handler := New(chain, syncer, nil, "", log)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	time.Sleep(50 * time.Millisecond)

	server := jsonrpc.NewServer(1, log)
	methods, _ := handler.Methods()
	require.NoError(t, server.RegisterMethods(methods...))

	return handler, server
}

// sendWsMessage sends a message to a websocket connection and returns the response
func sendWsMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, message string) string {
	t.Helper()

	err := conn.Write(ctx, websocket.MessageText, []byte(message))
	require.NoError(t, err)

	_, response, err := conn.Read(ctx)
	require.NoError(t, err)
	return string(response)
}

func marshalSubEventsResp(e *EmittedEvent, id uint64) ([]byte, error) {
	return json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  "starknet_subscriptionEvents",
		Params: map[string]any{
			"subscription_id": id,
			"result":          e,
		},
	})
}

func testHeader(t *testing.T) *core.Header {
	t.Helper()

	header := &core.Header{
		Hash:             utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"),
		ParentHash:       utils.HexToFelt(t, "0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb"),
		Number:           2,
		GlobalStateRoot:  utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9"),
		Timestamp:        1637084470,
		SequencerAddress: utils.HexToFelt(t, "0x0"),
		L1DataGasPrice: &core.GasPrice{
			PriceInFri: utils.HexToFelt(t, "0x0"),
			PriceInWei: utils.HexToFelt(t, "0x0"),
		},
		GasPrice:        utils.HexToFelt(t, "0x0"),
		GasPriceSTRK:    utils.HexToFelt(t, "0x0"),
		L1DAMode:        core.Calldata,
		ProtocolVersion: "",
	}
	return header
}
