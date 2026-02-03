package rpcv7

import (
	"errors"

	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *rpcv6.FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *rpcv6.FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *rpcv6.FunctionInvocation
	var executeInvocation *rpcv6.ExecuteInvocation
	var functionInvocation *rpcv6.FunctionInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.ConstructorInvocation))
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(rpcv6.AdaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			if trace.FunctionInvocation.FunctionInvocation != nil {
				functionInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.FunctionInvocation.FunctionInvocation))
			} else {
				defaultResult := rpcv6.DefaultL1HandlerFunctionInvocation()
				functionInvocation = &defaultResult
			}
		}
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.HeapPtr(adaptVMExecutionResources(trace.ExecutionResources))
	}

	var stateDiff *rpcv6.StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(rpcv6.AdaptVMStateDiff(trace.StateDiff))
	}

	traceType := TransactionType(trace.Type)
	if traceType == TxnDeploy {
		// There is no DEPLOY_TXN_TRACE thus we need to convert the type to `DEPLOY_ACCOUNT`
		//nolint:lll // url exceeds line limit
		// see https://github.com/starkware-libs/starknet-specs/blob/76bdde23c7dae370a3340e40f7ca2ef2520e75b9/api/starknet_trace_api_openrpc.json#L150
		traceType = TxnDeployAccount
	}

	return TransactionTrace{
		Type:                  traceType,
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
		ExecutionResources:    resources,
	}
}

func adaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
	// Adapt data availability
	var adaptedDataAvailability *DataAvailability
	if r.DataAvailability != nil {
		adaptedDataAvailability = &DataAvailability{
			L1Gas:     r.DataAvailability.L1Gas,
			L1DataGas: r.DataAvailability.L1DataGas,
		}
	}

	return ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        r.Steps,
			MemoryHoles:  r.MemoryHoles,
			Pedersen:     r.Pedersen,
			RangeCheck:   r.RangeCheck,
			Bitwise:      r.Bitwise,
			Ecdsa:        r.Ecdsa,
			EcOp:         r.EcOp,
			Keccak:       r.Keccak,
			Poseidon:     r.Poseidon,
			SegmentArena: r.SegmentArena,
		},
		DataAvailability: adaptedDataAvailability,
	}
}

/****************************************************
		Feeder Adapters
*****************************************************/

func AdaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}

	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}

	// Adapt every feeder block trace to rpc v7 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if fee := feederTrace.FeeTransferInvocation; fee != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(fee))
		}

		if val := feederTrace.ValidateInvocation; val != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(val))
		}

		var fnInvocation *rpcv6.FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(fct))
		}

		switch trace.Type {
		case TxnDeploy, TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(rpcv6.ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case TxnL1Handler:
			trace.FunctionInvocation = fnInvocation
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}
