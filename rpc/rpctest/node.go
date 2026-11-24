// Package rpctest starts in-process Juno nodes for RPC tests. A node has a real chain, the real
// VM and one JSON-RPC server for each RPC version, set up as the juno command does.
package rpctest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/rpc"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	// The chain stores classes and transactions as tagged CBOR.
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/require"
)

// Version is the URL path of the JSON-RPC server of an RPC version.
type Version string

// Version0_9 and Version0_10 select the servers of RPC v0.9 and v0.10.
const (
	Version0_9  Version = "/v0_9"
	Version0_10 Version = "/v0_10"
)

// The defaults of the juno command.
const (
	maxBatchElements      = 1000
	maxBatchResponseBytes = 64 * db.Megabyte
)

// Node is an in-process Juno node with one JSON-RPC server for each version.
type Node struct {
	Chain   *blockchain.Blockchain
	servers map[Version]http.Handler
}

// Response is a JSON-RPC response with the HTTP header of its HTTP response.
type Response struct {
	Result json.RawMessage
	Error  json.RawMessage
	Header http.Header
}

func chainInfo(network *networks.Network) *vm.ChainInfo {
	return &vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: networks.DefaultFeeTokenAddresses,
	}
}

func newVM(network *networks.Network, logger log.Logger) vm.VM {
	maxVMs := 3 * runtime.GOMAXPROCS(0)
	nodeVM := vm.New(chainInfo(network), false, logger)
	return node.NewThrottledVM(nodeVM, uint(maxVMs), uint64(2*maxVMs))
}

func newHandler(
	chain *blockchain.Blockchain, syncReader sync.Reader, network *networks.Network, logger log.Logger,
) *rpc.Handler {
	return rpc.New(chain, syncReader, newVM(network, logger), "rpctest", logger, network).
		WithCompiler(compiler.NewUnsafe()).
		WithCallMaxSteps(vm.DefaultMaxSteps).
		WithCallMaxGas(vm.DefaultMaxGas)
}

func newNode(t testing.TB, chain *blockchain.Blockchain, handler *rpc.Handler) *Node {
	t.Helper()
	logger := log.NewNopZapLogger()
	batchPool := pool.New().WithMaxGoroutines(2 * runtime.GOMAXPROCS(0))

	newServer := func(validator jsonrpc.Validator, methods []jsonrpc.Method) http.Handler {
		server := jsonrpc.NewServer(batchPool, logger).
			WithValidator(validator).
			WithMaxBatchElements(maxBatchElements).
			WithMaxBatchResponseBytes(maxBatchResponseBytes)
		require.NoError(t, server.RegisterMethods(methods...))
		return jsonrpc.NewHTTP(server, logger)
	}

	methodsV10, pathV10 := handler.MethodsV0_10()
	methodsV09, pathV09 := handler.MethodsV0_9()
	return &Node{
		Chain: chain,
		servers: map[Version]http.Handler{
			Version(pathV10): newServer(rpcv10.Validator(), methodsV10),
			Version(pathV09): newServer(rpcv9.Validator(), methodsV09),
		},
	}
}

// Post sends a request body to the server of the version and returns the HTTP response.
func (n *Node) Post(t testing.TB, v Version, body string) *http.Response {
	t.Helper()
	server, ok := n.servers[v]
	require.True(t, ok, "no server for version %s", v)

	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, string(v), strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	return recorder.Result()
}

// Call sends a request for the method with the params and returns the response.
func (n *Node) Call(t testing.TB, v Version, method, params string) Response {
	t.Helper()
	httpResponse := n.Post(t, v, `{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`)
	defer httpResponse.Body.Close()

	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	require.NoError(t, json.NewDecoder(httpResponse.Body).Decode(&response))
	require.Equal(t, "2.0", response.JSONRPC)
	require.JSONEq(t, "1", string(response.ID))
	return Response{Result: response.Result, Error: response.Error, Header: httpResponse.Header}
}

// Result sends a request for the method with the params and returns the result. The request must
// succeed.
func (n *Node) Result(t testing.TB, v Version, method, params string) json.RawMessage {
	t.Helper()
	response := n.Call(t, v, method, params)
	require.Empty(t, response.Error, "%s %s", method, params)
	return response.Result
}
