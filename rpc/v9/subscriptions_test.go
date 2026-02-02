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
func (fs *fakeSyncer) PendingBlock() *core.Block                             { return nil }
func (fs *fakeSyncer) PendingState() (core.StateReader, func() error, error) { return nil, nil, nil }

func (fs *fakeSyncer) PendingStateBeforeIndex(index int) (core.StateReader, func() error, error) {
	return nil, nil, nil
}

// setupMockWithSingleEvent sets up mocks for a single event list
func setupMockEventFilterer(
	mockChain *mocks.MockReader,
	mockEventFilterer *mocks.MockEventFilterer,
	header *core.Header,
	l1HeadNumber uint64,
	filteredEvents []blockchain.FilteredEvent,
) {
	mockChain.EXPECT().HeadsHeader().Return(header, nil)
	mockChain.EXPECT().L1Head().Return(
		core.L1Head{BlockNumber: l1HeadNumber},
		nil,
	)
	mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
		Return(filteredEvents, blockchain.ContinuationToken{}, nil)
}

// setupMockWithPending sets up mocks for multiple event lists
func setupMockEventFiltererWithMultiple(
	mockChain *mocks.MockReader,
	mockEventFilterer *mocks.MockEventFilterer,
	header *core.Header,
	l1HeadNumber uint64,
	filteredEvents ...[]blockchain.FilteredEvent,
) {
	mockChain.EXPECT().HeadsHeader().Return(header, nil)
	mockChain.EXPECT().L1Head().Return(
		core.L1Head{BlockNumber: l1HeadNumber},
		nil,
	)

	// Combine multiple event lists
	result := make([]blockchain.FilteredEvent, 0)
	for _, events := range filteredEvents {
		result = append(result, events...)
	}

	mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(
		result,
		blockchain.ContinuationToken{},
		nil,
	)
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
		fromAddr := felt.NewFromBytes[felt.Address]([]byte("from_address"))

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

	b3, err := gw.BlockByNumber(t.Context(), 56379)
	require.NoError(t, err)

	b1Filtered, b1Emitted := createTestEvents(
		t,
		b1,
		nil,
		nil,
		TxnAcceptedOnL2,
		false,
	)

	_, b1EmittedAsAcceptedOnL1 := createTestEvents(
		t,
		b1,
		nil,
		nil,
		TxnAcceptedOnL1,
		false,
	)
	b2Filtered, b2Emitted := createTestEvents(
		t,
		b2,
		nil,
		nil,
		TxnAcceptedOnL2,
		false,
	)

	pending := createTestPending(t, b2, 6)
	pendingFiltered, pendingEmitted := createTestEvents(
		t,
		pending.Block,
		nil,
		nil,
		TxnAcceptedOnL2,
		false,
	)
	pending2 := createTestPending(t, b2, 10)

	_, pending2Emitted := createTestEvents(
		t,
		pending2.Block,
		nil,
		nil,
		TxnAcceptedOnL2,
		false,
	)
	b2PreConfirmedPartial := createTestPreConfirmed(t, b2, 3)
	b2PreConfirmedExtended := createTestPreConfirmed(t, b2, 6)

	b2PreConfirmedPartialFiltered, b2PreConfirmedPartialEmitted := createTestEvents(
		t,
		b2PreConfirmedPartial.Block,
		nil,
		nil,
		TxnPreConfirmed,
		false,
	)
	_, b2PreConfirmedExtendedEmitted := createTestEvents(
		t,
		b2PreConfirmedExtended.Block,
		nil,
		nil,
		TxnPreConfirmed,
		false,
	)

	// Create PreLatest block for testing
	preLatestTxCount := len(b2.Transactions)
	b2PreLatest := core.PreLatest(createTestPending(t, b2, preLatestTxCount))
	b2PrelatestFiltered, b2PreLatestEmitted := createTestEvents(
		t,
		b2PreLatest.Block,
		nil,
		nil,
		TxnAcceptedOnL2,
		true,
	)

	b3PreConfirmedPartial := createTestPreConfirmed(t, b3, len(b3.Transactions)-1)
	b3PreConfirmedFull := createTestPreConfirmed(t, b3, len(b3.Transactions))
	b3PreConfirmedPartialFiltered, b3PreConfirmedPartialEmitted := createTestEvents(
		t,
		b3PreConfirmedPartial.Block,
		nil,
		nil,
		TxnPreConfirmed,
		false,
	)
	_, b3PreConfirmedFullEmitted := createTestEvents(
		t,
		b3PreConfirmedFull.Block,
		nil,
		nil,
		TxnPreConfirmed,
		false,
	)
	targetAddr, err := felt.NewFromString[felt.Address](
		"0x246ff8c7b475ddfb4cb5035867cba76025f08b22938e5684c18c2ab9d9f36d3",
	)
	require.NoError(t, err)
	b1FilteredByAddr, b1EmittedByAddr := createTestEvents(
		t,
		b1,
		targetAddr,
		nil,
		TxnAcceptedOnL2,
		false,
	)

	targetKey, err := felt.NewFromString[felt.Felt](
		"0x1dcde06aabdbca2f80aa51392b345d7549d7757aa855f7e37f5d335ac8243b1",
	)
	require.NoError(t, err)
	keys := [][]felt.Felt{{*targetKey}}

	b1FilteredByAddrAndKey, b1EmittedByAddrAndKey := createTestEvents(
		t,
		b1,
		targetAddr,
		keys,
		TxnAcceptedOnL2,
		false,
	)

	b2PreConfirmedPartialFilteredByAddrAndKey,
		b2PreConfirmedPartialEmittedByAddrAndKey := createTestEvents(
		t,
		b2PreConfirmedPartial.Block,
		targetAddr,
		keys,
		TxnPreConfirmed,
		false,
	)

	_, b2PreConfirmedExtendedEmittedByAddrAndKey := createTestEvents(
		t,
		b2PreConfirmedExtended.Block,
		targetAddr,
		keys,
		TxnPreConfirmed,
		false,
	)

	_, b2EmittedByAddrAndKey := createTestEvents(
		t,
		b2,
		targetAddr,
		keys,
		TxnAcceptedOnL2,
		false,
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
		expect      [][]SubscriptionEmittedEvent
	}

	type testCase struct {
		description    string
		blockID        *SubscriptionBlockID
		finalityStatus *TxnFinalityStatusWithoutL1
		keys           [][]felt.Felt
		fromAddr       *felt.Address
		steps          []stepInfo
		setupMocks     func()
	}

	preStarknet0_14_0basicSubscription := testCase{
		description: "Events from new blocks - default status, Starknet version < 0.14.0",
		blockID:     nil,
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			setupMockEventFilterer(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1Filtered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest on start",
				expect:      [][]SubscriptionEmittedEvent{b1Emitted},
			},
			{
				description: "on pending block",
				notify: func() {
					handler.pendingData.Send(&pending)
				},
				expect: [][]SubscriptionEmittedEvent{pendingEmitted},
			},
			{
				description: "on pending block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&pending2)
				},
				expect: [][]SubscriptionEmittedEvent{
					pending2Emitted[len(pendingEmitted):],
				},
			},
			{
				description: "on new head, without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{
					b2Emitted[len(pending2Emitted):],
				},
			},
		},
	}

	preStarknet0_14_0basicSubscriptionWithPending := testCase{
		description:    "Events from blocks + pending at start, Starknet < 0.14.0",
		blockID:        nil,
		keys:           nil,
		fromAddr:       nil,
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1Filtered,
				pendingFiltered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest and pending on start",
				expect: [][]SubscriptionEmittedEvent{
					b1Emitted, pendingEmitted,
				},
			},
			{
				description: "on pending block update with duplicates",
				notify: func() {
					handler.pendingData.Send(&pending2)
				},
				expect: [][]SubscriptionEmittedEvent{pending2Emitted},
			},
			{
				description: "on new head, without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{b2Emitted[len(pending2Emitted):]},
			},
		},
	}

	basicSubscription := testCase{
		description: "Events from new blocks - default status",
		blockID:     nil,
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			setupMockEventFilterer(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1Filtered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest on start",
				expect:      [][]SubscriptionEmittedEvent{b1Emitted},
			},
			{
				description: "on pre_confirmed block",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]SubscriptionEmittedEvent{},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]SubscriptionEmittedEvent{},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{b2Emitted},
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
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1Filtered,
				b2PreConfirmedPartialFiltered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed",
				expect: [][]SubscriptionEmittedEvent{
					b1Emitted, b2PreConfirmedPartialEmitted,
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]SubscriptionEmittedEvent{
					b2PreConfirmedExtendedEmitted[len(b2PreConfirmedPartialEmitted):],
				},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{b2Emitted},
			},
		},
	}

	preLatestEvents := testCase{
		description: "Events from PreLatest block - default status",
		blockID:     nil,
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			setupMockEventFilterer(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1Filtered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest on start",
				expect:      [][]SubscriptionEmittedEvent{b1Emitted},
			},
			{
				description: "on PreLatest block",
				notify: func() {
					handler.preLatestFeed.Send(&b2PreLatest)
				},
				expect: [][]SubscriptionEmittedEvent{b2PreLatestEmitted},
			},
			{
				description: "on new head after PreLatest, without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{},
			},
		},
	}

	eventsFromHistoricalBlocks := testCase{
		description: "Events from historical blocks - default status, events from 2 block",
		blockID:     utils.HeapPtr(SubscriptionBlockID(BlockIDFromNumber(b1.Number))),
		keys:        nil,
		fromAddr:    nil,
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b2.Header,
				b1.Header.Number,
				b1Filtered,
				b2Filtered,
			)
			mockChain.EXPECT().BlockHeaderByNumber(b1.Number).Return(b1.Header, nil)
		},
		steps: []stepInfo{
			{
				description: "events from ACCEPTED_ON_L1 on start, with blockNumber query",
				expect:      [][]SubscriptionEmittedEvent{b1EmittedAsAcceptedOnL1},
			},
			{
				description: "events from ACCEPTED_ON_L2 on start",
				expect:      [][]SubscriptionEmittedEvent{b2Emitted},
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
			mockChain.EXPECT().L1Head().Return(
				core.L1Head{BlockNumber: uint64(max(0, int(b1.Header.Number)-1))},
				nil,
			)
			cToken := blockchain.ContinuationToken{}
			require.NoError(t, cToken.FromString(fmt.Sprintf("%d-0", b2.Number)))
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).Return(b1Filtered, cToken, nil)
			mockEventFilterer.EXPECT().Events(gomock.Any(), gomock.Any()).
				Return(b2Filtered, blockchain.ContinuationToken{}, nil)
		},
		steps: []stepInfo{
			{
				description: "events from last 2 blocks with continuation token",
				expect: [][]SubscriptionEmittedEvent{
					b1Emitted, b2Emitted,
				},
			},
		},
	}

	b2PreConfirmedPartialFilteredByAddr, b2PreConfirmedPartialEmittedByAddr := createTestEvents(
		t,
		b2PreConfirmedPartial.Block,
		targetAddr,
		nil,
		TxnPreConfirmed,
		false,
	)

	_, b2PreConfirmedExtendedEmittedByAddr := createTestEvents(
		t,
		b2PreConfirmedExtended.Block,
		targetAddr,
		nil,
		TxnPreConfirmed,
		false,
	)

	_, b2EmittedByAddr := createTestEvents(
		t,
		b2,
		targetAddr,
		nil,
		TxnAcceptedOnL2,
		false,
	)

	eventsWithFromAddressAndPreConfirmed := testCase{ //nolint:dupl // params and return values are different
		description:    "Events with from_address filter, finality PRE_CONFIRMED",
		blockID:        nil,
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		fromAddr:       targetAddr,
		keys:           nil,
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1FilteredByAddr,
				b2PreConfirmedPartialFilteredByAddr,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed",
				expect: [][]SubscriptionEmittedEvent{
					b1EmittedByAddr, b2PreConfirmedPartialEmittedByAddr,
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]SubscriptionEmittedEvent{
					b2PreConfirmedExtendedEmittedByAddr[len(b2PreConfirmedPartialEmittedByAddr):],
				},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{b2EmittedByAddr},
			},
		},
	}

	eventsWithAllFilterAndPreConfirmed := testCase{ //nolint:dupl // params and return values are different
		description:    "Events with from_address and key, finality PRE_CONFIRMED",
		blockID:        nil,
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		fromAddr:       targetAddr,
		keys:           keys,
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				b1.Header.Number-1,
				b1FilteredByAddrAndKey,
				b2PreConfirmedPartialFilteredByAddrAndKey,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed on start",
				expect: [][]SubscriptionEmittedEvent{
					b1EmittedByAddrAndKey, b2PreConfirmedPartialEmittedByAddrAndKey,
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]SubscriptionEmittedEvent{
					b2PreConfirmedExtendedEmittedByAddrAndKey[len(b2PreConfirmedPartialEmittedByAddrAndKey):],
				},
			},
			{
				description: "on new head",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{b2EmittedByAddrAndKey},
			},
		},
	}

	deduplication := testCase{
		description:    "deduplicate events",
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				uint64(max(0, int(b1.Header.Number)-1)),
				b1Filtered,
				b2PreConfirmedPartialFiltered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest and preconfirmed",
				expect: [][]SubscriptionEmittedEvent{
					b1Emitted, b2PreConfirmedPartialEmitted,
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]SubscriptionEmittedEvent{
					b2PreConfirmedExtendedEmitted[len(b2PreConfirmedPartialEmitted):],
				},
			},
			{
				description: "pre_confirmed becomes pre_latest",
				notify: func() {
					handler.preLatestFeed.Send(&b2PreLatest)
				},
				expect: [][]SubscriptionEmittedEvent{b2PreLatestEmitted},
			},
			{
				description: "new pre_confirmed block",
				notify: func() {
					handler.pendingData.Send(&b3PreConfirmedPartial)
				},
				expect: [][]SubscriptionEmittedEvent{b3PreConfirmedPartialEmitted},
			},
			{
				description: "prelatest becomes head - without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{},
			},
			{
				description: "pre_confirmed update - without duplicates",
				notify: func() {
					handler.pendingData.Send(&b3PreConfirmedFull)
				},
				expect: [][]SubscriptionEmittedEvent{
					b3PreConfirmedFullEmitted[len(b3PreConfirmedPartialEmitted):],
				},
			},
		},
	}

	deduplicationWithPreLatestOnStart := testCase{
		description:    "deduplicate events with prelatest on start",
		finalityStatus: utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnPreConfirmed)),
		setupMocks: func() {
			setupMockEventFiltererWithMultiple(
				mockChain,
				mockEventFilterer,
				b1.Header,
				uint64(max(0, int(b1.Header.Number)-1)),
				b1Filtered,
				b2PrelatestFiltered,
				b3PreConfirmedPartialFiltered,
			)
		},
		steps: []stepInfo{
			{
				description: "events from latest, pre_latest and pre_confirmed",
				expect: [][]SubscriptionEmittedEvent{
					b1Emitted, b2PreLatestEmitted, b3PreConfirmedPartialEmitted,
				},
			},
			{
				description: "prelatest becomes head - without duplicates",
				notify: func() {
					handler.newHeads.Send(b2)
				},
				expect: [][]SubscriptionEmittedEvent{},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					handler.pendingData.Send(&b3PreConfirmedFull)
				},
				expect: [][]SubscriptionEmittedEvent{
					b3PreConfirmedFullEmitted[len(b3PreConfirmedPartialEmitted):],
				},
			},
		},
	}

	testCases := []testCase{
		preStarknet0_14_0basicSubscription,
		preStarknet0_14_0basicSubscriptionWithPending,
		basicSubscription,
		basicSubscriptionWithPreConfirmed,
		preLatestEvents,
		eventsFromHistoricalBlocks,
		eventsWithContinuationToken,
		eventsWithFromAddressAndPreConfirmed,
		eventsWithAllFilterAndPreConfirmed,
		deduplication,
		deduplicationWithPreLatestOnStart,
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
				if step.setupMocks != nil {
					step.setupMocks()
				}

				if step.notify != nil {
					step.notify()
				}

				if len(step.expect) == 0 {
					// If no events are expected, wait for a short period to ensure no events are sent
					assertNoEvents(t, conn, 50*time.Millisecond)
				} else {
					for _, expectedEvents := range step.expect {
						assertNextEvents(t, conn, subID, expectedEvents)
					}
				}
			}
		})
	}
}

func TestSubscribeTxnStatus(t *testing.T) {
	log := utils.NewNopZapLogger()
	txHash := felt.NewFromUint64[felt.Felt](1)
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

		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txHash),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound).AnyTimes()
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).AnyTimes()
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
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).AnyTimes()
		t.Run("reverted", func(t *testing.T) {
			txHash, err := felt.NewFromString[felt.Felt]("0x1011")
			require.NoError(t, err)

			mockChain.EXPECT().BlockNumberAndIndexByTxHash(
				(*felt.TransactionHash)(txHash),
			).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
			id, conn := createTestTxStatusWebsocket(t, handler, txHash)
			assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnFailure, "some error")
		})
		t.Run("accepted on L1", func(t *testing.T) {
			txHash, err := felt.NewFromString[felt.Felt]("0x1010")
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
			&txToBroadcast,
		)
		require.Nil(t, addErr)
		txHash := addRes.TransactionHash
		mockChain.EXPECT().BlockNumberAndIndexByTxHash((*felt.TransactionHash)(txHash)).Return(
			uint64(0), uint64(0), db.ErrKeyNotFound,
		)
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).Times(2)
		mockChain.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).Times(2)

		id, conn := createTestTxStatusWebsocket(t, handler, txHash)

		assertNextTxnStatus(t, conn, id, txHash, TxnStatusReceived, UnknownExecution, "")
		// Candidate Status
		mockChain.EXPECT().BlockNumberAndIndexByTxHash((*felt.TransactionHash)(txHash)).Return(
			uint64(0), uint64(0), db.ErrKeyNotFound,
		)
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					TransactionCount: 1,
				},
			},
			CandidateTxs: []core.Transaction{block.Transactions[0]},
		}

		mockSyncer.EXPECT().PendingData().Return(
			preConfirmed,
			nil,
		).Times(2)
		handler.pendingData.Send(preConfirmed)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusCandidate, UnknownExecution, "")
		require.Equal(t, block.Transactions[0].Hash(), txHash)

		// PreConfirmed Status
		rpcTx := AdaptTransaction(block.Transactions[0])
		rpcTx.Hash = txHash

		preConfirmed = &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					TransactionCount: 1,
				},
				Transactions: []core.Transaction{
					block.Transactions[0],
				},
				Receipts: []*core.TransactionReceipt{block.Receipts[0]},
			},
			CandidateTxs: []core.Transaction{},
		}
		mockSyncer.EXPECT().PendingData().Return(
			preConfirmed,
			nil,
		).Times(1)
		handler.pendingData.Send(preConfirmed)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusPreConfirmed, TxnSuccess, "")

		preConfirmed = &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number + 1,
					TransactionCount: 0,
				},
				Transactions: []core.Transaction{},
				Receipts:     []*core.TransactionReceipt{},
			},
			CandidateTxs: []core.Transaction{},
		}
		mockSyncer.EXPECT().PendingData().Return(
			preConfirmed,
			nil,
		).Times(1)
		// Accepted on l2 Status
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

		handler.newHeads.Send(block)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL2, TxnSuccess, "")

		mockSyncer.EXPECT().PendingData().Return(
			preConfirmed,
			nil,
		).Times(1)
		l1Head := core.L1Head{BlockNumber: block.Number}
		mockChain.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txHash),
		).Return(block.Number, uint64(0), nil)
		mockChain.EXPECT().TransactionByBlockNumberAndIndex(
			block.Number, uint64(0)).Return(block.Transactions[0], nil)
		mockChain.EXPECT().ReceiptByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(*block.Receipts[0], block.Hash, nil)
		mockChain.EXPECT().L1Head().Return(l1Head, nil)
		handler.l1Heads.Send(&l1Head)
		assertNextTxnStatus(t, conn, id, txHash, TxnStatusAcceptedOnL1, TxnSuccess, "")
	})

	t.Run("Transaction status from pre-latest block", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
		adapterFeeder := adaptfeeder.New(client)
		mockSyncer := mocks.NewMockSyncReader(mockCtrl)
		handler := New(nil, mockSyncer, nil, log)
		block, err := adapterFeeder.BlockByNumber(t.Context(), 38748)
		require.NoError(t, err)

		targetTxn := block.Transactions[0]
		targetReceipt := block.Receipts[0]

		preConfirmedData1 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					ParentHash:       block.ParentHash,
					TransactionCount: 1,
				},
				Transactions: []core.Transaction{targetTxn},
				Receipts:     []*core.TransactionReceipt{targetReceipt},
			},
		}
		// PreLatest Status - should check transaction status
		preLatest := &core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					ParentHash:       block.ParentHash,
					TransactionCount: 1,
				},
				Transactions: []core.Transaction{targetTxn},
				Receipts:     []*core.TransactionReceipt{targetReceipt},
			},
		}

		preConfirmedData2 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: preLatest.Block.Number + 1,
				},
			},
			PreLatest: preLatest,
		}

		// We need to return some status at start, otherwise it will re-try for a while
		// and mocking `PendingData` will be observed before prelatest feed trigger.
		mockSyncer.EXPECT().PendingData().Return(preConfirmedData1, nil)
		id, conn := createTestTxStatusWebsocket(t, handler, targetTxn.Hash())
		assertNextTxnStatus(t, conn, id, targetTxn.Hash(), TxnStatusPreConfirmed, TxnSuccess, "")

		mockSyncer.EXPECT().PendingData().Return(preConfirmedData2, nil)
		handler.preLatestFeed.Send(preLatest)
		assertNextTxnStatus(t, conn, id, targetTxn.Hash(), TxnStatusAcceptedOnL2, TxnSuccess, "")
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
	mockChain.EXPECT().L1Head().Return(core.L1Head{BlockNumber: 0}, nil)
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
		{
			name:            "reorg event in starknet_subscribeNewTransactionReceipts",
			subscribeMethod: "starknet_subscribeNewTransactionReceipts",
			ignoreFirst:     false,
		},
		{
			name:            "reorg event in starknet_subscribeNewTransactions",
			subscribeMethod: "starknet_subscribeNewTransactions",
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

	mockChain.EXPECT().HeadsHeader().Return(&core.Header{}, nil).Times(2)
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
	pending := createTestPending(t, newHead2, pendingBlockTxCount)

	partialPreConfirmedCount := 3
	extendedPreConfirmedCount := 6

	// Pre-confirmed blocks for block 56377
	b1PreConfirmedPartial := createTestPreConfirmed(t, newHead1, partialPreConfirmedCount)
	b1PreConfirmedExtended := createTestPreConfirmed(t, newHead1, extendedPreConfirmedCount)
	b1PreConfirmedFull := createTestPreConfirmed(t, newHead1, len(newHead1.Transactions))

	// Pre-latest block for block 56377
	b1PreLatest := core.PreLatest(createTestPending(t, newHead1, len(newHead1.Transactions)))

	// Pre-confirmed blocks for block 56378
	b2PreConfirmedPartial := createTestPreConfirmed(t, newHead2, partialPreConfirmedCount)
	b2PreConfirmedExtended := createTestPreConfirmed(t, newHead2, extendedPreConfirmedCount)
	b2PreConfirmedFull := createTestPreConfirmed(t, newHead2, len(newHead2.Transactions))

	// Pre-latest block for block 56378
	b2PreLatest := core.PreLatest(createTestPending(t, newHead2, len(newHead2.Transactions)))

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
					toTransactionsWithFinalityStatus(
						pending.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
					),
				},
			},
			{
				description: "pending becomes new head without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						newHead2.Transactions[pendingBlockTxCount:],
						TxnStatusWithoutL1(TxnAcceptedOnL2),
					),
				},
			},
		},
	}

	// Only ACCEPTED_ON_L2 as default
	defaultFinality := testCase{
		description:   "Basic subcription - default finality status. Starknet version >= 0.14.0",
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
				description: "on new pre_confirmed no stream",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "pre_confirmed becomes pre_latest",
				notify: func() {
					syncer.preLatest.Send(&b2PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreLatest.Block.Transactions,
						TxnStatusWithoutL1(TxnAcceptedOnL2),
					),
				},
			},
			{
				description: "pre_latest become new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
		},
	}

	onlyPreConfirmed := testCase{
		description:   "Basic subcription - only PRE_CONFIRMED",
		statuses:      []TxnStatusWithoutL1{TxnStatusWithoutL1(TxnStatusPreConfirmed)},
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on new pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreConfirmedPartial.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "move all candidates moved to PRE_CONFIRMED, without dup.",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedFull)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreConfirmedFull.Block.Transactions[partialPreConfirmedCount:],
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "pre_confirmed becomes pre_latest, no stream",
				notify: func() {
					syncer.preLatest.Send(&b1PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "on new pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "pre_confirmed becomes head, no stream",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
		},
	}

	onlyCandidate := testCase{
		description:   "Basic subcription - only CANDIDATE",
		statuses:      []TxnStatusWithoutL1{TxnStatusWithoutL1(TxnStatusCandidate)},
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on new head do not stream",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "on new pre_confirmed, only stream CANDIDATES",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.CandidateTxs,
						TxnStatusWithoutL1(TxnStatusCandidate),
					),
				},
			},
			{
				description: "on pre_confirmed update subset of candidates moved to PRE_CONFIRMED, without dup. do not stream",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "pre_confirmed become new head do not stream",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{},
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
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.CandidateTxs,
						TxnStatusWithoutL1(TxnStatusCandidate),
					),
				},
			},
			{
				description: "on pre_confirmed update subset of candidates moved to PRE_CONFIRMED, without dup.",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedExtended.Block.Transactions[partialPreConfirmedCount:],
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "on pre_confirmed update all candidates moved to PRE_CONFIRMED, without dup.",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedFull)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedFull.Block.Transactions[extendedPreConfirmedCount:],
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "pre_confirmed becomes pre_latest",
				notify: func() {
					syncer.preLatest.Send(&b2PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreLatest.Block.Transactions,
						TxnStatusWithoutL1(TxnAcceptedOnL2),
					),
				},
			},
			{
				description: "pre_latest becomes new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{},
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
					syncer.pendingData.Send(&b2PreConfirmedFull)
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

	preLatestTransactions := testCase{
		description:   "Transactions from PreLatest blocks - default status",
		statuses:      nil,
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on pre-latest block",
				notify: func() {
					syncer.preLatest.Send(&b1PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreLatest.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
					),
				},
			},
			{
				description: "pre-latest becomes new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "on new pre-latest block",
				notify: func() {
					syncer.preLatest.Send(&b2PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreLatest.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
					),
				},
			},
			{
				description: "pre-latest becomes new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
		},
	}

	deduplication := testCase{
		description: "deduplicate transactions",
		statuses: []TxnStatusWithoutL1{
			TxnStatusWithoutL1(TxnStatusReceived),
			TxnStatusWithoutL1(TxnStatusCandidate),
			TxnStatusWithoutL1(TxnStatusPreConfirmed),
			TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
		},
		senderAddress: nil,
		steps: []stepInfo{
			{
				description: "on pre_confirmed block",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreConfirmedPartial.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
					toTransactionsWithFinalityStatus(
						b1PreConfirmedPartial.CandidateTxs,
						TxnStatusWithoutL1(TxnStatusCandidate),
					),
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedExtended)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreConfirmedExtended.Block.Transactions[partialPreConfirmedCount:],
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
			{
				description: "pre_confirmed becomes pre_latest",
				notify: func() {
					syncer.preLatest.Send(&b1PreLatest)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b1PreLatest.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
					),
				},
			},
			{
				description: "new pre_confirmed block",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.Block.Transactions,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
					toTransactionsWithFinalityStatus(
						b2PreConfirmedPartial.CandidateTxs,
						TxnStatusWithoutL1(TxnStatusCandidate),
					),
				},
			},
			{
				description: "prelatest becomes head - without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*SubscriptionNewTransaction{},
			},
			{
				description: "pre_confirmed update - without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]*SubscriptionNewTransaction{
					toTransactionsWithFinalityStatus(
						b2PreConfirmedExtended.Block.Transactions[partialPreConfirmedCount:],
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					),
				},
			},
		},
	}

	testCases := []testCase{
		preStarknet0_14_0DefaultFinality,
		defaultFinality, // onlyAcceptedOnL2
		onlyPreConfirmed,
		onlyCandidate,
		allStatuses,
		allStatusesWithFilter,
		preLatestTransactions,
		deduplication,
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			subID, conn := createTestNewTransactionsWebsocket(t, handler, tc.statuses, tc.senderAddress)
			for _, step := range tc.steps {
				if step.notify != nil {
					step.notify()
				}

				if len(step.expect) == 0 {
					// If no transactions are expected, wait for a short period to ensure no transactions are sent
					assertNoEvents(t, conn, 50*time.Millisecond)
				} else {
					for _, expectedTransactions := range step.expect {
						assertNextTransactions(t, conn, subID, expectedTransactions)
					}
				}
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

	toAdaptedReceiptsWithFilter := func(
		b *core.Block,
		senderAddress []felt.Felt,
		finalityStatus TxnFinalityStatus,
		isPreLatest bool,
	) []*TransactionReceipt {
		receipts := make([]*TransactionReceipt, 0)
		for i, receipt := range b.Receipts {
			txn := b.Transactions[i]
			if filterTxBySender(txn, senderAddress) {
				receipts = append(
					receipts,
					AdaptReceiptWithBlockInfo(
						receipt,
						txn,
						finalityStatus,
						b.Hash,
						b.Number,
						isPreLatest,
					),
				)
			}
		}
		return receipts
	}

	partialPendingCount := 3
	extendedPendingCount := 6
	pendingPartial := createTestPending(t, newHead2, partialPendingCount)
	pendingExtended := createTestPending(t, newHead2, extendedPendingCount)

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
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2, false),
				},
			},
			{
				description: "on pending",
				notify: func() {
					syncer.pendingData.Send(&pendingPartial)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						pendingPartial.Block,
						nil,
						TxnAcceptedOnL2,
						false,
					),
				},
			},
			{
				description: "on pending block update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&pendingExtended)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						pendingExtended.Block,
						nil,
						TxnAcceptedOnL2,
						false,
					)[partialPendingCount:],
				},
			},
			{
				description: "on next head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						newHead2,
						nil,
						TxnAcceptedOnL2,
						false,
					)[extendedPendingCount:],
				},
			},
		},
	}

	partialPreConfirmedCount := 3
	extendedPreConfirmedCount := 6
	b1PreConfirmedPartial := createTestPreConfirmed(t, newHead1, partialPreConfirmedCount)
	b1PreConfirmedExtended := createTestPreConfirmed(t, newHead1, extendedPreConfirmedCount)

	b1PreLatest := core.PreLatest(createTestPending(t, newHead1, len(newHead1.Transactions)))
	b2PreLatest := core.PreLatest(createTestPending(t, newHead2, len(newHead2.Transactions)))

	b2PreConfirmedPartial := createTestPreConfirmed(t, newHead2, partialPreConfirmedCount)
	b2PreConfirmedExtended := createTestPreConfirmed(t, newHead2, extendedPreConfirmedCount)
	b2PreConfirmedFull := createTestPreConfirmed(t, newHead2, len(newHead2.Transactions))

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
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2, false),
				},
			},
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, nil, TxnAcceptedOnL2, false),
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
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedPartial.Block,
						nil,
						TxnPreConfirmed,
						false,
					),
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedExtended.Block,
						nil,
						TxnPreConfirmed,
						false,
					)[partialPreConfirmedCount:],
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
		statuses: []TxnFinalityStatusWithoutL1{
			TxnFinalityStatusWithoutL1(TxnPreConfirmed),
			TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
		},
		steps: []stepInfo{
			{
				description: "on new head",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead1, nil, TxnAcceptedOnL2, false),
				},
			},
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedPartial.Block,
						nil,
						TxnPreConfirmed,
						false,
					),
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedExtended)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedExtended.Block,
						nil,
						TxnPreConfirmed,
						false,
					)[partialPreConfirmedCount:],
				},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(newHead2, nil, TxnAcceptedOnL2, false),
				},
			},
		},
	}

	senderAddress := AdaptTransaction(newHead2.Transactions[0]).SenderAddress
	senderFilter := []felt.Felt{*senderAddress}
	b2PreConfirmedPartialFilteredReceipts := toAdaptedReceiptsWithFilter(
		b2PreConfirmedPartial.Block,
		senderFilter,
		TxnPreConfirmed,
		false,
	)

	allStatusesWithFilter := testCase{
		description:   "subscription with filter and all statuses",
		senderAddress: senderFilter,
		statuses: []TxnFinalityStatusWithoutL1{
			TxnFinalityStatusWithoutL1(TxnPreConfirmed),
			TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
		},
		steps: []stepInfo{
			{
				description: "on pre_confirmed",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{
					b2PreConfirmedPartialFilteredReceipts,
				},
			},
			{
				description: "on pre_confirmed update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedFull)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedFull.Block,
						senderFilter,
						TxnPreConfirmed,
						false,
					)[len(b2PreConfirmedPartialFilteredReceipts):],
				},
			},
			{
				description: "on next head",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						newHead2,
						senderFilter,
						TxnAcceptedOnL2,
						false,
					),
				},
			},
		},
	}

	// Test case for PreLatest receipts
	preLatestReceipts := testCase{
		description: "Receipts from pre-latest block - default status",
		statuses:    nil,
		steps: []stepInfo{
			{
				description: "on pre-latest block",
				notify: func() {
					syncer.preLatest.Send(&b1PreLatest)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b1PreLatest.Block,
						nil,
						TxnAcceptedOnL2,
						true,
					),
				},
			},
			{
				description: "on new pre-confirmed block - no stream",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "pre-latest becomes new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "pre-confirmed becomes pre-latest",
				notify: func() {
					syncer.preLatest.Send(&b2PreLatest)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreLatest.Block,
						nil,
						TxnAcceptedOnL2,
						true,
					),
				},
			},
			{
				description: "pre-latest becomes new head, without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead2)
				},
				expect: [][]*TransactionReceipt{},
			},
		},
	}

	receiptDeduplication := testCase{
		description: "deduplicate receipts",
		statuses: []TxnFinalityStatusWithoutL1{
			TxnFinalityStatusWithoutL1(TxnPreConfirmed),
			TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
		},
		steps: []stepInfo{
			{
				description: "on pre_confirmed block",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b1PreConfirmedPartial.Block,
						nil,
						TxnPreConfirmed,
						false,
					),
				},
			},
			{
				description: "on pre_confirmed block update, without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b1PreConfirmedExtended)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b1PreConfirmedExtended.Block,
						nil,
						TxnPreConfirmed,
						false,
					)[partialPreConfirmedCount:],
				},
			},
			{
				description: "pre_confirmed becomes pre_latest",
				notify: func() {
					syncer.preLatest.Send(&b1PreLatest)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b1PreLatest.Block,
						nil,
						TxnAcceptedOnL2,
						true,
					),
				},
			},
			{
				description: "new pre_confirmed block",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedPartial)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedPartial.Block,
						nil,
						TxnPreConfirmed,
						false,
					),
				},
			},
			{
				description: "prelatest becomes head - without duplicates",
				notify: func() {
					syncer.newHeads.Send(newHead1)
				},
				expect: [][]*TransactionReceipt{},
			},
			{
				description: "pre_confirmed update - without duplicates",
				notify: func() {
					syncer.pendingData.Send(&b2PreConfirmedFull)
				},
				expect: [][]*TransactionReceipt{
					toAdaptedReceiptsWithFilter(
						b2PreConfirmedFull.Block,
						nil,
						TxnPreConfirmed,
						false,
					)[partialPreConfirmedCount:],
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
		preLatestReceipts,
		receiptDeduplication,
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			subID, conn := createTestTransactionReceiptsWebsocket(t, handler, tc.senderAddress, tc.statuses)
			for _, step := range tc.steps {
				if step.notify != nil {
					step.notify()
				}

				if len(step.expect) == 0 {
					// If no receipts are expected, wait for a short period to ensure no receipts are sent
					assertNoEvents(t, conn, 50*time.Millisecond)
				} else {
					for _, expectedReceipts := range step.expect {
						assertNextReceipts(t, conn, subID, expectedReceipts)
					}
				}
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
	require.Equal(t, string(resp), string(got))
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
	emittedEvents []SubscriptionEmittedEvent,
) {
	t.Helper()

	for _, emitted := range emittedEvents {
		assertNextMessage(t, conn, id, "starknet_subscriptionEvents", emitted)
	}
}

func assertNoEvents(t *testing.T, conn net.Conn, waitDuration time.Duration) {
	t.Helper()

	// Set a read deadline to wait for the specified duration
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(waitDuration)))

	// Try to read from the connection - this should timeout if no events are sent
	buffer := make([]byte, 1024)
	_, err := conn.Read(buffer)

	// Clear the read deadline to avoid affecting subsequent tests
	require.NoError(t, conn.SetReadDeadline(time.Time{}))

	// We expect a timeout error if no events are sent
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// This is expected - no events were received within the timeout period
			return
		}
		// If it's not a timeout error, fail the test
		require.NoError(t, err)
	}

	// If we get here, it means we received data when we shouldn't have
	t.Errorf("Expected no events but received data: %s", string(buffer))
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
		assertNextMessage(t, conn, id, "starknet_subscriptionNewTransaction", txn)
	}
}

func createTestPending(t *testing.T, b *core.Block, txCount int) core.Pending {
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
	return core.Pending{
		Block: &pending,
	}
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
	fromAddress *felt.Address,
	keys [][]felt.Felt,
	finalityStatus TxnFinalityStatus,
	isPreLatest bool,
) ([]blockchain.FilteredEvent, []SubscriptionEmittedEvent) {
	t.Helper()
	var blockNumber *uint64
	// if header.Hash == nil and parentHash != nil it's a pending block
	// if header.Hash == nil and parentHash == nil it's a pre_confirmed block
	if b.Hash != nil || b.ParentHash == nil || isPreLatest {
		blockNumber = &b.Number
	}
	var addresses []felt.Address
	if fromAddress != nil {
		addresses = []felt.Address{*fromAddress}
	}
	eventMatcher := blockchain.NewEventMatcher(addresses, keys)
	var filtered []blockchain.FilteredEvent
	var responses []SubscriptionEmittedEvent
	for _, receipt := range b.Receipts {
		for i, event := range receipt.Events {
			// todo: remove the cast to felt.Felt
			if fromAddress != nil && !event.From.Equal((*felt.Felt)(fromAddress)) {
				continue
			}

			if !eventMatcher.MatchesEventKeys(event.Keys) {
				continue
			}

			filtered = append(filtered, blockchain.FilteredEvent{
				Event:           event,
				BlockNumber:     blockNumber,
				BlockHash:       b.Hash,
				BlockParentHash: b.ParentHash,
				TransactionHash: receipt.TransactionHash,
				EventIndex:      uint(i),
			})
			responses = append(responses, SubscriptionEmittedEvent{
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
	fromAddr *felt.Address,
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
