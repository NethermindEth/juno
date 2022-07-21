package jsonrpc

import "errors"

const (
	parseErrorCode     = -32700
	invalidRequestCode = -32600
	methodNotFoundCode = -32601
	invalidParamsCode  = -32602
	internalErrorCode  = -32603
	serverErrorCode    = -32000
)

var (
	errParseError     = errors.New("parse error")
	errInvalidRequest = errors.New("invalid request")
	errMethodNotFound = errors.New("method not found")
	errInvalidParams  = errors.New("invalid params")
	errInternalError  = errors.New("internal error")
	errServerError    = errors.New("server error")
)

// RpcError represents an error that occurred during JSON-RPC processing.
type RpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface.
func (err *RpcError) Error() string {
	return err.Message
}

func newErrParseError(data any) *RpcError {
	return &RpcError{
		Code:    parseErrorCode,
		Message: "Parse error",
		Data:    data,
	}
}

func newErrInvalidRequest(data any) *RpcError {
	return &RpcError{
		Code:    invalidRequestCode,
		Message: "Invalid Request",
		Data:    data,
	}
}

func newErrMethodNotFound(data any) *RpcError {
	return &RpcError{
		Code:    methodNotFoundCode,
		Message: "Method not found",
		Data:    data,
	}
}

func newErrInvalidParams(data any) *RpcError {
	return &RpcError{
		Code:    invalidParamsCode,
		Message: "Invalid params",
		Data:    data,
	}
}

func newErrInternalError(data any) *RpcError {
	return &RpcError{
		Code:    internalErrorCode,
		Message: "Internal error",
		Data:    data,
	}
}

func newErrServerError(data any) *RpcError {
	return &RpcError{
		Code:    serverErrorCode,
		Message: "Server error",
		Data:    data,
	}
}
