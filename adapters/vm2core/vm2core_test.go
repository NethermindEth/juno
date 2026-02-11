package vm2core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestAdaptOrderedEvent(t *testing.T) {
	require.Equal(t, &core.Event{
		From: new(felt.Felt).SetUint64(2),
		Keys: []*felt.Felt{new(felt.Felt).SetUint64(3)},
		Data: []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}, vm2core.AdaptOrderedEvent(vm.OrderedEvent{
		Order: 1,
		From:  new(felt.Felt).SetUint64(2),
		Keys:  []*felt.Felt{new(felt.Felt).SetUint64(3)},
		Data:  []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}))
}

func TestAdaptOrderedEvents(t *testing.T) {
	numEvents := 5
	events := make([]vm.OrderedEvent, 0, numEvents)
	for i := numEvents - 1; i >= 0; i-- {
		events = append(events, vm.OrderedEvent{Order: uint64(i)})
	}
	require.Equal(t, []*core.Event{
		vm2core.AdaptOrderedEvent(events[4]),
		vm2core.AdaptOrderedEvent(events[3]),
		vm2core.AdaptOrderedEvent(events[2]),
		vm2core.AdaptOrderedEvent(events[1]),
		vm2core.AdaptOrderedEvent(events[0]),
	}, vm2core.AdaptOrderedEvents(events))
}

func TestAdaptOrderedMessageToL1(t *testing.T) {
	require.Equal(t, &core.L2ToL1Message{
		From:    felt.NewFromUint64[felt.Address](2),
		To:      types.NewFromUint64[types.L1Address](3),
		Payload: []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}, vm2core.AdaptOrderedMessageToL1(&vm.OrderedL2toL1Message{
		Order:   1,
		From:    felt.NewFromUint64[felt.Felt](2),
		To:      felt.NewFromUint64[felt.Address](0x3),
		Payload: []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}))
}

func TestAdaptOrderedMessagesToL1(t *testing.T) {
	numMessages := 5
	messages := make([]vm.OrderedL2toL1Message, 0, numMessages)
	for i := numMessages - 1; i >= 0; i-- {
		messages = append(messages, vm.OrderedL2toL1Message{Order: uint64(i)})
	}
	require.Equal(t, []*core.L2ToL1Message{
		vm2core.AdaptOrderedMessageToL1(&messages[4]),
		vm2core.AdaptOrderedMessageToL1(&messages[3]),
		vm2core.AdaptOrderedMessageToL1(&messages[2]),
		vm2core.AdaptOrderedMessageToL1(&messages[1]),
		vm2core.AdaptOrderedMessageToL1(&messages[0]),
	}, vm2core.AdaptOrderedMessagesToL1(messages))
}
