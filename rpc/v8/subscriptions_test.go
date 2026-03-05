// todo(rdr): is ok for this tests to use rpcv8 instead of rpcv8 pkg
package rpcv8

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
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
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
		handler := New(mockChain, mockSyncer, nil, log)

		keys := make([][]felt.Felt, 1024+1)
		fromAddr := felt.NewFromBytes[felt.Address]([]byte("from_address"))

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, nil)
		assert.Zero(t, id)
		assert.Equal(t, rpccore.ErrTooManyKeysInFilter, rpcErr)
	})

	t.Run("Return error if block is too far back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		keys := make([][]felt.Felt, 1)
		fromAddr := felt.NewFromBytes[felt.Address]([]byte("from_address"))

		blockID := SubscriptionBlockID(BlockIDFromNumber(0))

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		// Note the end of the window doesn't need to be tested because if requested block number
		// is more than the head, a block not found error will be returned. This behaviour has been
		// tested in various other tests, and we don't need to test it here again.
		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number()).
				Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number()).
				Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})
	})

	n := &utils.Sepolia
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	b1, err := gw.BlockByNumber(t.Context(), 56377)
	require.NoError(t, err)

	b2, err := gw.BlockByNumber(t.Context(), 56378)
	require.NoError(t, err)

	pending1 := createTestPendingBlock(t, b2, 3)
	pending2 := createTestPendingBlock(t, b2, 6)

	fromAddr := felt.NewFromBytes[felt.Address]([]byte("some address"))
	keys := [][]felt.Felt{{felt.FromBytes[felt.Felt]([]byte("key1"))}}

	b1Filtered, b1Emitted := createTestEvents(t, b1)
	b2Filtered, b2Emitted := createTestEvents(t, b2)
	pending1Filtered, pending1Emitted := createTestEvents(t, pending1)
	pending2Filtered, pending2Emitted := createTestEvents(t, pending2)

	t.Run("Events from new blocks", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)

		handler := New(mockChain, mockSyncer, nil, log)

		mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
		mockChain.EXPECT().EventFilter(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockEventFilterer, nil).AnyTimes()
		mockChain.EXPECT().BlockByNumber(gomock.Any()).Return(b1, nil).AnyTimes()
		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).
			Return(nil).AnyTimes()
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b1Filtered, blockchain.ContinuationToken{}, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		id, clientConn := createTestEventsWebsocket(t, handler, fromAddr, keys, nil)

		assertNextEvents(t, clientConn, id, b1Emitted)

		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b2Filtered, blockchain.ContinuationToken{}, nil)
		handler.newHeads.Send(b2)
		assertNextEvents(t, clientConn, id, b2Emitted)
	})

	t.Run("Events from old blocks", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
		mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
		mockChain.EXPECT().EventFilter(
			[]felt.Address{*fromAddr},
			keys,
			gomock.Any(),
		).Return(mockEventFilterer, nil)

		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b1Filtered, blockchain.ContinuationToken{}, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		id, clientConn := createTestEventsWebsocket(t, handler, fromAddr, keys, &b1.Number)

		assertNextEvents(t, clientConn, id, b1Emitted)
	})

	t.Run("Events when continuation token is not nil", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
		mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
		mockChain.EXPECT().EventFilter(
			[]felt.Address{*fromAddr},
			keys,
			gomock.Any(),
		).Return(mockEventFilterer, nil)

		cToken := blockchain.ContinuationToken{}
		require.NoError(t, cToken.FromString(fmt.Sprintf("%d-0", b2.Number)))
		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(2)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, cToken, nil)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b2Filtered, blockchain.ContinuationToken{}, nil)
		mockEventFilterer.EXPECT().Close().AnyTimes()

		id, clientConn := createTestEventsWebsocket(t, handler, fromAddr, keys, &b1.Number)

		assertNextEvents(t, clientConn, id, b1Emitted)
		assertNextEvents(t, clientConn, id, b2Emitted)
	})

	t.Run("Events from pending block without duplicates", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)

		handler := New(mockChain, mockSyncer, nil, log)

		mockChain.EXPECT().EventFilter(
			[]felt.Address{*fromAddr},
			keys,
			gomock.Any(),
		).Return(mockEventFilterer, nil).AnyTimes()
		mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		mockEventFilterer.EXPECT().Close().AnyTimes()

		mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b1Filtered, blockchain.ContinuationToken{}, nil)

		id, clientConn := createTestEventsWebsocket(t, handler, fromAddr, keys, nil)

		assertNextEvents(t, clientConn, id, b1Emitted)

		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(pending1Filtered, blockchain.ContinuationToken{}, nil)
		pendingData1 := core.NewPending(pending1, nil, nil)
		handler.pendingData.Send(&pendingData1)
		assertNextEvents(t, clientConn, id, pending1Emitted)

		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(pending2Filtered, blockchain.ContinuationToken{}, nil)
		pendingData2 := core.NewPending(pending2, nil, nil)
		handler.pendingData.Send(&pendingData2)
		assertNextEvents(t, clientConn, id, pending2Emitted[len(pending1Emitted):])

		mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
			Return(b2Filtered, blockchain.ContinuationToken{}, nil)
		handler.newHeads.Send(b2)
		assertNextEvents(t, clientConn, id, b2Emitted[len(pending2Emitted):])
	})
}

func TestSubscribeTxnStatus(t *testing.T) {
	log := utils.NewNopZapLogger()
	txHash := new(felt.Felt).SetUint64(1)
	t.Run("Don't return error even when transaction is not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		oldTimeout, oldTicker := subscribeTxStatusTimeout, subscribeTxStatusTickerDuration
		subscribeTxStatusTimeout, subscribeTxStatusTickerDuration = 100*time.Millisecond, 10*time.Millisecond
		t.Cleanup(func() {
			subscribeTxStatusTimeout = oldTimeout
			subscribeTxStatusTickerDuration = oldTicker
		})

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txHash),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound).AnyTimes()
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).AnyTimes()
		id, _ := createTestTxStatusWebsocket(t, handler, txHash)

		_, hasSubscription := handler.subscriptions.Load(string(id))
		require.True(t, hasSubscription)

		time.Sleep(200 * time.Millisecond)
		_, hasSubscription = handler.subscriptions.Load(string(id))
		require.False(t, hasSubscription)
	})

	t.Run("Transaction status is final", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)
		handler.WithFeeder(feeder.NewTestClient(t, &utils.SepoliaIntegration))
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).AnyTimes()
		t.Run("reverted", func(t *testing.T) {
			txHash, err := new(felt.Felt).SetString("0x1011")
			require.NoError(t, err)

			mockChain.EXPECT().BlockNumberAndIndexByTxHash(
				(*felt.TransactionHash)(txHash),
			).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnFailure, "some error")
		})

		t.Run("rejected", func(t *testing.T) {
			txHash, err := new(felt.Felt).SetString("0x1111")
			require.NoError(t, err)

			mockChain.EXPECT().BlockNumberAndIndexByTxHash(
				(*felt.TransactionHash)(txHash),
			).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusRejected, 0, "some error")
		})

		t.Run("accepted on L1", func(t *testing.T) {
			txHash, err := new(felt.Felt).SetString("0x1010")
			require.NoError(t, err)

			mockChain.EXPECT().BlockNumberAndIndexByTxHash(
				(*felt.TransactionHash)(txHash),
			).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL1, TxnSuccess, "")
		})
	})

	t.Run("Multiple transaction status update", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
		gw := adaptfeeder.New(client)
		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)
		handler.WithFeeder(client)

		block, err := gw.BlockByNumber(t.Context(), 38748)
		require.NoError(t, err)

		txHash, err := new(felt.Felt).SetString("0x1001")
		require.NoError(t, err)

		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			gomock.Any(),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		id, conn := createTestTxStatusWebsocket(t, handler, txHash)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusReceived, TxnSuccess, "")

		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txHash),
		).Return(block.Number, uint64(0), nil)
		mockChain.EXPECT().TransactionByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(block.Transactions[0], nil)
		mockChain.EXPECT().ReceiptByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(*block.Receipts[0], block.Hash, nil)
		mockChain.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)
		for i := range 3 {
			handler.pendingData.Send(&core.Pending{Block: &core.Block{Header: &core.Header{}}})
			handler.pendingData.Send(&core.Pending{Block: &core.Block{Header: &core.Header{}}})
			handler.newHeads.Send(&core.Block{Header: &core.Header{Number: block.Number + 1 + uint64(i)}})
		}
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnSuccess, "")

		l1Head := core.L1Head{BlockNumber: block.Number}
		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txHash),
		).Return(block.Number, uint64(0), nil)
		mockChain.EXPECT().TransactionByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(block.Transactions[0], nil)
		mockChain.EXPECT().ReceiptByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(*block.Receipts[0], block.Hash, nil)
		mockChain.EXPECT().L1Head().Return(l1Head, nil)
		handler.l1Heads.Send(&l1Head)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL1, TxnSuccess, "")
	})
}

type fakeSyncer struct {
	newHeads    *feed.Feed[*core.Block]
	reorgs      *feed.Feed[*sync.ReorgBlockRange]
	pendingData *feed.Feed[core.PendingData]
	preLatest   *feed.Feed[*core.PreLatest]
}

func newFakeSyncer() *fakeSyncer {
	return &fakeSyncer{
		newHeads:    feed.New[*core.Block](),
		reorgs:      feed.New[*sync.ReorgBlockRange](),
		pendingData: feed.New[core.PendingData](),
		preLatest:   feed.New[*core.PreLatest](),
	}
}

func (fs *fakeSyncer) SubscribeNewHeads() sync.NewHeadSubscription {
	return sync.NewHeadSubscription{Subscription: fs.newHeads.Subscribe()}
}

func (fs *fakeSyncer) SubscribeReorg() sync.ReorgSubscription {
	return sync.ReorgSubscription{Subscription: fs.reorgs.Subscribe()}
}

func (fs *fakeSyncer) SubscribePendingData() sync.PendingDataSubscription {
	return sync.PendingDataSubscription{Subscription: fs.pendingData.Subscribe()}
}

func (fs *fakeSyncer) SubscribePreLatest() sync.PreLatestDataSubscription {
	return sync.PreLatestDataSubscription{Subscription: fs.preLatest.Subscribe()}
}

func (fs *fakeSyncer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (fs *fakeSyncer) HighestBlockHeader() *core.Header {
	return nil
}

func (fs *fakeSyncer) PendingData() (core.PendingData, error) {
	return nil, core.ErrPendingDataNotFound
}
func (fs *fakeSyncer) PendingBlock() *core.Block { return nil }
func (fs *fakeSyncer) PendingState() (core.StateReader, func() error, error) {
	return nil, nil, nil
}

func (fs *fakeSyncer) PendingStateBeforeIndex(
	index int,
) (core.StateReader, func() error, error) {
	return nil, nil, nil
}

func TestSubscribeNewHeads(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("Return error if block is too far back", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		blockID := SubscriptionBlockID(BlockIDFromNumber(0))

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		t.Run("head is 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 1024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number()).
				Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeNewHeads(subCtx, &blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number()).
				Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeNewHeads(subCtx, &blockID)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})
	})

	t.Run("new block is received", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		mockChain := mocks.NewMockReader(mockCtrl)
		syncer := newFakeSyncer()

		l1Feed := feed.New[*core.L1Head]()
		mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})

		handler, server := setupRPC(t, ctx, mockChain, syncer)
		conn := createWsConn(t, ctx, server)

		id := "1"
		handler.WithIDGen(func() string { return id })

		got := sendWsMessage(t, ctx, conn, subMsg("starknet_subscribeNewHeads"))
		require.Equal(t, subResp(id), got)

		// Ignore the first mock header
		_, _, err := conn.Read(ctx)
		require.NoError(t, err)

		// Simulate a new block
		syncer.newHeads.Send(testHeadBlock(t))

		// Receive a block header.
		_, headerGot, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, newHeadsResponse(id), string(headerGot))
	})
}

func TestSubscribeNewHeadsHistorical(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Mainnet)
	assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

	chain = blockchain.New(testDB, &utils.Mainnet)
	syncer := newFakeSyncer()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	handler, server := setupRPC(t, ctx, chain, syncer)

	conn := createWsConn(t, ctx, server)

	id := "1"
	handler.WithIDGen(func() string { return id })

	subMsg := `{"jsonrpc":"2.0","id":"1","method":"starknet_subscribeNewHeads", "params":{"block_id":{"block_number":0}}}`
	got := sendWsMessage(t, ctx, conn, subMsg)
	require.Equal(t, subResp(id), got)

	// Check block 0 content
	want := `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943","parent_hash":"0x0","block_number":0,"new_root":"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6","timestamp":1637069048,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":"","l2_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"}},"subscription_id":"%s"}}`
	want = fmt.Sprintf(want, id)
	_, block0Got, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(block0Got))

	// Simulate a new block
	syncer.newHeads.Send(testHeadBlock(t))

	// Check new block content
	_, newBlockGot, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(id), string(newBlockGot))
}

func TestMultipleSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	ctx, cancel := context.WithCancel(t.Context())
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

	firstID := "1"
	secondID := "2"

	handler.WithIDGen(func() string { return firstID })
	firstGot := sendWsMessage(t, ctx, conn1, subMsg("starknet_subscribeNewHeads"))
	require.NoError(t, err)
	require.Equal(t, subResp(firstID), firstGot)

	handler.WithIDGen(func() string { return secondID })
	secondGot := sendWsMessage(t, ctx, conn2, subMsg("starknet_subscribeNewHeads"))
	require.NoError(t, err)
	require.Equal(t, subResp(secondID), secondGot)

	// Ignore the first mock header
	_, _, err = conn1.Read(ctx)
	require.NoError(t, err)
	_, _, err = conn2.Read(ctx)
	require.NoError(t, err)

	// Simulate a new block
	syncer.newHeads.Send(testHeadBlock(t))

	// Receive a block header.
	_, firstHeaderGot, err := conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(firstID), string(firstHeaderGot))

	_, secondHeaderGot, err := conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, newHeadsResponse(secondID), string(secondHeaderGot))

	// Unsubscribe
	unsubMsg := `{"jsonrpc":"2.0","id":"1","method":"starknet_unsubscribe","params":[%s]}`
	require.NoError(
		t, conn1.Write(ctx, websocket.MessageBinary, fmt.Appendf([]byte{}, unsubMsg, firstID)),
	)
	require.NoError(
		t, conn2.Write(ctx, websocket.MessageBinary, fmt.Appendf([]byte{}, unsubMsg, secondID)),
	)
}

func TestSubscriptionReorg(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
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

	mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
	mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
		Return(nil, blockchain.ContinuationToken{}, nil).AnyTimes()
	mockEventFilterer.EXPECT().Close().Return(nil).AnyTimes()

	mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil).Times(len(testCases))
	mockChain.EXPECT().EventFilter(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mockEventFilterer, nil).AnyTimes()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := createWsConn(t, ctx, server)

			id := "1"
			handler.WithIDGen(func() string { return id })

			got := sendWsMessage(t, ctx, conn, subMsg(tc.subscribeMethod))
			require.Equal(t, subResp(id), got)

			if tc.ignoreFirst {
				_, _, err := conn.Read(ctx)
				require.NoError(t, err)
			}

			// Simulate a reorg
			syncer.reorgs.Send(&sync.ReorgBlockRange{
				StartBlockHash: felt.NewUnsafeFromString[felt.Felt](
					"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6",
				),
				StartBlockNum: 0,
				EndBlockHash: felt.NewUnsafeFromString[felt.Felt](
					"0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86",
				),
				EndBlockNum: 2,
			})

			// Receive reorg event
			expectedRes := `{"jsonrpc":"2.0","method":"starknet_subscriptionReorg","params":{"result":{"starting_block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","starting_block_number":0,"ending_block_hash":"0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86","ending_block_number":2},"subscription_id":"%s"}}`
			want := fmt.Sprintf(expectedRes, id)
			_, reorgGot, err := conn.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, want, string(reorgGot))
		})
	}
}

func TestSubscribePendingTxs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})
	mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).Times(3)
	syncer := newFakeSyncer()
	handler, server := setupRPC(t, ctx, mockChain, syncer)

	t.Run("Basic subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":"1","method":"starknet_subscribePendingTransactions"}`
		id := "1"
		handler.WithIDGen(func() string { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		parentHash := new(felt.Felt).SetUint64(1)

		hash1 := new(felt.Felt).SetUint64(1)
		addr1 := new(felt.Felt).SetUint64(11)

		hash2 := new(felt.Felt).SetUint64(2)
		addr2 := new(felt.Felt).SetUint64(22)

		hash3 := new(felt.Felt).SetUint64(3)
		hash4 := new(felt.Felt).SetUint64(4)
		hash5 := new(felt.Felt).SetUint64(5)

		syncer.pendingData.Send(&core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: parentHash,
				},
				Transactions: []core.Transaction{
					&core.InvokeTransaction{TransactionHash: hash1, SenderAddress: addr1},
					&core.DeclareTransaction{TransactionHash: hash2, SenderAddress: addr2},
					&core.DeployTransaction{TransactionHash: hash3},
					&core.DeployAccountTransaction{DeployTransaction: core.DeployTransaction{TransactionHash: hash4}},
					&core.L1HandlerTransaction{TransactionHash: hash5},
				},
			},
		})

		for _, expectedResult := range []string{"0x1", "0x2", "0x3", "0x4", "0x5"} {
			want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":"%s","subscription_id":"%s"}}`
			want = fmt.Sprintf(want, expectedResult, id)
			_, pendingTxsGot, err := conn.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, want, string(pendingTxsGot))
		}
	})

	t.Run("Filtered subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":"1","method":"starknet_subscribePendingTransactions", "params":{"sender_address":["0xb", "0x16"]}}`
		id := "1"
		handler.WithIDGen(func() string { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		parentHash := new(felt.Felt).SetUint64(1)

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

		syncer.pendingData.Send(&core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: parentHash,
				},
				Transactions: []core.Transaction{
					&core.InvokeTransaction{TransactionHash: hash1, SenderAddress: addr1},
					&core.DeclareTransaction{TransactionHash: hash2, SenderAddress: addr2},
					&core.DeployTransaction{TransactionHash: hash3},
					&core.DeployAccountTransaction{DeployTransaction: core.DeployTransaction{TransactionHash: hash4}},
					&core.L1HandlerTransaction{TransactionHash: hash5},
					&core.InvokeTransaction{TransactionHash: hash6, SenderAddress: addr6},
					&core.DeclareTransaction{TransactionHash: hash7, SenderAddress: addr7},
				},
			},
		})

		for _, expectedResult := range []string{"0x1", "0x2"} {
			want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":"%s","subscription_id":"%s"}}`
			want = fmt.Sprintf(want, expectedResult, id)
			_, pendingTxsGot, err := conn.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, want, string(pendingTxsGot))
		}
	})

	t.Run("Full details subscription", func(t *testing.T) {
		conn := createWsConn(t, ctx, server)

		subMsg := `{"jsonrpc":"2.0","id":"1","method":"starknet_subscribePendingTransactions", "params":{"transaction_details": true}}`
		id := "1"
		handler.WithIDGen(func() string { return id })
		got := sendWsMessage(t, ctx, conn, subMsg)
		require.Equal(t, subResp(id), got)

		parentHash := new(felt.Felt).SetUint64(1)
		syncer.pendingData.Send(&core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: parentHash,
				},
				Transactions: []core.Transaction{
					&core.InvokeTransaction{
						TransactionHash:      new(felt.Felt).SetUint64(1),
						CallData:             []*felt.Felt{new(felt.Felt).SetUint64(2)},
						TransactionSignature: []*felt.Felt{new(felt.Felt).SetUint64(3)},
						MaxFee:               new(felt.Felt).SetUint64(4),
						ContractAddress:      new(felt.Felt).SetUint64(5),
						Version:              new(core.TransactionVersion).SetUint64(3),
						EntryPointSelector:   new(felt.Felt).SetUint64(6),
						Nonce:                new(felt.Felt).SetUint64(7),
						SenderAddress:        new(felt.Felt).SetUint64(8),
						ResourceBounds: map[core.Resource]core.ResourceBounds{
							core.ResourceL1Gas: {
								MaxAmount:       1,
								MaxPricePerUnit: new(felt.Felt).SetUint64(1),
							},
							core.ResourceL2Gas: {
								MaxAmount:       1,
								MaxPricePerUnit: new(felt.Felt).SetUint64(1),
							},
							core.ResourceL1DataGas: {
								MaxAmount:       1,
								MaxPricePerUnit: new(felt.Felt).SetUint64(1),
							},
						},
						Tip:                   9,
						PaymasterData:         []*felt.Felt{new(felt.Felt).SetUint64(10)},
						AccountDeploymentData: []*felt.Felt{new(felt.Felt).SetUint64(11)},
					},
				},
			},
		})

		want := `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":{"transaction_hash":"0x1","type":"INVOKE","version":"0x3","nonce":"0x7","max_fee":"0x4","contract_address":"0x5","sender_address":"0x8","signature":["0x3"],"calldata":["0x2"],"entry_point_selector":"0x6","resource_bounds":{"l1_gas":{"max_amount":"0x1","max_price_per_unit":"0x1"},"l2_gas":{"max_amount":"0x1","max_price_per_unit":"0x1"},"l1_data_gas":{"max_amount":"0x1","max_price_per_unit":"0x1"}},"tip":"0x9","paymaster_data":["0xa"],"account_deployment_data":["0xb"],"nonce_data_availability_mode":"L1","fee_data_availability_mode":"L1"},"subscription_id":"%s"}}`
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

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribePendingTxs(subCtx, nil, addresses)
		assert.Zero(t, id)
		assert.Equal(t, rpccore.ErrTooManyAddressesInFilter, rpcErr)
	})
}

func TestUnsubscribe(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("error when no connection in context", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		success, rpcErr := handler.Unsubscribe(t.Context(), "1")
		assert.False(t, success)
		assert.Equal(t, jsonrpc.Err(jsonrpc.MethodNotFound, nil), rpcErr)
	})

	t.Run("error when subscription ID doesn't exist", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		ctx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
		success, rpcErr := handler.Unsubscribe(ctx, "999")
		assert.False(t, success)
		assert.Equal(t, rpccore.ErrInvalidSubscriptionID, rpcErr)
	})

	t.Run("return false when connection doesn't match", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		// Create original subscription
		serverConn1, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn1.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn1})
		_, subscriptionCtxCancel := context.WithCancel(subCtx)
		sub := &subscription{
			cancel: subscriptionCtxCancel,
			conn:   &fakeConn{w: serverConn1},
		}
		handler.subscriptions.Store(uint64(1), sub)

		// Try to unsubscribe with different connection
		serverConn2, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn2.Close())
		})

		unsubCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn2})
		success, rpcErr := handler.Unsubscribe(unsubCtx, "1")
		assert.False(t, success)
		assert.NotNil(t, rpcErr)
	})

	t.Run("successful unsubscribe", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		conn := &fakeConn{w: serverConn}
		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, conn)
		_, subscriptionCtxCancel := context.WithCancel(subCtx)
		sub := &subscription{
			cancel: subscriptionCtxCancel,
			conn:   conn,
		}
		handler.subscriptions.Store("1", sub)

		success, rpcErr := handler.Unsubscribe(subCtx, "1")
		assert.True(t, success)
		assert.Nil(t, rpcErr)

		// Verify subscription was removed
		_, exists := handler.subscriptions.Load("1")
		assert.False(t, exists)
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

func subResp(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":"1"}`, id)
}

func subMsg(method string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":"1","method":%q}`, method)
}

func testHeadBlock(t *testing.T) *core.Block {
	t.Helper()

	n := utils.HeapPtr(utils.Sepolia)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	b1, err := gw.BlockByNumber(t.Context(), 56377)
	require.NoError(t, err)

	return b1
}

func newHeadsResponse(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x609e8ffabfdca05b5a2e7c1bd99fc95a757e7b4ef9186aeb1f301f3741458ce","parent_hash":"0x5d5e7c03c7ef4419c0847d7ae1d1079b6f91fa952ebdb20b74ca2e621017f02","block_number":56377,"new_root":"0x2a899e1200baa9b843cbfb65d63f4f746cec27f8edb42f8446ae349b532f8b3","timestamp":1712213818,"sequencer_address":"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8","l1_gas_price":{"price_in_fri":"0x1d1a94a20000","price_in_wei":"0x4a817c800"},"l1_data_gas_price":{"price_in_fri":"0x2dfb78bf913d","price_in_wei":"0x6b85dda55"},"l1_da_mode":"BLOB","starknet_version":"0.13.1","l2_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"}},"subscription_id":%q}}`, id)
}

// setupRPC creates a RPC handler that runs in a goroutine and a JSONRPC server that can be used to test subscriptions
func setupRPC(t *testing.T, ctx context.Context, chain blockchain.Reader, syncer sync.Reader) (*Handler, *jsonrpc.Server) {
	t.Helper()

	log := utils.NewNopZapLogger()
	handler := New(chain, syncer, nil, log)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	time.Sleep(50 * time.Millisecond)

	server := jsonrpc.NewServer(1, log)
	methods, _ := handler.methods()
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

func marshalSubEventsResp(method string, result any, id SubscriptionID) ([]byte, error) {
	return json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  method,
		Params: map[string]any{
			"subscription_id": id,
			"result":          result,
		},
	})
}

func assertNextMessage(t *testing.T, conn net.Conn, id SubscriptionID, method string, result any) {
	t.Helper()

	resp, err := marshalSubEventsResp(method, result, id)
	require.NoError(t, err)

	got := make([]byte, len(resp))
	_, err = conn.Read(got)
	require.NoError(t, err)
	assert.Equal(t, string(resp), string(got))
}

func assertNextTxnStatus(t *testing.T, conn net.Conn, id SubscriptionID, txHash *felt.Felt, finality TxnStatus, execution TxnExecutionStatus, failureReason string) {
	t.Helper()

	assertNextMessage(t, conn, id, "starknet_subscriptionTransactionStatus", SubscriptionTransactionStatus{
		TransactionHash: txHash,
		Status: TransactionStatus{
			Finality:      finality,
			Execution:     execution,
			FailureReason: failureReason,
		},
	})
}

func assertNextEvents(
	t *testing.T,
	conn net.Conn,
	id SubscriptionID,
	emittedEvents []EmittedEvent,
) {
	t.Helper()

	for _, emitted := range emittedEvents {
		assertNextMessage(t, conn, id, "starknet_subscriptionEvents", emitted)
	}
}

func createTestPendingBlock(t *testing.T, b *core.Block, txCount int) *core.Block {
	t.Helper()

	pending := core.Block{
		Header: &core.Header{
			// Pending block does not have number but we internaly set it
			Number:           b.Number,
			ParentHash:       b.ParentHash,
			SequencerAddress: b.SequencerAddress,
		},
	}

	pending.Transactions = b.Transactions[:txCount]
	pending.Receipts = b.Receipts[:txCount]
	return &pending
}

func createTestEvents(
	t *testing.T,
	b *core.Block,
) ([]blockchain.FilteredEvent, []EmittedEvent) {
	t.Helper()

	var filtered []blockchain.FilteredEvent
	var emitted []EmittedEvent
	for _, receipt := range b.Receipts {
		for i, event := range receipt.Events {
			filtered = append(filtered, blockchain.FilteredEvent{
				Event:           event,
				BlockNumber:     &b.Number,
				BlockHash:       b.Hash,
				TransactionHash: receipt.TransactionHash,
				EventIndex:      uint(i),
			})
			emitted = append(emitted, EmittedEvent{
				Event: &Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
				BlockNumber:     &b.Number,
				BlockHash:       b.Hash,
				TransactionHash: receipt.TransactionHash,
			})
		}
	}
	return filtered, emitted
}

func createTestEventsWebsocket(
	t *testing.T,
	h *Handler,
	fromAddr *felt.Address,
	keys [][]felt.Felt,
	startBlock *uint64,
) (SubscriptionID, net.Conn) {
	t.Helper()

	return createTestWebsocket(t, func(ctx context.Context) (SubscriptionID, *jsonrpc.Error) {
		if startBlock != nil {
			blockID := SubscriptionBlockID(BlockIDFromNumber(*startBlock))
			return h.SubscribeEvents(ctx, fromAddr, keys, &blockID)
		}
		return h.SubscribeEvents(ctx, fromAddr, keys, nil)
	})
}

func createTestTxStatusWebsocket(
	t *testing.T, h *Handler, txHash *felt.Felt,
) (SubscriptionID, net.Conn) {
	t.Helper()

	return createTestWebsocket(t, func(ctx context.Context) (SubscriptionID, *jsonrpc.Error) {
		return h.SubscribeTransactionStatus(ctx, txHash)
	})
}

func createTestWebsocket(t *testing.T, subscribe func(context.Context) (SubscriptionID, *jsonrpc.Error)) (SubscriptionID, net.Conn) {
	t.Helper()

	serverConn, clientConn := net.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr := subscribe(subCtx)
	require.Nil(t, rpcErr)

	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		require.NoError(t, clientConn.Close())
		cancel()
		time.Sleep(100 * time.Millisecond)
	})

	return id, clientConn
}
