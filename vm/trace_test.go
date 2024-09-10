package vm_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestRevertReason(t *testing.T) {
	require.Equal(t, (&vm.TransactionTrace{
		ExecuteInvocation: &vm.ExecuteInvocation{
			RevertReason: "reason",
		},
	}).RevertReason(), "reason")

	require.Empty(t, (&vm.TransactionTrace{
		ExecuteInvocation: &vm.ExecuteInvocation{},
	}).RevertReason())
}

//nolint:dupl
func TestAllEvents(t *testing.T) {
	numEvents := uint64(10)
	events := make([]vm.OrderedEvent, 0, numEvents)
	for i := uint64(0); i < numEvents; i++ {
		events = append(events, vm.OrderedEvent{Order: i})
	}
	tests := map[string]*vm.TransactionTrace{
		"many top-level invocations": {
			ValidateInvocation: &vm.FunctionInvocation{
				Events: []vm.OrderedEvent{events[0]},
			},
			FunctionInvocation: &vm.FunctionInvocation{
				Events: []vm.OrderedEvent{events[1]},
			},
			ConstructorInvocation: &vm.FunctionInvocation{
				Events: []vm.OrderedEvent{events[2]},
			},
			ExecuteInvocation: &vm.ExecuteInvocation{
				FunctionInvocation: &vm.FunctionInvocation{
					Events: []vm.OrderedEvent{events[3]},
				},
			},
			FeeTransferInvocation: &vm.FunctionInvocation{
				Events: events[4:],
			},
		},
		"only validate invocation": {
			ValidateInvocation: &vm.FunctionInvocation{
				Events: events,
			},
		},
		"present in some sub-calls": {
			ValidateInvocation: &vm.FunctionInvocation{
				Events: []vm.OrderedEvent{events[0]},
				Calls: []vm.FunctionInvocation{
					{
						Events: events[1:5],
					},
				},
			},
			FunctionInvocation: &vm.FunctionInvocation{
				Events: []vm.OrderedEvent{events[5]},
				Calls: []vm.FunctionInvocation{
					{
						Events: events[6:],
					},
				},
			},
		},
	}

	for description, trace := range tests {
		t.Run(description, func(t *testing.T) {
			require.ElementsMatch(t, events, trace.AllEvents())
		})
	}
}

//nolint:dupl
func TestAllMessages(t *testing.T) {
	nummessages := uint64(10)
	messages := make([]vm.OrderedL2toL1Message, 0, nummessages)
	for i := uint64(0); i < nummessages; i++ {
		messages = append(messages, vm.OrderedL2toL1Message{Order: i})
	}
	tests := map[string]*vm.TransactionTrace{
		"many top-level invocations": {
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{messages[0]},
			},
			FunctionInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{messages[1]},
			},
			ConstructorInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{messages[2]},
			},
			ExecuteInvocation: &vm.ExecuteInvocation{
				FunctionInvocation: &vm.FunctionInvocation{
					Messages: []vm.OrderedL2toL1Message{messages[3]},
				},
			},
			FeeTransferInvocation: &vm.FunctionInvocation{
				Messages: messages[4:],
			},
		},
		"only validate invocation": {
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: messages,
			},
		},
		"present in some sub-calls": {
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{messages[0]},
				Calls: []vm.FunctionInvocation{
					{
						Messages: messages[1:5],
					},
				},
			},
			FunctionInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{messages[5]},
				Calls: []vm.FunctionInvocation{
					{
						Messages: messages[6:],
					},
				},
			},
		},
	}

	for description, trace := range tests {
		t.Run(description, func(t *testing.T) {
			require.ElementsMatch(t, messages, trace.AllMessages())
		})
	}
}

func TestTotalExecutionResources(t *testing.T) {
	resources := &vm.ExecutionResources{
		ComputationResources: vm.ComputationResources{
			Steps:        1,
			MemoryHoles:  2,
			Pedersen:     3,
			RangeCheck:   4,
			Bitwise:      5,
			Ecdsa:        6,
			EcOp:         7,
			Keccak:       8,
			Poseidon:     9,
			SegmentArena: 10,
		},
	}
	tests := map[string]struct {
		multiplier uint64
		trace      *vm.TransactionTrace
	}{
		"many top-level invocations": {
			multiplier: 5,
			trace: &vm.TransactionTrace{
				ValidateInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
				},
				FunctionInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
				},
				ConstructorInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
				},
				ExecuteInvocation: &vm.ExecuteInvocation{
					FunctionInvocation: &vm.FunctionInvocation{
						ExecutionResources: resources,
					},
				},
				FeeTransferInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"only validate invocation": {
			multiplier: 1,
			trace: &vm.TransactionTrace{
				ValidateInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"present in some sub-calls": {
			multiplier: 2,
			trace: &vm.TransactionTrace{
				ValidateInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []vm.FunctionInvocation{
						{
							ExecutionResources: resources,
						},
					},
				},
				FunctionInvocation: &vm.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []vm.FunctionInvocation{
						{
							ExecutionResources: resources,
						},
					},
				},
			},
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			require.Equal(t, &vm.ExecutionResources{
				ComputationResources: vm.ComputationResources{
					Steps:        resources.Steps * test.multiplier,
					MemoryHoles:  resources.MemoryHoles * test.multiplier,
					Pedersen:     resources.Pedersen * test.multiplier,
					RangeCheck:   resources.RangeCheck * test.multiplier,
					Bitwise:      resources.Bitwise * test.multiplier,
					Ecdsa:        resources.Ecdsa * test.multiplier,
					EcOp:         resources.EcOp * test.multiplier,
					Keccak:       resources.Keccak * test.multiplier,
					Poseidon:     resources.Poseidon * test.multiplier,
					SegmentArena: resources.SegmentArena * test.multiplier,
				},
			}, test.trace.TotalExecutionResources())
		})
	}
}

func TestExecuteInvocation(t *testing.T) {
	tests := map[string]struct {
		inv      vm.ExecuteInvocation
		expected string
	}{
		"success": {
			inv: vm.ExecuteInvocation{
				FunctionInvocation: &vm.FunctionInvocation{
					CallType: "DEADBEEF",
				},
			},
			expected: `{"contract_address":"0x0","calldata":null,"caller_address":"0x0","call_type":"DEADBEEF","result":null,"calls":null,"events":null,"messages":null}`,
		},
		"reverted with reason": {
			inv: vm.ExecuteInvocation{
				RevertReason: "Oops",
			},
			expected: `{"revert_reason":"Oops"}`,
		},
		"reverted without reason": {
			inv:      vm.ExecuteInvocation{},
			expected: `{"revert_reason":""}`,
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			t.Run("value", func(t *testing.T) {
				j, err := json.Marshal(test.inv)
				require.NoError(t, err)
				require.JSONEq(t, test.expected, string(j), string(j))
			})

			t.Run("pointer", func(t *testing.T) {
				j, err := json.Marshal(&test.inv)
				require.NoError(t, err)
				require.JSONEq(t, test.expected, string(j), string(j))
			})
		})
	}
}
