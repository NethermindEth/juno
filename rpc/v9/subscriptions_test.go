package rpcv9

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
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
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

type fakeSyncer struct {
	newHeads    *feed.Feed[*core.Block]
	reorgs      *feed.Feed[*sync.ReorgBlockRange]
	pendingData *feed.Feed[core.PendingData]
}

func newFakeSyncer() *fakeSyncer {
	return &fakeSyncer{
		newHeads:    feed.New[*core.Block](),
		reorgs:      feed.New[*sync.ReorgBlockRange](),
		pendingData: feed.New[core.PendingData](),
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

func (fs *fakeSyncer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (fs *fakeSyncer) HighestBlockHeader() *core.Header {
	return nil
}

func (fs *fakeSyncer) PendingData() (core.PendingData, error) {
	return nil, sync.ErrPendingBlockNotFound
}
func (fs *fakeSyncer) PendingBlock() *core.Block { return nil }

func (fs *fakeSyncer) PendingState() (core.StateReader, func() error, error) { return nil, nil, nil }

func (fs *fakeSyncer) PendingStateBeforeIndex(index int) (core.StateReader, func() error, error) {
	return nil, nil, nil
}

func TestSubscribeEventsInvalidInputs(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("Return error if too many keys in filter", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(mockChain, mockSyncer, nil, log)

		keys := make([][]felt.Felt, 1024+1)
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, nil, nil)
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
		fromAddr := new(felt.Felt).SetBytes([]byte("from_address"))

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

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &blockID, nil)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})

		t.Run("head is more than 1024", func(t *testing.T) {
			mockChain.EXPECT().HeadsHeader().Return(&core.Header{Number: 2024}, nil)
			mockChain.EXPECT().BlockHeaderByNumber(blockID.Number()).
				Return(&core.Header{Number: 0}, nil)

			id, rpcErr := handler.SubscribeEvents(subCtx, fromAddr, keys, &blockID, nil)
			assert.Zero(t, id)
			assert.Equal(t, rpccore.ErrTooManyBlocksBack, rpcErr)
		})
	})
}

func TestSubscribeEvents(t *testing.T) {
	log := utils.NewNopZapLogger()

	n := &utils.Sepolia
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	b1, err := gw.BlockByNumber(t.Context(), 56377)
	require.NoError(t, err)

	b2, err := gw.BlockByNumber(t.Context(), 56378)
	require.NoError(t, err)

	b1Filtered, b1Emitted := createTestEvents(
		t,
		b1,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)
	b2Filtered, b2Emitted := createTestEvents(
		t,
		b2,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)

	pendingB := createTestPendingBlock(t, b2, 6)
	pending := sync.NewPending(pendingB, nil, nil)
	_, pendingEmitted := createTestEvents(
		t,
		pendingB,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)
	pendingB2 := createTestPendingBlock(t, b2, 10)

	pending2 := sync.NewPending(pendingB2, nil, nil)
	_, pending2Emitted := createTestEvents(
		t,
		pendingB2,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)
	preConfirmed1 := createTestPreConfirmed(t, b2, 3)
	preConfirmed2 := createTestPreConfirmed(t, b2, 6)

	preConfirmed1Filtered, preConfirmed1Emitted := createTestEvents(
		t,
		preConfirmed1.Block,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)
	_, preConfirmed2Emitted := createTestEvents(
		t,
		preConfirmed2.Block,
		nil,
		nil,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	mockSyncer := mocks.NewMockSyncReader(mockCtrl)
	mockEventFilterer := mocks.NewMockEventFilterer(mockCtrl)
	mockChain.EXPECT().EventFilter(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mockEventFilterer, nil).AnyTimes()
	mockEventFilterer.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	mockEventFilterer.EXPECT().Close().AnyTimes()

	handler := New(mockChain, mockSyncer, nil, log)

	type stepInfo struct {
		description string
		setupMocks  func()
		notify      func()
		expect      [][]*SubscriptionEmittedEvent
	}

	type testCase struct {
		description    string
		blockID        *SubscriptionBlockID
		finalityStatus *TxnFinalityStatusWithoutL1
		keys           [][]felt.Felt
		fromAddr       *felt.Felt
		steps          []stepInfo
		setupMocks     func()
	}

	preStarknet0_14_0basicSubscription := testCase{
		description: "Events from new blocks - default status, Starknet version < 0.14.0",
		blockID:     nil,
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil).Times(1)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from latest on start",
				expect:      [][]*SubscriptionEmittedEvent{b1Emitted},
			},
			{
				description: "on pending block",
				notify: func() {
					handler.pendingData.Send(&pending)
				},
				expect: [][]*SubscriptionEmittedEvent{pendingEmitted},
			},
			{
				description: "on pending block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&pending2)
				},
				expect: [][]*SubscriptionEmittedEvent{pending2Emitted[len(pendingEmitted):]},
			},
			{
				description: "on new head, without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]*SubscriptionEmittedEvent{b2Emitted[len(pending2Emitted):]},
			},
		},
	}

	basicSubscription := testCase{
		description: "Events from new blocks - default status",
		blockID:     nil,
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from latest on start",
				expect:      [][]*SubscriptionEmittedEvent{b1Emitted},
			},
			{
				description: "on pre_confirmed block",
				notify: func() {
					handler.pendingData.Send(&preConfirmed1)
				},
				expect: [][]*SubscriptionEmittedEvent{},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*SubscriptionEmittedEvent{},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]*SubscriptionEmittedEvent{b2Emitted},
			},
		},
	}

	basicSubscriptionWithPreConfirmed := testCase{
		description:    "Events from new blocks - status PRE_CONFIRMED",
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		blockID:        nil,
		keys:           nil,
		fromAddr:       nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, nil, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(preConfirmed1Filtered, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed",
				expect:      [][]*SubscriptionEmittedEvent{append(b1Emitted, preConfirmed1Emitted...)},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*SubscriptionEmittedEvent{preConfirmed2Emitted[len(preConfirmed1Emitted):]},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]*SubscriptionEmittedEvent{b2Emitted},
			},
		},
	}

	eventsFromHistoricalBlocks := testCase{
		description: "Events from historical blocks - default status",
		blockID:     utils.HeapPtr(SubscriptionBlockID(BlockIDFromNumber(b1.Number))),
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b2.Header, nil)
			mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(append(b1Filtered, b2Filtered...), nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from last 2 blocks",
				expect:      [][]*SubscriptionEmittedEvent{append(b1Emitted, b2Emitted...)},
			},
		},
	}

	eventsWithContinuationToken := testCase{
		description: "Events with continuation token - default status",
		blockID:     utils.HeapPtr(SubscriptionBlockID(BlockIDFromNumber(b1.Number))),
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b2.Header, nil)
			mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)

			cToken := new(blockchain.ContinuationToken)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, cToken, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b2Filtered, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from last 2 blocks with continuation token",
				expect:      [][]*SubscriptionEmittedEvent{append(b1Emitted, b2Emitted...)},
			},
		},
	}
	targetAddress, err := new(felt.Felt).SetString("0x246ff8c7b475ddfb4cb5035867cba76025f08b22938e5684c18c2ab9d9f36d3")
	require.NoError(t, err)
	b1FilteredBySenders, b1EmittedFiltered := createTestEvents(
		t,
		b1,
		targetAddress,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)

	preConfirmedFilteredBySenders, preConfirmedEmittedFiltered := createTestEvents(
		t,
		preConfirmed1.Block,
		targetAddress,
		nil,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)

	_, preConfirmed2EmittedFiltered := createTestEvents(
		t,
		preConfirmed2.Block,
		targetAddress,
		nil,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)

	_, b2EmittedFiltered := createTestEvents(
		t,
		b2,
		targetAddress,
		nil,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)

	eventsWithFromAddressAndPreConfirmed := testCase{ //nolint:dupl // params and return values are different
		description:    "Events with from_address filter, finality PRE_CONFIRMED",
		blockID:        nil,
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		fromAddr:       targetAddress,
		keys:           nil,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1FilteredBySenders, nil, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(preConfirmedFilteredBySenders, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed",
				expect:      [][]*SubscriptionEmittedEvent{append(b1EmittedFiltered, preConfirmedEmittedFiltered...)},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*SubscriptionEmittedEvent{preConfirmed2EmittedFiltered[len(preConfirmedEmittedFiltered):]},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]*SubscriptionEmittedEvent{b2EmittedFiltered},
			},
		},
	}

	targetKey, err := new(felt.Felt).SetString("0x1dcde06aabdbca2f80aa51392b345d7549d7757aa855f7e37f5d335ac8243b1")
	require.NoError(t, err)
	keys := [][]felt.Felt{{*targetKey}}

	b1FilteredByFromAddressAndKey, b1EmittedWFilters := createTestEvents(
		t,
		b1,
		targetAddress,
		keys,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)

	preConfirmedFilteredBySendersAndKey, preConfirmedEmittedWFilters := createTestEvents(
		t,
		preConfirmed1.Block,
		targetAddress,
		keys,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)

	_, preConfirmed2EmittedWFilters := createTestEvents(
		t,
		preConfirmed2.Block,
		targetAddress,
		keys,
		TxnFinalityStatusWithoutL1(TxnPreConfirmed),
	)

	_, b2EmittedWFilters := createTestEvents(
		t,
		b2,
		targetAddress,
		keys,
		TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
	)

	eventsWithAllFilterAndPreConfirmed := testCase{ //nolint:dupl // params and return values are different
		description:    "Events with from_address and key, finality PRE_CONFIRMED",
		blockID:        nil,
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		fromAddr:       targetAddress,
		keys:           keys,
		setupMocks: func() {
			mockChain.EXPECT().HeadsHeader().Return(b1.Header, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1FilteredByFromAddressAndKey, nil, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(preConfirmedFilteredBySendersAndKey, nil, nil)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed on start",
				expect:      [][]*SubscriptionEmittedEvent{append(b1EmittedWFilters, preConfirmedEmittedWFilters...)},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*SubscriptionEmittedEvent{preConfirmed2EmittedWFilters[len(preConfirmedEmittedWFilters):]},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]*SubscriptionEmittedEvent{b2EmittedWFilters},
			},
		},
	}

	testCases := []testCase{
		preStarknet0_14_0basicSubscription,
		basicSubscription,
		basicSubscriptionWithPreConfirmed,
		eventsFromHistoricalBlocks,
		eventsWithContinuationToken,
		eventsWithFromAddressAndPreConfirmed,
		eventsWithAllFilterAndPreConfirmed,
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks()
			}
			subID, conn := createTestEventsWebsocket(
				t,
				handler,
				tc.fromAddr,
				tc.keys,
				tc.blockID,
				tc.finalityStatus,
			)

			for _, step := range tc.steps {
				t.Run(step.description, func(t *testing.T) {
					if step.setupMocks != nil {
						step.setupMocks()
					}

					if step.notify != nil {
						step.notify()
					}

					for _, expectedEvents := range step.expect {
						assertNextEvents(t, conn, subID, expectedEvents)
					}
				})
			}
		})
	}
}

func TestSubscribeTxnStatus(t *testing.T) {
	log := utils.NewNopZapLogger()
	txHash := new(felt.Felt).SetUint64(1)
	cacheSize := uint(5)
	cacheEntryTimeOut := time.Second

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
		cache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		handler := New(mockChain, mockSyncer, nil, log).WithSubmittedTransactionsCache(cache)

		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound).AnyTimes()
		mockSyncer.EXPECT().PendingData().Return(nil, sync.ErrPendingBlockNotFound).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).AnyTimes()
		mockSyncer.EXPECT().PendingBlock().Return(nil).AnyTimes()
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
		mockSyncer.EXPECT().PendingData().Return(nil, sync.ErrPendingBlockNotFound).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).AnyTimes()
		t.Run("reverted", func(t *testing.T) {
			txHash, err := new(felt.Felt).SetString("0x1011")
			require.NoError(t, err)

			mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)

			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnFailure, "some error")
		})
		t.Run("accepted on L1", func(t *testing.T) {
			txHash, err := new(felt.Felt).SetString("0x1010")
			require.NoError(t, err)

			mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL1, TxnSuccess, "")
		})
	})

	t.Run("Multiple transaction status update", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
		mockGateway := mocks.NewMockGateway(mockCtrl)
		adapterFeeder := adaptfeeder.New(client)
		mockChain := mocks.NewMockReader(mockCtrl)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		cache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		handler := New(mockChain, mockSyncer, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(cache)

		block, err := adapterFeeder.BlockByNumber(t.Context(), 38748)
		require.NoError(t, err)

		txToBroadcast := BroadcastedTransaction{Transaction: *AdaptTransaction(block.Transactions[0])}

		var tempGatewayResponse struct {
			TransactionHash *felt.Felt `json:"transaction_hash"`
			ContractAddress *felt.Felt `json:"address"`
			ClassHash       *felt.Felt `json:"class_hash"`
		}

		tempGatewayResponse.TransactionHash = txToBroadcast.Hash
		resRaw, err := json.Marshal(tempGatewayResponse)
		require.NoError(t, err)
		mockGateway.
			EXPECT().
			AddTransaction(gomock.Any(), gomock.Any()).Return(resRaw, nil).
			AnyTimes()

		addRes, addErr := handler.AddTransaction(
			t.Context(),
			txToBroadcast,
		)
		require.Nil(t, addErr)
		txHash := addRes.TransactionHash
		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingData().Return(nil, sync.ErrPendingBlockNotFound).Times(2)
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).Times(2)

		id, conn := createTestTxStatusWebsocket(t, handler, txHash)

		assertNextTxnStatus(t, conn, id, txHash, TxnStatusReceived, UnknownExecution, "")
		// Candidate Status
		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		preConfirmed := core.NewPreConfirmed(
			&core.Block{Header: block.Header},
			nil,
			nil,
			[]core.Transaction{block.Transactions[0]})

		mockSyncer.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		).Times(4)
		handler.pendingData.Send(&core.PreConfirmed{
			Block: &core.Block{Header: &core.Header{}},
		})

		assertNextTxnStatus(t, conn, id, txHash, TxnStatusCandidate, UnknownExecution, "")
		require.Equal(t, block.Transactions[0].Hash(), txHash)

		// PreConfirmed Status
		rpcTx := AdaptTransaction(block.Transactions[0])
		rpcTx.Hash = txHash

		mockChain.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		preConfirmed = core.PreConfirmed{
			Block: &core.Block{
				Transactions: []core.Transaction{
					block.Transactions[0],
				},
				Receipts: []*core.TransactionReceipt{block.Receipts[0]},
			},
			CandidateTxs: []core.Transaction{block.Transactions[0]},
		}

		handler.pendingData.Send(&preConfirmed)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusPreConfirmed, TxnSuccess, "")
		// Accepted on l1 Status
		mockChain.EXPECT().TransactionByHash(txHash).Return(block.Transactions[0], nil)
		mockChain.EXPECT().Receipt(txHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
		mockChain.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		handler.newHeads.Send(&core.Block{Header: &core.Header{Number: block.Number + 1}})

		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnSuccess, "")

		l1Head := &core.L1Head{BlockNumber: block.Number}
		mockChain.EXPECT().TransactionByHash(txHash).Return(block.Transactions[0], nil)
		mockChain.EXPECT().Receipt(txHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
		mockChain.EXPECT().L1Head().Return(l1Head, nil)
		handler.l1Heads.Send(l1Head)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL1, TxnSuccess, "")
	})
}

func TestSubscribeNewHeads(t *testing.T) {
	log := utils.NewNopZapLogger()

	t.Run("BlockID - Number, Invalid Input", func(t *testing.T) {
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

		t.Run("BlockID head is 1024", func(t *testing.T) {
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
		Return(nil, nil, nil).AnyTimes()
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
				StartBlockHash: utils.HexToFelt(
					t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6",
				),
				StartBlockNum: 0,
				EndBlockHash: utils.HexToFelt(
					t, "0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86",
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

func TestSubscribeNewTransactions(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	syncer := newFakeSyncer()
	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})
	handler, _ := setupRPC(t, ctx, mockChain, syncer)

	n := &utils.Sepolia
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	newHead1, err := gw.BlockByNumber(t.Context(), 56377)
	require.NoError(t, err)

	newHead2, err := gw.BlockByNumber(t.Context(), 56378)
	require.NoError(t, err)

	toTransactionsWithFinalityStatus := func(txs []core.Transaction, finalityStatus TxnStatusWithoutL1) []*SubscriptionNewTransaction {
		txsWithStatus := make([]*SubscriptionNewTransaction, len(txs))
		for i, txn := range txs {
			txsWithStatus[i] = &SubscriptionNewTransaction{
				Transaction:    *AdaptTransaction(txn),
				FinalityStatus: finalityStatus,
			}
		}
		return txsWithStatus
	}

	pendingBlockTxCount := 6
	pendingBlock := createTestPendingBlock(t, newHead2, pendingBlockTxCount)
	pending := sync.NewPending(pendingBlock, nil, nil)

	initialPreconfirmedCount := 3
	secondPreConfirmedCount := 6
	thirdPreConfirmedCount := len(newHead2.Transactions)

	type stepInfo struct {
		description string
		notify      func()
		expect      [][]*SubscriptionNewTransaction
	}

	type testCase struct {
		description   string
		statuses      []TxnStatusWithoutL1
		senderAddress []felt.Felt
		steps         []stepInfo
	}

	preStarknet0_14_0DefaultFinality := testCase{
		description:   "Basic subcription - default finality status. Starknet version < 0.14.0, Pending txs without duplicates",
		statuses:      nil,
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "onNewHead",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead1.Transactions, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)),
				},
			},
			{
				description: "onPendingBLock",
				notify: func() {
					syncer.pendingData.Send(&pending)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(pendingBlock.Transactions, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)),
				},
			},
			{
				description: "pending becomes new head without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions[pendingBlockTxCount:], TxnStatusWithoutL1(TxnAcceptedOnL2)),
				},
			},
		},
	}

	defaultFinality := testCase{
		description:   "Basic subcription - default finality status. Starknet version >= 0.14.0, Transactions from pre_confirmed block",
		statuses:      nil,
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on new head receive all txs with ACCEPTED_ON_L2",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead1.Transactions, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)),
				},
			},
			{
				description: "on new pre_confirmed receive PRE_CONFIRMED and CANDIDATE txs",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, initialPreconfirmedCount)))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions[:initialPreconfirmedCount], TxnStatusWithoutL1(TxnStatusPreConfirmed)),
					toTransactionsWithFinalityStatus(newHead2.Transactions[initialPreconfirmedCount:], TxnStatusWithoutL1(TxnStatusCandidate)),
				},
			},
			{
				description: "on pre_confirmed update subset of candidates moved to PRE_CONFIRMED, without dup.",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, secondPreConfirmedCount)))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions[initialPreconfirmedCount:secondPreConfirmedCount], TxnStatusWithoutL1(TxnStatusPreConfirmed)),
				},
			},
			{
				description: "on pre_confirmed update all candidates moved to PRE_CONFIRMED, without dup.",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, thirdPreConfirmedCount)))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions[secondPreConfirmedCount:], TxnStatusWithoutL1(TxnStatusPreConfirmed)),
				},
			},
			{
				description: "pre_confirmed become new head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions, TxnStatusWithoutL1(TxnAcceptedOnL2)),
				},
			},
		},
	}

	onlyAcceptedOnL2 := testCase{
		description:   "Basic subcription - only ACCEPTED_ON_L2",
		statuses:      []TxnStatusWithoutL1{TxnStatusWithoutL1(TxnStatusAcceptedOnL2)},
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on new head receive all txs with ACCEPTED_ON_L2",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead1.Transactions, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)),
				},
			},
			{
				description: "on new pre_confirmed no stream",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, initialPreconfirmedCount)))
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "pre_confirmed become new head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions, TxnStatusWithoutL1(TxnAcceptedOnL2)),
				},
			},
		},
	}

	allStatuses := testCase{
		description: "Basic Subscription- all statuses",
		statuses: []TxnStatusWithoutL1{
			TxnStatusWithoutL1(TxnStatusReceived),
			TxnStatusWithoutL1(TxnStatusCandidate),
			TxnStatusWithoutL1(TxnStatusPreConfirmed),
			TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
		},
		senderAddress: nil,
		steps: []stepInfo{
			// {
			// 	description: "on receiving new transaction",
			// 	notify: func() {
			// 		handler.receivedTxFeed.Send(newHead2.Transactions[0])
			// 	},
			// 	expect: [][]*NewTransactionSubscriptionResponse{
			// 		toTransactionsWithFinalityStatus(newHead2.Transactions[:1], TxnStatusWithoutL1(TxnStatusReceived)),
			// 	},
			// },
			{
				description: "on new pre_confirmed with candidates",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, secondPreConfirmedCount)))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions[:secondPreConfirmedCount], TxnStatusWithoutL1(TxnStatusPreConfirmed)),
					toTransactionsWithFinalityStatus(newHead2.Transactions[secondPreConfirmedCount:], TxnStatusWithoutL1(TxnStatusCandidate)),
				},
			},
			{
				description: "pre_confirmed becomes new head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(newHead2.Transactions, TxnStatusWithoutL1(TxnAcceptedOnL2)),
				},
			},
		},
	}

	senderAddress := AdaptTransaction(newHead2.Transactions[0]).SenderAddress
	senderFilter := []felt.Felt{*senderAddress}
	senderTransactions := make([]core.Transaction, 0)
	for _, txn := range newHead2.Transactions {
		if filterTxBySender(txn, senderFilter) {
			senderTransactions = append(senderTransactions, txn)
		}
	}

	allStatusesWithFilter := testCase{
		description: "Subscription with sender filter - all statuses",
		statuses: []TxnStatusWithoutL1{
			TxnStatusWithoutL1(TxnStatusReceived),
			TxnStatusWithoutL1(TxnStatusCandidate),
			TxnStatusWithoutL1(TxnStatusPreConfirmed),
			TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
		},
		senderAddress: []felt.Felt{*senderAddress},
		steps: []stepInfo{
			//  {
			//  	description: "on receiving new transaction",
			//  	notify: func() {
			//  		handler.receivedTxFeed.Send(newHead2.Transactions[0])
			//  	},
			//  	expect: [][]*NewTransactionSubscriptionResponse{
			//  		toTransactionsWithFinalityStatus(newHead2.Transactions[:1], TxnStatusWithoutL1(TxnStatusReceived)),
			//  	},
			//  },
			{
				description: "on new pre_confirmed full of candidates",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, 0)))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(senderTransactions, TxnStatusWithoutL1(TxnStatusCandidate)),
				},
			},
			{
				description: "on new pre_confirmed full of preconfirmed",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, len(newHead2.Transactions))))
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(senderTransactions, TxnStatusWithoutL1(TxnStatusPreConfirmed)),
				},
			},
			{
				description: "pre_confirmed becomes new head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(senderTransactions, TxnStatusWithoutL1(TxnAcceptedOnL2)),
				},
			},
		},
	}

	testCases := []testCase{
		preStarknet0_14_0DefaultFinality,
		defaultFinality,
		onlyAcceptedOnL2,
		allStatuses,
		allStatusesWithFilter,
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			subID, conn := createTestNewTransactionsWebsocket(t, handler, tc.statuses, tc.senderAddress)
			for _, step := range tc.steps {
				t.Run(step.description, func(t *testing.T) {
					step.notify()
					for _, expectedTransactions := range step.expect {
						assertNextTransactions(t, conn, subID, expectedTransactions)
					}
				})
			}
		})
	}

	t.Run("Return error if too many addresses in filter", func(t *testing.T) { //nolint:dupl // not duplicate, similar test for different method
		addresses := make([]felt.Felt, rpccore.MaxEventFilterKeys+1)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeNewTransactions(subCtx, nil, addresses)
		assert.Zero(t, id)
		assert.Equal(t, rpccore.ErrTooManyAddressesInFilter, rpcErr)
	})
}

func TestSubscribeTransactionReceipts(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockChain := mocks.NewMockReader(mockCtrl)
	syncer := newFakeSyncer()
	l1Feed := feed.New[*core.L1Head]()
	mockChain.EXPECT().SubscribeL1Head().Return(blockchain.L1HeadSubscription{Subscription: l1Feed.Subscribe()})
	handler, _ := setupRPC(t, ctx, mockChain, syncer)

	n := &utils.Sepolia
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	newHead1, err := gw.BlockByNumber(t.Context(), 56377)
	require.NoError(t, err)

	newHead2, err := gw.BlockByNumber(t.Context(), 56378)
	require.NoError(t, err)

	type stepInfo struct {
		description string
		notify      func()
		expect      [][]*TransactionReceipt
	}

	type testCase struct {
		description   string
		statuses      []TxnFinalityStatusWithoutL1
		senderAddress []felt.Felt
		steps         []stepInfo
	}

	toAdaptedReceiptsWithFilter := func(b *core.Block, senderAddress []felt.Felt, finalityStatus TxnFinalityStatus) []*TransactionReceipt {
		receipts := make([]*TransactionReceipt, 0)
		for i, receipt := range b.Receipts {
			txn := b.Transactions[i]
			if filterTxBySender(txn, senderAddress) {
				receipts = append(receipts, AdaptReceipt(receipt, txn, finalityStatus, b.Hash, b.Number))
			}
		}
		return receipts
	}

	pendingB1TxCount := 3
	pendingB2TxCount := 6
	pendingBlock1 := createTestPendingBlock(t, newHead2, pendingB1TxCount)
	pendingBlock2 := createTestPendingBlock(t, newHead2, pendingB2TxCount)

	preStarknet0_14_0defaultFinalityStatus := testCase{
		description: "Basic subscription with default finality status, starknet version < 0.14.0",
		statuses:    nil,
		steps: []stepInfo{
			{
				description: "on new head",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2),
				},
			},
			{
				description: "on pending",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(sync.NewPending(pendingBlock1, nil, nil)))
				},
				expect: [][]*TransactionReceipt{toAdaptedReceiptsWithFilter(pendingBlock1, nil, TxnAcceptedOnL2)},
			},
			{
				description: "on pending block update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(sync.NewPending(pendingBlock2, nil, nil)))
				},
				expect: [][]*TransactionReceipt{toAdaptedReceiptsWithFilter(pendingBlock2, nil, TxnAcceptedOnL2)[pendingB1TxCount:]},
			},
			{
				description: "on next head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, nil, TxnAcceptedOnL2)[pendingB2TxCount:],
				},
			},
		},
	}

	preConfirmedB1TxCount := 3
	preConfirmedB2TxCount := 6
	preConfirmed1 := createTestPreConfirmed(t, newHead2, preConfirmedB1TxCount)
	preConfirmed2 := createTestPreConfirmed(t, newHead2, preConfirmedB2TxCount)

	defaultFinalityStatus := testCase{
		description: "Basic subscription with default finality status",
		statuses:    nil,
		steps: []stepInfo{
			{
				description: "on new head",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2),
				},
			},
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(utils.HeapPtr(createTestPreConfirmed(t, newHead2, len(newHead2.Transactions))))
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, nil, TxnAcceptedOnL2),
				},
			},
		},
	}

	onlyPreConfirmed := testCase{
		description: "Basic subscription with only PRE_CONFIRMED status",
		statuses:    []TxnFinalityStatusWithoutL1{TxnFinalityStatusWithoutL1(TxnPreConfirmed)},
		steps: []stepInfo{
			{
				description: "on new head, no response",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(preConfirmed1.GetBlock(), nil, TxnPreConfirmed),
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(preConfirmed2.GetBlock(), nil, TxnPreConfirmed)[preConfirmedB1TxCount:],
				},
			},
			{
				description: "on next head, no response",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{},
			},
		},
	}

	allStatuses := testCase{
		description: "Basic subscription with all statuses",
		statuses:    []TxnFinalityStatusWithoutL1{TxnFinalityStatusWithoutL1(TxnPreConfirmed), TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)},
		steps: []stepInfo{
			{
				description: "on new head",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2),
				},
			},
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(preConfirmed1.GetBlock(), nil, TxnPreConfirmed),
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(preConfirmed2.GetBlock(), nil, TxnPreConfirmed)[preConfirmedB1TxCount:],
				},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, nil, TxnAcceptedOnL2),
				},
			},
		},
	}

	preConfirmed3 := createTestPreConfirmed(t, newHead2, len(newHead2.Transactions))
	senderAddress := AdaptTransaction(newHead2.Transactions[0]).SenderAddress
	senderFilter := []felt.Felt{*senderAddress}
	preConfirmed1FilteredReceipts := toAdaptedReceiptsWithFilter(preConfirmed1.GetBlock(), senderFilter, TxnPreConfirmed)

	allStatusesWithFilter := testCase{
		description:   "subscription with filter and all statuses",
		senderAddress: senderFilter,
		statuses:      []TxnFinalityStatusWithoutL1{TxnFinalityStatusWithoutL1(TxnPreConfirmed), TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)},
		steps: []stepInfo{
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed1)
				},
				expect: [][]*TransactionReceipt{
					preConfirmed1FilteredReceipts,
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&preConfirmed3)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(preConfirmed3.GetBlock(), senderFilter, TxnPreConfirmed)[len(preConfirmed1FilteredReceipts):],
				},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, senderFilter, TxnAcceptedOnL2),
				},
			},
		},
	}

	testCases := []testCase{
		defaultFinalityStatus,
		preStarknet0_14_0defaultFinalityStatus,
		allStatuses,
		allStatusesWithFilter,
		onlyPreConfirmed,
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			subID, conn := createTestTransactionReceiptsWebsocket(t, handler, tc.senderAddress, tc.statuses)
			for _, step := range tc.steps {
				t.Run(step.description, func(t *testing.T) {
					step.notify()
					for _, expectedReceipts := range step.expect {
						assertNextReceipts(t, conn, subID, expectedReceipts)
					}
				})
			}
		})
	}

	t.Run("Returns error if to many address in filter", func(t *testing.T) { //nolint:dupl // not duplicate, similar test for different method
		addresses := make([]felt.Felt, rpccore.MaxEventFilterKeys+1)

		serverConn, _ := net.Pipe()
		t.Cleanup(func() {
			require.NoError(t, serverConn.Close())
		})

		subCtx := context.WithValue(t.Context(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

		id, rpcErr := handler.SubscribeNewTransactionReceipts(subCtx, addresses, nil)
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

func assertNextEvents(t *testing.T, conn net.Conn, id SubscriptionID, emittedEvents []*SubscriptionEmittedEvent) {
	t.Helper()

	for _, emitted := range emittedEvents {
		assertNextMessage(t, conn, id, "starknet_subscriptionEvents", emitted)
	}
}

func assertNextReceipts(t *testing.T, conn net.Conn, id SubscriptionID, receipts []*TransactionReceipt) {
	t.Helper()

	for _, receipt := range receipts {
		assertNextMessage(t, conn, id, "starknet_subscriptionNewTransactionReceipts", receipt)
	}
}

func assertNextTransactions(t *testing.T, conn net.Conn, id SubscriptionID, transactions []*SubscriptionNewTransaction) {
	t.Helper()

	for _, txn := range transactions {
		assertNextMessage(t, conn, id, "starknet_subscriptionNewTransactions", txn)
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

func createTestPreConfirmed(t *testing.T, b *core.Block, preConfirmedCount int) core.PreConfirmed {
	t.Helper()

	actualTxCount := len(b.Transactions)
	var preConfirmed core.PreConfirmed
	if candidateCount := actualTxCount - preConfirmedCount; candidateCount > 0 {
		preConfirmed.CandidateTxs = make([]core.Transaction, candidateCount)
		candidateIndex := actualTxCount - candidateCount
		for i := range candidateCount {
			preConfirmed.CandidateTxs[i] = b.Transactions[candidateIndex]
			candidateIndex += 1
		}
	}

	preConfirmedBlock := core.Block{
		Header: &core.Header{
			Hash:             nil,
			ParentHash:       nil,
			Number:           b.Number,
			GlobalStateRoot:  b.GlobalStateRoot,
			SequencerAddress: b.SequencerAddress,
			TransactionCount: uint64(preConfirmedCount),
		},
	}

	preConfirmedBlock.Transactions = b.Transactions[:preConfirmedCount]
	preConfirmedBlock.Receipts = b.Receipts[:preConfirmedCount]
	preConfirmedBlock.EventsBloom = core.EventsBloom(preConfirmedBlock.Receipts)
	preConfirmed.Block = &preConfirmedBlock
	return preConfirmed
}

func createTestEvents(
	t *testing.T,
	b *core.Block,
	fromAddress *felt.Felt,
	keys [][]felt.Felt,
	finalityStatus TxnFinalityStatusWithoutL1,
) ([]*blockchain.FilteredEvent, []*SubscriptionEmittedEvent) {
	t.Helper()
	var blockNumber *uint64
	// if header.Hash == nil and parentHash != nil it's a pending block
	// if header.Hash == nil and parentHash == nil it's a pre_confirmed block
	if b.Hash != nil || b.ParentHash == nil {
		blockNumber = &b.Number
	}
	eventMatcher := blockchain.NewEventMatcher(fromAddress, keys)
	var filtered []*blockchain.FilteredEvent
	var responses []*SubscriptionEmittedEvent
	for _, receipt := range b.Receipts {
		for i, event := range receipt.Events {
			if fromAddress != nil && !event.From.Equal(fromAddress) {
				continue
			}

			if !eventMatcher.MatchesEventKeys(event.Keys) {
				continue
			}

			filtered = append(filtered, &blockchain.FilteredEvent{
				Event:           event,
				BlockNumber:     blockNumber,
				BlockHash:       b.Hash,
				TransactionHash: receipt.TransactionHash,
				EventIndex:      i,
			})
			responses = append(responses, &SubscriptionEmittedEvent{
				EmittedEvent: rpcv6.EmittedEvent{
					Event: &rpcv6.Event{
						From: event.From,
						Keys: event.Keys,
						Data: event.Data,
					},
					BlockNumber:     blockNumber,
					BlockHash:       b.Hash,
					TransactionHash: receipt.TransactionHash,
				},
				FinalityStatus: finalityStatus,
			})
		}
	}
	return filtered, responses
}

func createTestEventsWebsocket(
	t *testing.T,
	h *Handler,
	fromAddr *felt.Felt,
	keys [][]felt.Felt,
	blockID *SubscriptionBlockID,
	finalityStatus *TxnFinalityStatusWithoutL1,
) (SubscriptionID, net.Conn) {
	t.Helper()

	return createTestWebsocket(t, func(ctx context.Context) (SubscriptionID, *jsonrpc.Error) {
		return h.SubscribeEvents(ctx, fromAddr, keys, blockID, finalityStatus)
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

func createTestNewTransactionsWebsocket(
	t *testing.T, h *Handler, finalityStatus []TxnStatusWithoutL1, senderAddress []felt.Felt,
) (SubscriptionID, net.Conn) {
	t.Helper()

	return createTestWebsocket(t, func(ctx context.Context) (SubscriptionID, *jsonrpc.Error) {
		return h.SubscribeNewTransactions(ctx, finalityStatus, senderAddress)
	})
}

func createTestTransactionReceiptsWebsocket(
	t *testing.T, h *Handler, senderAddress []felt.Felt, finalityStatus []TxnFinalityStatusWithoutL1,
) (SubscriptionID, net.Conn) {
	t.Helper()

	return createTestWebsocket(t, func(ctx context.Context) (SubscriptionID, *jsonrpc.Error) {
		return h.SubscribeNewTransactionReceipts(ctx, senderAddress, finalityStatus)
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
