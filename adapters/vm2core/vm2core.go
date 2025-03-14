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

func AdaptStateDiff(fromStateDiff *vm.StateDiff) core.StateDiff {
	var result core.StateDiff
	if fromStateDiff == nil {
		return result
	}

	// Preallocate all maps with known sizes from fromStateDiff
	result = core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(fromStateDiff.StorageDiffs)),
		Nonces:            make(map[felt.Felt]*felt.Felt, len(fromStateDiff.Nonces)),
		DeployedContracts: make(map[felt.Felt]*felt.Felt, len(fromStateDiff.DeployedContracts)),
		DeclaredV0Classes: make([]*felt.Felt, len(fromStateDiff.DeprecatedDeclaredClasses)),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, len(fromStateDiff.DeclaredClasses)),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt, len(fromStateDiff.ReplacedClasses)),
	}

	for _, sd := range fromStateDiff.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt, len(sd.StorageEntries))
		for _, entry := range sd.StorageEntries {
			val := entry.Value
			entries[entry.Key] = &val
		}
		result.StorageDiffs[sd.Address] = entries
	}

	for _, nonce := range fromStateDiff.Nonces {
		newNonce := nonce.Nonce
		result.Nonces[nonce.ContractAddress] = &newNonce
	}

	for _, dc := range fromStateDiff.DeployedContracts {
		ch := dc.ClassHash
		result.DeployedContracts[dc.Address] = &ch
	}

	for _, dc := range fromStateDiff.DeclaredClasses {
		cch := dc.CompiledClassHash
		result.DeclaredV1Classes[dc.ClassHash] = &cch
	}

	for _, rc := range fromStateDiff.ReplacedClasses {
		ch := rc.ClassHash
		result.ReplacedClasses[rc.ContractAddress] = &ch
	}
	return result
}
