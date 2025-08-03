package rpcv9_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvents(t *testing.T) {
	var preConfirmedB *core.Block

	testDB := memory.New()
	n := &utils.Sepolia
	chain := blockchain.New(testDB, n, statetestutils.UseNewState())

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)
	for i := range 7 {
		b, err := gw.BlockByNumber(t.Context(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(t.Context(), uint64(i))
		require.NoError(t, err)

		if b.Number < 6 {
			require.NoError(t, chain.Store(b, &core.BlockCommitments{}, s, nil))
		} else {
			b.Hash = nil
			b.GlobalStateRoot = nil
			preConfirmedB = b
		}
	}

	handler := rpc.New(chain, mockSyncReader, nil, utils.NewNopZapLogger())
	from := utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	blockNumber := blockIDNumber(t, 0)
	latest := blockIDLatest(t)
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

	t.Run("filter non-existent", func(t *testing.T) {
		t.Run("block number - bounds to latest", func(t *testing.T) {
			number := blockIDNumber(t, 55)
			args.ToBlock = &number
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 4)
		})

		t.Run("block hash", func(t *testing.T) {
			hash := blockIDHash(t, new(felt.Felt).SetUint64(55))
			args.ToBlock = &hash
			_, err := handler.Events(args)
			require.Equal(t, rpccore.ErrBlockNotFound, err)
		})
	})

	t.Run("filter with from_block > to_block", func(t *testing.T) {
		fromBlock := blockIDNumber(t, 1)
		toBlock := blockIDNumber(t, 0)
		fArgs := rpc.EventArgs{
			EventFilter: rpc.EventFilter{
				FromBlock: &fromBlock,
				ToBlock:   &toBlock,
			},
			ResultPageRequest: rpcv6.ResultPageRequest{
				ChunkSize:         100,
				ContinuationToken: "",
			},
		}

		events, err := handler.Events(fArgs)
		require.Nil(t, err)
		require.Empty(t, events.Events)
	})

	t.Run("filter with no from_block", func(t *testing.T) {
		args.FromBlock = nil
		args.ToBlock = &latest
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no to_block", func(t *testing.T) {
		number := blockIDNumber(t, 0)
		args.FromBlock = &number
		args.ToBlock = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no address", func(t *testing.T) {
		args.ToBlock = &latest
		args.Address = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no keys", func(t *testing.T) {
		var allEvents []*rpcv6.EmittedEvent
		t.Run("get canonical events without pagination", func(t *testing.T) {
			args.ToBlock = &latest
			args.Address = from
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 4)
			require.Empty(t, events.ContinuationToken)
			allEvents = events.Events
		})

		t.Run("accumulate events with pagination", func(t *testing.T) {
			var accEvents []*rpcv6.EmittedEvent
			args.ChunkSize = 1

			for range len(allEvents) + 1 {
				events, err := handler.Events(args)
				require.Nil(t, err)
				accEvents = append(accEvents, events.Events...)
				args.ContinuationToken = events.ContinuationToken
				if args.ContinuationToken == "" {
					break
				}
			}
			require.Equal(t, allEvents, accEvents)
		})
	})

	t.Run("filter with keys", func(t *testing.T) {
		key := utils.HexToFelt(t, "0x2e8a4ec40a36a027111fafdb6a46746ff1b0125d5067fbaebd8b5f227185a1e")

		t.Run("get all events without pagination", func(t *testing.T) {
			args.ChunkSize = 100
			args.Keys = append(args.Keys, []felt.Felt{*key})
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 1)
			require.Empty(t, events.ContinuationToken)

			require.Equal(t, from, events.Events[0].From)
			require.Equal(t, []*felt.Felt{key}, events.Events[0].Keys)
			require.Equal(t, []*felt.Felt{
				utils.HexToFelt(t, "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5"),
				utils.HexToFelt(t, "0x0"),
				utils.HexToFelt(t, "0x4"),
				utils.HexToFelt(t, "0x4574686572"),
				utils.HexToFelt(t, "0x455448"),
				utils.HexToFelt(t, "0x12"),
				utils.HexToFelt(t, "0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f"),
				utils.HexToFelt(t, "0x0"),
			}, events.Events[0].Data)
			require.Equal(t, uint64(4), *events.Events[0].BlockNumber)
			require.Equal(t, utils.HexToFelt(t, "0x445152a69e628774b0f78a952e6f9ba0ffcda1374724b314140928fd2f31f4c"), events.Events[0].BlockHash)
			require.Equal(t, utils.HexToFelt(t, "0x3c9dfcd3fe66be18b661ee4ebb62520bb4f13d4182b040b3c2be9a12dbcc09b"), events.Events[0].TransactionHash)
		})
	})

	t.Run("large page size", func(t *testing.T) {
		args.ChunkSize = 10240 + 1
		events, err := handler.Events(args)
		require.Equal(t, rpccore.ErrPageSizeTooBig, err)
		require.Empty(t, events)
	})

	t.Run("too many keys", func(t *testing.T) {
		args.ChunkSize = 2
		args.Keys = make([][]felt.Felt, 1024+1)
		events, err := handler.Events(args)
		require.Equal(t, rpccore.ErrTooManyKeysInFilter, err)
		require.Empty(t, events)
	})

	t.Run("filter with limit", func(t *testing.T) {
		handler = handler.WithFilterLimit(1)
		args.Address = utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
		args.ChunkSize = 100
		args.Keys = make([][]felt.Felt, 0)

		events, err := handler.Events(args)
		require.Nil(t, err)
		require.Equal(t, "4-0", events.ContinuationToken)
		require.NotEmpty(t, events.Events)

		handler = handler.WithFilterLimit(7)
		events, err = handler.Events(args)
		require.Nil(t, err)
		require.Empty(t, events.ContinuationToken)
		require.NotEmpty(t, events.Events)
	})

	t.Run("get pre_confirmed events without pagination", func(t *testing.T) {
		preConfirmed := core.NewPreConfirmed(preConfirmedB, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)
		preConfirmedID := blockIDPreConfirmed(t)
		args = rpc.EventArgs{
			EventFilter: rpc.EventFilter{
				FromBlock: &preConfirmedID,
				ToBlock:   &preConfirmedID,
			},
			ResultPageRequest: rpcv6.ResultPageRequest{
				ChunkSize:         100,
				ContinuationToken: "",
			},
		}
		events, err := handler.Events(args)
		require.Nil(t, err)
		require.Len(t, events.Events, 2)
		require.Empty(t, events.ContinuationToken)

		assert.Nil(t, events.Events[0].BlockHash)
		assert.Nil(t, events.Events[0].BlockNumber)
		assert.Equal(t, utils.HexToFelt(t, "0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5"), events.Events[0].TransactionHash)
	})

	t.Run("get pre_confirmed events with pagination", func(t *testing.T) {
		var err error
		preConfirmedB, err = gw.BlockByNumber(t.Context(), 6)
		require.Nil(t, err)
		preConfirmedID := blockIDPreConfirmed(t)
		args = rpc.EventArgs{
			EventFilter: rpc.EventFilter{
				FromBlock: &preConfirmedID,
				ToBlock:   &preConfirmedID,
			},
			ResultPageRequest: rpcv6.ResultPageRequest{
				ChunkSize: 1,
			},
		}

		allEvents := []*core.Event{}

		for _, receipt := range preConfirmedB.Receipts {
			allEvents = append(allEvents, receipt.Events...)
		}

		preConfirmed := core.NewPreConfirmed(preConfirmedB, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		).Times(len(allEvents))

		for i, expectedEvent := range allEvents {
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 1)
			actualEvent := events.Events[0]
			if i == len(allEvents)-1 {
				require.Empty(t, events.ContinuationToken)
			} else {
				require.NotEmpty(t, events.ContinuationToken)
			}

			assert.Equal(t, expectedEvent.From.String(), actualEvent.From.String())
			assert.Equal(t, expectedEvent.Keys, actualEvent.Keys)
			assert.Equal(t, expectedEvent.Data, actualEvent.Data)
			args.ContinuationToken = events.ContinuationToken
		}
	})

	t.Run("get events from `l1_accepted` block", func(t *testing.T) {
		l1AcceptedID := blockIDL1Accepted(t)
		args = rpc.EventArgs{
			EventFilter: rpc.EventFilter{
				FromBlock: &l1AcceptedID,
				ToBlock:   &l1AcceptedID,
			},
			ResultPageRequest: rpcv6.ResultPageRequest{
				ChunkSize: 100,
			},
		}

		block, err := gw.BlockByNumber(t.Context(), 4)
		require.NoError(t, err)

		require.NoError(t, chain.SetL1Head(&core.L1Head{
			BlockNumber: block.Number,
			BlockHash:   block.Hash,
			StateRoot:   block.GlobalStateRoot,
		}))

		expectedEvents := []*core.Event{}

		for _, receipt := range block.Receipts {
			expectedEvents = append(expectedEvents, receipt.Events...)
		}

		require.NotEmpty(t, expectedEvents, "test data must contain some events")
		events, rpcErr := handler.Events(args)
		require.Nil(t, rpcErr)
		require.Equal(t, len(expectedEvents), len(events.Events))
		require.Empty(t, events.ContinuationToken)

		for i, expectedEvent := range expectedEvents {
			actualEvent := events.Events[i]

			assert.Equal(t, expectedEvent.From.String(), actualEvent.From.String())
			assert.Equal(t, expectedEvent.Keys, actualEvent.Keys)
			assert.Equal(t, expectedEvent.Data, actualEvent.Data)
		}
	})
}
