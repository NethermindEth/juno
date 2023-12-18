package rpc

import (
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

func adaptVMTrace(trace *vm.TransactionTrace) *TransactionTrace {
	return &TransactionTrace{
		Type:               TransactionType(trace.Type),
		ValidateInvocation: adaptVMFunctionInvocation(trace.ValidateInvocation),
		ExecuteInvocation: func() *ExecuteInvocation {
			if trace.ExecuteInvocation == nil {
				return nil
			}
			return &ExecuteInvocation{
				RevertReason:       trace.ExecuteInvocation.RevertReason,
				FunctionInvocation: adaptVMFunctionInvocation(trace.ExecuteInvocation.FunctionInvocation),
			}
		}(),
		FeeTransferInvocation: adaptVMFunctionInvocation(trace.FeeTransferInvocation),
		ConstructorInvocation: adaptVMFunctionInvocation(trace.ConstructorInvocation),
		FunctionInvocation:    adaptVMFunctionInvocation(trace.FunctionInvocation),
		StateDiff:             adaptVMStateDiff(trace.StateDiff),
	}
}

func adaptVMFunctionInvocation(invocation *vm.FunctionInvocation) *FunctionInvocation {
	if invocation == nil {
		return nil
	}
	return &FunctionInvocation{
		ContractAddress:    invocation.ContractAddress,
		EntryPointSelector: invocation.EntryPointSelector,
		Calldata:           invocation.Calldata,
		CallerAddress:      invocation.CallerAddress,
		ClassHash:          invocation.ClassHash,
		EntryPointType:     invocation.EntryPointType,
		CallType:           invocation.CallType,
		Result:             invocation.Result,
		Calls: utils.Map(invocation.Calls, func(invocation vm.FunctionInvocation) FunctionInvocation {
			return *adaptVMFunctionInvocation(&invocation)
		}),
		Events:   utils.Map(invocation.Events, adaptVMOrderedEvent),
		Messages: utils.Map(invocation.Messages, adaptVML2toL1Message),
		ExecutionResources: func() *ExecutionResources {
			if invocation.ExecutionResources == nil {
				return nil
			}
			return adaptVMExecutionResources(invocation.ExecutionResources)
		}(),
	}
}

func adaptVMOrderedEvent(event vm.OrderedEvent) OrderedEvent {
	return OrderedEvent{
		Order: event.Order,
		Event: Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		},
	}
}

func adaptVML2toL1Message(message vm.OrderedL2toL1Message) OrderedL2toL1Message {
	return OrderedL2toL1Message{
		Order: message.Order,
		MsgToL1: MsgToL1{
			From:    message.From,
			To:      message.To,
			Payload: message.Payload,
		},
	}
}

func adaptVMExecutionResources(resources *vm.ExecutionResources) *ExecutionResources {
	return &ExecutionResources{
		Steps:        resources.Steps,
		MemoryHoles:  resources.MemoryHoles,
		Pedersen:     resources.Pedersen,
		RangeCheck:   resources.RangeCheck,
		Bitwise:      resources.Bitwise,
		Ecsda:        resources.Ecsda,
		EcOp:         resources.EcOp,
		Keccak:       resources.Keccak,
		Poseidon:     resources.Poseidon,
		SegmentArena: resources.SegmentArena,
	}
}

func adaptVMStateDiff(stateDiff *vm.StateDiff) *StateDiff {
	return &StateDiff{
		StorageDiffs:              utils.Map(stateDiff.StorageDiffs, adaptVMStorageDiff),
		Nonces:                    utils.Map(stateDiff.Nonces, adaptVMNonce),
		DeployedContracts:         utils.Map(stateDiff.DeployedContracts, adaptVMDeployedContract),
		DeprecatedDeclaredClasses: stateDiff.DeprecatedDeclaredClasses,
		DeclaredClasses:           utils.Map(stateDiff.DeclaredClasses, adaptVMDeclaredClass),
		ReplacedClasses:           utils.Map(stateDiff.ReplacedClasses, adaptVMReplacedClass),
	}
}

func adaptVMStorageDiff(diff vm.StorageDiff) StorageDiff {
	return StorageDiff{
		Address: diff.Address,
		StorageEntries: utils.Map(diff.StorageEntries, func(entry vm.Entry) Entry {
			return Entry{
				Key:   entry.Key,
				Value: entry.Value,
			}
		}),
	}
}

func adaptVMNonce(nonce vm.Nonce) Nonce {
	return Nonce{
		ContractAddress: nonce.ContractAddress,
		Nonce:           nonce.Nonce,
	}
}

func adaptVMDeployedContract(contract vm.DeployedContract) DeployedContract {
	return DeployedContract{
		Address:   contract.Address,
		ClassHash: contract.ClassHash,
	}
}

func adaptVMDeclaredClass(declaredClass vm.DeclaredClass) DeclaredClass {
	return DeclaredClass{
		ClassHash:         declaredClass.ClassHash,
		CompiledClassHash: declaredClass.CompiledClassHash,
	}
}

func adaptVMReplacedClass(replacedClass vm.ReplacedClass) ReplacedClass {
	return ReplacedClass{
		ContractAddress: replacedClass.ContractAddress,
		ClassHash:       replacedClass.ClassHash,
	}
}
