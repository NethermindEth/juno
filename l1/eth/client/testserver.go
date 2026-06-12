package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
)

// TestServer is a minimal JSON-RPC server suitable for unit-testing
// the Client. It accepts requests over POST application/json and over
// a websocket upgrade on the same URL.
//
// Behaviour is driven by a single Handler function set per-test. The
// server tracks live websocket connections so tests can push
// subscription notifications or sever the connection mid-call.
type TestServer struct {
	srv *httptest.Server

	mu      sync.Mutex
	handler TestHandler
	wsConns []*websocket.Conn
}

// TestHandler returns the JSON-RPC reply for a single request. result
// is encoded as the response's "result" field; if rerr is non-nil it
// is encoded as the "error" field (and result is ignored).
type TestHandler func(req TestRequest) (result any, rerr *TestRPCError)

// TestRequest is the decoded JSON-RPC request handed to a TestHandler.
type TestRequest struct {
	Method string
	Params []json.RawMessage
	ID     uint64
}

// TestRPCError mirrors the JSON-RPC error object that the handler can
// emit. Code is required; Data is optional.
type TestRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewTestServer constructs an unstarted TestServer with a default
// "method not found" handler. Install your own with SetHandler.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()
	ts := &TestServer{
		handler: func(req TestRequest) (any, *TestRPCError) {
			return nil, &TestRPCError{Code: -32601, Message: "method not found: " + req.Method}
		},
	}
	ts.srv = httptest.NewServer(http.HandlerFunc(ts.serveHTTP))
	t.Cleanup(ts.Close)
	return ts
}

// SetHandler installs a per-request handler. Thread-safe; can be
// changed between calls.
func (ts *TestServer) SetHandler(h TestHandler) {
	ts.mu.Lock()
	ts.handler = h
	ts.mu.Unlock()
}

// URL returns the base http:// URL for unary tests.
func (ts *TestServer) URL() string { return ts.srv.URL }

// WSURL returns the same endpoint as a ws:// URL for subscription tests.
func (ts *TestServer) WSURL() string {
	return "ws" + strings.TrimPrefix(ts.srv.URL, "http")
}

// Close stops the server and tears down any live websocket connections.
func (ts *TestServer) Close() {
	ts.mu.Lock()
	conns := ts.wsConns
	ts.wsConns = nil
	ts.mu.Unlock()
	for _, c := range conns {
		_ = c.CloseNow()
	}
	ts.srv.Close()
}

// KillWSConns severs every live websocket connection with the given
// status code. Useful for testing client-side resubscribe behaviour.
func (ts *TestServer) KillWSConns() {
	ts.mu.Lock()
	conns := ts.wsConns
	ts.wsConns = nil
	ts.mu.Unlock()
	for _, c := range conns {
		_ = c.Close(websocket.StatusInternalError, "test sever")
	}
}

// PushNotification broadcasts an eth_subscription notification with
// the given subscription id and result payload to every live ws conn.
// Returns the first write error, if any.
func (ts *TestServer) PushNotification(ctx context.Context, subID string, payload any) error {
	frame := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_subscription",
		"params": map[string]any{
			"subscription": subID,
			"result":       payload,
		},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	ts.mu.Lock()
	conns := append([]*websocket.Conn(nil), ts.wsConns...)
	ts.mu.Unlock()
	var firstErr error
	for _, c := range conns {
		if werr := c.Write(ctx, websocket.MessageText, data); werr != nil && firstErr == nil {
			firstErr = werr
		}
	}
	return firstErr
}

func (ts *TestServer) callHandler(req TestRequest) (any, *TestRPCError) {
	ts.mu.Lock()
	h := ts.handler
	ts.mu.Unlock()
	if h == nil {
		return nil, &TestRPCError{Code: -32603, Message: "no handler set"}
	}
	return h(req)
}

func (ts *TestServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		ts.serveWebsocket(w, r)
		return
	}
	ts.serveOnePost(w, r)
}

func (ts *TestServer) serveOnePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var raw rawRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	resp := ts.respondTo(raw)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (ts *TestServer) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)
	ts.mu.Lock()
	ts.wsConns = append(ts.wsConns, conn)
	ts.mu.Unlock()
	defer func() { _ = conn.CloseNow() }()

	ctx := r.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var raw rawRPCRequest
		if jerr := json.Unmarshal(data, &raw); jerr != nil {
			continue
		}
		resp := ts.respondTo(raw)
		respData, jerr := json.Marshal(resp)
		if jerr != nil {
			continue
		}
		if werr := conn.Write(ctx, websocket.MessageText, respData); werr != nil {
			return
		}
	}
}

// rawRPCRequest is the on-the-wire request shape; deliberately
// duplicated from rpcRequest so the test server can read malformed
// frames without imposing client-side constraints.
type rawRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      uint64            `json:"id"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
}

func (ts *TestServer) respondTo(req rawRPCRequest) map[string]any {
	out := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	result, rerr := ts.callHandler(TestRequest{
		Method: req.Method,
		Params: req.Params,
		ID:     req.ID,
	})
	if rerr != nil {
		out["error"] = rerr
		return out
	}
	out["result"] = result
	return out
}
