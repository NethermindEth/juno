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
		fnInvocation.Calls = append(fnInvocation.Calls, *AdaptVMFunctionInvocation(&vmFnInvocation.Calls[index]))
	}

	// Adapt execution resources
	r := vmFnInvocation.ExecutionResources
	if r != nil {
		fnInvocation.ExecutionResources = &ComputationResources{
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

	return &fnInvocation
}

func AdaptVMStateDiff(vmStateDiff *vm.StateDiff) *StateDiff {
	stateDiff := &StateDiff{
		StorageDiffs:              make([]StorageDiff, 0, len(vmStateDiff.StorageDiffs)),
		Nonces:                    make([]Nonce, 0, len(vmStateDiff.Nonces)),
		DeployedContracts:         make([]DeployedContract, 0, len(vmStateDiff.DeployedContracts)),
		DeprecatedDeclaredClasses: vmStateDiff.DeprecatedDeclaredClasses,
		DeclaredClasses:           make([]DeclaredClass, 0, len(vmStateDiff.DeclaredClasses)),
		ReplacedClasses:           make([]ReplacedClass, 0, len(vmStateDiff.ReplacedClasses)),
	}

	// Adapt storage diffs
	for index := range vmStateDiff.StorageDiffs {
		vmStorageDiff := &vmStateDiff.StorageDiffs[index]

		// Adapt storage entries
		entries := make([]Entry, 0, len(vmStorageDiff.StorageEntries))

		for entryIndex := range vmStorageDiff.StorageEntries {
			vmEntry := &vmStorageDiff.StorageEntries[entryIndex]

			entries = append(entries, Entry{
				Key:   vmEntry.Key,
				Value: vmEntry.Value,
			})
		}

		stateDiff.StorageDiffs = append(stateDiff.StorageDiffs, StorageDiff{
			Address:        vmStorageDiff.Address,
			StorageEntries: entries,
		})
	}

	// Adapt nonces
	for index := range vmStateDiff.Nonces {
		vmNonce := &vmStateDiff.Nonces[index]

		stateDiff.Nonces = append(stateDiff.Nonces, Nonce{
			ContractAddress: vmNonce.ContractAddress,
			Nonce:           vmNonce.Nonce,
		})
	}

	// Adapt deployed contracts
	for index := range vmStateDiff.DeployedContracts {
		vmDeployedContract := &vmStateDiff.DeployedContracts[index]

		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, DeployedContract{
			Address:   vmDeployedContract.Address,
			ClassHash: vmDeployedContract.ClassHash,
		})
	}

	// Adapt declared classes
	for index := range vmStateDiff.DeclaredClasses {
		vmDeclaredClass := &vmStateDiff.DeclaredClasses[index]

		stateDiff.DeclaredClasses = append(stateDiff.DeclaredClasses, DeclaredClass{
			ClassHash:         vmDeclaredClass.ClassHash,
			CompiledClassHash: vmDeclaredClass.CompiledClassHash,
		})
	}

	// Adapt replaced classes
	for index := range vmStateDiff.ReplacedClasses {
		vmReplacedClass := &vmStateDiff.ReplacedClasses[index]

		stateDiff.ReplacedClasses = append(stateDiff.ReplacedClasses, ReplacedClass{
			ContractAddress: vmReplacedClass.ContractAddress,
			ClassHash:       vmReplacedClass.ClassHash,
		})
	}

	return stateDiff
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
	traces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
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

		traces = append(traces, TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		})
	}
	return traces, nil
}

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) *FunctionInvocation {
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

	// Adapt internal calls
	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index]))
	}

	// Adapt events
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		fnInvocation.Events = append(fnInvocation.Events, vm.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
		})
	}

	// TODO: try using utils.Map
	// Adapt messages
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		fnInvocation.Messages = append(fnInvocation.Messages, vm.OrderedL2toL1Message{
			Order:   snMessage.Order,
			To:      snMessage.ToAddr,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
		})
	}

	return &fnInvocation
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
