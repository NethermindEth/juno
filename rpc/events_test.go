package rpc_test

import (
	"context"
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
	testResponse = `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`
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
	newHeads *feed.Feed[*core.Header]
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

func TestSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Parallel()

	chain := blockchain.New(pebble.NewMemTest(t), &utils.Mainnet)
	syncer := &fakeSyncer{newHeads: feed.New[*core.Header]()}
	handler := rpc.New(chain, syncer, nil, "", utils.NewNopZapLogger())

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

	// Subscribe without setting the connection on the context.
	id, rpcErr := handler.SubscribeNewHeads(ctx, nil)
	require.Zero(t, id)
	require.Equal(t, jsonrpc.MethodNotFound, rpcErr.Code)

	// Subscribe correctly.
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr = handler.SubscribeNewHeads(subCtx, nil)
	require.Nil(t, rpcErr)

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Receive a block header.
	want := fmt.Sprintf(testResponse, id.ID)
	got := make([]byte, len(want))
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

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

func TestMultipleSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Parallel()

	log := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), &utils.Mainnet)
	syncer := &fakeSyncer{newHeads: feed.New[*core.Header]()}
	handler := rpc.New(chain, syncer, nil, "", log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	// Technically, there's a race between goroutine above and the SubscribeNewHeads call down below.
	// Sleep for a moment just in case.
	time.Sleep(50 * time.Millisecond)

	server := jsonrpc.NewServer(1, log)
	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_subscribeNewHeads",
		Params:  []jsonrpc.Parameter{{Name: "block", Optional: true}},
		Handler: handler.SubscribeNewHeads,
	}, jsonrpc.Method{
		Name:    "juno_unsubscribe",
		Params:  []jsonrpc.Parameter{{Name: "id"}},
		Handler: handler.Unsubscribe,
	}))
	ws := jsonrpc.NewWebsocket(server, log)
	httpSrv := httptest.NewServer(ws)
	conn1, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)
	conn2, _, err := websocket.Dial(ctx, httpSrv.URL, nil)
	require.NoError(t, err)

	subscribeMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"starknet_subscribeNewHeads"}`)

	firstID := uint64(1)
	secondID := uint64(2)
	handler.WithIDGen(func() uint64 { return firstID })
	require.NoError(t, conn1.Write(ctx, websocket.MessageText, subscribeMsg))

	want := `{"jsonrpc":"2.0","result":{"subscription_id":%d},"id":1}`
	firstWant := fmt.Sprintf(want, firstID)
	_, firstGot, err := conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, firstWant, string(firstGot))

	handler.WithIDGen(func() uint64 { return secondID })
	require.NoError(t, conn2.Write(ctx, websocket.MessageText, subscribeMsg))
	secondWant := fmt.Sprintf(want, secondID)
	_, secondGot, err := conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, secondWant, string(secondGot))

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Receive a block header.
	firstWant = fmt.Sprintf(testResponse, firstID)
	_, firstGot, err = conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, firstWant, string(firstGot))
	secondWant = fmt.Sprintf(testResponse, secondID)
	_, secondGot, err = conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, secondWant, string(secondGot))

	// Unsubscribe
	unsubMsg := `{"jsonrpc":"2.0","id":1,"method":"juno_unsubscribe","params":[%d]}`
	require.NoError(t, conn1.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, firstID))))
	require.NoError(t, conn2.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, secondID))))
}

func TestSubscribeNewHeadsHistorical(t *testing.T) {
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
	syncer := &fakeSyncer{newHeads: feed.New[*core.Header]()}
	handler := rpc.New(chain, syncer, nil, "", utils.NewNopZapLogger())

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

	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})

	// Subscribe to a block that doesn't exist.
	id, rpcErr := handler.SubscribeNewHeads(subCtx, &rpc.BlockID{Number: 1025})
	require.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	require.Zero(t, id)

	// Subscribe to a block that exists.
	id, rpcErr = handler.SubscribeNewHeads(subCtx, &rpc.BlockID{Number: 0})
	require.Nil(t, rpcErr)
	require.NotZero(t, id)

	// Check block 0 content
	want := `{"jsonrpc":"2.0","method":"starknet_subscriptionNewHeads","params":{"result":{"block_hash":"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943","parent_hash":"0x0","block_number":0,"new_root":"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6","timestamp":1637069048,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":""},"subscription_id":%d}}`
	want = fmt.Sprintf(want, id.ID)
	got := make([]byte, len(want))
	_, err = clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

	// Simulate a new block
	syncer.newHeads.Send(testHeader(t))

	// Check new block content
	want = fmt.Sprintf(testResponse, id.ID)
	got = make([]byte, len(want))
	_, err = clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))
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
