package l1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logStateUpdateTopicHex is keccak256("LogStateUpdate(uint256,int256,uint256)").
// Hard-coded so the test doesn't need an abigen runtime dependency.
const logStateUpdateTopicHex = "0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c"

// --- minimal JSON-RPC 2.0 test server ---------------------------------

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      json.RawMessage `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// gethJSONRPCServer is a tiny in-memory JSON-RPC 2.0 endpoint for unit-
// testing GethSettlement against geth's ethclient. Handlers are
// registered per RPC method; unknown methods return method-not-found.
type gethJSONRPCServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	handlers map[string]func(json.RawMessage) (any, *jsonRPCError)
}

func newGethJSONRPCServer(t *testing.T) *gethJSONRPCServer {
	t.Helper()
	s := &gethJSONRPCServer{
		handlers: make(map[string]func(json.RawMessage) (any, *jsonRPCError)),
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.dispatch))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *gethJSONRPCServer) URL() string { return s.srv.URL }

func (s *gethJSONRPCServer) on(method string, h func(json.RawMessage) (any, *jsonRPCError)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

func (s *gethJSONRPCServer) dispatch(w http.ResponseWriter, r *http.Request) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	h, ok := s.handlers[req.Method]
	s.mu.Unlock()
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	if !ok {
		resp.Error = &jsonRPCError{Code: -32601, Message: "method not found: " + req.Method}
	} else {
		result, jerr := h(req.Params)
		if jerr != nil {
			resp.Error = jerr
		} else {
			// Marshal explicitly so a nil result becomes literal `null`
			// in the wire response — geth's ethclient maps that to
			// ethereum.NotFound for HeaderByNumber/TransactionReceipt.
			raw, err := json.Marshal(result)
			if err != nil {
				resp.Error = &jsonRPCError{Code: -32603, Message: err.Error()}
			} else {
				resp.Result = raw
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// --- recording listener -----------------------------------------------

type recordingListener struct {
	mu    sync.Mutex
	calls []recordedCall
}

type recordedCall struct {
	method string
	dur    time.Duration
}

func (r *recordingListener) OnNewL1Head(_ *core.L1Head) {}
func (r *recordingListener) OnL1Call(method string, took time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedCall{method, took})
}

func (r *recordingListener) methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	for i, c := range r.calls {
		out[i] = c.method
	}
	return out
}

// --- test helpers -----------------------------------------------------

func newTestSettlement(t *testing.T, srv *gethJSONRPCServer) *l1.GethSettlement {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s, err := l1.NewGethSettlement(
		ctx,
		srv.URL(),
		eth.Address{},
		l1.WithSettlementLogger(log.NewNopZapLogger()),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

// --- tests ------------------------------------------------------------

func TestGethSettlement_DialError(t *testing.T) {
	// Closed server URL — dial succeeds (HTTP is connectionless) but the
	// first RPC call should fail. We test failure via a real method
	// instead of the constructor since http dialing doesn't actually
	// open a connection eagerly.
	srv := newGethJSONRPCServer(t)
	srv.srv.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	s, err := l1.NewGethSettlement(ctx, srv.URL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)
	_, err = s.ChainID(ctx)
	require.Error(t, err)
}

func TestGethSettlement_MalformedURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err := l1.NewGethSettlement(ctx, "::::not-a-url", eth.Address{})
	require.Error(t, err)
}

func TestGethSettlement_ChainID(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_chainId", func(json.RawMessage) (any, *jsonRPCError) {
		return "0x5", nil // goerli-shaped value, picked for distinctness
	})
	s := newTestSettlement(t, srv)
	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(5), id.Int64())
}

func TestGethSettlement_LatestHeight(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_blockNumber", func(json.RawMessage) (any, *jsonRPCError) {
		return "0x123", nil
	})
	s := newTestSettlement(t, srv)
	h, err := s.LatestHeight(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(0x123), h)
}

func TestGethSettlement_FinalisedHeight(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getBlockByNumber", func(p json.RawMessage) (any, *jsonRPCError) {
		// Geth's Header JSON decoder requires every standard field.
		return map[string]any{
			"parentHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
			"sha3Uncles":       "0x0000000000000000000000000000000000000000000000000000000000000000",
			"miner":            "0x0000000000000000000000000000000000000000",
			"stateRoot":        "0x0000000000000000000000000000000000000000000000000000000000000000",
			"transactionsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"receiptsRoot":     "0x0000000000000000000000000000000000000000000000000000000000000000",
			"logsBloom":        "0x" + zeros(512),
			"difficulty":       "0x0",
			"number":           "0x2a",
			"gasLimit":         "0x0",
			"gasUsed":          "0x0",
			"timestamp":        "0x0",
			"extraData":        "0x",
			"mixHash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
			"nonce":            "0x0000000000000000",
			"hash":             "0x1111111111111111111111111111111111111111111111111111111111111111",
		}, nil
	})
	s := newTestSettlement(t, srv)
	h, err := s.FinalisedHeight(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(0x2a), h)
}

func TestGethSettlement_FinalisedHeight_NotFound(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getBlockByNumber", func(json.RawMessage) (any, *jsonRPCError) {
		// JSON-RPC nil result → ethclient.HeaderByNumber surfaces
		// ethereum.NotFound, which the settlement wraps as eth.ErrNotFound.
		return nil, nil
	})
	s := newTestSettlement(t, srv)
	_, err := s.FinalisedHeight(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, eth.ErrNotFound)
}

func TestGethSettlement_FilterStateUpdate_Empty(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getLogs", func(json.RawMessage) (any, *jsonRPCError) {
		return []any{}, nil
	})
	s := newTestSettlement(t, srv)
	updates, err := s.FilterStateUpdate(t.Context(), 0, 100)
	require.NoError(t, err)
	assert.Empty(t, updates)
}

func TestGethSettlement_FilterStateUpdate_ErrorWrap(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getLogs", func(json.RawMessage) (any, *jsonRPCError) {
		return nil, &jsonRPCError{Code: -32000, Message: "synthetic"}
	})
	s := newTestSettlement(t, srv)
	_, err := s.FilterStateUpdate(t.Context(), 0, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter LogStateUpdate [0,100]")
	assert.Contains(t, err.Error(), "synthetic")
}

func TestGethSettlement_TransactionReceipt(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getTransactionReceipt", func(json.RawMessage) (any, *jsonRPCError) {
		return map[string]any{
			"transactionHash":   "0x1111111111111111111111111111111111111111111111111111111111111111",
			"transactionIndex":  "0x0",
			"blockHash":         "0x2222222222222222222222222222222222222222222222222222222222222222",
			"blockNumber":       "0x10",
			"from":              "0x0000000000000000000000000000000000000000",
			"to":                "0x0000000000000000000000000000000000000000",
			"cumulativeGasUsed": "0x0",
			"gasUsed":           "0x0",
			"contractAddress":   nil,
			"logs": []any{
				map[string]any{
					"address":          "0x0000000000000000000000000000000000000000",
					"topics":           []string{logStateUpdateTopicHex},
					"data":             "0x",
					"blockNumber":      "0x10",
					"transactionHash":  "0x1111111111111111111111111111111111111111111111111111111111111111",
					"transactionIndex": "0x0",
					"blockHash":        "0x2222222222222222222222222222222222222222222222222222222222222222",
					"logIndex":         "0x0",
					"removed":          false,
				},
			},
			"logsBloom": "0x" + zeros(512),
			"status":    "0x1",
			"type":      "0x0",
		}, nil
	})
	s := newTestSettlement(t, srv)
	r, err := s.TransactionReceipt(t.Context(), eth.Hash{})
	require.NoError(t, err)
	require.Len(t, r.Logs, 1)
	assert.Equal(t, logStateUpdateTopicHex, r.Logs[0].Topics[0].Hex())
}

func TestGethSettlement_TransactionReceipt_NotFound(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_getTransactionReceipt", func(json.RawMessage) (any, *jsonRPCError) {
		return nil, nil
	})
	s := newTestSettlement(t, srv)
	_, err := s.TransactionReceipt(t.Context(), eth.Hash{})
	require.Error(t, err)
	assert.ErrorIs(t, err, eth.ErrNotFound)
}

func TestGethSettlement_Listener_RecordsCalls(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_chainId", func(json.RawMessage) (any, *jsonRPCError) { return "0x1", nil })
	srv.on("eth_blockNumber", func(json.RawMessage) (any, *jsonRPCError) {
		return nil, &jsonRPCError{Code: -32000, Message: "synthetic"}
	})
	rec := &recordingListener{}
	s := newTestSettlement(t, srv)
	s.SetListener(rec)
	_, _ = s.ChainID(t.Context())
	_, _ = s.LatestHeight(t.Context())
	assert.Equal(t, []string{"eth_chainId", "eth_blockNumber"}, rec.methods())
}

func TestGethSettlement_Close_Idempotent(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_chainId", func(json.RawMessage) (any, *jsonRPCError) { return "0x1", nil })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s, err := l1.NewGethSettlement(ctx, srv.URL(), eth.Address{})
	require.NoError(t, err)
	s.Close()
	// Second close shouldn't panic. go-ethereum's rpc.Client.Close is
	// safe to call repeatedly.
	require.NotPanics(t, s.Close)
}

func TestGethSettlement_OptionLogger_Wired(t *testing.T) {
	srv := newGethJSONRPCServer(t)
	srv.on("eth_chainId", func(json.RawMessage) (any, *jsonRPCError) { return "0x1", nil })
	// Verifies that the WithSettlementLogger option compiles and
	// constructor accepts it. Logger itself is no-op; no behaviour
	// assertion possible.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	s, err := l1.NewGethSettlement(
		ctx,
		srv.URL(),
		eth.Address{},
		l1.WithSettlementLogger(log.NewNopZapLogger()),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)
	_, err = s.ChainID(ctx)
	require.NoError(t, err)
}

// zeros returns a string of n '0' characters; helper for fixed-width
// 32-byte / 256-byte hex fillers in mock responses.
func zeros(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
