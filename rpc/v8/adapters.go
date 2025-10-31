package rpcv8

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
		validateInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *FunctionInvocation
	var executeInvocation *ExecuteInvocation
	var functionInvocation *FunctionInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.ConstructorInvocation))
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(adaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			if trace.FunctionInvocation.FunctionInvocation != nil {
				functionInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.FunctionInvocation.FunctionInvocation))
			} else {
				defaultResult := DefaultL1HandlerFunctionInvocation()
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

func adaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = utils.HeapPtr(adaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation))
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
			// todo(rdr): This is not the rigth fix, this casting should be unnecessary but the
			//            right fix includes a huge refactor that is better done separately
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

func AdaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
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

		if feederTrace.FeeTransferInvocation != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation))
		}

		if feederTrace.ValidateInvocation != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(feederTrace.ValidateInvocation))
		}

		var fnInvocation *FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(fct))
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
			trace.FunctionInvocation = fnInvocation
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

		toAddr, _ := new(felt.Felt).SetString(snMessage.ToAddr)

		adaptedMessages[index] = rpcv6.OrderedL2toL1Message{
			Order:   snMessage.Order,
			From:    &snFnInvocation.ContractAddress,
			To:      toAddr,
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

func DefaultL1HandlerFunctionInvocation() FunctionInvocation {
	return FunctionInvocation{
		CallType:           "CALL",
		Calldata:           []felt.Felt{},
		CallerAddress:      felt.Zero,
		Calls:              []FunctionInvocation{},
		ClassHash:          &felt.Zero,
		ContractAddress:    felt.Zero,
		EntryPointSelector: &felt.Zero,
		EntryPointType:     "L1_HANDLER",
		Events:             []rpcv6.OrderedEvent{},
		ExecutionResources: &InnerExecutionResources{
			L1Gas: 0,
			L2Gas: 0,
		},
		IsReverted: true,
		Messages:   []rpcv6.OrderedL2toL1Message{},
		Result:     []felt.Felt{},
	}
}
