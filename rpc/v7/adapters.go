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
	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    rpcv6.AdaptVMFunctionInvocation(trace.ValidateInvocation),
		ExecuteInvocation:     rpcv6.AdaptVMExecuteInvocation(trace.ExecuteInvocation),
		FeeTransferInvocation: rpcv6.AdaptVMFunctionInvocation(trace.FeeTransferInvocation),
		ConstructorInvocation: rpcv6.AdaptVMFunctionInvocation(trace.ConstructorInvocation),
		FunctionInvocation:    rpcv6.AdaptVMFunctionInvocation(trace.FunctionInvocation),
		StateDiff:             rpcv6.AdaptVMStateDiff(trace.StateDiff),
		ExecutionResources:    adaptVMExecutionResources(trace.ExecutionResources),
	}
}

func adaptVMExecutionResources(r *vm.ExecutionResources) *ExecutionResources {
	if r == nil {
		return nil
	}

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

func adaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
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
			Type:                  block.Transactions[index].Type,
			FeeTransferInvocation: rpcv6.AdaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			ValidateInvocation:    rpcv6.AdaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
		}

		fnInvocation := rpcv6.AdaptFeederFunctionInvocation(feederTrace.FunctionInvocation)
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

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}
