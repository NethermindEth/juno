package mcp_test

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mcp"
	"github.com/NethermindEth/juno/utils/log"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateRejectsToolCallsWhenFull(t *testing.T) {
	var calls atomic.Int32
	rpcServer := newGateRPCServer(t, func() (uint64, *jsonrpc.Error) {
		calls.Add(1)
		return 1, nil
	})
	gate := jsonrpc.NewGate(1, 0)
	session := newGateSession(t, mcp.New(rpcServer, "test", log.NewNopZapLogger()).WithGate(gate))

	// The test holds the only slot, as an RPC request over HTTP would.
	require.True(t, gate.TryAcquire())

	ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
	defer cancel()
	_, err := session.ListTools(ctx, nil)
	require.NoError(t, err, "listing tools must not need a slot")

	result := callGateTool(t, session)
	assert.True(t, result.IsError)
	assert.JSONEq(t, `{"code":-32004,"message":"server busy"}`, resultText(t, result))
	assert.Zero(t, calls.Load(), "a rejected call must not reach the JSON-RPC method")
	assert.Equal(t, uint64(1), gate.Rejected())

	gate.Release()
	result = callGateTool(t, session)
	assert.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, "1", resultText(t, result))
	assert.Equal(t, int32(1), calls.Load())
}

func TestGateQueuedToolCallTimesOut(t *testing.T) {
	var calls atomic.Int32
	rpcServer := newGateRPCServer(t, func() (uint64, *jsonrpc.Error) {
		calls.Add(1)
		return 1, nil
	})
	gate := jsonrpc.NewGate(1, 1)
	server := mcp.New(rpcServer, "test", log.NewNopZapLogger()).
		WithRequestTimeout(50 * time.Millisecond).
		WithGate(gate)
	session := newGateSession(t, server)

	// The test holds the only slot, so the call can only wait in the queue.
	require.True(t, gate.TryAcquire())
	defer gate.Release()

	result := callGateTool(t, session)
	assert.True(t, result.IsError)
	assert.JSONEq(t,
		`{"code":-32004,"message":"server busy","data":"request timed out while queued"}`,
		resultText(t, result),
	)
	assert.Zero(t, calls.Load(), "a call that timed out in the queue must not run")
	assert.Zero(t, gate.Queued(), "the call must leave the queue")
	assert.Zero(t, gate.Rejected(), "the call was queued, not rejected")
}

func TestGateReleasesSlot(t *testing.T) {
	tests := []struct {
		name    string
		respond func() (uint64, *jsonrpc.Error)
		want    string
		isError bool
	}{
		{
			name:    "result",
			respond: func() (uint64, *jsonrpc.Error) { return 1, nil },
			want:    "1",
		},
		{
			name: "JSON-RPC error",
			respond: func() (uint64, *jsonrpc.Error) {
				return 0, &jsonrpc.Error{Code: 24, Message: "Block not found"}
			},
			want:    `{"code":24,"message":"Block not found"}`,
			isError: true,
		},
		{
			name:    "panic",
			respond: func() (uint64, *jsonrpc.Error) { panic("boom") },
			want:    "internal error",
			isError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			rpcServer := newGateRPCServer(t, func() (uint64, *jsonrpc.Error) {
				calls.Add(1)
				return test.respond()
			})
			gate := jsonrpc.NewGate(1, 0)
			server := mcp.New(rpcServer, "test", log.NewNopZapLogger()).WithGate(gate)
			session := newGateSession(t, server)

			// With one slot and no queue, the second call is admitted only if the first one
			// freed its slot.
			for range 2 {
				result := callGateTool(t, session)
				assert.Equal(t, test.isError, result.IsError)
				assert.Equal(t, test.want, resultText(t, result))
				assert.Zero(t, gate.Running(), "the slot must be free once the call returns")
				assert.Zero(t, gate.Queued())
			}
			assert.Equal(t, int32(2), calls.Load())
			assert.Zero(t, gate.Rejected())
		})
	}
}

// newGateRPCServer returns a JSON-RPC server whose only method, starknet_blockNumber, is
// answered by handler.
func newGateRPCServer(t *testing.T, handler func() (uint64, *jsonrpc.Error)) *jsonrpc.Server {
	t.Helper()

	rpcServer := jsonrpc.NewServer(pool.New().WithMaxGoroutines(1), log.NewNopZapLogger())
	require.NoError(t, rpcServer.RegisterMethods(jsonrpc.Method{
		Name:    "starknet_blockNumber",
		Handler: handler,
	}))
	return rpcServer
}

// newGateSession serves server over HTTP and connects a client to it.
func newGateSession(t *testing.T, server *mcp.Server) *mcpsdk.ClientSession {
	t.Helper()

	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "test"}, nil)
	ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
	defer cancel()
	transport := &mcpsdk.StreamableClientTransport{Endpoint: httpServer.URL}
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// callGateTool calls starknet_blockNumber, the only tool newGateRPCServer can answer.
func callGateTool(t *testing.T, session *mcpsdk.ClientSession) *mcpsdk.CallToolResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
	defer cancel()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "starknet_blockNumber"})
	require.NoError(t, err)
	return result
}
