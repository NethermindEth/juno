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
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

func TestEvents(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Goerli2)

	client := feeder.NewTestClient(t, &utils.Goerli2)
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
			require.Len(t, events.Events, 3)
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
			require.Len(t, events.Events, 2)
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
		key := utils.HexToFelt(t, "0x3774b0545aabb37c45c1eddc6a7dae57de498aae6d5e3589e362d4b4323a533")

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
				utils.HexToFelt(t, "0x2ee9bf3da86f3715e8a20429feed8e37fef58004ee5cf52baf2d8fc0d94c9c8"),
				utils.HexToFelt(t, "0x2ee9bf3da86f3715e8a20429feed8e37fef58004ee5cf52baf2d8fc0d94c9c8"),
			}, events.Events[0].Data)
			require.Equal(t, uint64(5), *events.Events[0].BlockNumber)
			require.Equal(t, utils.HexToFelt(t, "0x3b43b334f46b921938854ba85ffc890c1b1321f8fd69e7b2961b18b4260de14"), events.Events[0].BlockHash)
			require.Equal(t, utils.HexToFelt(t, "0x6d1431d875ba082365b888c1651e026012a94172b04589c91c2adeb6c1b7ace"), events.Events[0].TransactionHash)
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
		key := utils.HexToFelt(t, "0x3774b0545aabb37c45c1eddc6a7dae57de498aae6d5e3589e362d4b4323a533")
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
		require.Len(t, events.Events, 1)
		require.Empty(t, events.ContinuationToken)

		assert.Nil(t, events.Events[0].BlockHash)
		assert.Nil(t, events.Events[0].BlockNumber)
		assert.Equal(t, utils.HexToFelt(t, "0x5fe34d6903420e489b6faa8804c7a1af311446934bac1ba1e79b53cee61756c"), events.Events[0].TransactionHash)
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

func TestSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Skip()
	t.Parallel()
	log := utils.NewNopZapLogger()
	network := utils.Mainnet
	client := feeder.NewTestClient(t, &network)
	gw := adaptfeeder.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), &network)
	syncer := sync.New(chain, gw, log, 0, false)
	handler := rpc.New(chain, syncer, nil, "", log)

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
	id, rpcErr := handler.SubscribeNewHeads(ctx)
	require.Zero(t, id)
	require.Equal(t, jsonrpc.MethodNotFound, rpcErr.Code)

	// Sync blocks and then revert head.
	// This is a super hacky way to deterministically receive a single block on the subscription.
	// It would be nicer if we could tell the synchronizer to exit after a certain block height, but, alas, we can't do that.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()
	// This is technically an unsafe thing to do. We're modifying the synchronizer's blockchain while it is owned by the synchronizer.
	// But it works.
	require.NoError(t, chain.RevertHead())

	// Subscribe.
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr = handler.SubscribeNewHeads(subCtx)
	require.Nil(t, rpcErr)

	// Sync the block we reverted above.
	syncCtx, syncCancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()

	// Receive a block header.
	want := `{"jsonrpc":"2.0","method":"juno_subscribeNewHeads","params":{"result":{"block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"starknet_version":""},"subscription":%d}}`
	want = fmt.Sprintf(want, id)
	got := make([]byte, len(want))
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

	// Unsubscribe without setting the connection on the context.
	ok, rpcErr := handler.Unsubscribe(ctx, id)
	require.Equal(t, jsonrpc.MethodNotFound, rpcErr.Code)
	require.False(t, ok)

	// Unsubscribe on correct connection with the incorrect id.
	ok, rpcErr = handler.Unsubscribe(subCtx, id+1)
	require.Equal(t, rpc.ErrSubscriptionNotFound, rpcErr)
	require.False(t, ok)

	// Unsubscribe on incorrect connection with the correct id.
	subCtx = context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{})
	ok, rpcErr = handler.Unsubscribe(subCtx, id)
	require.Equal(t, rpc.ErrSubscriptionNotFound, rpcErr)
	require.False(t, ok)

	// Unsubscribe on correct connection with the correct id.
	subCtx = context.WithValue(context.Background(), jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	ok, rpcErr = handler.Unsubscribe(subCtx, id)
	require.Nil(t, rpcErr)
	require.True(t, ok)
}

func TestMultipleSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Skip()
	t.Parallel()
	log := utils.NewNopZapLogger()
	network := utils.Mainnet
	feederClient := feeder.NewTestClient(t, &network)
	gw := adaptfeeder.New(feederClient)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), &network)
	syncer := sync.New(chain, gw, log, 0, false)
	handler := rpc.New(chain, syncer, nil, "", log)
	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	// Technically, there's a race between goroutine above and the SubscribeNewHeads call down below.
	// Sleep for a moment just in case.
	time.Sleep(50 * time.Millisecond)

	// Sync blocks and then revert head.
	// This is a super hacky way to deterministically receive a single block on the subscription.
	// It would be nicer if we could tell the synchronizer to exit after a certain block height, but, alas, we can't do that.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()
	// This is technically an unsafe thing to do. We're modifying the synchronizer's blockchain while it is owned by the synchronizer.
	// But it works.
	require.NoError(t, chain.RevertHead())

	server := jsonrpc.NewServer(1, log)
	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name:    "juno_subscribeNewHeads",
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

	subscribeMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"juno_subscribeNewHeads"}`)

	firstID := uint64(1)
	secondID := uint64(2)
	handler.WithIDGen(func() uint64 { return firstID })
	require.NoError(t, conn1.Write(ctx, websocket.MessageText, subscribeMsg))

	want := `{"jsonrpc":"2.0","result":%d,"id":1}`
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

	// Now we're subscribed. Sync the block we reverted above.
	syncCtx, syncCancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()

	// Receive a block header.
	want = `{"jsonrpc":"2.0","method":"juno_subscribeNewHeads","params":{"result":{"block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"starknet_version":""},"subscription":%d}}`
	firstWant = fmt.Sprintf(want, firstID)
	_, firstGot, err = conn1.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, firstWant, string(firstGot))
	secondWant = fmt.Sprintf(want, secondID)
	_, secondGot, err = conn2.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, secondWant, string(secondGot))

	// Unsubscribe
	unsubMsg := `{"jsonrpc":"2.0","id":1,"method":"juno_unsubscribe","params":[%d]}`
	require.NoError(t, conn1.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, firstID))))
	require.NoError(t, conn2.Write(ctx, websocket.MessageBinary, []byte(fmt.Sprintf(unsubMsg, secondID))))
}
