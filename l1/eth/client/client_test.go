package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, srv *clienttest.TestServer) *client.Client {
	t.Helper()
	c, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

// Exactly one of result / rpcErr should be non-nil.
type methodResponse struct {
	result any
	rpcErr *clienttest.TestRPCError
}

// captureHandler's requests accessor is safe to call concurrently with the
// server goroutine: the handler runs on the ws read loop, not the test goroutine.
func captureHandler(
	t *testing.T,
	responses map[string]methodResponse,
) (srv *clienttest.TestServer, requests func() []clienttest.TestRequest) {
	t.Helper()
	srv = clienttest.NewTestServer(t)
	var mu sync.Mutex
	captured := make([]clienttest.TestRequest, 0)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		mu.Lock()
		captured = append(captured, req)
		mu.Unlock()
		if r, ok := responses[req.Method]; ok {
			return r.result, r.rpcErr
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: "method not found: " + req.Method}
	})
	return srv, func() []clienttest.TestRequest {
		mu.Lock()
		defer mu.Unlock()
		out := make([]clienttest.TestRequest, len(captured))
		copy(out, captured)
		return out
	}
}

func TestNew_WithLogger(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	c, err := client.New(t.Context(), srv.WSURL(),
		client.WithLogger(log.NewNopZapLogger()))
	require.NoError(t, err)
	t.Cleanup(c.Close)
}

func TestTestServer_URL(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	u := srv.URL()
	assert.Truef(t, strings.HasPrefix(u, "http://"),
		"URL() must return an http:// URL, got %q", u)
}

func TestNew_SchemeDispatch(t *testing.T) {
	cases := []struct {
		url     string
		wantErr string
	}{
		{"http://example.com", "unsupported url scheme"},
		{"https://example.com", "unsupported url scheme"},
		{"ipc:///tmp/geth.ipc", "unsupported url scheme"},
		{"file:///tmp/x", "unsupported url scheme"},
		{"::not-a-url", "parsing url"},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			_, err := client.New(t.Context(), c.url)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
		})
	}
}

func TestChainID_Success(t *testing.T) {
	srv, calls := captureHandler(t, map[string]methodResponse{
		"eth_chainId": {result: "0x539"},
	})
	cli := newTestClient(t, srv)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1337), id)
	require.Len(t, calls(), 1)
	assert.Equal(t, "eth_chainId", calls()[0].Method)
	assert.Empty(t, calls()[0].Params)
}

func TestChainID_LargeValue(t *testing.T) {
	// uint64 max + 1 must round-trip through *big.Int.
	const big65bit = "0x10000000000000000"
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_chainId": {result: big65bit},
	})
	cli := newTestClient(t, srv)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	want, _ := new(big.Int).SetString("10000000000000000", 16)
	assert.Equal(t, want, id)
}

func TestChainID_ServerError(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_chainId": {rpcErr: &clienttest.TestRPCError{Code: -32603, Message: "internal error"}},
	})
	cli := newTestClient(t, srv)

	_, err := cli.ChainID(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain ID")
	assert.Contains(t, err.Error(), "internal error")
}

func TestChainID_DecodeErrors(t *testing.T) {
	cases := []struct {
		name    string
		raw     any
		wantSub string
	}{
		{"non-string", 1, "decoding quantity"},
		{"signed", "0x-1", "sign prefix"},
		{"signed with zero", "0x-01", "sign prefix"},
		{"missing prefix", "abc", "missing 0x prefix"},
		{"no digits", "0x", "no digits"},
		{"leading zero", "0x01", "leading zero"},
		{"invalid hex", "0xZZ", "invalid hex"},
		{"oversized", "0x1" + strings.Repeat("0", 64), "too long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := captureHandler(t, map[string]methodResponse{
				"eth_chainId": {result: c.raw},
			})
			cli := newTestClient(t, srv)

			_, err := cli.ChainID(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantSub)
		})
	}
}

func TestBlockNumber_DecodeError(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_blockNumber": {result: "not-hex"},
	})
	cli := newTestClient(t, srv)

	_, err := cli.BlockNumber(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding quantity")
}

func TestFilterLogs_DecodeFailure(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getLogs": {result: map[string]any{"unexpected": "shape"}},
	})
	cli := newTestClient(t, srv)

	_, err := cli.FilterLogs(t.Context(), client.FilterQuery{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding logs")
}

func TestTransactionReceipt_DecodeFailure(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getTransactionReceipt": {result: map[string]any{"logs": "not-an-array"}},
	})
	cli := newTestClient(t, srv)

	_, err := cli.TransactionReceipt(t.Context(), eth.Hash{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding receipt")
}

func TestBlockNumber_Success(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_blockNumber": {result: "0x10"},
	})
	cli := newTestClient(t, srv)

	n, err := cli.BlockNumber(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(16), n)
}

func TestHeaderByNumber_Finalized(t *testing.T) {
	srv, calls := captureHandler(t, map[string]methodResponse{
		"eth_getBlockByNumber": {result: map[string]any{
			"number":     "0x539",
			"hash":       "0x" + zeroHex(64),
			"parentHash": "0x" + zeroHex(64),
		}},
	})
	cli := newTestClient(t, srv)

	h, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.NoError(t, err)
	assert.Equal(t, uint64(1337), uint64(h.Number))

	require.Len(t, calls(), 1)
	c := calls()[0]
	require.Len(t, c.Params, 2)
	assert.JSONEq(t, `"finalized"`, string(c.Params[0]))
	assert.JSONEq(t, `false`, string(c.Params[1]))
}

func TestHeaderByNumber_NotFound_NullResult(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getBlockByNumber": {result: nil},
	})
	cli := newTestClient(t, srv)

	_, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.ErrorIs(t, err, eth.ErrNotFound)
}

func TestHeaderByNumber_BadHeader(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getBlockByNumber": {result: map[string]any{"number": "not-hex"}},
	})
	cli := newTestClient(t, srv)

	_, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding header")
}

func TestTransactionReceipt_Success(t *testing.T) {
	srv, calls := captureHandler(t, map[string]methodResponse{
		"eth_getTransactionReceipt": {result: map[string]any{
			"logs": []any{
				map[string]any{
					"topics": []string{
						"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",
					},
					"data":        "0xdeadbeef",
					"blockNumber": "0x10",
					"removed":     false,
				},
			},
		}},
	})
	cli := newTestClient(t, srv)

	txHash := eth.HashFromString("0x" + repeatHex("ab", 32))
	r, err := cli.TransactionReceipt(t.Context(), txHash)
	require.NoError(t, err)
	require.Len(t, r.Logs, 1)
	assert.Equal(t, uint64(16), uint64(r.Logs[0].BlockNumber))
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, []byte(r.Logs[0].Data))

	require.Len(t, calls(), 1)
	c := calls()[0]
	require.Len(t, c.Params, 1)
	assert.JSONEq(t, `"`+txHash.Hex()+`"`, string(c.Params[0]))
}

func TestTransactionReceipt_NotFound(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getTransactionReceipt": {result: nil},
	})
	cli := newTestClient(t, srv)
	_, err := cli.TransactionReceipt(t.Context(), eth.Hash{})
	require.ErrorIs(t, err, eth.ErrNotFound)
}

func TestFilterLogs_Empty(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getLogs": {result: []any{}},
	})
	cli := newTestClient(t, srv)

	logs, err := cli.FilterLogs(t.Context(), client.FilterQuery{
		FromBlock: ptr(uint64(1)),
		ToBlock:   ptr(uint64(2)),
	})
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestFilterLogs_OneLog(t *testing.T) {
	const sigHash = "0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"
	srv, calls := captureHandler(t, map[string]methodResponse{
		"eth_getLogs": {result: []any{
			map[string]any{
				"topics":      []string{sigHash},
				"data":        "0x",
				"blockNumber": "0x100",
				"removed":     false,
			},
		}},
	})
	cli := newTestClient(t, srv)

	addr := eth.AddressFromString("0x000000000000000000000000000000000000beef")
	q := client.FilterQuery{
		FromBlock: ptr(uint64(1)),
		ToBlock:   ptr(uint64(1000)),
		Addresses: []eth.Address{addr},
		Topics:    [][]eth.Hash{{eth.HashFromString(sigHash)}},
	}
	logs, err := cli.FilterLogs(t.Context(), q)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, uint64(256), uint64(logs[0].BlockNumber))

	require.Len(t, calls(), 1)
	c := calls()[0]
	require.Len(t, c.Params, 1)
	var sentFilter struct {
		FromBlock string   `json:"fromBlock"`
		ToBlock   string   `json:"toBlock"`
		Address   []string `json:"address"`
		Topics    []any    `json:"topics"`
	}
	require.NoError(t, json.Unmarshal(c.Params[0], &sentFilter))
	assert.Equal(t, "0x1", sentFilter.FromBlock)
	assert.Equal(t, "0x3e8", sentFilter.ToBlock)
	require.Len(t, sentFilter.Address, 1)
	assert.Equal(t, addrHex(addr), sentFilter.Address[0])
	require.Len(t, sentFilter.Topics, 1)
	topicStr, ok := sentFilter.Topics[0].(string)
	require.True(t, ok, "topic[0] should be a string when only one hash")
	assert.Equal(t, sigHash, topicStr)
}

func TestFilterLogs_ServerError(t *testing.T) {
	srv, _ := captureHandler(t, map[string]methodResponse{
		"eth_getLogs": {rpcErr: &clienttest.TestRPCError{
			Code:    -32005,
			Message: "query returned more than 10000 results",
		}},
	})
	cli := newTestClient(t, srv)

	_, err := cli.FilterLogs(t.Context(), client.FilterQuery{
		FromBlock: ptr(uint64(1)),
		ToBlock:   ptr(uint64(1_000_000)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filtering logs")
	assert.Contains(t, err.Error(), "10000 results")
}

func TestMethods_ContextCancelled(t *testing.T) {
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		<-gate
		return nil, nil
	})
	cli := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := cli.ChainID(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "got: %v", err)
}

// --- helpers ---

func zeroHex(n int) string { return repeatHex("0", n) }
