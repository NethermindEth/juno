package jsonrpc

type rpcResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	Id      any    `json:"id,omitempty"`
}
