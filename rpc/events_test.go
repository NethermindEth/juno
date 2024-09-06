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
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
			stored, err := chain.StorePending(&blockchain.Pending{
				Block:       b,
				StateUpdate: s,
			})
			require.True(t, stored)
			require.NoError(t, err)
		}
	}

	handler := rpc.New(chain, nil, nil, "", utils.NewNopZapLogger(), nil)
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

func TestSubscribeNewHeadsAndUnsubscribe(t *testing.T) {
	t.Skip() // todo: turn this hacky test back on
	t.Parallel()
	log := utils.NewNopZapLogger()
	n := utils.Ptr(utils.Mainnet)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), n)
	syncer := sync.New(chain, gw, log, 0, false)
	handler := rpc.New(chain, syncer, nil, "", log, nil)

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
	id, rpcErr := handler.Subscribe(ctx, rpc.EventNewBlocks, false)
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
	id, rpcErr = handler.Subscribe(subCtx, rpc.EventNewBlocks, true)
	require.Nil(t, rpcErr)

	// Sync the block we reverted above.
	syncCtx, syncCancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()

	// Receive a block.
	want := `{"jsonrpc":"2.0","method":"juno_subscription","params":{"result":{"status":"ACCEPTED_ON_L2","block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":"","transactions":[{"transaction_hash":"0x723b57825c177d66fdc1ee1b7d22bd937503cd66808edf87294e88ee26601b6","type":"DEPLOY","version":"0x0","contract_address_salt":"0x3cec13aab076764c273a75acac9ebdbadfa1c45eca9777ff3090c84fa62aff3","class_hash":"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8","constructor_calldata":["0x772c29fae85f8321bb38c9c3f6edb0957379abedc75c17f32bcef4e9657911a","0x6d4ca0f72b553f5338a95625782a939a49b98f82f449c20f49b42ec60ed891c"]},{"transaction_hash":"0x4e10133a1ce9255236282b0c060e0054f3fe9c24387e047d6a2dd65febc7ab3","type":"DEPLOY","version":"0x0","contract_address_salt":"0x2a38ec8dc71fcbc19edea67ae77989f4bfb46ef17443aecdbe5a9546e3830d","class_hash":"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8","constructor_calldata":["0x4f2c206f3f2f1380beeb9fe4302900701e1cb48b9b33cbe1a84a175d7ce8b50","0x2a614ae71faa2bcdacc5fd66965429c57c4520e38ebc6344f7cf2e78b21bd2f"]},{"transaction_hash":"0x5a8629d7852d3c8f4fda51d83b48cc8b2184763c46383419c1beeadaea1e66e","type":"DEPLOY","version":"0x0","contract_address_salt":"0x23a93d3a3463ac1539852fcb9dbf58ed9581e4abbb4a828889768fbbbdb9bcd","class_hash":"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8","constructor_calldata":["0x7f93985c1baa5bd9b2200dd2151821bd90abb87186d0be295d7d4b9bc8ca41f","0x127cd00a078199381403a33d315061123ce246c8e5f19aa7f66391a9d3bf7c6"]},{"transaction_hash":"0x2e530fe2f39ba92380de33cfca060f68c2f50b8af954dae7370c97bf97e1e55","type":"INVOKE","version":"0x0","max_fee":"0x0","contract_address":"0x2d6c9569dea5f18628f1ef7c15978ee3093d2d3eec3b893aac08004e678ead3","signature":[],"calldata":["0xdaee7b1ac98d5d3fa7cf5dcfa0dd5f47dc8728fc"],"entry_point_selector":"0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6"},{"transaction_hash":"0x7f3166343d5aa5511582fcc8ad0a16bfb0124e3874085529ce010e2173fb699","type":"DEPLOY","version":"0x0","contract_address_salt":"0x8132d5429d1cf0ead19827b55be870842dc9bcb69892f9ceaa7615c36e0a5a","class_hash":"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8","constructor_calldata":["0x56c060e7902b3d4ec5a327f1c6e083497e586937db00af37fe803025955678f","0x75495b43f53bd4b9c9179db113626af7b335be5744d68c6552e3d36a16a747c"]},{"transaction_hash":"0x2c68262e46df9ab5144743869d828b88753805ea1d8e6f3145351b7f04b53e6","type":"INVOKE","version":"0x0","max_fee":"0x0","contract_address":"0x5790719f16afe1450b67a92461db7d0e36298d6a5f8bab4f7fd282050e02f4f","signature":[],"calldata":["0xd2b87a5bcea9d58af40dfdddfcc2edf66b3c9c8f"],"entry_point_selector":"0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6"}]},"subscription":%d}}`
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
	t.Parallel()
	log := utils.NewNopZapLogger()
	n := utils.Ptr(utils.Mainnet)
	feederClient := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(feederClient)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), n)
	syncer := sync.New(chain, gw, log, 0, false)
	handler := rpc.New(chain, syncer, nil, "", log, nil)
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
		Name:    "juno_subscribe",
		Params:  []jsonrpc.Parameter{{Name: "event"}, {Name: "withTxs", Optional: true}},
		Handler: handler.Subscribe,
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

	subscribeMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"juno_subscribe","params":["newBlocks",false]}`)

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

	// Receive a block.
	want = `{"jsonrpc":"2.0","method":"juno_subscription","params":{"result":{"status":"ACCEPTED_ON_L2","block_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","parent_hash":"0x2a70fb03fe363a2d6be843343a1d81ce6abeda1e9bd5cc6ad8fa9f45e30fdeb","block_number":2,"new_root":"0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9","timestamp":1637084470,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":"","transactions":["0x723b57825c177d66fdc1ee1b7d22bd937503cd66808edf87294e88ee26601b6","0x4e10133a1ce9255236282b0c060e0054f3fe9c24387e047d6a2dd65febc7ab3","0x5a8629d7852d3c8f4fda51d83b48cc8b2184763c46383419c1beeadaea1e66e","0x2e530fe2f39ba92380de33cfca060f68c2f50b8af954dae7370c97bf97e1e55","0x7f3166343d5aa5511582fcc8ad0a16bfb0124e3874085529ce010e2173fb699","0x2c68262e46df9ab5144743869d828b88753805ea1d8e6f3145351b7f04b53e6"]},"subscription":%d}}`
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

func TestSubscribePendingHeads(t *testing.T) {
	log := utils.NewNopZapLogger()
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), network)
	syncer := sync.New(chain, gw, log, time.Nanosecond, false)
	handler := rpc.New(chain, syncer, nil, "", log, nil)
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

	// Subscribe.
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr := handler.Subscribe(subCtx, rpc.EventPendingBlocks, false)
	require.Nil(t, rpcErr)

	// Sync blocks.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()

	// Receive a block header.
	want := `{"jsonrpc":"2.0","method":"juno_subscription","params":{"result":{"parent_hash":"0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6","timestamp":1637091683,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":"","transactions":["0x69385371f5725ae56843f44ecceba2fbf02cd7c96f0d32e5b4a79f9511cf6d4","0x586dd5935b33f89bd83a56a132b06abbd7264617d6f8a3bafd6e6d5ddecfd70","0x7891e4941527084a144b80963074070aa0fd0632e3c0a62de94dbdfdfc99fd","0x6bb3031050c160045af32d0fb36ad54909453ff0177e5822d0827538509f3a0","0x50f85bfe057cd882b6a05d9bd7b1e37b1e8204cd2aed5fbe7bb9a46f2525462","0x2a74daca88a03a98a408ee51b9ff4f871855610b03bf9331a1b3b502b5384f","0x1866427f42c7cb6b6addc616af7c48fca2b2b47ddba47d8add70757e3684284","0x33f4e8efb56694297e29df6f0a70e2032620c1dc73afa443d2d8810f9a88086","0x563c537ae133895cc5dd2d4796ddbf55c762dbb06c4c67a1afc7620bc877257","0x416cd1eb3a5c0651f7edeea5a7c0f29fa40c3b486a52f97a360ea868c47f760","0x132326af6e67634bad11d08ff28247379ffeceba7f301b78a2b62a733d2075","0x4a2b7d763c4f89fb3a5bbf5d0dfeec5eaa62f4be5b3d7c9a3749eeb095d4125","0xc5803408d2996eb2b33ace6fe526ae9c5c9a2cbd0c29077d471cec31fefccf","0x2d7d6248c0fb9da2bd9bbcc8e12a1aaffc92636e26b28482fe95e270b1405e2","0x388567e40f75583811d8c9ffd03b333b181a0ee17ebab8875547e685fca8eed","0x593fcc8d4515e35d768c38518a815b13663ef3a679cdfcfb8ceb5e5a2cb853c","0x697f855111c4b8c6ccb628e0e9e07023ef0fdbfcd30b74350ec1de94a52ec52","0x71645719e9bb685681d736b529d9c3905742a1c1b83c31017cf30101b089659","0x1178a94ffb3a42998585ecc306a9cee3c8ca327359595b9eedcb3bbecb746f4","0x6f6c19248456126cd82b5b322b2ddbfa0ea68b47b345ad90513687decc11de4","0x64067d3fcb100026e378290041b6c4349cbe3fac5d1202387bc556054924c4c","0x31f3fb51405f05c4cca53f67ae552aa367417399f4005204ba6b52d7b79d0ca","0xaa2b6c48e4452c070bca64d31728e832d53807ad622a0f6a8c4598b9e750bf","0x26aa67c759a560155d5b4651dae8ca4fdb7bf90c6595dc40ff54d44c57a90e8","0x1b652dafbe00c032253ae810e469d0d118e54f2a580340ef1b2e08065e0911b","0x297778648b7ad25b7cc6ed460585450bf14f42153445717d2b606280f2aaef2","0x6c96d5dbb61affebcaae37f9b426a7545272bf29183a77dd0124155932e7a14","0x2d1f71c1cd88832be28e302d55c7f4e98c4733d71640174620b3e55df6dd1b1","0x4a80a947ab81840cf3aef27c78c69603314b6ad670f1820d0105f5488a8820","0x90c26cab9df6cde417f2b884d526e76a33868c3b513d139fffb3b966017769","0x7f4249ef834c586b75855176bb156411f59f7fa572834cb9e2f231efe054224","0x69d1a71ce1f965ecf5d0862b228e852fe95fd713a28b1487de2a8d78ae0a410","0x146535794027a7b45d3d2831087cd0d1c461e237f64645ddb1c21c23df7972e","0x79c6fe04996648a8d0620094d35d8929b0d8f8ac1007b4ee65bdb0fd778f530","0x3c28efbc632f0ece25dc30aa2c5035f9aa907e43a42103609b9aded29fde516","0x4db5b5ebf18b66f7c1badada7f73acad6991aa10a50b3542fcfe782075fc2c8","0x160d07b065887fec1f898405d12874b742c553d8dfc52e6dc5a8667b4d05e63"]},"subscription":%d}}`
	want = fmt.Sprintf(want, id)
	got := make([]byte, len(want))
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

func TestSubscribeL1Heads(t *testing.T) {
	log := utils.NewNopZapLogger()
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chain := blockchain.New(pebble.NewMemTest(t), network)
	syncer := sync.New(chain, gw, log, 0, false)
	ctrl := gomock.NewController(t)

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		require.NoError(t, clientConn.Close())
	})

	// If there is no L1 node, we shouldn't be able to subscribe to l1Blocks.
	handlerWithoutL1 := rpc.New(chain, syncer, nil, "", log, nil)
	subCtx := context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	_, rpcErr := handlerWithoutL1.Subscribe(subCtx, rpc.EventL1Blocks, false)
	require.NotNil(t, rpcErr)
	require.Equal(t, rpcErr.Code, jsonrpc.InternalError)
	require.Contains(t, rpcErr.Data, "subscription event not supported")

	// If there is an L1 node, we should be able to subscribe to l1Blocks.
	masterFeed := feed.New[*core.L1Head]()
	l1Reader := mocks.NewL1Reader(ctrl)
	l1Reader.
		EXPECT().
		SubscribeL1Heads().
		Return(l1.L1HeadSubscription{Subscription: masterFeed.Subscribe()}).
		Times(1)

	handler := rpc.New(chain, syncer, nil, "", log, l1Reader)
	go func() {
		require.NoError(t, handler.Run(ctx))
	}()
	// Technically, there's a race between goroutine above and the SubscribeNewHeads call down below.
	// Sleep for a moment just in case.
	time.Sleep(50 * time.Millisecond)

	// Subscribe.
	subCtx = context.WithValue(ctx, jsonrpc.ConnKey{}, &fakeConn{w: serverConn})
	id, rpcErr := handler.Subscribe(subCtx, rpc.EventL1Blocks, false)
	require.Nil(t, rpcErr)

	// Sync blocks.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	require.NoError(t, syncer.Run(syncCtx))
	syncCancel()
	masterFeed.Send(&core.L1Head{
		BlockNumber: 0,
		BlockHash:   new(felt.Felt),
		StateRoot:   new(felt.Felt),
	})

	// Receive a block header.
	want := `{"jsonrpc":"2.0","method":"juno_subscription","params":{"result":{"status":"ACCEPTED_ON_L1","block_hash":"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943","parent_hash":"0x0","block_number":0,"new_root":"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6","timestamp":1637069048,"sequencer_address":"0x0","l1_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_data_gas_price":{"price_in_fri":"0x0","price_in_wei":"0x0"},"l1_da_mode":"CALLDATA","starknet_version":"","transactions":["0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75","0x12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1","0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624","0x1c924916a84ef42a3d25d29c5d1085fe212de04feadc6e88d4c7a6e5b9039bf","0xa66c346e273cc49510ef2e1620a1a7922135cb86ab227b86e0afd12243bd90","0x5c71675616b49fb9d16cac8beaaa65f62dc5a532e92785055c15c825166dbbf","0x60e05c41a6622592a2e2eff90a9f2e495296a3be9596e7bc4dfbafce00d7a6a","0x5634f2847140263ba59480ad4781dacc9991d0365145489b27a198ebed2f969","0xb049c384cf75174150a2540835cc2abdcca1d3a3750298a1741a621983e35a","0x227f3d9d5ce6680bdf2991576c1a90aca8184ca26055bae92d16c58e3e13340","0x376ff82431b52ca1fbc4942de80bc1b01d8e5cd1eeab5a277b601b510f2cab2","0x25f20c74821d84f62989a71fceef08c967837b63bae31b279a11343f10d874a","0x2d10272a8ba726793fd15aa23a1e3c42447d7483ebb0b49df8b987590fe0055","0xb05ba5cd0b9e0464d2c1790ad93a159c6ef0594513758bca9111e74e4099d4","0x4d16393d940fb4a97f20b9034e2a5e954201fee827b2b5c6daa38ec272e7c9c","0x9e80672edd4927a79f5384e656416b066f8ef58238227ac0fcea01952b70b5","0x387b5b63e40d4426754895fe52adf668cf8fde2a02aa9b6d761873f31af3462","0x4f0cdff0d72fc758413a16db2bc7580dfec7889a8b921f0fe08641fa265e997"]},"subscription":%d}}`
	want = fmt.Sprintf(want, id)
	got := make([]byte, len(want))
	_, err := clientConn.Read(got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}
