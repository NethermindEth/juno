package rpcv9

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *FunctionInvocation
	var executeInvocation *ExecuteInvocation
	var functionInvocation *ExecuteInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(trace.ConstructorInvocation))
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(AdaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			functionInvocation = utils.HeapPtr(AdaptVMExecuteInvocation(trace.FunctionInvocation))
		}
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.HeapPtr(AdaptVMExecutionResources(trace.ExecutionResources))
	}

	var stateDiff *rpcv6.StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(rpcv6.AdaptVMStateDiff(trace.StateDiff))
	}

	return TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
		ExecutionResources:    resources,
	}
}

func AdaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation))
	}

	return ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: functionInvocation,
	}
}

func AdaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls[index] = AdaptVMFunctionInvocation(&vmFnInvocation.Calls[index])
	}

	// Adapt events
	adaptedEvents := make([]rpcv6.OrderedEvent, len(vmFnInvocation.Events))
	for index := range vmFnInvocation.Events {
		vmEvent := &vmFnInvocation.Events[index]

		adaptedEvents[index] = rpcv6.OrderedEvent{
			Order: vmEvent.Order,
			Keys:  vmEvent.Keys,
			Data:  vmEvent.Data,
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv6.OrderedL2toL1Message, len(vmFnInvocation.Messages))
	for index := range vmFnInvocation.Messages {
		vmMessage := &vmFnInvocation.Messages[index]

		adaptedMessages[index] = rpcv6.OrderedL2toL1Message{
			Order: vmMessage.Order,
			From:  vmMessage.From,
			// todo(rdr): we shouldn't need to do this casting but it affects a lot of code to
			//            do it right
			To:      (*felt.Felt)(vmMessage.To),
			Payload: vmMessage.Payload,
		}
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
		Events:             adaptedEvents,
		Messages:           adaptedMessages,
		ExecutionResources: adaptedResources,
		IsReverted:         vmFnInvocation.IsReverted,
	}
}

func AdaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
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

func AdaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}

	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}

	// Adapt every feeder block trace to rpc v9 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if feederTrace.FeeTransferInvocation != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(
				AdaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			)
		}

		if feederTrace.ValidateInvocation != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(
				AdaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
			)
		}

		var fnInvocation *FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(AdaptFeederFunctionInvocation(fct))
		}

		switch trace.Type {
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
			trace.FunctionInvocation = new(ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.FunctionInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.FunctionInvocation.FunctionInvocation = fnInvocation
			}
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
	}

	// Adapt events
	adaptedEvents := make([]rpcv6.OrderedEvent, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		adaptedEvents[index] = rpcv6.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.HeapPtr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.HeapPtr[felt.Felt]),
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv6.OrderedL2toL1Message, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		toAddr, _ := felt.FromString[felt.Felt](snMessage.ToAddr)

		adaptedMessages[index] = rpcv6.OrderedL2toL1Message{
			Order:   snMessage.Order,
			From:    &snFnInvocation.ContractAddress,
			To:      &toAddr,
			Payload: utils.Map(snMessage.Payload, utils.HeapPtr[felt.Felt]),
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
		ExecutionResources: utils.HeapPtr(adaptFeederExecutionResources(&snFnInvocation.ExecutionResources)),
		IsReverted:         snFnInvocation.Failed,
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
