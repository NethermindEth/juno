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

func AdaptStateDiff(sd *vm.StateDiff) *core.StateDiff {
	result := core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}
	if sd == nil {
		return &result
	}
	for _, entries := range sd.StorageDiffs {
		KeyVals := map[felt.Felt]*felt.Felt{}
		for _, entry := range entries.StorageEntries {
			KeyVals[entry.Key] = &entry.Value
		}
		result.StorageDiffs[entries.Address] = KeyVals
	}
	for _, addrNonce := range sd.Nonces {
		result.Nonces[addrNonce.ContractAddress] = &addrNonce.Nonce
	}
	for _, addrClassHash := range sd.DeployedContracts {
		result.Nonces[addrClassHash.Address] = &addrClassHash.ClassHash
	}
	for _, hashes := range sd.DeclaredClasses {
		result.DeclaredV1Classes[hashes.ClassHash] = &hashes.CompiledClassHash
	}
	for _, addrClassHash := range sd.ReplacedClasses {
		result.ReplacedClasses[addrClassHash.ClassHash] = &addrClassHash.ClassHash
	}
	result.DeclaredV0Classes = append(result.DeclaredV0Classes, sd.DeprecatedDeclaredClasses...)
	return &result
}

func StateDiff(trace *vm.TransactionTrace) *core.StateDiff {
	if trace.StateDiff == nil {
		return nil
	}
	stateDiff := trace.StateDiff
	newStorageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for _, sd := range stateDiff.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt)
		for _, entry := range sd.StorageEntries {
			val := entry.Value
			entries[entry.Key] = &val
		}
		newStorageDiffs[sd.Address] = entries
	}

	newNonces := make(map[felt.Felt]*felt.Felt)
	for _, nonce := range stateDiff.Nonces {
		nonc := nonce.Nonce
		newNonces[nonce.ContractAddress] = &nonc
	}

	newDeployedContracts := make(map[felt.Felt]*felt.Felt)
	for _, dc := range stateDiff.DeployedContracts {
		ch := dc.ClassHash
		newDeployedContracts[dc.Address] = &ch
	}

	newDeclaredV1Classes := make(map[felt.Felt]*felt.Felt)
	for _, dc := range stateDiff.DeclaredClasses {
		cch := dc.CompiledClassHash
		newDeclaredV1Classes[dc.ClassHash] = &cch
	}

	newReplacedClasses := make(map[felt.Felt]*felt.Felt)
	for _, rc := range stateDiff.ReplacedClasses {
		ch := rc.ClassHash
		newReplacedClasses[rc.ContractAddress] = &ch
	}

	return &core.StateDiff{
		StorageDiffs:      newStorageDiffs,
		Nonces:            newNonces,
		DeployedContracts: newDeployedContracts,
		DeclaredV0Classes: stateDiff.DeprecatedDeclaredClasses,
		DeclaredV1Classes: newDeclaredV1Classes,
		ReplacedClasses:   newReplacedClasses,
	}
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
