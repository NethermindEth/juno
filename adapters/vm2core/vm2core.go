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

func AdaptStateDiff(stateDiff *vm.StateDiff) *core.StateDiff {
	if stateDiff == nil {
		return core.EmptyStateDiff()
	}
	newStorageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(stateDiff.StorageDiffs))
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
