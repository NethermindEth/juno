package rpcv7_test

import (
	"testing"

	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/stretchr/testify/require"
)

func TestTotalExecutionResources(t *testing.T) {
	resources := &rpcv6.ComputationResources{
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
	}

	tests := map[string]struct {
		multiplier uint64
		trace      *rpcv7.TransactionTrace
	}{
		"many top-level invocations": {
			multiplier: 5,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				FunctionInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				ConstructorInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				ExecuteInvocation: &rpcv6.ExecuteInvocation{
					FunctionInvocation: &rpcv6.FunctionInvocation{
						ExecutionResources: resources,
					},
				},
				FeeTransferInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"only validate invocation": {
			multiplier: 1,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"present in some sub-calls": {
			multiplier: 2,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []rpcv6.FunctionInvocation{
						{
							ExecutionResources: resources,
						},
					},
				},
				FunctionInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []rpcv6.FunctionInvocation{
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
			require.Equal(t, rpcv7.ComputationResources{
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
			}, test.trace.TotalComputationResources())
		})
	}
}
