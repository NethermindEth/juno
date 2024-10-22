package rpc_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"go.uber.org/mock/gomock"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyCommitments = core.BlockCommitments{}

const (
	subscribeNewHeads = `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribeNewHeads"}`
	newHeadsResponse  = `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`
	subscribeResponse = `{"jsonrpc":"2.0","result":{"subscription_id":%d},"id":1}`
)

func TestEvents(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	n := utils.Ptr(utils.Sepolia)
	chain := blockchain.New(testDB, n)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	for i := range 7 {
		b, err := gw.BlockByNumber(context.Background(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(context.Background(), uint64(i))
		require.NoError(t, err)

		if b.Number < 6 {
			require.NoError(t, chain.Store(b, &core.BlockCommitments{}, s, nil))
		} else {
			b.Hash = nil
			b.GlobalStateRoot = nil
			require.NoError(t, chain.StorePending(&blockchain.Pending{
				Block:       b,
				StateUpdate: s,
			}))
		}
	}

	handler := rpc.New(chain, nil, nil, "", utils.NewNopZapLogger())
	from := utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
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
		t.Run("block number", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Number: 55}
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 5)
		})

		t.Run("block hash", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Hash: new(felt.Felt).SetUint64(55)}
			_, err := handler.Events(args)
			require.Equal(t, rpc.ErrBlockNotFound, err)
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

	t.Run("filter with no address", func(t *testing.T) {
		args.ToBlock = &rpc.BlockID{Latest: true}
		args.Address = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no keys", func(t *testing.T) {
		var allEvents []*rpc.EmittedEvent
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
			var accEvents []*rpc.EmittedEvent
			args.ChunkSize = 1

			for i := 0; i < len(allEvents)+1; i++ {
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
		require.Equal(t, rpc.ErrPageSizeTooBig, err)
		require.Nil(t, events)
	})

	t.Run("too many keys", func(t *testing.T) {
		args.ChunkSize = 2
		args.Keys = make([][]felt.Felt, 1024+1)
		events, err := handler.Events(args)
		require.Equal(t, rpc.ErrTooManyKeysInFilter, err)
		require.Nil(t, events)
	})

	t.Run("filter with limit", func(t *testing.T) {
		handler = handler.WithFilterLimit(1)
		key := utils.HexToFelt(t, "0x2e8a4ec40a36a027111fafdb6a46746ff1b0125d5067fbaebd8b5f227185a1e")
		args.ChunkSize = 100
		args.Keys = make([][]felt.Felt, 0)
		args.Keys = append(args.Keys, []felt.Felt{*key})
		events, err := handler.Events(args)
		require.Nil(t, err)
		require.Equal(t, "1-0", events.ContinuationToken)
		require.Empty(t, events.Events)
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
		assert.Equal(t, utils.HexToFelt(t, "0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5"), events.Events[0].TransactionHash)
	})
}

type fakeSyncer struct {
	newHeads   *feed.Feed[*core.Header]
	reorgs     *feed.Feed[*sync.ReorgData]
	pendingTxs *feed.Feed[[]core.Transaction]
}

func newFakeSyncer() *fakeSyncer {
	return &fakeSyncer{
		newHeads:   feed.New[*core.Header](),
		reorgs:     feed.New[*sync.ReorgData](),
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

func TestSubscribeNewHeads(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler, syncer, server := setupSubscriptionTest(t, ctx)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeNewHeads",
		Params:  []jsonrpc.Parameter{{Name: "block", Optional: true}},
		Handler: handler.SubscribeNewHeads,
	}))

	ws := jsonrpc.NewWebsocket(server, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	conn, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)

	id := uint64(1)
	handler.WithIDGen(func() uint64 { return id })

	got := sendAndReceiveMessage(t, ctx, conn, subscribeNewHeads)
	want := fmt.Sprintf(subscribeResponse, id)
	require.Equal(t, want, got)

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Receive a block header.
	want = fmt.Sprintf(newHeadsResponse, id)
	_, headerGot, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(headerGot))
}

func TestMultipleSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler, syncer, server := setupSubscriptionTest(t, ctx)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeNewHeads",
		Params:  []jsonrpc.Parameter{{Name: "block", Optional: true}},
		Handler: handler.SubscribeNewHeads,
	}, jsonrpc.Method{
		Name:    "juno_unsubscribe",
		Params:  []jsonrpc.Parameter{{Name: "id"}},
		Handler: handler.Unsubscribe,
	}))

	ws := jsonrpc.NewWebsocket(server, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)
	conn2, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)

	firstID := uint64(1)
	secondID := uint64(2)

	handler.WithIDGen(func() uint64 { return firstID })
	firstWant := fmt.Sprintf(subscribeResponse, firstID)
	firstGot := sendAndReceiveMessage(t, ctx, conn1, subscribeNewHeads)
	require.NoError(t, err)
	require.Equal(t, firstWant, firstGot)

	handler.WithIDGen(func() uint64 { return secondID })
	secondWant := fmt.Sprintf(subscribeResponse, secondID)
	secondGot := sendAndReceiveMessage(t, ctx, conn2, subscribeNewHeads)
	require.NoError(t, err)
	require.Equal(t, secondWant, secondGot)

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Receive a block header.
	firstHeaderWant := fmt.Sprintf(newHeadsResponse, firstID)
	_, firstHeaderGot, err := conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, firstHeaderWant, string(firstHeaderGot))

	secondHeaderWant := fmt.Sprintf(newHeadsResponse, secondID)
	_, secondHeaderGot, err := conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, secondHeaderWant, string(secondHeaderGot))

	// Unsubscribe
	unsubMsg := `{"jsonrpc":"2.0","id":1,"method":"juno_unsubscribe","params":[%d]}`
	require.NoError(t, conn1.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, firstID))))
	require.NoError(t, conn2.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, secondID))))
}

func TestSubscribeNewHeadsHistorical(t *testing.T) {
	t.Parallel()

	log := utils.NewNopZapLogger()
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	block0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Mainnet)
	assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

	chain = blockchain.New(testDB, &utils.Mainnet)
	syncer := newFakeSyncer()
	handler := rpc.New(chain, syncer, nil, "", utils.NewNopZapLogger())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	time.Sleep(50 * time.Millisecond)

	server := jsonrpc.NewServer(1, log)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeNewHeads",
		Params:  []jsonrpc.Parameter{{Name: "block", Optional: true}},
		Handler: handler.SubscribeNewHeads,
	}))

	ws := jsonrpc.NewWebsocket(server, log)
	httpSrv := httptest.NewServer(ws)

	conn, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)

	id := uint64(1)
	handler.WithIDGen(func() uint64 { return id })

	subscribeMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribeNewHeads", "params":{"block":{"block_number":0}}}`
	got := sendAndReceiveMessage(t, ctx, conn, subscribeMsg)
	want := fmt.Sprintf(subscribeResponse, id)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// Check block 0 content
	want = `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943","parent_hash":"0x0","block_number":0,"new_root":"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6","timestamp":1637069048,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`
	want = fmt.Sprintf(want, id)
	_, block0Got, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(block0Got))

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Check new block content
	want = fmt.Sprintf(newHeadsResponse, id)
	_, newBlockGot, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(newBlockGot))
}

func TestSubscriptionReorg(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler, syncer, server := setupSubscriptionTest(t, ctx)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeNewHeads",
		Params:  []jsonrpc.Parameter{{Name: "block", Optional: true}},
		Handler: handler.SubscribeNewHeads,
	}))

	ws := jsonrpc.NewWebsocket(server, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	conn, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)

	id := uint64(1)
	handler.WithIDGen(func() uint64 { return id })

	got := sendAndReceiveMessage(t, ctx, conn, subscribeNewHeads)
	want := fmt.Sprintf(subscribeResponse, id)
	require.Equal(t, want, got)

	// Simulate a reorg
	syncer.reorgs.Send(&sync.ReorgData{
		StartBlockHash: utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"),
		StartBlockNum:  0,
		EndBlockHash:   utils.HexToFelt(t, "0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86"),
		EndBlockNum:    2,
	})

	// Receive reorg event
	want = `{"jsonrpc":"2.0","method":"starknet_subscriptionReorg","params":{"result":{"starting_block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","starting_block_number":0,"ending_block_hash":"0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86","ending_block_number":2},"subscription_id":%d}}`
	want = fmt.Sprintf(want, id)
	_, reorgGot, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, want, string(reorgGot))
}

func TestSubscribePendingTxs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler, syncer, server := setupSubscriptionTest(t, ctx)

	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribePendingTransactions",
		Params:  []jsonrpc.Parameter{{Name: "transaction_details", Optional: true}, {Name: "sender_address", Optional: true}},
		Handler: handler.SubscribePendingTxs,
	}))

	ws := jsonrpc.NewWebsocket(server, utils.NewNopZapLogger())
	httpSrv := httptest.NewServer(ws)

	t.Run("Basic subscription", func(t *testing.T) {
		conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)

		subscribeMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions"}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendAndReceiveMessage(t, ctx, conn1, subscribeMsg)
		want := fmt.Sprintf(subscribeResponse, id)
		require.Equal(t, want, got)

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

		want = `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":["0x1","0x2","0x3","0x4","0x5"],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
	})

	t.Run("Filtered subscription", func(t *testing.T) {
		conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)

		subscribeMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions", "params":{"sender_address":["0xb", "0x16"]}}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendAndReceiveMessage(t, ctx, conn1, subscribeMsg)
		want := fmt.Sprintf(subscribeResponse, id)
		require.Equal(t, want, got)

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

		want = `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":["0x1","0x2"],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
	})

	t.Run("Full details subscription", func(t *testing.T) {
		t.Parallel()
		conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
		require.NoError(t, err)

		subscribeMsg := `{"jsonrpc":"2.0","id":1,"method":"starknet_subscribePendingTransactions", "params":{"transaction_details": true}}`
		id := uint64(1)
		handler.WithIDGen(func() uint64 { return id })
		got := sendAndReceiveMessage(t, ctx, conn1, subscribeMsg)
		want := fmt.Sprintf(subscribeResponse, id)
		require.Equal(t, want, got)

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

		want = `{"jsonrpc":"2.0","method":"starknet_subscriptionPendingTransactions","params":{"result":[{"transaction_hash":"0x1","type":"INVOKE","version":"0x3","nonce":"0x7","max_fee":"0x4","contract_address":"0x5","sender_address":"0x8","signature":["0x3"],"calldata":["0x2"],"entry_point_selector":"0x6","resource_bounds":{},"tip":"0x9","paymaster_data":["0xa"],"account_deployment_data":["0xb"],"nonce_data_availability_mode":"L1","fee_data_availability_mode":"L1"}],"subscription_id":%d}}`
		want = fmt.Sprintf(want, id)
		_, pendingTxsGot, err := conn1.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, want, string(pendingTxsGot))
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

func setupSubscriptionTest(t *testing.T, ctx context.Context) (*rpc.Handler, *fakeSyncer, *jsonrpc.Server) {
	t.Helper()

	log := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), &utils.Mainnet)
	syncer := newFakeSyncer()
	handler := rpc.New(chain, syncer, nil, "", log)

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

func TestSubscribeTxStatusAndUnsubscribe(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)

	syncer := newFakeSyncer()
	log, _ := utils.NewZapLogger(utils.INFO, false)
	handler := rpc.New(mockReader, syncer, nil, "", log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	// Technically, there's a race between goroutine above and the SubscribeNewHeads call down below.
	// Sleep for a moment just in case.
	time.Sleep(50 * time.Millisecond)

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		require.NoError(t, clientConn.Close())
	})

	txnHash := utils.HexToFelt(t, "0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f")
	txn := &core.DeployTransaction{TransactionHash: txnHash, Version: (*core.TransactionVersion)(&felt.Zero)}
	receipt := &core.TransactionReceipt{
		TransactionHash: txnHash,
		Reverted:        false,
	}

	mockReader.EXPECT().TransactionByHash(txnHash).Return(txn, nil).Times(1)
	mockReader.EXPECT().Receipt(txnHash).Return(receipt, nil, uint64(1), nil).Times(1)
	mockReader.EXPECT().TransactionByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound).AnyTimes()

	// Subscribe without setting the connection on the context.
	id, rpcErr := handler.SubscribeTxnStatus(ctx, felt.Zero, nil)
	require.Nil(t, id)
	require.Equal(t, jsonrpc.MethodNotFound, rpcErr.Code)

	// Subscribe correctly but for the unknown transaction
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr = handler.SubscribeTxnStatus(subCtx, felt.Zero, nil)
	require.Equal(t, rpc.ErrTxnHashNotFound, rpcErr)
	require.Nil(t, id)

	// Subscribe correctly for the known transaction
	subCtx = context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr = handler.SubscribeTxnStatus(subCtx, *txnHash, nil)
	require.Nil(t, rpcErr)

	// Receive a block header.
	time.Sleep(100 * time.Millisecond)
	got := make([]byte, 0, 300)
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, "", string(got))

	// Unsubscribe without setting the connection on the context.
	ok, rpcErr := handler.Unsubscribe(ctx, id.ID)
	require.Equal(t, jsonrpc.MethodNotFound, rpcErr.Code)
	require.False(t, ok)

	// Unsubscribe on correct connection with the incorrect id.
	ok, rpcErr = handler.Unsubscribe(subCtx, id.ID+1)
	require.Equal(t, rpc.ErrSubscriptionNotFound, rpcErr)
	require.False(t, ok)

	// Unsubscribe on incorrect connection with the correct id.
	subCtx = context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{})
	ok, rpcErr = handler.Unsubscribe(subCtx, id.ID)
	require.Equal(t, rpc.ErrSubscriptionNotFound, rpcErr)
	require.False(t, ok)

	// Unsubscribe on correct connection with the correct id.
	subCtx = context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	ok, rpcErr = handler.Unsubscribe(subCtx, id.ID)
	require.Nil(t, rpcErr)
	require.True(t, ok)
}

func TestSubscribeTxStatusAndUnsubscribeSimple(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)

	syncer := newFakeSyncer()
	log, _ := utils.NewZapLogger(utils.INFO, false)
	handler := rpc.New(mockReader, syncer, nil, "", log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	// Technically, there's a race between goroutine above and the SubscribeNewHeads call down below.
	// Sleep for a moment just in case.
	time.Sleep(50 * time.Millisecond)

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		require.NoError(t, clientConn.Close())
	})

	txnHash := utils.HexToFelt(t, "0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f")
	txn := &core.DeployTransaction{TransactionHash: txnHash, Version: (*core.TransactionVersion)(&felt.Zero)}
	receipt := &core.TransactionReceipt{
		TransactionHash: txnHash,
		Reverted:        false,
	}

	mockReader.EXPECT().TransactionByHash(txnHash).Return(txn, nil).Times(1)
	mockReader.EXPECT().Receipt(txnHash).Return(receipt, nil, uint64(1), nil).Times(1)
	mockReader.EXPECT().TransactionByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound).AnyTimes()

	// Subscribe correctly for the known transaction
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr := handler.SubscribeTxnStatus(subCtx, *txnHash, nil)
	require.Nil(t, rpcErr)

	// Receive a block header.
	time.Sleep(100 * time.Millisecond)
	got := make([]byte, 0, 300)
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, "", string(got))

	// Unsubscribe on correct connection with the correct id.
	subCtx = context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	ok, rpcErr := handler.Unsubscribe(subCtx, id.ID)
	require.Nil(t, rpcErr)
	require.True(t, ok)
}
