package jsonrpc

const (
	parseErrorCode     = -32700
	invalidRequestCode = -32600
	methodNotFoundCode = -32601
	invalidParamsCode  = -32602
	internalErrorCode  = -32603
)

var (
	errParseError = &RpcError{
		Code:    parseErrorCode,
		Message: "Parse error",
	}
	errInvalidRequest = &RpcError{
		Code:    invalidRequestCode,
		Message: "Invalid Request",
	}
	errMethodNotFound = &RpcError{
		Code:    methodNotFoundCode,
		Message: "Method not found",
	}
	errInvalidParams = &RpcError{
		Code:    invalidParamsCode,
		Message: "Invalid params",
	}
	errInternalError = &RpcError{
		Code:    internalErrorCode,
		Message: "Internal error",
	}
)

// RpcError represents an error that occurred during JSON-RPC processing.
type RpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface.
// notest
func (err *RpcError) Error() string {
	return err.Message
}
