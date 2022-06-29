package jsonrpc

type rpcResponse struct {
	Jsonrpc string    `json:"jsonrpc"`
	Result  any       `json:"result,omitempty"`
	Error   *RpcError `json:"error,omitempty"`
	Id      any       `json:"id"`
}
