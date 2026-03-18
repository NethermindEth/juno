package node_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleReadySync(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	readinessBlockTolerance := uint(6)
	readinessHandlers := node.NewReadinessHandlers(mockReader, synchronizer, readinessBlockTolerance)
	ctx := t.Context()

	t.Run("ready and blockNumber outside blockRange to highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := blockNum + uint64(readinessBlockTolerance) + 1
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ready & blockNumber is larger than highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := uint64(1)

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ready & blockNumber is in blockRange of highestBlock", func(t *testing.T) {
		blockNum := uint64(3)
		highestBlock := blockNum + uint64(readinessBlockTolerance)

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestHandleLive(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	readinessBlockTolerance := uint(6)
	readinessHandlers := node.NewReadinessHandlers(mockReader, synchronizer, readinessBlockTolerance)
	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/live", http.NoBody)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()

	readinessHandlers.HandleLive(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func freePort(t *testing.T) uint16 {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	require.NoError(t, err)

	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	defer l.Close()
	return uint16(l.Addr().(*net.TCPAddr).Port)
}

func waitForPort(t *testing.T, ctx context.Context, port uint16) {
	t.Helper()
	addr := net.JoinHostPort("localhost", strconv.Itoa(int(port)))
	dialer := &net.Dialer{Timeout: 50 * time.Millisecond}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestHTTPServer_Lifecycle(t *testing.T) {
	port := freePort(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := node.MakeHTTPService("localhost", port, handler)

	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	waitCtx, waitCancel := context.WithTimeout(t.Context(), time.Second)
	defer waitCancel()
	waitForPort(t, waitCtx, port)
	require.NoError(t, waitCtx.Err())

	// Test reachability
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("http://localhost:%d", port), http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	cancel()
	require.NoError(t, <-errCh)
}

func TestHTTPServer_JSONRPC(t *testing.T) {
	port := freePort(t)

	log := utils.NewNopZapLogger()
	server := jsonrpc.NewServer(1, log)
	// Register a dummy method to avoid 404/MethodNotFound
	err := server.RegisterMethods(jsonrpc.Method{
		Name: "test_method",
		Handler: func() (string, *jsonrpc.Error) {
			return "ok", nil
		},
	})
	require.NoError(t, err)

	srv := node.MakeRPCOverHTTP("localhost", port, map[string]*jsonrpc.Server{"/": server}, nil, log, false, false, 0)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = srv.Run(ctx)
	}()

	waitCtx, waitCancel := context.WithTimeout(t.Context(), time.Second)
	defer waitCancel()
	waitForPort(t, waitCtx, port)
	require.NoError(t, waitCtx.Err())

	url := fmt.Sprintf("http://localhost:%d", port)

	t.Run("Valid POST", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"test_method","id":1}`
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Reject Non-JSON Content-Type", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"test_method","id":1}`
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})

	t.Run("GET request to / returns 200", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPServer_Websocket(t *testing.T) {
	port := freePort(t)

	log := utils.NewNopZapLogger()
	server := jsonrpc.NewServer(1, log)
	srv := node.MakeRPCOverWebsocket("localhost", port, map[string]*jsonrpc.Server{"/": server}, log, false, false)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = srv.Run(ctx)
	}()

	waitCtx, waitCancel := context.WithTimeout(t.Context(), time.Second)
	defer waitCancel()
	waitForPort(t, waitCtx, port)
	require.NoError(t, waitCtx.Err())

	// WebSocket handshake
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort("localhost", strconv.Itoa(int(port))))
	require.NoError(t, err)
	defer conn.Close()

	handshake := "GET / HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"

	_, err = conn.Write([]byte(handshake))
	require.NoError(t, err)

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Contains(t, string(buf[:n]), "101 Switching Protocols")
}

func testEndpointReachability(t *testing.T, srv interface{ Run(context.Context) error }, port uint16, path string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = srv.Run(ctx)
	}()

	waitCtx, waitCancel := context.WithTimeout(t.Context(), time.Second)
	defer waitCancel()
	waitForPort(t, waitCtx, port)
	require.NoError(t, waitCtx.Err())

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, path), http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPServer_Metrics(t *testing.T) {
	port := freePort(t)
	srv := node.MakeMetrics("localhost", port)
	testEndpointReachability(t, srv, port, "/metrics")
}

func TestHTTPServer_PPROF(t *testing.T) {
	port := freePort(t)
	srv := node.MakePPROF("localhost", port)
	testEndpointReachability(t, srv, port, "/debug/pprof/")
}
