package jsonrpc

const (
	parseErrorCode     = -32700
	invalidRequestCode = -32600
	methodNotFoundCode = -32601
	invalidParamsCode  = -32602
	internalErrorCode  = -32603
)

var (
	ErrParseError = &RpcError{
		Code:    parseErrorCode,
		Message: "Parse error",
	}
	ErrInvalidRequest = &RpcError{
		Code:    invalidRequestCode,
		Message: "Invalid Request",
	}
	ErrMethodNotFound = &RpcError{
		Code:    methodNotFoundCode,
		Message: "Method not found",
	}
	ErrInvalidParams = &RpcError{
		Code:    invalidParamsCode,
		Message: "Invalid params",
	}
	ErrInternalError = &RpcError{
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

func NewInternalError(message string) *RpcError {
	return &RpcError{
		Code:    internalErrorCode,
		Message: message,
	}
}
