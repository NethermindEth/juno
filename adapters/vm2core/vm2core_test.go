package vm2core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
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

//nolint:dupl
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
		From:    new(felt.Felt).SetUint64(2),
		To:      common.HexToAddress("0x3"),
		Payload: []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}, vm2core.AdaptOrderedMessageToL1(vm.OrderedL2toL1Message{
		Order:   1,
		From:    new(felt.Felt).SetUint64(2),
		To:      "0x3",
		Payload: []*felt.Felt{new(felt.Felt).SetUint64(4)},
	}))
}

//nolint:dupl
func TestAdaptOrderedMessagesToL1(t *testing.T) {
	numMessages := 5
	messages := make([]vm.OrderedL2toL1Message, 0, numMessages)
	for i := numMessages - 1; i >= 0; i-- {
		messages = append(messages, vm.OrderedL2toL1Message{Order: uint64(i)})
	}
	require.Equal(t, []*core.L2ToL1Message{
		vm2core.AdaptOrderedMessageToL1(messages[4]),
		vm2core.AdaptOrderedMessageToL1(messages[3]),
		vm2core.AdaptOrderedMessageToL1(messages[2]),
		vm2core.AdaptOrderedMessageToL1(messages[1]),
		vm2core.AdaptOrderedMessageToL1(messages[0]),
	}, vm2core.AdaptOrderedMessagesToL1(messages))
}

func TestAdaptExecutionResources(t *testing.T) {
	require.Equal(t, &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:     1,
			RangeCheck:   2,
			Bitwise:      3,
			Ecsda:        4,
			EcOp:         5,
			Keccak:       6,
			Poseidon:     7,
			SegmentArena: 8,
			Output:       11,
		},
		MemoryHoles:      9,
		Steps:            10,
		TotalGasConsumed: &core.GasConsumed{L1Gas: 1, L1DataGas: 2},
	}, vm2core.AdaptExecutionResources(&vm.ExecutionResources{
		ComputationResources: vm.ComputationResources{
			Pedersen:     1,
			RangeCheck:   2,
			Bitwise:      3,
			Ecdsa:        4,
			EcOp:         5,
			Keccak:       6,
			Poseidon:     7,
			SegmentArena: 8,
			MemoryHoles:  9,
			Steps:        10,
			Output:       11,
		},
	}, &vm.GasConsumed{L1Gas: 1, L1DataGas: 2}))
}

func TestStateDiff(t *testing.T) {
	address1 := *new(felt.Felt).SetUint64(1)
	key1 := *new(felt.Felt).SetUint64(2)
	value1 := *new(felt.Felt).SetUint64(3)
	nonce1 := *new(felt.Felt).SetUint64(4)
	classHash1 := *new(felt.Felt).SetUint64(5)
	compiledClassHash1 := *new(felt.Felt).SetUint64(6)
	trace := &vm.TransactionTrace{
		StateDiff: &vm.StateDiff{
			StorageDiffs: []vm.StorageDiff{
				{
					Address: address1,
					StorageEntries: []vm.Entry{
						{Key: key1, Value: value1},
					},
				},
			},
			Nonces:                    []vm.Nonce{{ContractAddress: address1, Nonce: nonce1}},
			DeployedContracts:         []vm.DeployedContract{{Address: address1, ClassHash: classHash1}},
			DeclaredClasses:           []vm.DeclaredClass{{ClassHash: classHash1, CompiledClassHash: compiledClassHash1}},
			ReplacedClasses:           []vm.ReplacedClass{{ContractAddress: address1, ClassHash: classHash1}},
			DeprecatedDeclaredClasses: []*felt.Felt{&classHash1},
		},
	}

	expected := &core.StateDiff{
		StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
			address1: {key1: &value1},
		},
		Nonces:            map[felt.Felt]*felt.Felt{address1: &nonce1},
		DeployedContracts: map[felt.Felt]*felt.Felt{address1: &classHash1},
		DeclaredV0Classes: []*felt.Felt{&classHash1},
		DeclaredV1Classes: map[felt.Felt]*felt.Felt{classHash1: &compiledClassHash1},
		ReplacedClasses:   map[felt.Felt]*felt.Felt{address1: &classHash1},
	}
	require.Equal(t, expected, vm2core.StateDiff(trace))
}

func TestReceipt(t *testing.T) {
	fee := new(felt.Felt).SetUint64(1)
	feeUnit := core.WEI
	txHash := new(felt.Felt).SetUint64(2)
	trace := &vm.TransactionTrace{
		Type:                  vm.TxnInvoke,
		ValidateInvocation:    &vm.FunctionInvocation{},
		ExecuteInvocation:     &vm.ExecuteInvocation{},
		FeeTransferInvocation: &vm.FunctionInvocation{},
		ConstructorInvocation: &vm.FunctionInvocation{},
		FunctionInvocation:    &vm.FunctionInvocation{},
		StateDiff:             &vm.StateDiff{},
		ExecutionResources:    &vm.ExecutionResources{},
	}
	txnReceipt := &vm.TransactionReceipt{
		Fee:   fee,
		Gas:   vm.GasConsumed{L1Gas: 1, L1DataGas: 2},
		DAGas: vm.DataAvailability{L1Gas: 1, L1DataGas: 2},
	}
	expectedReceipt := &core.TransactionReceipt{
		Fee:                fee,
		FeeUnit:            feeUnit,
		Events:             vm2core.AdaptOrderedEvents(trace.AllEvents()),
		ExecutionResources: vm2core.AdaptExecutionResources(trace.TotalExecutionResources(), &txnReceipt.Gas),
		L2ToL1Message:      vm2core.AdaptOrderedMessagesToL1(trace.AllMessages()),
		TransactionHash:    txHash,
		Reverted:           trace.IsReverted(),
		RevertReason:       trace.RevertReason(),
	}
	require.Equal(t, expectedReceipt, vm2core.Receipt(fee, feeUnit, txHash, trace, txnReceipt))
}
