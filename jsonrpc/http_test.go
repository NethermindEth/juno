package jsonrpc_test

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP(t *testing.T) {
	method := jsonrpc.Method{
		Name: "echo",
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
		Params: []jsonrpc.Parameter{{Name: "msg"}},
	}
	listener := CountingEventListener{}
	logger := log.NewNopZapLogger()
	rpc := jsonrpc.NewServer(1, logger).WithListener(&listener)
	require.NoError(t, rpc.RegisterMethods(method))

	// Server
	srv := httptest.NewServer(jsonrpc.NewHTTP(rpc, logger))
	t.Cleanup(srv.Close)

	// Client
	client := new(http.Client)

	msg := `{"jsonrpc" : "2.0", "method" : "echo", "params" : [ "abc123" ], "id" : 1}`
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		srv.URL,
		bytes.NewReader([]byte(msg)),
	)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, listener.OnNewRequestLogs, 1)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	require.NoError(t, err)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	t.Run("GET", func(t *testing.T) {
		t.Run("root path", func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				srv.URL,
				http.NoBody,
			)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})

		t.Run("non-root path", func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				srv.URL+"/notfound",
				http.NoBody,
			)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
		assert.Len(t, listener.OnNewRequestLogs, 1)
	})
}

func TestHTTPRequestGate(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }

	method := jsonrpc.Method{
		Name: "block",
		Handler: func() (string, *jsonrpc.Error) {
			started <- struct{}{}
			<-release
			return "ok", nil
		},
	}
	logger := log.NewNopZapLogger()
	rpc := jsonrpc.NewServer(1, logger)
	require.NoError(t, rpc.RegisterMethods(method))

	// Gate with a single slot and no queue: one request runs, the next is rejected.
	httpHandler := jsonrpc.NewHTTP(rpc, logger).WithGate(jsonrpc.NewGate(1, 0))
	srv := httptest.NewServer(httpHandler)
	t.Cleanup(srv.Close)
	// Registered after srv.Close so it runs first (cleanups are LIFO): it unblocks
	// the in-flight handler so srv.Close can drain even if the test fails early.
	t.Cleanup(releaseAll)

	client := new(http.Client)
	doPost := func() (*http.Response, error) {
		msg := `{"jsonrpc":"2.0","method":"block","id":1}`
		req, err := http.NewRequestWithContext(
			t.Context(), http.MethodPost, srv.URL, bytes.NewReader([]byte(msg)),
		)
		if err != nil {
			return nil, err
		}
		return client.Do(req)
	}

	// First request occupies the only slot and blocks in the handler.
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		resp, err := doPost()
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, resp.Body.Close())
		}
	}()
	<-started // the handler is running, so the gate slot is taken

	// Second request: the gate is full, expect an immediate 503 + Retry-After.
	resp, err := doPost()
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("Retry-After"))
	require.NoError(t, resp.Body.Close())

	// GET "/" is never gated, even while the gate is saturated.
	getReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)
	getResp, err := client.Do(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)
	require.NoError(t, getResp.Body.Close())

	// Release the first request and confirm it completed successfully.
	releaseAll()
	<-firstDone
}

func TestGzipResponse(t *testing.T) {
	method := jsonrpc.Method{
		Name: "echo",
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
		Params: []jsonrpc.Parameter{{Name: "msg"}},
	}
	logger := log.NewNopZapLogger()
	rpc := jsonrpc.NewServer(1, logger)
	require.NoError(t, rpc.RegisterMethods(method))

	srv := httptest.NewServer(jsonrpc.NewHTTP(rpc, logger))
	t.Cleanup(srv.Close)
	client := new(http.Client)

	payload := rand.Text()
	msg := fmt.Sprintf(`{"jsonrpc":"2.0", "method":"echo", "params":[%q], "id":1}`, payload)
	expected := fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, payload)
	commonHeaders := map[string]string{
		"Accept-Encoding": "gzip, deflate, br",
		"Content-Type":    "application/json",
	}
	t.Run("success: gzip encoded response", func(t *testing.T) {
		resp := setHeaderAndProcessRequest(client, commonHeaders, bytes.NewReader([]byte(msg)), t, srv)
		defer resp.Body.Close()
		verifyResponse(resp, t, expected)
	})
}

func setHeaderAndProcessRequest(
	client *http.Client,
	headers map[string]string,
	msg io.Reader,
	t *testing.T,
	srv *httptest.Server,
) *http.Response {
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		srv.URL,
		msg,
	)
	t.Cleanup(func() { req.Body.Close() })
	require.NoError(t, err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func verifyResponse(resp *http.Response, t *testing.T, expected string) {
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gzr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer gzr.Close()

	decompressedBody, err := io.ReadAll(gzr)
	require.NoError(t, err)
	assert.Equal(t, string(decompressedBody), expected)
}
