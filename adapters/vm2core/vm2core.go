package vm2core

import (
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptExecutionResources(resources *vm.ExecutionResources) *core.ExecutionResources {
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
		},
		MemoryHoles: resources.MemoryHoles,
		Steps:       resources.Steps,
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
	slices.SortFunc(messages, func(a vm.OrderedL2toL1Message, b vm.OrderedL2toL1Message) int {
		switch {
		case a.Order > b.Order:
			return 1
		case a.Order < b.Order:
			return -1
		default:
			return 0
		}
	})
	return utils.Map(messages, AdaptOrderedMessageToL1)
}

func AdaptOrderedEvents(events []vm.OrderedEvent) []*core.Event {
	slices.SortFunc(events, func(a vm.OrderedEvent, b vm.OrderedEvent) int {
		switch {
		case a.Order > b.Order:
			return 1
		case a.Order < b.Order:
			return -1
		default:
			return 0
		}
	})
	return utils.Map(events, AdaptOrderedEvent)
}
