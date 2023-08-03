package jsonrpc_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	method := jsonrpc.Method{
		Name: "echo",
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
		Params: []jsonrpc.Parameter{{Name: "msg"}},
	}
	log := utils.NewNopZapLogger()
	rpc := jsonrpc.NewServer(log)
	require.NoError(t, rpc.RegisterMethod(method))
	server := jsonrpc.NewHTTP("/vX.Y.Z", listener, rpc, log)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(func() {
		cancel()
	})
	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	msg := `{"jsonrpc" : "2.0", "method" : "echo", "params" : [ "abc123" ], "id" : 1}`
	client := new(http.Client)
	url := "http://" + listener.Addr().String()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(msg)))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
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
			req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})

		t.Run("non-root path", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, "GET", url+"/notfound", http.NoBody)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	})
}
