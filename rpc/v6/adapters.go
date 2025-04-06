package rpcv6

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
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
	var functionInvocation *FunctionInvocation

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
			functionInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(trace.FunctionInvocation))
		}
	}

	var stateDiff *rpcv8.StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(rpcv8.AdaptVMStateDiff(trace.StateDiff))
	}

	return TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
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
	adaptedEvents := make([]rpcv8.OrderedEvent, len(vmFnInvocation.Events))
	for index := range vmFnInvocation.Events {
		vmEvent := &vmFnInvocation.Events[index]

		adaptedEvents[index] = rpcv8.OrderedEvent{
			Order: vmEvent.Order,
			Keys:  vmEvent.Keys,
			Data:  vmEvent.Data,
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv8.OrderedL2toL1Message, len(vmFnInvocation.Messages))
	for index := range vmFnInvocation.Messages {
		vmMessage := &vmFnInvocation.Messages[index]

		toAddr, _ := new(felt.Felt).SetString(vmMessage.To)

		adaptedMessages[index] = rpcv8.OrderedL2toL1Message{
			Order:   vmMessage.Order,
			From:    vmMessage.From,
			To:      toAddr,
			Payload: vmMessage.Payload,
		}
	}

	// Adapt execution resources
	var adaptedResources *ComputationResources
	if r := vmFnInvocation.ExecutionResources; r != nil {
		adaptedResources = &ComputationResources{
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

	// Adapt every feeder block trace to rpc v6 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if fee := feederTrace.FeeTransferInvocation; fee != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(AdaptFeederFunctionInvocation(fee))
		}

		if val := feederTrace.ValidateInvocation; val != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(AdaptFeederFunctionInvocation(val))
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
			trace.FunctionInvocation = fnInvocation
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) FunctionInvocation {
	// Adapt internal calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
	}

	// Adapt events
	adaptedEvents := make([]rpcv8.OrderedEvent, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		adaptedEvents[index] = rpcv8.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.HeapPtr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.HeapPtr[felt.Felt]),
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv8.OrderedL2toL1Message, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		toAddr, _ := new(felt.Felt).SetString(snMessage.ToAddr)

		adaptedMessages[index] = rpcv8.OrderedL2toL1Message{
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
	}
}

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) ComputationResources {
	builtins := &resources.BuiltinInstanceCounter

	return ComputationResources{
		Steps:        resources.Steps,
		MemoryHoles:  resources.MemoryHoles,
		Pedersen:     builtins.Pedersen,
		RangeCheck:   builtins.RangeCheck,
		Bitwise:      builtins.Bitwise,
		Ecdsa:        builtins.Ecsda,
		EcOp:         builtins.EcOp,
		Keccak:       builtins.Keccak,
		Poseidon:     builtins.Poseidon,
		SegmentArena: builtins.SegmentArena,
	}
}
