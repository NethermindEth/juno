package rpccore

import "github.com/NethermindEth/juno/core"

type ExecutionResourcesLike interface {
	IsExecutionResourcesLike()
}

type TypeFactory interface {
	ToExecutionResources(e *core.ExecutionResources) ExecutionResourcesLike
}
