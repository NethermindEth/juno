package vm2core

import (
	"cmp"
	"slices"

	"github.com/NethermindEth/juno/core"
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
