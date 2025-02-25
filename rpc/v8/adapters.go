package rpcv8

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) *TransactionTrace {
	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    adaptVMFunctionInvocation(trace.ValidateInvocation),
		ExecuteInvocation:     adaptVMExecuteInvocation(trace.ExecuteInvocation),
		FeeTransferInvocation: adaptVMFunctionInvocation(trace.FeeTransferInvocation),
		ConstructorInvocation: adaptVMFunctionInvocation(trace.ConstructorInvocation),
		FunctionInvocation:    adaptVMFunctionInvocation(trace.FunctionInvocation),
		StateDiff:             trace.StateDiff,
		ExecutionResources:    adaptVMExecutionResources(trace.ExecutionResources),
	}
}

func adaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) *ExecuteInvocation {
	if vmFnInvocation == nil {
		return nil
	}

	return &ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: adaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation),
	}
}

func adaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) *FunctionInvocation {
	if vmFnInvocation == nil {
		return nil
	}

	fnInvocation := FunctionInvocation{
		ContractAddress:    vmFnInvocation.ContractAddress,
		EntryPointSelector: vmFnInvocation.EntryPointSelector,
		Calldata:           vmFnInvocation.Calldata,
		CallerAddress:      vmFnInvocation.CallerAddress,
		ClassHash:          vmFnInvocation.ClassHash,
		EntryPointType:     vmFnInvocation.EntryPointType,
		CallType:           vmFnInvocation.CallType,
		Result:             vmFnInvocation.Result,
		Calls:              make([]FunctionInvocation, 0, len(vmFnInvocation.Calls)),
		Events:             vmFnInvocation.Events,
		Messages:           vmFnInvocation.Messages,
	}

	// Adapt inner calls
	for index := range vmFnInvocation.Calls {
		fnInvocation.Calls = append(fnInvocation.Calls, *adaptVMFunctionInvocation(&vmFnInvocation.Calls[index]))
	}

	// Adapt execution resources
	r := vmFnInvocation.ExecutionResources
	if r != nil {
		fnInvocation.ExecutionResources = &InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		}
	}

	return &fnInvocation
}

func adaptVMExecutionResources(r *vm.ExecutionResources) *ExecutionResources {
	if r == nil {
		return nil
	}

	execResources := &ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		},
		L1DataGas: r.L1DataGas,
	}

	return execResources
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

	traces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
	// Adapt every feeder block trace to rpc v8 trace
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type:                  TransactionType(block.Transactions[index].Type),
			FeeTransferInvocation: adaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			ValidateInvocation:    adaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
		}

		fnInvocation := adaptFeederFunctionInvocation(feederTrace.FunctionInvocation)
		switch block.Transactions[index].Type {
		case TxnDeploy, TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case TxnL1Handler:
			trace.FunctionInvocation = fnInvocation
		}

		traces = append(traces, TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		})
	}

	return traces, nil
}

func adaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) *FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	fnInvocation := FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              make([]FunctionInvocation, 0, len(snFnInvocation.InternalCalls)),
		Events:             make([]vm.OrderedEvent, 0, len(snFnInvocation.Events)),
		Messages:           make([]vm.OrderedL2toL1Message, 0, len(snFnInvocation.Messages)),
		ExecutionResources: adaptFeederExecutionResources(&snFnInvocation.ExecutionResources),
	}

	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *adaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index]))
	}
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]
		fnInvocation.Events = append(fnInvocation.Events, vm.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
		})
	}
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]
		fnInvocation.Messages = append(fnInvocation.Messages, vm.OrderedL2toL1Message{
			Order:   snMessage.Order,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
			To:      snMessage.ToAddr,
		})
	}

	return &fnInvocation
}

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) *InnerExecutionResources {
	executionResources := &InnerExecutionResources{}

	if tgs := resources.TotalGasConsumed; tgs != nil {
		executionResources.L1Gas = tgs.L1Gas
		executionResources.L2Gas = tgs.L2Gas
	}

	return executionResources
}
