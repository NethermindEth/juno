package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// toolGate admits tool calls through the gate that bounds the node's concurrent RPC requests.
type toolGate struct {
	*jsonrpc.Gate
	// busyLogger is sampled, so that a busy node doesn't flood the log.
	busyLogger log.StructuredLogger
}

// WithGate makes every tool call take a processing slot from gate before it is forwarded to the
// JSON-RPC server, so that tool calls count towards the same limit as RPC requests. Like HTTP RPC
// requests, tool calls wait in the gate's queue until a slot frees up or their timeout expires.
// A nil gate means no limit.
func (s *Server) WithGate(gate *jsonrpc.Gate) *Server {
	if gate == nil {
		s.gate = nil
		return s
	}

	const busyLogInterval = time.Second
	s.gate = &toolGate{
		Gate:       gate,
		busyLogger: log.Sampled(s.logger, busyLogInterval, 1, 0),
	}
	return s
}

// admit takes a processing slot for a tool call. It returns the function that frees the slot or,
// when no slot can be had, the result to return instead. In that case the call must not run.
func (s *Server) admit(ctx context.Context) (release func(), rejected *mcpsdk.CallToolResult) {
	if s.gate == nil {
		return func() {}, nil
	}

	err := s.gate.Acquire(ctx)
	switch {
	case err == nil:
		return s.gate.Release, nil
	case errors.Is(err, jsonrpc.ErrServerBusy):
		s.gate.logBusy()
		return nil, busyResult(nil)
	case errors.Is(err, context.DeadlineExceeded):
		return nil, busyResult("request timed out while queued")
	default:
		// The client has gone away, so nobody reads this result.
		return nil, errorResult(err.Error())
	}
}

func (g *toolGate) logBusy() {
	g.busyLogger.Warn("Rejected MCP tool call: server is busy",
		zap.Int("running", g.Running()),
		zap.Int("queued", g.Queued()),
		zap.Uint64("rejected", g.Rejected()),
	)
}

// busyResult returns the JSON-RPC error of a request that was not executed because the server had
// no free capacity.
func busyResult(data any) *mcpsdk.CallToolResult {
	rpcError, err := json.Marshal(jsonrpc.Err(jsonrpc.ServerBusy, data))
	if err != nil {
		return errorResult(err.Error())
	}
	return errorResult(string(rpcError))
}
