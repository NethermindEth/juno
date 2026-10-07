// Package mcp serves part of the Starknet JSON-RPC API as Model Context Protocol (MCP) tools.
//
// Every tool mirrors the JSON-RPC method of the same name. Tool calls are forwarded in-process to
// the node's JSON-RPC server, so their parameters, validation, errors and results are those of
// the RPC.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const instructions = "Juno is a Starknet full node. Its tools mirror the Starknet JSON-RPC v0.10 " +
	"methods of the same name: same parameters, same JSON results. Felts, addresses and " +
	"hashes are 0x-prefixed hex strings. A block_id is \"latest\", \"pre_confirmed\", " +
	"\"l1_accepted\", a block number or a block hash. An entry_point_selector can be given " +
	"as the function name, such as \"balance_of\"."

// Server serves the MCP tools over the streamable HTTP transport.
type Server struct {
	rpc            *jsonrpc.Server
	logger         log.StructuredLogger
	requestTimeout time.Duration
	handler        http.Handler
}

// New creates an MCP server whose tool calls are answered by rpc, which must serve the v0.10
// Starknet JSON-RPC API.
func New(rpc *jsonrpc.Server, version string, logger log.StructuredLogger) *Server {
	s := &Server{
		rpc:    rpc,
		logger: logger,
	}

	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "juno", Version: version},
		&mcpsdk.ServerOptions{
			Instructions: instructions,
			// A stateless server can't notify clients of changes, so only tools are advertised.
			Capabilities: &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{}},
		},
	)
	for i := range tools {
		server.AddTool(tools[i].mcpTool(), s.toolHandler(&tools[i]))
	}

	streamable := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		&mcpsdk.StreamableHTTPOptions{
			// Tool calls are plain request-response exchanges, so no session is kept.
			Stateless:                    true,
			JSONResponse:                 true,
			PropagateRequestCancellation: true,
		},
	)
	s.handler = http.NewCrossOriginProtection().Handler(streamable)
	return s
}

// WithRequestTimeout sets the maximum duration of a tool call. Zero means no limit.
func (s *Server) WithRequestTimeout(timeout time.Duration) *Server {
	s.requestTimeout = timeout
	return s
}

// ServeHTTP handles an MCP request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) toolHandler(t *tool) mcpsdk.ToolHandler {
	return func(
		ctx context.Context, req *mcpsdk.CallToolRequest,
	) (result *mcpsdk.CallToolResult, _ error) {
		// Tool handlers run on goroutines of the MCP SDK, where a panic would crash the node.
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("Recovered from a panic in an MCP tool call",
					zap.String("tool", t.name),
					zap.Any("panic", recovered),
					zap.Stack("stack"),
				)
				result = errorResult("internal error")
			}
		}()

		var arguments json.RawMessage
		if req.Params != nil {
			arguments = req.Params.Arguments
		}
		params, err := t.params(arguments)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		if s.requestTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.requestTimeout)
			defer cancel()
		}
		return s.call(ctx, t.name, params), nil
	}
}

// call forwards a request to the JSON-RPC server and turns its response into a tool result.
// Errors are returned as tool results too, so that the model can see them and adjust its call.
func (s *Server) call(
	ctx context.Context, method string, params map[string]json.RawMessage,
) *mcpsdk.CallToolResult {
	// A request without id is a notification, which gets no response.
	request := jsonrpc.Request{Version: "2.0", Method: method, ID: 1}
	if len(params) > 0 {
		request.Params = params
	}
	body, err := json.Marshal(request)
	if err != nil {
		return errorResult(err.Error())
	}

	responseBody, _, err := s.rpc.HandleReader(ctx, bytes.NewReader(body))
	if err != nil {
		return errorResult(err.Error())
	}

	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *jsonrpc.Error  `json:"error"`
	}
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return errorResult(err.Error())
	}
	if response.Error != nil {
		rpcError, err := json.Marshal(response.Error)
		if err != nil {
			return errorResult(err.Error())
		}
		return errorResult(string(rpcError))
	}
	if response.Result == nil {
		return textResult("null")
	}
	return textResult(string(response.Result))
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}}}
}

func errorResult(text string) *mcpsdk.CallToolResult {
	result := textResult(text)
	result.IsError = true
	return result
}
