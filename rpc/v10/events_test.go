package rpcv10_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// createEventPreConfirmedFromBlock creates a pre_confirmed block from the given block and
// returns the pre_confirmed data and emitted events
func createEventPreConfirmedFromBlock(
	block *core.Block,
) (pending.PreConfirmed, []rpc.EmittedEvent) {
	newHeader := &core.Header{
		Number:           block.Header.Number,
		Timestamp:        block.Header.Timestamp,
		SequencerAddress: block.Header.SequencerAddress,
		EventsBloom:      core.EventsBloom(block.Receipts),
	}

	preConfirmedBlock := &core.Block{
		Header:       newHeader,
		Transactions: block.Transactions,
		Receipts:     block.Receipts,
	}

	preConfirmed := pending.NewPreConfirmed(preConfirmedBlock, nil, nil, "")

	// Extract events from the block and convert to emitted events
	var events []rpc.EmittedEvent
	for txIndex, receipt := range preConfirmedBlock.Receipts {
		for eventIndex, event := range receipt.Events {
			events = append(events, rpc.EmittedEvent{
				Event: &rpc.Event{
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
					Event: &rpc.Event{
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

// filterEventsByAddress returns the events emitted from the given address.
func filterEventsByAddress(events []rpc.EmittedEvent, address *felt.Address) []rpc.EmittedEvent {
	var filtered []rpc.EmittedEvent
	for _, event := range events {
		if event.From != nil && event.From.Equal((*felt.Felt)(address)) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// filterEventsByAddressAndKey returns the events emitted from
// the given address whose first key matches.
func filterEventsByAddressAndKey(
	events []rpc.EmittedEvent,
	address *felt.Address,
	key *felt.Felt,
) []rpc.EmittedEvent {
	filtered := []rpc.EmittedEvent{}
	for _, event := range events {
		if event.From != nil && event.From.Equal((*felt.Felt)(address)) &&
			len(event.Keys) > 0 && event.Keys[0].Equal(key) {
			filtered = append(filtered, event)
		}
	}
	return filtered
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
	network *networks.Network,
	numBlocks uint64,
) (*blockchain.Blockchain, *adaptfeeder.Feeder) {
	t.Helper()
	testDB := memory.New()
	chain := blockchain.New(
		testDB,
		network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	for i := range numBlocks {
		fetchAndStoreBlock(t, chain, gw, i)
	}

	return chain, gw
}

func TestEvents(t *testing.T) {
	network := networks.Sepolia
	numCanonicalBlocks := uint64(5)
	chain, gw := setupTestChain(t, &network, numCanonicalBlocks)

	block5, err := gw.BlockByNumber(t.Context(), numCanonicalBlocks)
	require.NoError(t, err)

	block6, err := gw.BlockByNumber(t.Context(), numCanonicalBlocks+1)
	require.NoError(t, err)

	canonicalEvents := extractCanonicalEvents(t, chain, 0, numCanonicalBlocks-1)

	// block5 and block6 sit above the canonical head (head=4) and are used
	// only as pre_confirmed fixtures — never stored in `chain`. preConfirmed5
	// is reused across two alternative pre_confirmed ChainReader fixtures
	// below (singlePreConfirmedChain and the bottom slot of multiPreConfirmed);
	// each test case selects one via the mock, so the two are never live at
	// the same time.
	preConfirmed5, preConfirmed5Events := createEventPreConfirmedFromBlock(block5)
	preConfirmed6, preConfirmed6Events := createEventPreConfirmedFromBlock(block6)
	multiPreConfirmed := mustNewChain(t, &preConfirmed5, &preConfirmed6)

	multiPreConfirmedEvents := make(
		[]rpc.EmittedEvent,
		0,
		len(preConfirmed5Events)+len(preConfirmed6Events),
	)
	multiPreConfirmedEvents = append(multiPreConfirmedEvents, preConfirmed5Events...)
	multiPreConfirmedEvents = append(multiPreConfirmedEvents, preConfirmed6Events...)

	canonicalPreConfirmed := make(
		[]rpc.EmittedEvent,
		0,
		len(canonicalEvents)+len(preConfirmed5Events),
	)
	canonicalPreConfirmed = append(canonicalPreConfirmed, canonicalEvents...)
	canonicalPreConfirmed = append(canonicalPreConfirmed, preConfirmed5Events...)

	canonicalMultiPreConfirmed := make(
		[]rpc.EmittedEvent,
		0,
		len(canonicalEvents)+len(multiPreConfirmedEvents),
	)
	canonicalMultiPreConfirmed = append(canonicalMultiPreConfirmed, canonicalEvents...)
	canonicalMultiPreConfirmed = append(canonicalMultiPreConfirmed, multiPreConfirmedEvents...)

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
		preConfirmed   *preconfirmed.ChainReader
		expectedEvents []rpc.EmittedEvent
		expectError    *jsonrpc.Error
	}

	singlePreConfirmedChain := mustNewChain(t, &preConfirmed5)

	// Test address and key for filtering
	testAddress := []felt.Address{
		felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}
	testKey := felt.NewUnsafeFromString[felt.Felt](
		"0x2e8a4ec40a36a027111fafdb6a46746ff1b0125d5067fbaebd8b5f227185a1e",
	)

	futureBlockNumber := rpc.BlockIDFromNumber(^uint64(0))
	nonExistingBlockHash := rpc.BlockIDFromHash(felt.NewUnsafeFromString[felt.Felt]("0x1BadB10c3"))
	defaultPageRequest := rpc.ResultPageRequest{
		ChunkSize:         100,
		ContinuationToken: "",
	}

	latestID := rpc.BlockIDLatest()
	preConfirmedID := rpc.BlockIDPreConfirmed()
	l1AcceptedID := rpc.BlockIDL1Accepted()
	blockIDZero := rpc.BlockIDFromNumber(0)
	blockID3 := rpc.BlockIDFromNumber(3)
	blockID4 := rpc.BlockIDFromNumber(4)

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
		ResultPageRequest: rpc.ResultPageRequest{
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
		ResultPageRequest: rpc.ResultPageRequest{
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
		ResultPageRequest: rpc.ResultPageRequest{
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
				ResultPageRequest: rpc.ResultPageRequest{
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
			description:    "pre_confirmed events only - no pagination",
			args:           onlyPreConfirmedNoPagination,
			preConfirmed:   &singlePreConfirmedChain,
			expectedEvents: preConfirmed5Events,
		},
		{
			description:    "pre_confirmed events with pagination",
			args:           onlyPreConfirmedWithPagination,
			preConfirmed:   &singlePreConfirmedChain,
			expectedEvents: preConfirmed5Events,
		},
		{
			description:    "canonical + pre_confirmed events - no pagination",
			args:           genesisToPreConfirmedNoPagination,
			preConfirmed:   &singlePreConfirmedChain,
			expectedEvents: canonicalPreConfirmed,
		},
		{
			description:    "canonical + pre_confirmed events with pagination",
			args:           genesisToPreConfirmedWithPagination,
			preConfirmed:   &singlePreConfirmedChain,
			expectedEvents: canonicalPreConfirmed,
		},
		{
			description:    "multi pre_confirmed events only - no pagination",
			args:           onlyPreConfirmedNoPagination,
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: multiPreConfirmedEvents,
		},
		{
			description:    "multi pre_confirmed events with chunk-size 1 pagination",
			args:           onlyPreConfirmedWithPagination,
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: multiPreConfirmedEvents,
		},
		{
			description: "multi pre_confirmed events with mid-block pagination",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &preConfirmedID,
					ToBlock:   &preConfirmedID,
				},
				ResultPageRequest: rpc.ResultPageRequest{
					// Force a continuation token that lands inside the
					// boundary between the two pre_confirmed blocks for at
					// least one page boundary (canonical block 5 carries
					// >1 events on Sepolia testdata).
					ChunkSize:         uint64(max(1, len(preConfirmed5Events)-1)),
					ContinuationToken: "",
				},
			},
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: multiPreConfirmedEvents,
		},
		{
			description:    "canonical + multi pre_confirmed - no pagination",
			args:           genesisToPreConfirmedNoPagination,
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: canonicalMultiPreConfirmed,
		},
		{
			description:    "canonical + multi pre_confirmed with pagination",
			args:           genesisToPreConfirmedWithPagination,
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: canonicalMultiPreConfirmed,
		},
		{
			// Chunk size equal to bottom slot's event count returns exactly
			// the bottom slot's events on the first page, with a continuation
			// token that advances into the tip slot. Final pagination yields
			// the tip slot's events.
			description: "multi pre_confirmed - pagination boundary aligns with bottom slot",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &preConfirmedID,
					ToBlock:   &preConfirmedID,
				},
				ResultPageRequest: rpc.ResultPageRequest{
					ChunkSize:         uint64(len(preConfirmed5Events)),
					ContinuationToken: "",
				},
			},
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: multiPreConfirmedEvents,
		},
		{
			description: "multi pre_confirmed - address filter across both slots",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &preConfirmedID,
					ToBlock:   &preConfirmedID,
					Address:   testAddress,
				},
				ResultPageRequest: defaultPageRequest,
			},
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: filterEventsByAddress(multiPreConfirmedEvents, &testAddress[0]),
		},
		{
			description: "multi pre_confirmed - address+key filter across both slots with chunk-size 1",
			args: rpc.EventArgs{
				EventFilter: rpc.EventFilter{
					FromBlock: &preConfirmedID,
					ToBlock:   &preConfirmedID,
					Address:   testAddress,
					Keys:      [][]felt.Felt{{*testKey}},
				},
				ResultPageRequest: rpc.ResultPageRequest{
					ChunkSize:         1,
					ContinuationToken: "",
				},
			},
			preConfirmed:   &multiPreConfirmed,
			expectedEvents: filterEventsByAddressAndKey(multiPreConfirmedEvents, &testAddress[0], testKey),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(chain, mockSyncReader, nil, log.NewNopZapLogger())

			// Set up mock expectations
			if tc.preConfirmed != nil {
				mockSyncReader.EXPECT().PreConfirmedChain().Return(*tc.preConfirmed, nil).AnyTimes()
			} else {
				mockSyncReader.EXPECT().
					PreConfirmedChain().
					Return(preconfirmed.ChainReader{}, db.ErrKeyNotFound).
					AnyTimes()
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
	n := &networks.Sepolia
	chain, _ := setupTestChain(t, n, uint64(6))

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockSyncReader.EXPECT().
		PreConfirmedChain().
		Return(preconfirmed.ChainReader{}, db.ErrKeyNotFound).
		AnyTimes()
	handler := rpc.New(chain, mockSyncReader, nil, log.NewNopZapLogger())

	from := []felt.Address{
		felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}
	blockNumber := rpc.BlockIDFromNumber(0)
	latest := rpc.BlockIDLatest()

	args := rpc.EventArgs{
		EventFilter: rpc.EventFilter{
			FromBlock: &blockNumber,
			ToBlock:   &latest,
			Address:   from,
			Keys:      [][]felt.Felt{},
		},
		ResultPageRequest: rpc.ResultPageRequest{
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

// TestEvents_ChainProgressesWhilePaginating was removed: it exercised the
// pre_latest → pre_confirmed → canonical promotion path which no longer
// exists with the multi-preconfirmed model. Equivalent multi-block coverage
// lives in sync/preconfirmed/chain_storage_test.go.

func TestAddressListUnmarshalJSON(t *testing.T) {
	addr1 := felt.FromUint64[felt.Address](0x1)
	addr2 := felt.FromUint64[felt.Address](0x2)

	tests := []struct {
		name    string
		input   string
		want    rpc.AddressList
		wantErr bool
	}{
		{
			name:  "null",
			input: "null",
			want:  rpc.AddressList{},
		},
		{
			name:  "empty array",
			input: "[]",
			want:  rpc.AddressList{},
		},
		{
			name:  "single address",
			input: `"0x1"`,
			want:  rpc.AddressList{addr1},
		},
		{
			name:  "array of addresses",
			input: `["0x1","0x2"]`,
			want:  rpc.AddressList{addr1, addr2},
		},
		{
			name:  "array with duplicates",
			input: `["0x1","0x1"]`,
			want:  rpc.AddressList{addr1},
		},
		{
			name:    "invalid JSON",
			input:   "not-json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got rpc.AddressList
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAddressListContains(t *testing.T) {
	addr1 := felt.FromUint64[felt.Address](0x1)
	addr2 := felt.FromUint64[felt.Address](0x2)
	addr3 := felt.FromUint64[felt.Address](0x3)

	tests := []struct {
		name string
		list rpc.AddressList
		addr felt.Address
		want bool
	}{
		{
			name: "empty list is wildcard",
			list: rpc.AddressList{},
			addr: addr1,
			want: true,
		},
		{
			name: "address in list",
			list: rpc.AddressList{addr1, addr2},
			addr: addr1,
			want: true,
		},
		{
			name: "address not in list",
			list: rpc.AddressList{addr1, addr2},
			addr: addr3,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.list.Contains(&tt.addr))
		})
	}
}
