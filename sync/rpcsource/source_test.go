package rpcsource_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/adaptfeeder"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/rpc"
	"github.com/NethermindEth/juno/clients/rpc/testutils"
	"github.com/NethermindEth/juno/clients/timeout"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/sync/rpcsource"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	syncTimeout = 30 * time.Second

	mainnetChainHeight = 3
	mainnetClassHash   = "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"
)

func newMainnetChain(t *testing.T) *blockchain.Blockchain {
	t.Helper()
	return testutils.NewChainFor(t, &networks.Mainnet, mainnetChainHeight, mainnetClassHash)
}

func newChainOn(database db.KeyValueStore, network *networks.Network) *blockchain.Blockchain {
	return blockchain.New(database, network, blockchain.WithNewState(statetestutils.UseNewState()))
}

func newLocalChain(t *testing.T, network *networks.Network) *blockchain.Blockchain {
	t.Helper()
	return newChainOn(memory.New(), network)
}

func newPrunedChain(
	t *testing.T,
	network *networks.Network,
	height int,
	prunedBelow uint64,
) *blockchain.Blockchain {
	t.Helper()
	database := memory.New()
	chain := newChainOn(database, network)
	storeBlocks(t, chain, newGateway(t, network), height)

	_, _, err := pruner.PruneUpto(t.Context(), database, prunedBelow, db.Megabyte)
	require.NoError(t, err)

	return chain
}

func newGateway(t *testing.T, network *networks.Network) *adaptfeeder.Feeder {
	t.Helper()
	return adaptfeeder.New(feeder.NewTestClient(t, network))
}

func storeBlocks(
	t *testing.T,
	chain *blockchain.Blockchain,
	gateway *adaptfeeder.Feeder,
	count int,
) {
	t.Helper()
	feederSource := sync.NewFeederGatewayDataSource(chain, gateway)
	for number := range uint64(count) {
		committed, err := feederSource.BlockByNumber(t.Context(), number)
		require.NoError(t, err)
		require.NoError(t, chain.Store(
			committed.Block,
			&core.BlockCommitments{},
			committed.StateUpdate,
			committed.NewClasses,
			core.EventsBloom(committed.Block.Receipts),
		))
	}
}

func newClientAt(t *testing.T, serverURL *url.URL) *rpc.Client {
	t.Helper()
	client, err := rpc.New(
		t.Context(),
		serverURL,
		timeout.New([]time.Duration{time.Second}, true),
		log.NewNopZapLogger(),
		rpc.WithMinWait(time.Millisecond),
		rpc.WithMaxWait(time.Millisecond),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func newClient(t *testing.T, handler http.Handler) *rpc.Client {
	t.Helper()
	return newClientAt(t, testutils.Listen(t, handler))
}

func newUnreachableClient(t *testing.T) *rpc.Client {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := newClientAt(t, serverURL)
	server.Close()
	return client
}

func newSource(t *testing.T, chain *blockchain.Blockchain, client *rpc.Client) *rpcsource.Source {
	t.Helper()
	return rpcsource.New(
		chain, client, log.NewNopZapLogger(), rpcsource.WithErrorPause(time.Millisecond),
	)
}

func classHashes(classes map[felt.Felt]core.ClassDefinition) []felt.Felt {
	hashes := make([]felt.Felt, 0, len(classes))
	for hash := range classes {
		hashes = append(hashes, hash)
	}
	return hashes
}

func TestCheckRemote(t *testing.T) {
	remote := testutils.NewChain(t)
	const prunedBelow = 2
	prunedRemote := newPrunedChain(t, &networks.Mainnet, mainnetChainHeight, prunedBelow)

	tests := []struct {
		name        string
		network     *networks.Network
		localBlocks int
		client      func(t *testing.T) *rpc.Client
		wantErr     string
	}{
		{
			name:    "node on the same network",
			network: &networks.Sepolia,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, remote))
			},
		},
		{
			name:    "node on another network",
			network: &networks.Mainnet,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, remote))
			},
			wantErr: fmt.Sprintf(
				"RPC sync node serves chain id %s, want %s (%s) for network %s",
				networks.Sepolia.L2ChainIDFelt(),
				networks.Mainnet.L2ChainIDFelt(),
				networks.Mainnet.L2ChainID,
				networks.Mainnet.Name,
			),
		},
		{
			name:    "node serving an older spec",
			network: &networks.Sepolia,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.ServeV0_9(t, remote))
			},
			wantErr: "RPC sync node serves spec version 0.9.0, want 0.10; " +
				"point --rpc-sync-url at the node's v0_10 path",
		},
		{
			name:    "unreachable node",
			network: &networks.Sepolia,
			client:  newUnreachableClient,
			wantErr: "querying the RPC sync node chain id",
		},
		{
			name:    "node that pruned the next block",
			network: &networks.Mainnet,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, prunedRemote))
			},
			wantErr: "RPC sync node has pruned block 0, which this node needs next; " +
				"point --rpc-sync-url at an archive node or start from a recent database snapshot",
		},
		{
			name:        "pruned node that kept the next block",
			network:     &networks.Mainnet,
			localBlocks: prunedBelow,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, prunedRemote))
			},
		},
		{
			name:        "node at this node's height",
			network:     &networks.Mainnet,
			localBlocks: mainnetChainHeight,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, prunedRemote))
			},
		},
		{
			name:    "node with no blocks",
			network: &networks.Sepolia,
			client: func(t *testing.T) *rpc.Client {
				return newClient(t, testutils.Serve(t, newLocalChain(t, &networks.Sepolia)))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := newLocalChain(t, tt.network)
			storeBlocks(t, chain, newGateway(t, tt.network), tt.localBlocks)
			source := newSource(t, chain, tt.client(t))

			err := source.CheckRemote(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBlockByNumber(t *testing.T) {
	sepolia := testutils.NewChain(t)
	mainnet := newMainnetChain(t)

	tests := []struct {
		name        string
		remote      *blockchain.Blockchain
		localBlocks int
		blockNumber uint64
		wantClasses []string
	}{
		{
			name:        "genesis declaring classes into an empty database",
			remote:      sepolia,
			blockNumber: 0,
			wantClasses: testutils.GenesisClassHashes,
		},
		{
			name:        "block deploying a class the database lacks",
			remote:      mainnet,
			blockNumber: 2,
			wantClasses: []string{mainnetClassHash},
		},
		{
			name:        "block deploying a class the database holds",
			remote:      mainnet,
			localBlocks: 1,
			blockNumber: 1,
			wantClasses: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := newGateway(t, tt.remote.Network())
			chain := newLocalChain(t, tt.remote.Network())
			storeBlocks(t, chain, gateway, tt.localBlocks)
			source := newSource(t, chain, newClient(t, testutils.Serve(t, tt.remote)))
			expected, err := sync.NewFeederGatewayDataSource(chain, gateway).
				BlockByNumber(t.Context(), tt.blockNumber)
			require.NoError(t, err)

			got, err := source.BlockByNumber(t.Context(), tt.blockNumber)

			require.NoError(t, err)
			assert.Equal(t, expected.Block.Hash, got.Block.Hash)
			assert.Equal(t, expected.Block.Number, got.Block.Number)
			assert.Len(t, got.Block.Transactions, len(expected.Block.Transactions))
			assert.Len(t, got.Block.Receipts, len(expected.Block.Receipts))
			assert.Equal(t, expected.StateUpdate, got.StateUpdate)
			wantHashes := make([]felt.Felt, 0, len(tt.wantClasses))
			for _, hash := range tt.wantClasses {
				wantHashes = append(wantHashes, *felt.NewUnsafeFromString[felt.Felt](hash))
			}
			assert.ElementsMatch(t, wantHashes, classHashes(expected.NewClasses))
			assert.ElementsMatch(t, wantHashes, classHashes(got.NewClasses))
			for hash, class := range got.NewClasses {
				gotHash, err := class.Hash()
				require.NoError(t, err)
				expectedHash, err := expected.NewClasses[hash].Hash()
				require.NoError(t, err)
				assert.Equal(t, expectedHash, gotHash)
			}
			assert.NotNil(t, got.Persisted)
		})
	}
}

func TestBlockByNumberFetchesClassDefinitions(t *testing.T) {
	remote := testutils.NewGenesisChain(
		t, &networks.Sepolia, testutils.DeprecatedClassHash, testutils.SierraClassHash,
	)
	client := newClient(t, testutils.Serve(t, remote))
	source := newSource(t, newLocalChain(t, remote.Network()), client)

	got, err := source.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	tests := []struct {
		name      string
		classHash string
		wantType  core.ClassDefinition
	}{
		{
			name:      "deprecated class",
			classHash: testutils.DeprecatedClassHash,
			wantType:  &core.DeprecatedCairoClass{},
		},
		{
			name:      "sierra class with its compiled form",
			classHash: testutils.SierraClassHash,
			wantType:  &core.SierraClass{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classHash := felt.NewUnsafeFromString[felt.Felt](tt.classHash)

			class := got.NewClasses[*classHash]

			require.IsType(t, tt.wantType, class)
			if sierra, ok := class.(*core.SierraClass); ok {
				assert.NotNil(t, sierra.Compiled)
			}
			computed, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, *classHash, computed)
		})
	}
}

func TestBlockByNumberPausesAfterError(t *testing.T) {
	remote := testutils.Serve(t, testutils.NewChain(t))
	const errorPause = 300 * time.Millisecond
	const cancelAfter = 50 * time.Millisecond

	tests := []struct {
		name           string
		client         func(t *testing.T) *rpc.Client
		ctx            func(t *testing.T) context.Context
		wantMinElapsed time.Duration
		wantMaxElapsed time.Duration
		wantLevel      zapcore.Level
		wantMessage    string
	}{
		{
			name:           "waits before reporting a missing block",
			client:         func(t *testing.T) *rpc.Client { return newClient(t, remote) },
			ctx:            func(t *testing.T) context.Context { return t.Context() },
			wantMinElapsed: errorPause,
			wantMaxElapsed: syncTimeout,
			wantLevel:      zapcore.DebugLevel,
			wantMessage:    "Block not available on the RPC sync node yet",
		},
		{
			name:           "waits before reporting an unreachable node",
			client:         newUnreachableClient,
			ctx:            func(t *testing.T) context.Context { return t.Context() },
			wantMinElapsed: errorPause,
			wantMaxElapsed: syncTimeout,
			wantLevel:      zapcore.WarnLevel,
			wantMessage:    "Failed fetching block from the RPC sync node",
		},
		{
			name:   "cancellation cuts the pause short",
			client: func(t *testing.T) *rpc.Client { return newClient(t, remote) },
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), cancelAfter)
				t.Cleanup(cancel)
				return ctx
			},
			wantMinElapsed: cancelAfter,
			wantMaxElapsed: errorPause,
			wantLevel:      zapcore.DebugLevel,
			wantMessage:    "Block not available on the RPC sync node yet",
		},
		{
			name:   "cancelled context skips the pause",
			client: func(t *testing.T) *rpc.Client { return newClient(t, remote) },
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantMaxElapsed: errorPause,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logCore, logs := observer.New(zapcore.DebugLevel)
			source := rpcsource.New(
				newLocalChain(t, &networks.Sepolia),
				tt.client(t),
				log.NewZapLoggerWithCore(logCore),
				rpcsource.WithErrorPause(errorPause),
			)
			start := time.Now()

			got, err := source.BlockByNumber(tt.ctx(t), testutils.MissingBlockNumber)

			elapsed := time.Since(start)
			require.Error(t, err)
			assert.Equal(t, sync.CommittedBlock{}, got)
			assert.GreaterOrEqual(t, elapsed, tt.wantMinElapsed)
			assert.Less(t, elapsed, tt.wantMaxElapsed)
			if tt.wantMessage == "" {
				assert.Zero(t, logs.Len())
				return
			}
			entries := logs.All()
			require.Len(t, entries, 1)
			assert.Equal(t, tt.wantLevel, entries[0].Level)
			assert.Equal(t, tt.wantMessage, entries[0].Message)
		})
	}
}

func TestBlockHeaderLatest(t *testing.T) {
	remote := testutils.NewChain(t)
	client := newClient(t, testutils.Serve(t, remote))
	source := newSource(t, newLocalChain(t, remote.Network()), client)
	remoteHead, err := remote.HeadsHeader()
	require.NoError(t, err)

	head, err := source.BlockHeaderLatest(t.Context())

	require.NoError(t, err)
	assert.Equal(t, &core.Header{Number: remoteHead.Number, Hash: remoteHead.Hash}, head)
}

func TestSyncFromRPCSource(t *testing.T) {
	remote := newMainnetChain(t)
	classHash := felt.NewUnsafeFromString[felt.Felt](mainnetClassHash)

	tests := []struct {
		name        string
		localBlocks int
	}{
		{name: "into an empty database"},
		{name: "into a database holding the genesis block", localBlocks: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := newGateway(t, remote.Network())
			chain := newLocalChain(t, remote.Network())
			storeBlocks(t, chain, gateway, tt.localBlocks)
			source := newSource(t, chain, newClient(t, testutils.Serve(t, remote)))
			ctx, cancel := context.WithTimeout(t.Context(), syncTimeout)
			t.Cleanup(cancel)
			synchronizer := sync.New(
				chain,
				source,
				gateway,
				log.NewNopZapLogger(),
				sync.WithPreConfirmed(preconfirmed.WithPollInterval(0)),
			).WithListener(&sync.SelectiveListener{
				OnSyncStepDoneCb: func(op string, blockNumber uint64, _ time.Duration) {
					if op == sync.OpStore && blockNumber == mainnetChainHeight-1 {
						cancel()
					}
				},
			})

			require.NoError(t, synchronizer.Run(ctx))

			require.ErrorIs(t, ctx.Err(), context.Canceled, "synchronizer never stored the remote head")
			for number := range uint64(mainnetChainHeight) {
				remoteBlock, err := remote.BlockByNumber(number)
				require.NoError(t, err)
				localBlock, err := chain.BlockByNumber(number)
				require.NoError(t, err)
				assert.Equal(t, remoteBlock.Hash, localBlock.Hash)
				remoteUpdate, err := remote.StateUpdateByNumber(number)
				require.NoError(t, err)
				localUpdate, err := chain.StateUpdateByNumber(number)
				require.NoError(t, err)
				assert.Equal(t, remoteUpdate, localUpdate)
			}
			state, closer, err := chain.HeadState()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, closer()) })
			_, err = state.Class(classHash)
			assert.NoError(t, err)
		})
	}
}
