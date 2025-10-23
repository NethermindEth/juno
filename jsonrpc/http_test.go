package jsonrpc_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
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
	log := utils.NewNopZapLogger()
	rpc := jsonrpc.NewServer(1, log).WithListener(&listener)
	require.NoError(t, rpc.RegisterMethods(method))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Server
	srv := httptest.NewServer(jsonrpc.NewHTTP(rpc, log))
	t.Cleanup(srv.Close)

	// Client
	client := new(http.Client)

	msg := `{"jsonrpc" : "2.0", "method" : "echo", "params" : [ "abc123" ], "id" : 1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, bytes.NewReader([]byte(msg)))
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
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})

		t.Run("non-root path", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/notfound", http.NoBody)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
		assert.Len(t, listener.OnNewRequestLogs, 1)
	})
}

func TestGzipResponse(t *testing.T) {
	method := jsonrpc.Method{
		Name: "echo",
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
		Params: []jsonrpc.Parameter{{Name: "msg"}},
	}
	log := utils.NewNopZapLogger()
	rpc := jsonrpc.NewServer(1, log)
	require.NoError(t, rpc.RegisterMethods(method))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv := httptest.NewServer(jsonrpc.NewHTTP(rpc, log))
	t.Cleanup(srv.Close)
	client := new(http.Client)

	payload := "rand.Text"
	msg := fmt.Sprintf(`{"jsonrpc":"2.0", "method":"echo", "params":[%q], "id":1}`, payload)
	expected := fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, payload)
	headers := map[string]string{
		"Accept-Encoding": "gzip, deflate, br",
		"Content-Type":    "application/json",
	}
	t.Run("success: gzip encoded response", func(t *testing.T) {
		resp := setHeaderAndProcessRequest(ctx, client, headers, bytes.NewReader([]byte(msg)), t, srv)
		defer resp.Body.Close()
		verifyResponse(resp, t, expected)
	})

	t.Run("success: gzip encoded request & response", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write([]byte(msg))
		require.NoError(t, err)
		require.NoError(t, gz.Close())
		headers["Content-Encoding"] = "gzip"
		resp := setHeaderAndProcessRequest(ctx, client, headers, &buf, t, srv)
		defer resp.Body.Close()
		verifyResponse(resp, t, expected)
	})

	t.Run("failed: request is not gzip encoded but set header as gzip encoded",
		func(t *testing.T) {
			resp := setHeaderAndProcessRequest(ctx, client, headers, bytes.NewReader([]byte(msg)), t, srv)
			defer resp.Body.Close()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
}

func setHeaderAndProcessRequest(
	ctx context.Context,
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
