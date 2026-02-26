package rpcv10_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// createEventPendingFromBlock creates a pending block from the given block and
// returns the pending data and emitted events
func createEventPendingFromBlock(block *core.Block) (core.Pending, []rpc.EmittedEvent) {
	newHeader := &core.Header{
		ParentHash: block.Header.ParentHash,
		Number:     block.Header.Number,
	}

	pendingBlock := &core.Block{
		Header:       newHeader,
		Transactions: block.Transactions,
		Receipts:     block.Receipts,
	}

	pending := core.NewPending(pendingBlock, nil, nil)

	// Extract events from the block and convert to emitted events
	var events []rpc.EmittedEvent
	for txIndex, receipt := range pendingBlock.Receipts {
		for eventIndex, event := range receipt.Events {
			events = append(events, rpc.EmittedEvent{
				Event: &rpcv6.Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
				BlockNumber:      nil, // Pending events have no block number
				BlockHash:        nil, // Pending events have no block hash
				TransactionHash:  receipt.TransactionHash,
				TransactionIndex: uint(txIndex),
				EventIndex:       uint(eventIndex),
			})
		}
	}

	return pending, events
}

// createEventPreConfirmedFromBlock creates a pre_confirmed block from the given block and
// returns the pre_confirmed data and emitted events
func createEventPreConfirmedFromBlock(block *core.Block) (
	core.PreConfirmed, []rpc.EmittedEvent,
) {
	newHeader := &core.Header{
		Number:           block.Header.Number,
		Timestamp:        block.Header.Timestamp,
		SequencerAddress: block.Header.SequencerAddress,
	}

	preConfirmedBlock := &core.Block{
		Header:       newHeader,
		Transactions: block.Transactions,
		Receipts:     block.Receipts,
	}

	preConfirmed := core.NewPreConfirmed(preConfirmedBlock, nil, nil, nil)

	// Extract events from the block and convert to emitted events
	var events []rpc.EmittedEvent
	for txIndex, receipt := range preConfirmedBlock.Receipts {
		for eventIndex, event := range receipt.Events {
			events = append(events, rpc.EmittedEvent{
				Event: &rpcv6.Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
				BlockNumber:      &preConfirmedBlock.Number, // Pre-confirmed events have block number
				BlockHash:        nil,                       // Pre-confirmed events have no block hash
				TransactionHash:  receipt.TransactionHash,
				TransactionIndex: uint(txIndex),
				EventIndex:       uint(eventIndex),
			})
		}
	}

	return preConfirmed, events
}

// createEventPreLatestFromBlock creates a pre_latest block from the given block and
// returns the pre_latest data and emitted events
func createEventPreLatestFromBlock(block *core.Block) (core.PreLatest, []rpc.EmittedEvent) {
	newHeader := &core.Header{
		ParentHash: block.Header.ParentHash,
		Number:     block.Header.Number,
	}

	preLatestBlock := &core.Block{
		Header:       newHeader,
		Transactions: block.Transactions,
		Receipts:     block.Receipts,
	}

	preLatest := core.PreLatest{Block: preLatestBlock}

	// Extract events from the block and convert to emitted events
	var events []rpc.EmittedEvent
	for txIndex, receipt := range preLatestBlock.Receipts {
		for eventIndex, event := range receipt.Events {
			events = append(events, rpc.EmittedEvent{
				Event: &rpcv6.Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
				BlockNumber:      &preLatestBlock.Number, // Pre-latest events have block number
				BlockHash:        nil,                    // Pre-latest events have no block hash
				TransactionHash:  receipt.TransactionHash,
				TransactionIndex: uint(txIndex),
				EventIndex:       uint(eventIndex),
			})
		}
	}

	return preLatest, events
}

// extractCanonicalEvents extracts all events from canonical blocks in the chain
func extractCanonicalEvents(
	t *testing.T,
	chain *blockchain.Blockchain,
	fromBlock uint64,
	toBlock uint64,
) []rpc.EmittedEvent {
	t.Helper()
	var events []rpc.EmittedEvent

	for i := fromBlock; i <= toBlock; i++ {
		block, err := chain.BlockByNumber(i)
		require.NoError(t, err)
		for txIndex, receipt := range block.Receipts {
			for eventIndex, event := range receipt.Events {
				events = append(events, rpc.EmittedEvent{
					Event: &rpcv6.Event{
						From: event.From,
						Keys: event.Keys,
						Data: event.Data,
					},
					BlockNumber:      &block.Number,
					BlockHash:        block.Hash,
					TransactionHash:  receipt.TransactionHash,
					TransactionIndex: uint(txIndex),
					EventIndex:       uint(eventIndex),
				})
			}
		}
	}

	return events
}

// collectAllEvents collects all events from the given handler and returns them
// asserts that the number of events in each chunk is less than or equal to the chunk size
func collectAllEvents(t *testing.T, h *rpc.Handler, args *rpc.EventArgs) []rpc.EmittedEvent {
	t.Helper()
	all := []rpc.EmittedEvent{}
	cur := *args
	for {
		chunk, err := h.Events(&cur)
		require.Nil(t, err)
		require.LessOrEqual(t, len(chunk.Events), int(args.ResultPageRequest.ChunkSize))
		all = append(all, chunk.Events...)
		if chunk.ContinuationToken == "" {
			break
		}
		cur.ContinuationToken = chunk.ContinuationToken
	}
	return all
}

// fetchAndStoreBlock fetches a block from the feeder and stores it in the blockchain
func fetchAndStoreBlock(
	t *testing.T,
	chain *blockchain.Blockchain,
	gw *adaptfeeder.Feeder,
	blockNumber uint64,
) {
	t.Helper()
	b, err := gw.BlockByNumber(t.Context(), blockNumber)
	require.NoError(t, err)
	s, err := gw.StateUpdate(t.Context(), blockNumber)
	require.NoError(t, err)
	require.NoError(t, chain.Store(b, &core.BlockCommitments{}, s, nil))
}

// setupTestChain sets up a test chain with the given number of blocks
func setupTestChain(
	t *testing.T,
	network *utils.Network,
	numBlocks uint64,
) (*blockchain.Blockchain, *adaptfeeder.Feeder) {
	t.Helper()
	testDB := memory.New()
	chain := blockchain.New(testDB, network, statetestutils.UseNewState())

	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	for i := range numBlocks {
		fetchAndStoreBlock(t, chain, gw, i)
	}

	return chain, gw
}

func TestEvents(t *testing.T) {
	network := utils.Sepolia
	numCanonicalBlocks := uint64(5)
	chain, gw := setupTestChain(t, &network, numCanonicalBlocks)

	firstPendingBlock, err := gw.BlockByNumber(t.Context(), numCanonicalBlocks)
	require.NoError(t, err)

	secondPendingBlock, err := gw.BlockByNumber(t.Context(), numCanonicalBlocks+1)
	require.NoError(t, err)

	canonicalEvents := extractCanonicalEvents(t, chain, 0, numCanonicalBlocks-1)

	pending, pendingEvents := createEventPendingFromBlock(firstPendingBlock)
	preConfirmed, preConfirmedEvents := createEventPreConfirmedFromBlock(firstPendingBlock)
	preLatest, preLatestEvents := createEventPreLatestFromBlock(firstPendingBlock)

	preConfirmedWithPreLatest,
		preConfirmedWithPreLatestEvents := createEventPreConfirmedFromBlock(secondPendingBlock)
	preConfirmedWithPreLatest.WithPreLatest(&preLatest)

	// Precalculate combined events to avoid repeated allocations
	canonicalPending := make(
		[]rpc.EmittedEvent,
		0,
		len(canonicalEvents)+len(pendingEvents),
	)
	canonicalPending = append(canonicalPending, canonicalEvents...)
	canonicalPending = append(canonicalPending, pendingEvents...)

	canonicalPreConfirmed := make(
		[]rpc.EmittedEvent,
		0,
		len(canonicalEvents)+len(preConfirmedEvents),
	)
	canonicalPreConfirmed = append(canonicalPreConfirmed, canonicalEvents...)
	canonicalPreConfirmed = append(canonicalPreConfirmed, preConfirmedEvents...)

	canonicalPreLatestPreConfirmed := make(
		[]rpc.EmittedEvent,
		0,
		len(canonicalEvents)+len(preLatestEvents)+len(preConfirmedWithPreLatestEvents),
	)
	canonicalPreLatestPreConfirmed = append(canonicalPreLatestPreConfirmed, canonicalEvents...)
	canonicalPreLatestPreConfirmed = append(canonicalPreLatestPreConfirmed, preLatestEvents...)
	canonicalPreLatestPreConfirmed = append(
		canonicalPreLatestPreConfirmed,
		preConfirmedWithPreLatestEvents...,
	)

	l1AcceptedBlockNumber := uint64(4)
	l1AcceptedBlock, err := gw.BlockByNumber(t.Context(), l1AcceptedBlockNumber)
	eventsFromL1AcceptedBlock := extractCanonicalEvents(
		t, chain, l1AcceptedBlockNumber, l1AcceptedBlockNumber,
	)
	require.NoError(t, err)
	require.NoError(t, chain.SetL1Head(&core.L1Head{
		BlockNumber: l1AcceptedBlockNumber,
		BlockHash:   l1AcceptedBlock.Hash,
		StateRoot:   l1AcceptedBlock.GlobalStateRoot,
	}))

	type eventTest struct {
		description    string
		args           rpc.EventArgs
		pendingData    core.PendingData
		expectedEvents []rpc.EmittedEvent
		expectError    *jsonrpc.Error
	}

	// Test address and key for filtering
	testAddress := []felt.Address{
		felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}
	testKey := felt.NewUnsafeFromString[felt.Felt](
		"0x2e8a4ec40a36a027111fafdb6a46746ff1b0125d5067fbaebd8b5f227185a1e",
	)

	futureBlockNumber := rpcv9.BlockIDFromNumber(^uint64(0))
	nonExistingBlockHash := rpcv9.BlockIDFromHash(felt.NewUnsafeFromString[felt.Felt]("0x1BadB10c3"))
	defaultPageRequest := rpcv6.ResultPageRequest{
		ChunkSize:         100,
		ContinuationToken: "",
	}

	latestID := rpcv9.BlockIDLatest()
	preConfirmedID := rpcv9.BlockIDPreConfirmed()
	l1AcceptedID := rpcv9.BlockIDL1Accepted()
	blockIDZero := rpcv9.BlockIDFromNumber(0)
	blockID3 := rpcv9.BlockIDFromNumber(3)
	blockID4 := rpcv9.BlockIDFromNumber(4)

	// Common filter patterns
	// Common event args patterns
	onlyPreConfirmedNoPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			FromBlock: &preConfirmedID,
			ToBlock:   &preConfirmedID,
		},
		ResultPageRequest: defaultPageRequest,
	}
	onlyPreConfirmedWithPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			FromBlock: &preConfirmedID,
			ToBlock:   &preConfirmedID,
		},
		ResultPageRequest: rpcv6.ResultPageRequest{
			ChunkSize:         1,
			ContinuationToken: "",
		},
	}
	genesisToPreConfirmedNoPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			ToBlock: &preConfirmedID,
		},
		ResultPageRequest: defaultPageRequest,
	}
	genesisToPreConfirmedWithPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			ToBlock: &preConfirmedID,
		},
		ResultPageRequest: rpcv6.ResultPageRequest{
			ChunkSize:         1,
			ContinuationToken: "",
		},
	}
	genesisToLatestNoPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			ToBlock: &latestID,
		},
		ResultPageRequest: defaultPageRequest,
	}
	genesisToLatestWithPagination := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			ToBlock: &latestID,
		},
		ResultPageRequest: rpcv6.ResultPageRequest{
			ChunkSize:         1,
			ContinuationToken: "",
		},
	}

	testCases := []eventTest{
		{
			description: "future toBlock number bounds to latest",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &futureBlockNumber,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: canonicalEvents,
		},
		{
			description: "invalid block hash",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &nonExistingBlockHash,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectError: rpccore.ErrBlockNotFound,
		},
		{
			description: "from_block > to_block",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &latestID,
					ToBlock:   &blockIDZero,
					Address:   testAddress,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: []rpc.EmittedEvent{},
		},
		{
			description: "filter with no from_block",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					ToBlock: &latestID,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: canonicalEvents,
		},
		{
			description: "filter with no to_block",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: canonicalEvents,
		},
		{
			description: "page size too large",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &latestID,
					Address:   testAddress,
				},
				ResultPageRequest: rpcv6.ResultPageRequest{
					ChunkSize:         10240 + 1,
					ContinuationToken: "",
				},
			},
			expectError: rpccore.ErrPageSizeTooBig,
		},
		{
			description: "too many keys",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &latestID,
					Address:   testAddress,
					Keys:      make([][]felt.Felt, 1024+1),
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectError: rpccore.ErrTooManyKeysInFilter,
		},
		{
			description:    "canonical events - no pagination",
			args:           genesisToLatestNoPagination,
			expectedEvents: canonicalEvents,
		},
		{
			description:    "canonical events with pagination",
			args:           genesisToLatestWithPagination,
			expectedEvents: canonicalEvents,
		},
		{
			description: "canonical events with single address filter",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &latestID,
					Address:   testAddress,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: func() []rpc.EmittedEvent {
				var filtered []rpc.EmittedEvent
				for _, event := range canonicalEvents {
					// todo: remove the cast to felt.Felt
					if event.From.Equal((*felt.Felt)(&testAddress[0])) {
						filtered = append(filtered, event)
					}
				}
				return filtered
			}(),
		},
		{
			description: "canonical events with multiple addresses filter",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &latestID,
					Address: []felt.Address{
						testAddress[0],
						felt.Address(felt.Zero), // non-existent address
					},
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: func() []rpc.EmittedEvent {
				var filtered []rpc.EmittedEvent
				for _, event := range canonicalEvents {
					// todo: remove the cast to felt.Felt
					if event.From != nil && event.From.Equal((*felt.Felt)(&testAddress[0])) {
						filtered = append(filtered, event)
					}
				}
				return filtered
			}(),
		},
		{
			description: "canonical events with address and key filter",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockIDZero,
					ToBlock:   &latestID,
					Address:   testAddress,
					Keys:      [][]felt.Felt{{*testKey}},
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: func() []rpc.EmittedEvent {
				var filtered []rpc.EmittedEvent
				for _, event := range canonicalEvents {
					// todo: remove the cast to felt.Felt
					if event.From.Equal((*felt.Felt)(&testAddress[0])) &&
						len(event.Keys) > 0 && event.Keys[0].Equal(testKey) {
						filtered = append(filtered, event)
					}
				}
				return filtered
			}(),
		},
		{
			description: "canonical events from l1_accepted block",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &l1AcceptedID,
					ToBlock:   &l1AcceptedID,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: eventsFromL1AcceptedBlock,
		},
		{
			description: "subset of canonical events - spans multiple blocks",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockID3,
					ToBlock:   &blockID4,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: extractCanonicalEvents(t, chain, 3, 4),
		},
		{
			description: "subset of canonical events - single block",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &blockID4,
					ToBlock:   &blockID4,
				},
				ResultPageRequest: defaultPageRequest,
			},
			expectedEvents: extractCanonicalEvents(t, chain, 4, 4),
		},
		{
			description:    "pending events only - no pagination",
			args:           onlyPreConfirmedNoPagination,
			pendingData:    &pending,
			expectedEvents: pendingEvents,
		},
		{
			description:    "pending events with pagination",
			args:           onlyPreConfirmedWithPagination,
			pendingData:    &pending,
			expectedEvents: pendingEvents,
		},
		{
			description:    "canonical + pending events - no pagination",
			args:           genesisToPreConfirmedNoPagination,
			pendingData:    &pending,
			expectedEvents: canonicalPending,
		},
		{
			description:    "canonical + pending events with pagination",
			args:           genesisToPreConfirmedWithPagination,
			pendingData:    &pending,
			expectedEvents: canonicalPending,
		},
		{
			description:    "pre_confirmed events only - no pagination",
			args:           onlyPreConfirmedNoPagination,
			pendingData:    &preConfirmed,
			expectedEvents: preConfirmedEvents,
		},
		{
			description:    "pre_confirmed events with pagination",
			args:           onlyPreConfirmedWithPagination,
			pendingData:    &preConfirmed,
			expectedEvents: preConfirmedEvents,
		},
		{
			description:    "canonical + pre_confirmed events - no pagination",
			args:           genesisToPreConfirmedNoPagination,
			pendingData:    &preConfirmed,
			expectedEvents: canonicalPreConfirmed,
		},
		{
			description:    "canonical + pre_confirmed events with pagination",
			args:           genesisToPreConfirmedWithPagination,
			pendingData:    &preConfirmed,
			expectedEvents: canonicalPreConfirmed,
		},
		{
			description:    "canonical + pre_latest + pre_confirmed events - no pagination",
			args:           genesisToPreConfirmedNoPagination,
			pendingData:    &preConfirmedWithPreLatest,
			expectedEvents: canonicalPreLatestPreConfirmed,
		},
		{
			description:    "canonical + pre_latest + pre_confirmed events with pagination",
			args:           genesisToPreConfirmedWithPagination,
			pendingData:    &preConfirmedWithPreLatest,
			expectedEvents: canonicalPreLatestPreConfirmed,
		},
		{
			description:    "pre_confirmed events only - when there is pre-latest",
			args:           onlyPreConfirmedNoPagination,
			pendingData:    &preConfirmedWithPreLatest,
			expectedEvents: preConfirmedWithPreLatestEvents,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(chain, mockSyncReader, nil, utils.NewNopZapLogger())

			// Set up mock expectations
			if tc.pendingData != nil {
				mockSyncReader.EXPECT().PendingData().Return(tc.pendingData, nil).AnyTimes()
			}

			// Error cases: assert once and return
			if tc.expectError != nil {
				chunk, err := handler.Events(&tc.args)
				require.Equal(t, tc.expectError, err)
				require.Empty(t, chunk.Events)
				require.Empty(t, chunk.ContinuationToken)
				return
			}

			// Success cases: collect all pages then compare
			got := collectAllEvents(t, handler, &tc.args)
			require.Equal(t, tc.expectedEvents, got)
		})
	}
}

func TestEvents_FilterWithLimit(t *testing.T) {
	n := &utils.Sepolia
	chain, _ := setupTestChain(t, n, uint64(6))

	handler := rpc.New(chain, nil, nil, utils.NewNopZapLogger())

	from := []felt.Address{
		felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}
	blockNumber := rpcv9.BlockIDFromNumber(0)
	latest := rpcv9.BlockIDLatest()

	args := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			FromBlock: &blockNumber,
			ToBlock:   &latest,
			Address:   from,
			Keys:      [][]felt.Felt{},
		},
		ResultPageRequest: rpcv6.ResultPageRequest{
			ChunkSize:         100,
			ContinuationToken: "",
		},
	}

	handler = handler.WithFilterLimit(1)
	events, err := handler.Events(&args)
	require.Nil(t, err)
	require.Equal(t, "4-0", events.ContinuationToken)
	require.NotEmpty(t, events.Events)

	handler = handler.WithFilterLimit(7)
	events, err = handler.Events(&args)
	require.Nil(t, err)
	require.Empty(t, events.ContinuationToken)
	require.NotEmpty(t, events.Events)
}

func TestEvents_ChainProgressesWhilePaginating(t *testing.T) {
	network := utils.Sepolia
	numCanonicalBlocks := uint64(5)
	chain, gw := setupTestChain(t, &network, numCanonicalBlocks)

	block5, err := gw.BlockByNumber(t.Context(), 5)
	require.NoError(t, err)

	block6, err := gw.BlockByNumber(t.Context(), 6)
	require.NoError(t, err)

	expectedEvents := extractCanonicalEvents(t, chain, 0, numCanonicalBlocks-1)
	preLatest, preLatestEvents := createEventPreLatestFromBlock(block5)

	preConfirmed,
		preConfirmedEvents := createEventPreConfirmedFromBlock(block6)
	preConfirmed.WithPreLatest(&preLatest)

	expectedEvents = append(expectedEvents, preLatestEvents...)
	expectedEvents = append(expectedEvents, preConfirmedEvents...)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpc.New(chain, mockSyncReader, nil, utils.NewNopZapLogger())

	preConfirmedID := rpcv9.BlockIDPreConfirmed()
	// Test pagination with small chunk size to trigger multiple calls
	args := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			ToBlock: &preConfirmedID,
		},
		ResultPageRequest: rpcv6.ResultPageRequest{
			ChunkSize:         1, // Small chunk to force pagination
			ContinuationToken: "",
		},
	}

	// Collect events through pagination until we reach pending blocks
	var allEvents []rpc.EmittedEvent
	curArgs := args

	// Collect canonical events
	mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
	for {
		chunk, err := handler.Events(&curArgs)
		require.Nil(t, err)

		allEvents = append(allEvents, chunk.Events...)
		require.NotEmpty(t, chunk.ContinuationToken)
		// Check if we've reached a pending block (pre_confirmed or pre_latest)
		var blockNum, processedEvents uint64
		_, parseErr := fmt.Sscanf(chunk.ContinuationToken, "%d-%d", &blockNum, &processedEvents)
		require.NoError(t, parseErr)

		curArgs.ContinuationToken = chunk.ContinuationToken

		if blockNum >= numCanonicalBlocks {
			break
		}
	}

	// Read one from pre_latest block 5
	mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
	chunk, rpcErr := handler.Events(&curArgs)
	require.Nil(t, rpcErr)
	require.NotEmpty(t, chunk.ContinuationToken)
	require.Equal(t, uint64(5), *chunk.Events[0].BlockNumber)
	allEvents = append(allEvents, chunk.Events...)
	curArgs.ContinuationToken = chunk.ContinuationToken

	// pre-latest becomes latest, continue pagination from block 5
	fetchAndStoreBlock(t, chain, gw, 5)
	mockSyncReader.EXPECT().PendingData().Return(preConfirmed.WithPreLatest(nil), nil)
	chunk, rpcErr = handler.Events(&curArgs)
	require.Nil(t, rpcErr)
	require.NotEmpty(t, chunk.ContinuationToken)
	require.Equal(t, uint64(5), *chunk.Events[0].BlockNumber)
	allEvents = append(allEvents, chunk.Events...)
	curArgs.ContinuationToken = chunk.ContinuationToken

	// continue from pre_confirmed, read one from block 6
	mockSyncReader.EXPECT().PendingData().Return(preConfirmed.WithPreLatest(nil), nil)
	chunk, rpcErr = handler.Events(&curArgs)
	require.Nil(t, rpcErr)
	require.NotEmpty(t, chunk.ContinuationToken)
	require.Equal(t, uint64(6), *chunk.Events[0].BlockNumber)
	allEvents = append(allEvents, chunk.Events...)
	curArgs.ContinuationToken = chunk.ContinuationToken

	// pre_confirmed becomes pre_latest, continue pagination from block 6
	preLatest2, _ := createEventPreLatestFromBlock(block6)
	preConfirmed2 := core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 7,
			},
		},
		PreLatest: &preLatest2,
	}

	for {
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmed2, nil)
		chunk, rpcErr = handler.Events(&curArgs)
		require.Nil(t, rpcErr)
		require.Equal(t, uint64(6), *chunk.Events[0].BlockNumber)
		allEvents = append(allEvents, chunk.Events...)
		if chunk.ContinuationToken == "" {
			break
		}
		curArgs.ContinuationToken = chunk.ContinuationToken
	}

	// Compare events ignoring block hash
	// Block hash is ignored for this test due to mixing different block types
	require.Equal(t, len(expectedEvents), len(allEvents))
	for i, expected := range expectedEvents {
		actual := allEvents[i]
		require.Equal(t, expected.BlockNumber, actual.BlockNumber)
		require.Equal(t, expected.TransactionHash, actual.TransactionHash)
		require.Equal(t, expected.From, actual.From)
		require.Equal(t, expected.Keys, actual.Keys)
		require.Equal(t, expected.Data, actual.Data)
	}
}
