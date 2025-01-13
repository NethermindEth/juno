package vm2core

import (
	"cmp"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptExecutionResources(resources *vm.ExecutionResources, totalGas *vm.GasConsumed) *core.ExecutionResources {
	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:     resources.Pedersen,
			RangeCheck:   resources.RangeCheck,
			Bitwise:      resources.Bitwise,
			Ecsda:        resources.Ecdsa,
			EcOp:         resources.EcOp,
			Keccak:       resources.Keccak,
			Poseidon:     resources.Poseidon,
			SegmentArena: resources.SegmentArena,
			Output:       resources.Output,
			AddMod:       resources.AddMod,
			MulMod:       resources.MulMod,
			RangeCheck96: resources.RangeCheck96,
		},
		MemoryHoles:      resources.MemoryHoles,
		Steps:            resources.Steps,
		DataAvailability: adaptDA(resources.DataAvailability),
		TotalGasConsumed: &core.GasConsumed{L1Gas: totalGas.L1Gas, L1DataGas: totalGas.L1DataGas},
	}
}

func AdaptOrderedEvent(event vm.OrderedEvent) *core.Event {
	return &core.Event{
		From: event.From,
		Keys: event.Keys,
		Data: event.Data,
	}
}

func AdaptOrderedMessageToL1(message vm.OrderedL2toL1Message) *core.L2ToL1Message {
	return &core.L2ToL1Message{
		From:    message.From,
		Payload: message.Payload,
		To:      common.HexToAddress(message.To),
	}
}

func AdaptOrderedMessagesToL1(messages []vm.OrderedL2toL1Message) []*core.L2ToL1Message {
	slices.SortFunc(messages, func(a, b vm.OrderedL2toL1Message) int {
		return cmp.Compare(a.Order, b.Order)
	})
	return utils.Map(messages, AdaptOrderedMessageToL1)
}

func AdaptOrderedEvents(events []vm.OrderedEvent) []*core.Event {
	slices.SortFunc(events, func(a, b vm.OrderedEvent) int {
		return cmp.Compare(a.Order, b.Order)
	})
	return utils.Map(events, AdaptOrderedEvent)
}

func adaptDA(da *vm.DataAvailability) *core.DataAvailability {
	if da == nil {
		return nil
	}

	return &core.DataAvailability{
		L1Gas:     da.L1Gas,
		L1DataGas: da.L1DataGas,
	}
}

func AdaptStateDiff(traceSD *vm.StateDiff) *core.StateDiff {
	result := core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}

	if traceSD == nil {
		return &result
	}

	for _, sd := range traceSD.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt)
		for _, entry := range sd.StorageEntries {
			val := entry.Value
			entries[entry.Key] = &val
		}
		result.StorageDiffs[sd.Address] = entries
	}

	for _, nonce := range traceSD.Nonces {
		nonc := nonce.Nonce
		result.Nonces[nonce.ContractAddress] = &nonc
	}

	for _, dc := range traceSD.DeployedContracts {
		ch := dc.ClassHash
		result.DeployedContracts[dc.Address] = &ch
	}

	result.DeclaredV0Classes = traceSD.DeprecatedDeclaredClasses

	for _, dc := range traceSD.DeclaredClasses {
		cch := dc.CompiledClassHash
		result.DeclaredV1Classes[dc.ClassHash] = &cch
	}

	for _, rc := range traceSD.ReplacedClasses {
		ch := rc.ClassHash
		result.ReplacedClasses[rc.ContractAddress] = &ch
	}

	return &result
}

func Receipt(fee *felt.Felt, feeUnit core.FeeUnit, txHash *felt.Felt,
	trace *vm.TransactionTrace, txnReceipt *vm.TransactionReceipt,
) *core.TransactionReceipt {
	return &core.TransactionReceipt{ //nolint:exhaustruct
		Fee:                fee,
		FeeUnit:            feeUnit,
		Events:             AdaptOrderedEvents(trace.AllEvents()),
		ExecutionResources: AdaptExecutionResources(trace.TotalExecutionResources(), &txnReceipt.Gas),
		L2ToL1Message:      AdaptOrderedMessagesToL1(trace.AllMessages()),
		TransactionHash:    txHash,
		Reverted:           trace.IsReverted(),
		RevertReason:       trace.RevertReason(),
	}
}
