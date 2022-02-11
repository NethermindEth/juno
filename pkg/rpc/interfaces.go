package rpc

import (
	"github.com/osamingo/jsonrpc/v2"
)

type HandleParamsResulter interface {
	jsonrpc.Handler
	Name() string
	Params() interface{}
	Result() interface{}
}

type ServiceDispatcher interface {
	MethodName(HandleParamsResulter) string
	Handlers() []HandleParamsResulter
}
