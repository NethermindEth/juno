package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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

const (
	unsubscribeMsg              = `{"jsonrpc":"2.0","id":1,"method":"juno_unsubscribe","params":[%d]}`
	unsubscribeNotFoundResponse = `{"jsonrpc":"2.0","error":{"code":100,"message":"Subscription not found"},"id":1}`
	subscribeResponse           = `{"jsonrpc":"2.0","result":{"subscription_id":%d},"id":1}`
	subscribeTxStatus           = `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribeTransactionStatus","params":{"transaction_hash":"%s"}}`
	txStatusNotFoundResponse    = `{"jsonrpc":"2.0","error":{"code":29,"message":"Transaction hash not found"},"id":1}`
	txStatusResponse            = `{"jsonrpc":"2.0","method":"starknet_subscriptionTransactionsStatus","params":{"result":{"transaction_hash":"%s","status":{%s}},"subscription_id":%d}}`
	txStatusStatusBothStatuses  = `"finality_status":"%s","execution_status":"%s"`
	txStatusStatusRejected      = `"finality_status":"%s","failure_reason":"%s"`
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

type fakeSyncer struct {
	newHeads   *feed.Feed[*core.Header]
	pendingTxs *feed.Feed[[]core.Transaction]
}

func newFakeSyncer() *fakeSyncer {
	return &fakeSyncer{
		newHeads:   feed.New[*core.Header](),
		pendingTxs: feed.New[[]core.Transaction](),
	}
}

func (fs *fakeSyncer) SubscribeNewHeads() sync.HeaderSubscription {
	return sync.HeaderSubscription{Subscription: fs.newHeads.Subscribe()}
}

func (fs *fakeSyncer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (fs *fakeSyncer) HighestBlockHeader() *core.Header {
	return nil
}

func (fs *fakeSyncer) Pending() (*sync.Pending, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fs *fakeSyncer) PendingBlock() *core.Block {
	return nil
}

func (fs *fakeSyncer) PendingState() (core.StateReader, func() error, error) {
	return nil, nil, fmt.Errorf("not implemented")
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

func TestSubscribeTxStatusAndUnsubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler, syncer, server := setupSubscriptionTest(t, ctx, mockReader)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeTransactionStatus",
		Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}, {Name: "block", Optional: true}},
		Handler: handler.SubscribeTxnStatus,
	}, jsonrpc.Method{
		Name:    "juno_unsubscribe",
		Params:  []jsonrpc.Parameter{{Name: "id"}},
		Handler: handler.Unsubscribe,
	}))

	ws := jsonrpc.NewWebsocket(server, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	// default returns from mocks
	txnHash := utils.HexToFelt(t, "0x111100000000111100000000111100000000111100000000111100000000111")
	txn := &core.DeployTransaction{TransactionHash: txnHash, Version: (*core.TransactionVersion)(&felt.Zero)}
	receipt := &core.TransactionReceipt{
		TransactionHash: txnHash,
		Reverted:        false,
	}
	mockReader.EXPECT().TransactionByHash(txnHash).Return(txn, nil).AnyTimes()
	mockReader.EXPECT().Receipt(txnHash).Return(receipt, nil, uint64(1), nil).AnyTimes()
	mockReader.EXPECT().TransactionByHash(&felt.Zero).Return(nil, db.ErrKeyNotFound).AnyTimes()

	firstID := uint64(1)
	secondID := uint64(2)

	t.Run("simple subscribe and unsubscribe", func(t *testing.T) {
		conn1, resp1, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)
		defer bodyCloser(t, resp1)

		conn2, resp2, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)
		defer bodyCloser(t, resp2)

		handler.WithIDGen(func() uint64 { return firstID })
		firstWant := txStatusNotFoundResponse
		// Notice we subscribe for non-existing tx, we expect automatic unsubscribe
		firstGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(subscribeTxStatus, felt.Zero.String()))
		require.NoError(t, err)
		require.Equal(t, firstWant, firstGot)

		handler.WithIDGen(func() uint64 { return secondID })
		secondWant := fmt.Sprintf(subscribeResponse, secondID)
		secondGot := sendAndReceiveMessage(t, ctx, conn2, fmt.Sprintf(subscribeTxStatus, txnHash))
		require.NoError(t, err)
		require.Equal(t, secondWant, secondGot)

		// as expected the subscription is gone
		firstUnsubGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(unsubscribeMsg, firstID))
		require.Equal(t, unsubscribeNotFoundResponse, firstUnsubGot)

		// Receive a block header.
		secondWant = formatTxStatusResponse(t, txnHash, TxnStatusAcceptedOnL2, TxnSuccess, secondID)
		_, secondHeaderGot, err := conn2.Read(ctx)
		secondGot = string(secondHeaderGot)
		require.NoError(t, err)
		require.Equal(t, secondWant, secondGot)

		// Unsubscribe
		require.NoError(t, conn2.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubscribeMsg, secondID))))
	})

	t.Run("no update is sent when status has not changed", func(t *testing.T) {
		conn1, resp1, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)
		defer bodyCloser(t, resp1)

		handler.WithIDGen(func() uint64 { return firstID })
		firstWant := fmt.Sprintf(subscribeResponse, firstID)
		firstGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(subscribeTxStatus, txnHash))
		require.NoError(t, err)
		require.Equal(t, firstWant, firstGot)

		firstStatusWant := formatTxStatusResponse(t, txnHash, TxnStatusAcceptedOnL2, TxnSuccess, firstID)
		_, firstStatusGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, firstStatusWant, string(firstStatusGot))

		// Simulate a new block
		syncer.newHeads.Send(testHeader(t))

		// expected no status is send
		timeoutCtx, toCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer toCancel()
		_, _, err = conn1.Read(timeoutCtx)
		require.Regexp(t, "failed to get reader: ", err.Error())

		// at this time connection is closed
		require.EqualError(t,
			conn1.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubscribeMsg, firstID))),
			"failed to write msg: use of closed network connection")
	})

	t.Run("update is only sent when new status is different", func(t *testing.T) {
		conn1, resp1, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)
		defer bodyCloser(t, resp1)

		otherTxn := utils.HexToFelt(t, "0x222200000000111100000000222200000000111100000000111100000000222")
		someBlkHash := utils.HexToFelt(t, "0x333300000000111100000000222200000000333300000000111100000000fff")
		txn := &core.DeployTransaction{TransactionHash: txnHash, Version: (*core.TransactionVersion)(&felt.Zero)}
		receipt := &core.TransactionReceipt{
			TransactionHash: otherTxn,
			Reverted:        false,
		}
		mockReader.EXPECT().TransactionByHash(otherTxn).Return(txn, nil).Times(2)
		mockReader.EXPECT().Receipt(otherTxn).Return(receipt, someBlkHash, uint64(1), nil).Times(2)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{BlockNumber: 0}, nil)

		handler.WithIDGen(func() uint64 { return firstID })
		firstWant := fmt.Sprintf(subscribeResponse, firstID)
		firstGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(subscribeTxStatus, otherTxn))
		require.NoError(t, err)
		require.Equal(t, firstWant, firstGot)

		firstStatusWant := formatTxStatusResponse(t, otherTxn, TxnStatusAcceptedOnL2, TxnSuccess, firstID)
		_, firstStatusGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, firstStatusWant, string(firstStatusGot))

		mockReader.EXPECT().L1Head().Return(&core.L1Head{BlockNumber: 5}, nil).Times(1)
		syncer.newHeads.Send(testHeader(t))

		secondStatusWant := formatTxStatusResponse(t, otherTxn, TxnStatusAcceptedOnL1, TxnSuccess, firstID)
		_, secondStatusGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, secondStatusWant, string(secondStatusGot))

		// second status is final - subcription should be automatically removed
		thirdUnsubGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(unsubscribeMsg, firstID))
		require.Equal(t, unsubscribeNotFoundResponse, thirdUnsubGot)
	})

	t.Run("subscription ends when tx reaches final status", func(t *testing.T) {
		conn1, resp1, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)
		defer bodyCloser(t, resp1)

		revertedTxn := utils.HexToFelt(t, "0x111100000000222200000000333300000000444400000000555500000000fff")
		mockReader.EXPECT().TransactionByHash(revertedTxn).Return(nil, db.ErrKeyNotFound).Times(2)

		handler.WithIDGen(func() uint64 { return firstID })
		handler.WithFeeder(feeder.NewTestClient(t, &utils.Mainnet))
		defer handler.WithFeeder(nil)

		firstWant := fmt.Sprintf(subscribeResponse, firstID)
		firstGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(subscribeTxStatus, revertedTxn))
		require.NoError(t, err)
		require.Equal(t, firstWant, firstGot)

		firstStatusWant := formatTxStatusResponse(t, revertedTxn, TxnStatusRejected, TxnFailure, firstID, "This is hand-made transaction used for txStatus endpoint test")
		_, firstStatusGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, firstStatusWant, string(firstStatusGot))

		// final status will be discovered after a new head is received
		syncer.newHeads.Send(testHeader(t))
		// and wait a bit for the subscription to process the event
		time.Sleep(50 * time.Millisecond)

		// second status is final - subcription should be automatically removed
		thirdUnsubGot := sendAndReceiveMessage(t, ctx, conn1, fmt.Sprintf(unsubscribeMsg, firstID))
		require.Equal(t, unsubscribeNotFoundResponse, thirdUnsubGot)
	})
}

func marshalSubscriptionResponse(e *EmittedEvent, id uint64) ([]byte, error) {
	return json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  "starknet_subscriptionEvents",
		Params: map[string]any{
			"subscription_id": id,
			"result":          e,
		},
	})
}

func setupSubscriptionTest(t *testing.T, ctx context.Context, srvs ...any) (*Handler, *fakeSyncer, *jsonrpc.Server) {
	t.Helper()

	var (
		log   utils.Logger
		chain blockchain.Reader
	)

	for _, srv := range srvs {
		switch srv := srv.(type) {
		case utils.Logger:
			log = srv
		case blockchain.Reader:
			chain = srv
		default:
			t.Fatalf("unexpected option type: %T", srv)
		}
	}

	// provide good defaults
	if log == nil {
		log = utils.NewNopZapLogger()
	}
	if chain == nil {
		chain = blockchain.New(pebble.NewMemTest(t), &utils.Mainnet, nil)
	}
	syncer := newFakeSyncer()

	handler := New(chain, syncer, nil, "", log)
	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	time.Sleep(50 * time.Millisecond)

	server := jsonrpc.NewServer(1, log)

	return handler, syncer, server
}

func sendAndReceiveMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, message string) string {
	t.Helper()

	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(message)))

	_, response, err := conn.Read(ctx)
	require.NoError(t, err)
	return string(response)
}

func formatTxStatusResponse(t *testing.T, txnHash *felt.Felt, finality TxnStatus, execution TxnExecutionStatus, id uint64, reason ...string) string {
	t.Helper()

	finStatusB, err := finality.MarshalText()
	require.NoError(t, err)
	exeStatusB, err := execution.MarshalText()
	require.NoError(t, err)

	statusBody := fmt.Sprintf(txStatusStatusBothStatuses, string(finStatusB), string(exeStatusB))
	if finality == TxnStatusRejected {
		statusBody = fmt.Sprintf(txStatusStatusRejected, string(finStatusB), reason[0])
	}
	return fmt.Sprintf(txStatusResponse, txnHash, statusBody, id)
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

// bodyCloser is making linter happy and closes response body
func bodyCloser(t *testing.T, resp *http.Response) {
	if resp.Body != nil {
		require.NoError(t, resp.Body.Close())
	}
}
