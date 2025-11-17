package rpcv6_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvents(t *testing.T) {
	var pendingB *core.Block

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
			pendingB = b
		}
	}

	pending := core.NewPending(pendingB, nil, nil)
	mockSyncReader.EXPECT().PendingData().Return(
		&pending,
		nil,
	)

	handler := rpc.New(chain, mockSyncReader, nil, n, utils.NewNopZapLogger())
	from := felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	args := rpc.EventsArg{
		EventFilter: rpc.EventFilter{
			FromBlock: &rpc.BlockID{Number: 0},
			ToBlock:   &rpc.BlockID{Latest: true},
			Address:   from,
			Keys:      [][]felt.Felt{},
		},
		ResultPageRequest: rpc.ResultPageRequest{
			ChunkSize:         100,
			ContinuationToken: "",
		},
	}

	t.Run("filter non-existent", func(t *testing.T) {
		t.Run("block number - bound to latest", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Number: 55}
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 4)
		})

		t.Run("block hash", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Hash: new(felt.Felt).SetUint64(55)}
			_, err := handler.Events(args)
			require.Equal(t, rpccore.ErrBlockNotFound, err)
		})
	})

	t.Run("filter with no from_block", func(t *testing.T) {
		args.FromBlock = nil
		args.ToBlock = &rpc.BlockID{Latest: true}
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no to_block", func(t *testing.T) {
		args.FromBlock = &rpc.BlockID{Number: 0}
		args.ToBlock = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with from_block > to_block", func(t *testing.T) {
		fArgs := rpc.EventsArg{
			EventFilter: rpc.EventFilter{
				FromBlock: &rpc.BlockID{Number: 1},
				ToBlock:   &rpc.BlockID{Number: 0},
			},
			ResultPageRequest: rpc.ResultPageRequest{
				ChunkSize:         100,
				ContinuationToken: "",
			},
		}

		events, err := handler.Events(fArgs)
		require.Nil(t, err)
		require.Empty(t, events.Events)
	})

	t.Run("filter with no address", func(t *testing.T) {
		args.ToBlock = &rpc.BlockID{Latest: true}
		args.Address = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no keys", func(t *testing.T) {
		var allEvents []rpc.EmittedEvent
		t.Run("get canonical events without pagination", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Latest: true}
			args.Address = from
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 4)
			require.Empty(t, events.ContinuationToken)
			allEvents = events.Events
		})

		t.Run("accumulate events with pagination", func(t *testing.T) {
			var accEvents []rpc.EmittedEvent
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
		key := felt.NewUnsafeFromString[felt.Felt]("0x2e8a4ec40a36a027111fafdb6a46746ff1b0125d5067fbaebd8b5f227185a1e")

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
				felt.NewUnsafeFromString[felt.Felt]("0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
				felt.NewUnsafeFromString[felt.Felt]("0x4"),
				felt.NewUnsafeFromString[felt.Felt]("0x4574686572"),
				felt.NewUnsafeFromString[felt.Felt]("0x455448"),
				felt.NewUnsafeFromString[felt.Felt]("0x12"),
				felt.NewUnsafeFromString[felt.Felt]("0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
			}, events.Events[0].Data)
			require.Equal(t, uint64(4), *events.Events[0].BlockNumber)
			require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x445152a69e628774b0f78a952e6f9ba0ffcda1374724b314140928fd2f31f4c"), events.Events[0].BlockHash)
			require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x3c9dfcd3fe66be18b661ee4ebb62520bb4f13d4182b040b3c2be9a12dbcc09b"), events.Events[0].TransactionHash)
		})
	})

	t.Run("large page size", func(t *testing.T) {
		args.ChunkSize = 10240 + 1
		events, err := handler.Events(args)
		require.Equal(t, rpccore.ErrPageSizeTooBig, err)
		require.Nil(t, events)
	})

	t.Run("too many keys", func(t *testing.T) {
		args.ChunkSize = 2
		args.Keys = make([][]felt.Felt, 1024+1)
		events, err := handler.Events(args)
		require.Equal(t, rpccore.ErrTooManyKeysInFilter, err)
		require.Nil(t, events)
	})

	t.Run("filter with limit", func(t *testing.T) {
		handler = handler.WithFilterLimit(1)
		args.Address = felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
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

	t.Run("get pending events without pagination", func(t *testing.T) {
		args = rpc.EventsArg{
			EventFilter: rpc.EventFilter{
				FromBlock: &rpc.BlockID{Pending: true},
				ToBlock:   &rpc.BlockID{Pending: true},
			},
			ResultPageRequest: rpc.ResultPageRequest{
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
		assert.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5"), events.Events[0].TransactionHash)
	})

	t.Run("get pending events with pagination", func(t *testing.T) {
		var err error
		pendingB, err = gw.BlockByNumber(t.Context(), 6)
		require.Nil(t, err)

		args = rpc.EventsArg{
			EventFilter: rpc.EventFilter{
				FromBlock: &rpc.BlockID{Pending: true},
				ToBlock:   &rpc.BlockID{Pending: true},
			},
			ResultPageRequest: rpc.ResultPageRequest{
				ChunkSize: 1,
			},
		}

		allEvents := []*core.Event{}

		for _, receipt := range pendingB.Receipts {
			allEvents = append(allEvents, receipt.Events...)
		}
		pending := core.NewPending(pendingB, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&pending,
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

			assert.Equal(t, expectedEvent.From, actualEvent.From)
			assert.Equal(t, expectedEvent.Keys, actualEvent.Keys)
			assert.Equal(t, expectedEvent.Data, actualEvent.Data)

			args.ContinuationToken = events.ContinuationToken
		}
	})

	t.Run("pending block always empty after starknet 0.14.0", func(t *testing.T) {
		args = rpc.EventsArg{
			EventFilter: rpc.EventFilter{
				FromBlock: &rpc.BlockID{Pending: true},
				ToBlock:   &rpc.BlockID{Pending: true},
			},
			ResultPageRequest: rpc.ResultPageRequest{
				ChunkSize:         100,
				ContinuationToken: "",
			},
		}
		preConfirmed := core.NewPreConfirmed(pendingB, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)

		events, err := handler.Events(args)
		require.Nil(t, err)
		require.Len(t, events.Events, 0)
		require.Empty(t, events.ContinuationToken)
	})
}
