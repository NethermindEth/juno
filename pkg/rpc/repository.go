package rpc

type (
	// A HandlerJsonRpc has JSON-RPC method functions.
	HandlerJsonRpc struct {
		StructRpc interface{}
	}
)

func NewHandlerJsonRpc(rpc interface{}) *HandlerJsonRpc {
	return &HandlerJsonRpc{
		StructRpc: rpc,
	}
}
