package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient dials the test server over WS and registers cleanup.
func newTestClient(t *testing.T, srv *client.TestServer) *client.Client {
	t.Helper()
	c, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

// captureHandler installs a TestHandler that records every request and
// dispatches by method to a per-method response function. methods not
// in the map fall through to a "method not found" error.
func captureHandler(t *testing.T, responses map[string]func(req client.TestRequest) (any, *client.TestRPCError)) (*client.TestServer, *[]client.TestRequest) {
	t.Helper()
	srv := client.NewTestServer(t)
	captured := make([]client.TestRequest, 0)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		captured = append(captured, req)
		if h, ok := responses[req.Method]; ok {
			return h(req)
		}
		return nil, &client.TestRPCError{Code: -32601, Message: "method not found: " + req.Method}
	})
	return srv, &captured
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
		{"::not-a-url", "parse url"},
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
	srv, calls := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_chainId": func(req client.TestRequest) (any, *client.TestRPCError) {
			return "0x539", nil
		},
	})
	cli := newTestClient(t, srv)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1337), id)
	require.Len(t, *calls, 1)
	assert.Equal(t, "eth_chainId", (*calls)[0].Method)
	assert.Empty(t, (*calls)[0].Params)
}

func TestChainID_LargeValue(t *testing.T) {
	// uint64 max + 1 must round-trip through *big.Int.
	const big65bit = "0x10000000000000000"
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_chainId": func(req client.TestRequest) (any, *client.TestRPCError) {
			return big65bit, nil
		},
	})
	cli := newTestClient(t, srv)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	want, _ := new(big.Int).SetString("10000000000000000", 16)
	assert.Equal(t, want, id)
}

func TestChainID_ServerError(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_chainId": func(req client.TestRequest) (any, *client.TestRPCError) {
			return nil, &client.TestRPCError{Code: -32603, Message: "internal error"}
		},
	})
	cli := newTestClient(t, srv)

	_, err := cli.ChainID(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eth_chainId")
	assert.Contains(t, err.Error(), "internal error")
}

func TestBlockNumber_Success(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_blockNumber": func(req client.TestRequest) (any, *client.TestRPCError) {
			return "0x10", nil
		},
	})
	cli := newTestClient(t, srv)

	n, err := cli.BlockNumber(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(16), n)
}

func TestHeaderByNumber_Finalized(t *testing.T) {
	srv, calls := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getBlockByNumber": func(req client.TestRequest) (any, *client.TestRPCError) {
			return map[string]any{
				"number":     "0x539",
				"hash":       "0x" + zeroHex(64),
				"parentHash": "0x" + zeroHex(64),
			}, nil
		},
	})
	cli := newTestClient(t, srv)

	h, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.NoError(t, err)
	require.NotNil(t, h)
	assert.Equal(t, uint64(1337), uint64(h.Number))

	require.Len(t, *calls, 1)
	c := (*calls)[0]
	require.Len(t, c.Params, 2)
	assert.JSONEq(t, `"finalized"`, string(c.Params[0]))
	assert.JSONEq(t, `false`, string(c.Params[1]))
}

func TestHeaderByNumber_NotFound_NullResult(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getBlockByNumber": func(req client.TestRequest) (any, *client.TestRPCError) {
			return nil, nil
		},
	})
	cli := newTestClient(t, srv)

	_, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.ErrorIs(t, err, eth.ErrNotFound)
}

func TestHeaderByNumber_BadHeader(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getBlockByNumber": func(req client.TestRequest) (any, *client.TestRPCError) {
			return map[string]any{"number": "not-hex"}, nil
		},
	})
	cli := newTestClient(t, srv)

	_, err := cli.HeaderByNumber(t.Context(), client.BlockFinalized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode header")
}

func TestTransactionReceipt_Success(t *testing.T) {
	srv, calls := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getTransactionReceipt": func(req client.TestRequest) (any, *client.TestRPCError) {
			return map[string]any{
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
			}, nil
		},
	})
	cli := newTestClient(t, srv)

	txHash := eth.HashFromString("0x" + repeatHex("ab", 32))
	r, err := cli.TransactionReceipt(t.Context(), txHash)
	require.NoError(t, err)
	require.Len(t, r.Logs, 1)
	assert.Equal(t, uint64(16), uint64(r.Logs[0].BlockNumber))
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, []byte(r.Logs[0].Data))

	require.Len(t, *calls, 1)
	c := (*calls)[0]
	require.Len(t, c.Params, 1)
	assert.JSONEq(t, `"`+txHash.Hex()+`"`, string(c.Params[0]))
}

func TestTransactionReceipt_NotFound(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getTransactionReceipt": func(req client.TestRequest) (any, *client.TestRPCError) {
			return nil, nil
		},
	})
	cli := newTestClient(t, srv)
	_, err := cli.TransactionReceipt(t.Context(), eth.Hash{})
	require.ErrorIs(t, err, eth.ErrNotFound)
}

func TestFilterLogs_Empty(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getLogs": func(req client.TestRequest) (any, *client.TestRPCError) {
			return []any{}, nil
		},
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
	srv, calls := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getLogs": func(req client.TestRequest) (any, *client.TestRPCError) {
			return []any{
				map[string]any{
					"topics":      []string{sigHash},
					"data":        "0x",
					"blockNumber": "0x100",
					"removed":     false,
				},
			}, nil
		},
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

	require.Len(t, *calls, 1)
	c := (*calls)[0]
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

func TestFilterQuery_MarshalShapes(t *testing.T) {
	addr := eth.AddressFromString("0x000000000000000000000000000000000000beef")
	hash1 := eth.HashFromString("0x" + repeatHex("11", 32))
	hash2 := eth.HashFromString("0x" + repeatHex("22", 32))

	cases := []struct {
		name   string
		q      client.FilterQuery
		assert func(t *testing.T, sent map[string]any)
	}{
		{
			// Unset FromBlock/ToBlock must NOT appear on the wire — this
			// is the eth_subscribe "live logs" shape; geth interprets an
			// explicit toBlock=0 as a bounded historical filter that
			// terminates at block 0.
			name: "unset block range omits keys",
			q:    client.FilterQuery{},
			assert: func(t *testing.T, sent map[string]any) {
				_, hasFrom := sent["fromBlock"]
				assert.False(t, hasFrom, "fromBlock must be omitted when unset")
				_, hasTo := sent["toBlock"]
				assert.False(t, hasTo, "toBlock must be omitted when unset")
				_, hasAddr := sent["address"]
				assert.False(t, hasAddr)
				_, hasTopics := sent["topics"]
				assert.False(t, hasTopics)
			},
		},
		{
			// Explicit zero is distinct from unset and still expressible
			// (e.g. eth_getLogs from genesis).
			name: "explicit block zero",
			q:    client.FilterQuery{FromBlock: ptr(uint64(0)), ToBlock: ptr(uint64(0))},
			assert: func(t *testing.T, sent map[string]any) {
				assert.Equal(t, "0x0", sent["fromBlock"])
				assert.Equal(t, "0x0", sent["toBlock"])
			},
		},
		{
			name: "single topic",
			q:    client.FilterQuery{Topics: [][]eth.Hash{{hash1}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				require.Len(t, topics, 1)
				_, isString := topics[0].(string)
				assert.True(t, isString)
			},
		},
		{
			name: "any-at-position-0 then exact-at-1",
			q:    client.FilterQuery{Topics: [][]eth.Hash{nil, {hash1}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				require.Len(t, topics, 2)
				assert.Nil(t, topics[0])
				_, isString := topics[1].(string)
				assert.True(t, isString)
			},
		},
		{
			name: "OR-list at position 0",
			q:    client.FilterQuery{Topics: [][]eth.Hash{{hash1, hash2}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				_, isArr := topics[0].([]any)
				assert.True(t, isArr)
			},
		},
		{
			name: "addresses",
			q:    client.FilterQuery{Addresses: []eth.Address{addr}},
			assert: func(t *testing.T, sent map[string]any) {
				addrs := sent["address"].([]any)
				require.Len(t, addrs, 1)
				assert.Equal(t, addrHex(addr), addrs[0])
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.q)
			require.NoError(t, err)
			var sent map[string]any
			require.NoError(t, json.Unmarshal(raw, &sent))
			c.assert(t, sent)
		})
	}
}

func TestFilterLogs_ServerError(t *testing.T) {
	srv, _ := captureHandler(t, map[string]func(req client.TestRequest) (any, *client.TestRPCError){
		"eth_getLogs": func(req client.TestRequest) (any, *client.TestRPCError) {
			return nil, &client.TestRPCError{Code: -32005, Message: "query returned more than 10000 results"}
		},
	})
	cli := newTestClient(t, srv)

	_, err := cli.FilterLogs(t.Context(), client.FilterQuery{
		FromBlock: ptr(uint64(1)),
		ToBlock:   ptr(uint64(1_000_000)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eth_getLogs")
	assert.Contains(t, err.Error(), "10000 results")
}

func TestMethods_ContextCancelled(t *testing.T) {
	// Handler that blocks forever; ctx-cancel must propagate.
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		<-gate
		return nil, nil
	})
	cli := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := cli.ChainID(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// --- helpers ---

func repeatHex(unit string, repeat int) string {
	out := make([]byte, 0, len(unit)*repeat)
	for range repeat {
		out = append(out, unit...)
	}
	return string(out)
}

func zeroHex(n int) string { return repeatHex("0", n) }

func addrHex(a eth.Address) string {
	b, _ := a.MarshalText()
	return string(b)
}

func ptr[T any](v T) *T { return &v }
