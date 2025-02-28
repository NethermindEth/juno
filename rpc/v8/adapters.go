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

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *FunctionInvocation
	if trace.ValidateInvocation != nil {
		validateInvocation = utils.Ptr(adaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var executeInvocation *ExecuteInvocation
	if trace.ExecuteInvocation != nil {
		executeInvocation = utils.Ptr(adaptVMExecuteInvocation(trace.ExecuteInvocation))
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil {
		feeTransferInvocation = utils.Ptr(adaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *FunctionInvocation
	if trace.ConstructorInvocation != nil {
		constructorInvocation = utils.Ptr(adaptVMFunctionInvocation(trace.ConstructorInvocation))
	}

	var functionInvocation *FunctionInvocation
	if trace.FunctionInvocation != nil {
		functionInvocation = utils.Ptr(adaptVMFunctionInvocation(trace.FunctionInvocation))
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.Ptr(adaptVMExecutionResources(trace.ExecutionResources))
	}

	return TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             trace.StateDiff,
		ExecutionResources:    resources,
	}
}

func adaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = utils.Ptr(adaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation))
	}

	return ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: functionInvocation,
	}
}

func adaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls[index] = adaptVMFunctionInvocation(&vmFnInvocation.Calls[index])
	}

	// Adapt execution resources
	var adaptedResources *InnerExecutionResources
	if r := vmFnInvocation.ExecutionResources; r != nil {
		adaptedResources = &InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		}
	}

	return FunctionInvocation{
		ContractAddress:    vmFnInvocation.ContractAddress,
		EntryPointSelector: vmFnInvocation.EntryPointSelector,
		Calldata:           vmFnInvocation.Calldata,
		CallerAddress:      vmFnInvocation.CallerAddress,
		ClassHash:          vmFnInvocation.ClassHash,
		EntryPointType:     vmFnInvocation.EntryPointType,
		CallType:           vmFnInvocation.CallType,
		Result:             vmFnInvocation.Result,
		Calls:              adaptedCalls,
		Events:             vmFnInvocation.Events,
		Messages:           vmFnInvocation.Messages,
		ExecutionResources: adaptedResources,
	}
}

func adaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
	return ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		},
		L1DataGas: r.L1DataGas,
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

	// Adapt every feeder block trace to rpc v8 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if feederTrace.FeeTransferInvocation != nil {
			trace.FeeTransferInvocation = utils.Ptr(adaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation))
		}

		if feederTrace.ValidateInvocation != nil {
			trace.ValidateInvocation = utils.Ptr(adaptFeederFunctionInvocation(feederTrace.ValidateInvocation))
		}

		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation := utils.Ptr(adaptFeederFunctionInvocation(fct))

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
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}

func adaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = adaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
	}

	// Adapt events
	adaptedEvents := make([]vm.OrderedEvent, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]
		adaptedEvents[index] = vm.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
		}
	}

	// Adapt messages
	adaptedMessages := make([]vm.OrderedL2toL1Message, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]
		adaptedMessages[index] = vm.OrderedL2toL1Message{
			Order:   snMessage.Order,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
			To:      snMessage.ToAddr,
		}
	}

	return FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              adaptedCalls,
		Events:             adaptedEvents,
		Messages:           adaptedMessages,
		ExecutionResources: utils.Ptr(adaptFeederExecutionResources(&snFnInvocation.ExecutionResources)),
	}
}

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) InnerExecutionResources {
	var l1Gas, l2Gas uint64
	if tgs := resources.TotalGasConsumed; tgs != nil {
		l1Gas = tgs.L1Gas
		l2Gas = tgs.L2Gas
	}

	return InnerExecutionResources{
		L1Gas: l1Gas,
		L2Gas: l2Gas,
	}
}
