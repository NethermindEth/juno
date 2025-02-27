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
	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    AdaptVMFunctionInvocation(trace.ValidateInvocation),
		ExecuteInvocation:     AdaptVMExecuteInvocation(trace.ExecuteInvocation),
		FeeTransferInvocation: AdaptVMFunctionInvocation(trace.FeeTransferInvocation),
		ConstructorInvocation: AdaptVMFunctionInvocation(trace.ConstructorInvocation),
		FunctionInvocation:    AdaptVMFunctionInvocation(trace.FunctionInvocation),
		StateDiff:             AdaptVMStateDiff(trace.StateDiff),
	}
}

func AdaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) *ExecuteInvocation {
	if vmFnInvocation == nil {
		return nil
	}

	return &ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: AdaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation),
	}
}

func AdaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) *FunctionInvocation {
	if vmFnInvocation == nil {
		return nil
	}

	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, 0, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls = append(adaptedCalls, *AdaptVMFunctionInvocation(&vmFnInvocation.Calls[index]))
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
	adaptedStorageDiffs := make([]StorageDiff, 0, len(vmStateDiff.StorageDiffs))
	for index := range vmStateDiff.StorageDiffs {
		vmStorageDiff := &vmStateDiff.StorageDiffs[index]

		// Adapt storage entries
		adaptedEntries := make([]Entry, 0, len(vmStorageDiff.StorageEntries))
		for entryIndex := range vmStorageDiff.StorageEntries {
			vmEntry := &vmStorageDiff.StorageEntries[entryIndex]

			adaptedEntries = append(adaptedEntries, Entry{
				Key:   vmEntry.Key,
				Value: vmEntry.Value,
			})
		}

		adaptedStorageDiffs = append(adaptedStorageDiffs, StorageDiff{
			Address:        vmStorageDiff.Address,
			StorageEntries: adaptedEntries,
		})
	}

	// Adapt adaptedNonces
	adaptedNonces := make([]Nonce, 0, len(vmStateDiff.Nonces))
	for index := range vmStateDiff.Nonces {
		vmNonce := &vmStateDiff.Nonces[index]

		adaptedNonces = append(adaptedNonces, Nonce{
			ContractAddress: vmNonce.ContractAddress,
			Nonce:           vmNonce.Nonce,
		})
	}

	// Adapt deployed contracts
	adaptedDeployedContracts := make([]DeployedContract, 0, len(vmStateDiff.DeployedContracts))
	for index := range vmStateDiff.DeployedContracts {
		vmDeployedContract := &vmStateDiff.DeployedContracts[index]

		adaptedDeployedContracts = append(adaptedDeployedContracts, DeployedContract{
			Address:   vmDeployedContract.Address,
			ClassHash: vmDeployedContract.ClassHash,
		})
	}

	// Adapt declared classes
	adaptedDeclaredClasses := make([]DeclaredClass, 0, len(vmStateDiff.DeclaredClasses))
	for index := range vmStateDiff.DeclaredClasses {
		vmDeclaredClass := &vmStateDiff.DeclaredClasses[index]

		adaptedDeclaredClasses = append(adaptedDeclaredClasses, DeclaredClass{
			ClassHash:         vmDeclaredClass.ClassHash,
			CompiledClassHash: vmDeclaredClass.CompiledClassHash,
		})
	}

	// Adapt replaced classes
	adaptedReplacedClasses := make([]ReplacedClass, 0, len(vmStateDiff.ReplacedClasses))
	for index := range vmStateDiff.ReplacedClasses {
		vmReplacedClass := &vmStateDiff.ReplacedClasses[index]

		adaptedReplacedClasses = append(adaptedReplacedClasses, ReplacedClass{
			ContractAddress: vmReplacedClass.ContractAddress,
			ClassHash:       vmReplacedClass.ClassHash,
		})
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

func adaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}

	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}

	// Adapt every feeder block trace to rpc v6 trace
	adaptedTraces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type:                  block.Transactions[index].Type,
			FeeTransferInvocation: AdaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			ValidateInvocation:    AdaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
		}

		fnInvocation := AdaptFeederFunctionInvocation(feederTrace.FunctionInvocation)
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

		adaptedTraces = append(adaptedTraces, TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		})
	}

	return adaptedTraces, nil
}

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) *FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	// Adapt internal calls
	adaptedCalls := make([]FunctionInvocation, 0, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls = append(adaptedCalls, *AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index]))
	}

	// Adapt events
	adaptedEvents := make([]vm.OrderedEvent, 0, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		adaptedEvents = append(adaptedEvents, vm.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
		})
	}

	// Adapt messages
	adaptedMessages := make([]vm.OrderedL2toL1Message, 0, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		adaptedMessages = append(adaptedMessages, vm.OrderedL2toL1Message{
			Order:   snMessage.Order,
			To:      snMessage.ToAddr,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
		})
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
