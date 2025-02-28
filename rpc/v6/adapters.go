package rpcv6

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
	var validateInvocation *FunctionInvocation
	if trace.ValidateInvocation != nil {
		validateInvocation = AdaptVMFunctionInvocation(trace.ValidateInvocation)
	}

	var executeInvocation *ExecuteInvocation
	if trace.ExecuteInvocation != nil {
		executeInvocation = AdaptVMExecuteInvocation(trace.ExecuteInvocation)
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil {
		feeTransferInvocation = AdaptVMFunctionInvocation(trace.FeeTransferInvocation)
	}

	var constructorInvocation *FunctionInvocation
	if trace.ConstructorInvocation != nil {
		constructorInvocation = AdaptVMFunctionInvocation(trace.ConstructorInvocation)
	}

	var functionInvocation *FunctionInvocation
	if trace.FunctionInvocation != nil {
		functionInvocation = AdaptVMFunctionInvocation(trace.FunctionInvocation)
	}

	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             AdaptVMStateDiff(trace.StateDiff),
	}
}

func AdaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) *ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = AdaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation)
	}

	return &ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: functionInvocation,
	}
}

func AdaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) *FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls[index] = *AdaptVMFunctionInvocation(&vmFnInvocation.Calls[index])
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

	return &FunctionInvocation{
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

func AdaptVMStateDiff(vmStateDiff *vm.StateDiff) *StateDiff {
	// Adapt storage diffs
	adaptedStorageDiffs := make([]StorageDiff, len(vmStateDiff.StorageDiffs))
	for index := range vmStateDiff.StorageDiffs {
		vmStorageDiff := &vmStateDiff.StorageDiffs[index]

		// Adapt storage entries
		adaptedEntries := make([]Entry, len(vmStorageDiff.StorageEntries))
		for entryIndex := range vmStorageDiff.StorageEntries {
			vmEntry := &vmStorageDiff.StorageEntries[entryIndex]

			adaptedEntries[entryIndex] = Entry{
				Key:   vmEntry.Key,
				Value: vmEntry.Value,
			}
		}

		adaptedStorageDiffs[index] = StorageDiff{
			Address:        vmStorageDiff.Address,
			StorageEntries: adaptedEntries,
		}
	}

	// Adapt nonces
	adaptedNonces := make([]Nonce, len(vmStateDiff.Nonces))
	for index := range vmStateDiff.Nonces {
		vmNonce := &vmStateDiff.Nonces[index]

		adaptedNonces[index] = Nonce{
			ContractAddress: vmNonce.ContractAddress,
			Nonce:           vmNonce.Nonce,
		}
	}

	// Adapt deployed contracts
	adaptedDeployedContracts := make([]DeployedContract, len(vmStateDiff.DeployedContracts))
	for index := range vmStateDiff.DeployedContracts {
		vmDeployedContract := &vmStateDiff.DeployedContracts[index]

		adaptedDeployedContracts[index] = DeployedContract{
			Address:   vmDeployedContract.Address,
			ClassHash: vmDeployedContract.ClassHash,
		}
	}

	// Adapt declared classes
	adaptedDeclaredClasses := make([]DeclaredClass, len(vmStateDiff.DeclaredClasses))
	for index := range vmStateDiff.DeclaredClasses {
		vmDeclaredClass := &vmStateDiff.DeclaredClasses[index]

		adaptedDeclaredClasses[index] = DeclaredClass{
			ClassHash:         vmDeclaredClass.ClassHash,
			CompiledClassHash: vmDeclaredClass.CompiledClassHash,
		}
	}

	// Adapt replaced classes
	adaptedReplacedClasses := make([]ReplacedClass, len(vmStateDiff.ReplacedClasses))
	for index := range vmStateDiff.ReplacedClasses {
		vmReplacedClass := &vmStateDiff.ReplacedClasses[index]

		adaptedReplacedClasses[index] = ReplacedClass{
			ContractAddress: vmReplacedClass.ContractAddress,
			ClassHash:       vmReplacedClass.ClassHash,
		}
	}

	return &StateDiff{
		StorageDiffs:              adaptedStorageDiffs,
		Nonces:                    adaptedNonces,
		DeployedContracts:         adaptedDeployedContracts,
		DeprecatedDeclaredClasses: vmStateDiff.DeprecatedDeclaredClasses,
		DeclaredClasses:           adaptedDeclaredClasses,
		ReplacedClasses:           adaptedReplacedClasses,
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

		if fee := feederTrace.FeeTransferInvocation; fee != nil {
			trace.FeeTransferInvocation = AdaptFeederFunctionInvocation(fee)
		}

		if val := feederTrace.ValidateInvocation; val != nil {
			trace.ValidateInvocation = AdaptFeederFunctionInvocation(val)
		}

		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation := AdaptFeederFunctionInvocation(fct)

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

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) *FunctionInvocation {
	// Adapt internal calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = *AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
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
			To:      snMessage.ToAddr,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
		}
	}

	return &FunctionInvocation{
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
		ExecutionResources: adaptFeederExecutionResources(&snFnInvocation.ExecutionResources),
	}
}

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) *ComputationResources {
	builtins := &resources.BuiltinInstanceCounter

	return &ComputationResources{
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
