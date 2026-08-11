// Package clienttest provides a minimal JSON-RPC test server for the client
// package. It is its own package (not a _test.go file) so l1 provider tests can
// share it, and lives under internal/ so production code cannot import it
// (it pulls in testing and httptest).
package clienttest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/coder/websocket"
)

// wsReadLimit mirrors the client's limit so the server accepts the same frame sizes.
const wsReadLimit = 16 << 20

// TestServer serves JSON-RPC over POST or a websocket upgrade on the same URL.
// Live ws conns are tracked so tests can push notifications or sever mid-call.
type TestServer struct {
	srv *httptest.Server

	mu      sync.Mutex
	handler TestHandler
	wsConns []*websocket.Conn

	pingsReceived atomic.Int64
	dropPings     atomic.Bool
}

// TestHandler returns the JSON-RPC reply for a request: result becomes "result",
// or a non-nil rerr becomes "error" (result ignored).
type TestHandler func(req TestRequest) (result any, rerr *TestRPCError)

type TestRequest struct {
	Method string
	Params []json.RawMessage
}

// Code is required; Data is optional.
type TestRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewTestServer(tb testing.TB) *TestServer {
	tb.Helper()
	ts := &TestServer{
		handler: func(req TestRequest) (any, *TestRPCError) {
			return nil, &TestRPCError{Code: -32601, Message: "method not found: " + req.Method}
		},
	}
	ts.srv = httptest.NewServer(http.HandlerFunc(ts.serveHTTP))
	tb.Cleanup(ts.Close)
	return ts
}

func (ts *TestServer) SetHandler(h TestHandler) {
	ts.mu.Lock()
	ts.handler = h
	ts.mu.Unlock()
}

func (ts *TestServer) URL() string { return ts.srv.URL }

func (ts *TestServer) WSURL() string {
	return "ws" + strings.TrimPrefix(ts.srv.URL, "http")
}

func (ts *TestServer) PingsReceived() int64 { return ts.pingsReceived.Load() }

func (ts *TestServer) WSConnCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.wsConns)
}

// SetDropPings makes the server count incoming pings but suppress the pong
// reply, to provoke client-side ping timeouts.
func (ts *TestServer) SetDropPings(b bool) { ts.dropPings.Store(b) }

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

func (ts *TestServer) KillWSConns() {
	ts.mu.Lock()
	conns := ts.wsConns
	ts.wsConns = nil
	ts.mu.Unlock()
	for _, c := range conns {
		_ = c.Close(websocket.StatusInternalError, "test server")
	}
}

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
	return ts.broadcast(ctx, data)
}

// PushRawFrame writes data verbatim, so tests can inject malformed frames a
// well-behaved server would never emit.
func (ts *TestServer) PushRawFrame(ctx context.Context, data []byte) error {
	return ts.broadcast(ctx, data)
}

// broadcast writes data to every live ws conn, returning the first write
// failure. It errors when no conn is live: a push that reaches nobody is a
// test-ordering bug (e.g. pushing before eth_subscribe completes), not success.
func (ts *TestServer) broadcast(ctx context.Context, data []byte) error {
	ts.mu.Lock()
	conns := append([]*websocket.Conn(nil), ts.wsConns...)
	ts.mu.Unlock()
	if len(conns) == 0 {
		return errors.New("no live websocket conns to broadcast to")
	}
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
	resp, reply := ts.respondTo(&raw)
	if !reply {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (ts *TestServer) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OnPingReceived: func(_ context.Context, _ []byte) bool {
			ts.pingsReceived.Add(1)
			// true: auto-reply with a pong; false: drop it silently.
			return !ts.dropPings.Load()
		},
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)
	ts.mu.Lock()
	ts.wsConns = append(ts.wsConns, conn)
	ts.mu.Unlock()
	defer func() {
		ts.mu.Lock()
		for i, c := range ts.wsConns {
			if c == conn {
				ts.wsConns = append(ts.wsConns[:i], ts.wsConns[i+1:]...)
				break
			}
		}
		ts.mu.Unlock()
		_ = conn.CloseNow()
	}()

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
		resp, reply := ts.respondTo(&raw)
		if !reply {
			continue
		}
		respData, jerr := json.Marshal(resp)
		if jerr != nil {
			continue
		}
		if werr := conn.Write(ctx, websocket.MessageText, respData); werr != nil {
			return
		}
	}
}

// rawRPCRequest is duplicated from the client's rpcRequest so the server can
// read malformed frames without the client package's constraints. id stays
// opaque so it round-trips verbatim rather than being narrowed to uint64.
type rawRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
}

// respondTo builds the reply frame for req, echoing its id verbatim. The bool
// is false for a notification (no id), which a conforming server never answers.
func (ts *TestServer) respondTo(req *rawRPCRequest) (map[string]any, bool) {
	if len(req.ID) == 0 {
		return nil, false
	}
	out := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	result, rerr := ts.callHandler(TestRequest{
		Method: req.Method,
		Params: req.Params,
	})
	if rerr != nil {
		out["error"] = rerr
		return out, true
	}
	out["result"] = result
	return out, true
}
