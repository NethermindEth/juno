package rpcv7

import (
	"errors"

	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) *TransactionTrace {
	var validateInvocation *rpcv6.FunctionInvocation
	if trace.ValidateInvocation != nil {
		validateInvocation = rpcv6.AdaptVMFunctionInvocation(trace.ValidateInvocation)
	}

	var executeInvocation *rpcv6.ExecuteInvocation
	if trace.ExecuteInvocation != nil {
		executeInvocation = rpcv6.AdaptVMExecuteInvocation(trace.ExecuteInvocation)
	}

	var feeTransferInvocation *rpcv6.FunctionInvocation
	if trace.FeeTransferInvocation != nil {
		feeTransferInvocation = rpcv6.AdaptVMFunctionInvocation(trace.FeeTransferInvocation)
	}

	var constructorInvocation *rpcv6.FunctionInvocation
	if trace.ConstructorInvocation != nil {
		constructorInvocation = rpcv6.AdaptVMFunctionInvocation(trace.ConstructorInvocation)
	}

	var functionInvocation *rpcv6.FunctionInvocation
	if trace.FunctionInvocation != nil {
		functionInvocation = rpcv6.AdaptVMFunctionInvocation(trace.FunctionInvocation)
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = adaptVMExecutionResources(trace.ExecutionResources)
	}

	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             rpcv6.AdaptVMStateDiff(trace.StateDiff),
		ExecutionResources:    resources,
	}
}

func adaptVMExecutionResources(r *vm.ExecutionResources) *ExecutionResources {
	// Adapt data availability
	var adaptedDataAvailability *DataAvailability
	if r.DataAvailability != nil {
		adaptedDataAvailability = &DataAvailability{
			L1Gas:     r.DataAvailability.L1Gas,
			L1DataGas: r.DataAvailability.L1DataGas,
		}
	}

	return &ExecutionResources{
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

		if fee := feederTrace.FeeTransferInvocation; fee != nil {
			trace.FeeTransferInvocation = rpcv6.AdaptFeederFunctionInvocation(fee)
		}

		if val := feederTrace.ValidateInvocation; val != nil {
			trace.ValidateInvocation = rpcv6.AdaptFeederFunctionInvocation(val)
		}

		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation := rpcv6.AdaptFeederFunctionInvocation(fct)

			switch block.Transactions[index].Type {
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
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}
