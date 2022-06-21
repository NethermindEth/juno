package rpcnew

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeServerErrorMin = -32000
	CodeServerErrorMax = -32099
)

func NewErrParseError(data interface{}) *RPCError {
	return &RPCError{
		Code:    -32700,
		Message: "Parse error",
		Data:    data,
	}
}

func NewErrInternalError(data interface{}) *RPCError {
	return &RPCError{
		Code:    -32603,
		Message: "Internal error",
		Data:    data,
	}
}

func NewErrInvalidRequest(data interface{}) *RPCError {
	return &RPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    data,
	}
}

func NewErrMethodNotFound(data interface{}) *RPCError {
	return &RPCError{
		Code:    -32601,
		Message: "Method not found",
		Data:    data,
	}
}

func NewErrInvalidParams(data interface{}) *RPCError {
	return &RPCError{
		Code:    -32602,
		Message: "Invalid params",
		Data:    data,
	}
}

func NewErrServerError(code int64, data interface{}) *RPCError {
	return &RPCError{
		Code:    code,
		Message: "Server error",
		Data:    data,
	}
}

func IsParseError(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsParseError()
	}
	return false
}

func IsInvalidRequest(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsInvalidRequest()
	}
	return false
}

func IsMethodNotFound(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsMethodNotFound()
	}
	return false
}

func IsInvalidParams(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsInvalidParams()
	}
	return false
}

func IsInternalError(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsInternalError()
	}
	return false
}

func IsServerError(err error) bool {
	if rpcErr, ok := err.(RPCError); ok {
		return rpcErr.IsServerError()
	}
	return false
}

type RPCError struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err RPCError) Error() string {
	return err.Message
}

func (err *RPCError) IsParseError() bool {
	return err.Code == CodeParseError
}

func (err *RPCError) IsInvalidRequest() bool {
	return err.Code == CodeInvalidRequest
}

func (err *RPCError) IsMethodNotFound() bool {
	return err.Code == CodeMethodNotFound
}

func (err *RPCError) IsInvalidParams() bool {
	return err.Code == CodeInvalidParams
}

func (err *RPCError) IsInternalError() bool {
	return err.Code == CodeInternalError
}

func (err *RPCError) IsServerError() bool {
	return CodeServerErrorMin <= err.Code && err.Code <= CodeServerErrorMax
}
