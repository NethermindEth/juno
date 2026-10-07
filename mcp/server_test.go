package mcp_test

import (
	"cmp"
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mcp"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// callTimeout bounds every MCP call: an unexpected mock call fails the test from the server's
// goroutine, which never answers.
const callTimeout = 30 * time.Second

func TestToolsMirrorRPCMethods(t *testing.T) {
	rpcServer, methods := newRPCServer(t, nil)
	methodsByName := make(map[string]jsonrpc.Method, len(methods))
	for _, method := range methods {
		methodsByName[method.Name] = method
	}

	for _, protocolVersion := range []string{"", "2025-11-25"} {
		t.Run("protocol "+cmp.Or(protocolVersion, "latest"), func(t *testing.T) {
			session := newSession(t, rpcServer, &mcpsdk.ClientSessionOptions{
				ProtocolVersion: protocolVersion,
			})
			ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
			defer cancel()

			result, err := session.ListTools(ctx, nil)
			require.NoError(t, err)
			require.NotEmpty(t, result.Tools)

			for _, tool := range result.Tools {
				method, ok := methodsByName[tool.Name]
				require.True(t, ok, "%s is not a JSON-RPC method", tool.Name)
				for _, excluded := range []string{"starknet_add", "starknet_subscribe", "unsubscribe"} {
					assert.NotContains(t, tool.Name, excluded)
				}
				require.NotNil(t, tool.Annotations, tool.Name)
				assert.True(t, tool.Annotations.ReadOnlyHint, tool.Name)

				var params, requiredParams []string
				for _, param := range method.Params {
					params = append(params, param.Name)
					if !param.Optional {
						requiredParams = append(requiredParams, param.Name)
					}
				}
				properties, required := schemaProperties(t, tool.InputSchema)
				assert.ElementsMatch(t, params, properties, tool.Name)
				assert.ElementsMatch(t, requiredParams, required, tool.Name)
			}
		})
	}
}

func TestCallTool(t *testing.T) {
	countAtBlock5 := func(reader *mocks.MockReader) {
		reader.EXPECT().BlockTransactionCountByNumber(uint64(5)).Return(uint64(7), nil)
	}

	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		mock      func(reader *mocks.MockReader)
		want      string
		wantError string
	}{
		{
			name: "no arguments",
			tool: "starknet_blockNumber",
			mock: func(reader *mocks.MockReader) {
				reader.EXPECT().Height().Return(uint64(1234), nil)
			},
			want: "1234",
		},
		{
			name:      "block number",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": 5},
			mock:      countAtBlock5,
			want:      "7",
		},
		{
			name:      "block number as a string",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": "5"},
			mock:      countAtBlock5,
			want:      "7",
		},
		{
			name:      "block hash",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": "0x1"},
			mock: func(reader *mocks.MockReader) {
				reader.EXPECT().BlockNumberByHash(felt.NewFromUint64[felt.Felt](1)).Return(uint64(5), nil)
				countAtBlock5(reader)
			},
			want: "7",
		},
		{
			name:      "block tag",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": "latest"},
			mock: func(reader *mocks.MockReader) {
				reader.EXPECT().Height().Return(uint64(5), nil)
				countAtBlock5(reader)
			},
			want: "7",
		},
		{
			name:      "JSON-RPC error",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": 5},
			mock: func(reader *mocks.MockReader) {
				reader.EXPECT().BlockTransactionCountByNumber(uint64(5)).Return(uint64(0), db.ErrKeyNotFound)
			},
			wantError: `"message":"Block not found"`,
		},
		{
			name:      "missing argument",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{},
			wantError: "missing required params: block_id",
		},
		{
			name:      "panic in the JSON-RPC handler",
			tool:      "starknet_getBlockTransactionCount",
			arguments: map[string]any{"block_id": nil},
			wantError: "internal error",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := mocks.NewMockReader(gomock.NewController(t))
			if test.mock != nil {
				test.mock(reader)
			}
			rpcServer, _ := newRPCServer(t, reader)
			session := newSession(t, rpcServer, nil)
			ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
			defer cancel()

			result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      test.tool,
				Arguments: test.arguments,
			})
			require.NoError(t, err)

			text := resultText(t, result)
			if test.wantError != "" {
				assert.True(t, result.IsError, text)
				assert.Contains(t, text, test.wantError)
				return
			}
			assert.False(t, result.IsError, text)
			assert.Equal(t, test.want, text)
		})
	}
}

func TestCallSelectorByName(t *testing.T) {
	rpcServer := jsonrpc.NewServer(pool.New().WithMaxGoroutines(1), log.NewNopZapLogger()).
		WithValidator(rpcv10.Validator())
	require.NoError(t, rpcServer.RegisterMethods(jsonrpc.Method{
		Name:   "starknet_call",
		Params: []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
		Handler: func(call *rpcv10.FunctionCall, _ *rpcv10.BlockID) (*felt.Felt, *jsonrpc.Error) {
			return &call.EntryPointSelector, nil
		},
	}))
	session := newSession(t, rpcServer, nil)

	balanceOf := crypto.StarknetKeccak([]byte("balance_of"))
	tests := map[string]string{
		"balance_of": balanceOf.String(),
		"0x2":        "0x2",
	}
	for selector, want := range tests {
		t.Run(selector, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
			defer cancel()

			result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
				Name: "starknet_call",
				Arguments: map[string]any{
					"request": map[string]any{
						"contract_address":     "0x1",
						"entry_point_selector": selector,
						"calldata":             []string{},
					},
					"block_id": "latest",
				},
			})
			require.NoError(t, err)

			text := resultText(t, result)
			assert.False(t, result.IsError, text)
			assert.Equal(t, strconv.Quote(want), text)
		})
	}
}

// newRPCServer returns a JSON-RPC server serving the v0.10 methods, and those methods.
func newRPCServer(t *testing.T, reader *mocks.MockReader) (*jsonrpc.Server, []jsonrpc.Method) {
	t.Helper()

	handler := rpc.New(reader, nil, nil, "test", log.NewNopZapLogger(), &networks.Mainnet)
	methods, _ := handler.MethodsV0_10()
	server := jsonrpc.NewServer(pool.New().WithMaxGoroutines(1), log.NewNopZapLogger()).
		WithValidator(rpcv10.Validator())
	require.NoError(t, server.RegisterMethods(methods...))
	return server, methods
}

func newSession(
	t *testing.T, rpcServer *jsonrpc.Server, opts *mcpsdk.ClientSessionOptions,
) *mcpsdk.ClientSession {
	t.Helper()

	server := httptest.NewServer(mcp.New(rpcServer, "test", log.NewNopZapLogger()))
	t.Cleanup(server.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "test"}, nil)
	ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
	defer cancel()
	transport := &mcpsdk.StreamableClientTransport{Endpoint: server.URL}
	session, err := client.Connect(ctx, transport, opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// schemaProperties returns the property names of an input schema, and the required ones.
func schemaProperties(t *testing.T, inputSchema any) (properties, required []string) {
	t.Helper()

	schema, ok := inputSchema.(map[string]any)
	require.True(t, ok)
	if schemaProperties, ok := schema["properties"].(map[string]any); ok {
		for name := range schemaProperties {
			properties = append(properties, name)
		}
	}
	if schemaRequired, ok := schema["required"].([]any); ok {
		for _, name := range schemaRequired {
			required = append(required, name.(string))
		}
	}
	return properties, required
}

func resultText(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()

	require.Len(t, result.Content, 1)
	content, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	return strings.TrimSpace(content.Text)
}
