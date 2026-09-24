// Package testutils serves feeder test data through Juno's own JSON-RPC stack.
package testutils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	junosync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/go-playground/validator/v10"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/require"
)

const (
	ChainHeight         = 5
	MissingBlockNumber  = 100
	DeprecatedClassHash = "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3"
	SierraClassHash     = "0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b"
)

var GenesisClassHashes = []string{
	DeprecatedClassHash,
	"0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
}

// No test data chain declares a Sierra class from genesis, so it is stored with block 0's
// deprecated ones.
func NewChain(t testing.TB) *blockchain.Blockchain {
	t.Helper()
	classHashes := append(slices.Clone(GenesisClassHashes), SierraClassHash)
	return NewChainFor(t, &networks.Sepolia, ChainHeight, classHashes...)
}

func NewChainFor(
	t testing.TB,
	network *networks.Network,
	height uint64,
	genesisClassHashes ...string,
) *blockchain.Blockchain {
	t.Helper()
	chain := blockchain.New(
		memory.New(),
		network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	gateway := adaptfeeder.New(feeder.NewTestClient(t, network))
	genesisClasses := fetchClasses(t, gateway, genesisClassHashes...)
	for number := range height {
		block, err := gateway.BlockByNumber(t.Context(), number)
		require.NoError(t, err)
		stateUpdate, err := gateway.StateUpdate(t.Context(), number)
		require.NoError(t, err)
		var newClasses map[felt.Felt]core.ClassDefinition
		if number == 0 {
			newClasses = genesisClasses
		}
		require.NoError(t, chain.Store(block, &core.BlockCommitments{}, stateUpdate, newClasses))
	}
	return chain
}

func Serve(t testing.TB, chain *blockchain.Blockchain) http.Handler {
	t.Helper()
	return serve(t, chain, (*rpc.Handler).MethodsV0_10, rpcv10.Validator())
}

func ServeV0_9(t testing.TB, chain *blockchain.Blockchain) http.Handler {
	t.Helper()
	return serve(t, chain, (*rpc.Handler).MethodsV0_9, rpcv9.Validator())
}

func serve(
	t testing.TB,
	chain *blockchain.Blockchain,
	methods func(*rpc.Handler) ([]jsonrpc.Method, string),
	validate *validator.Validate,
) http.Handler {
	t.Helper()
	logger := log.NewNopZapLogger()
	handler := rpc.New(chain, new(junosync.NoopSynchronizer), nil, "test", logger, chain.Network())
	server := jsonrpc.NewServer(pool.New(), logger).WithValidator(validate)
	versionMethods, _ := methods(handler)
	require.NoError(t, server.RegisterMethods(versionMethods...))
	return jsonrpc.NewHTTP(server, logger)
}

func Listen(t testing.TB, handler http.Handler) *url.URL {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	return serverURL
}

func fetchClasses(
	t testing.TB,
	gateway *adaptfeeder.Feeder,
	hashes ...string,
) map[felt.Felt]core.ClassDefinition {
	t.Helper()
	classes := make(map[felt.Felt]core.ClassDefinition, len(hashes))
	for _, hash := range hashes {
		classHash := felt.NewUnsafeFromString[felt.Felt](hash)
		class, err := gateway.Class(t.Context(), classHash)
		require.NoError(t, err)
		classes[*classHash] = class
	}
	return classes
}
