package vm2core

import (
	"cmp"
	"fmt"
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
			Output:       0, // todo(kirill) recheck, add Output field to core?
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
		fmt.Println("order ", a.Order, b.Order)
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
