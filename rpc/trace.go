package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

func adaptBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace, legacyJSON bool) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}
	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}
	traces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]
		trace := vm.TransactionTrace{}
		trace.Type = vm.TransactionType(block.Transactions[index].Type)

		trace.FeeTransferInvocation = adaptFunctionInvocation(feederTrace.FeeTransferInvocation, legacyJSON)
		trace.ValidateInvocation = adaptFunctionInvocation(feederTrace.ValidateInvocation, legacyJSON)

		fnInvocation := adaptFunctionInvocation(feederTrace.FunctionInvocation, legacyJSON)
		switch block.Transactions[index].Type {
		case TxnDeploy:
			trace.ConstructorInvocation = fnInvocation
		case TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(vm.ExecuteInvocation)
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

func adaptFunctionInvocation(snFnInvocation *starknet.FunctionInvocation, legacyJSON bool) *vm.FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	fnInvocation := vm.FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              make([]vm.FunctionInvocation, 0, len(snFnInvocation.InternalCalls)),
		Events:             make([]vm.OrderedEvent, 0, len(snFnInvocation.Events)),
		Messages:           make([]vm.OrderedL2toL1Message, 0, len(snFnInvocation.Messages)),
		ExecutionResources: adaptFeederExecutionResources(&snFnInvocation.ExecutionResources),
	}

	if legacyJSON {
		fnInvocation.ExecutionResources = nil
	}

	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *adaptFunctionInvocation(&snFnInvocation.InternalCalls[index], legacyJSON))
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

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) *vm.ExecutionResources {
	builtins := &resources.BuiltinInstanceCounter
	return &vm.ExecutionResources{
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
