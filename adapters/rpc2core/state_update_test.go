package rpc2core_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/rpc2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptStateUpdate(t *testing.T) {
	tests := []struct {
		network *networks.Network
		blocks  uint64
	}{
		{&networks.Mainnet, 3},
		{&networks.Sepolia, 3},
	}

	for _, test := range tests {
		t.Run(test.network.String(), func(t *testing.T) {
			handler, gateway := storeBlocks(t, test.network, test.blocks, nil)

			for number := range test.blocks {
				t.Run("block "+strconv.FormatUint(number, 10), func(t *testing.T) {
					expected, err := gateway.StateUpdate(t.Context(), number)
					require.NoError(t, err)

					id := rpcv10.BlockIDFromNumber(number)
					served, rpcErr := handler.StateUpdate(&id, nil)
					require.Nil(t, rpcErr)

					adapted, err := rpc2core.AdaptStateUpdate(overWire(t, &served))
					require.NoError(t, err)
					assert.Equal(t, expected, adapted)
				})
			}
		})
	}
}

func TestAdaptStateDiff(t *testing.T) {
	address := felt.NewUnsafeFromString[felt.Felt]("0xa")
	key := felt.NewUnsafeFromString[felt.Felt]("0xb")
	value := felt.NewUnsafeFromString[felt.Felt]("0xc")
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xd")
	compiledClassHash := felt.NewUnsafeFromString[felt.Felt]("0xe")

	tests := []struct {
		name     string
		served   *rpcv10.StateDiff
		expected func(diff *core.StateDiff)
	}{
		{
			name:     "empty",
			served:   &rpcv10.StateDiff{},
			expected: func(*core.StateDiff) {},
		},
		{
			name: "storage diffs",
			served: &rpcv10.StateDiff{StorageDiffs: []rpcv10.StorageDiff{{
				Address:        *address,
				StorageEntries: []rpcv10.Entry{{Key: *key, Value: *value}},
			}}},
			expected: func(diff *core.StateDiff) {
				diff.StorageDiffs[*address] = map[felt.Felt]*felt.Felt{*key: value}
			},
		},
		{
			name:   "nonces",
			served: &rpcv10.StateDiff{Nonces: []rpcv10.Nonce{{ContractAddress: *address, Nonce: *value}}},
			expected: func(diff *core.StateDiff) {
				diff.Nonces[*address] = value
			},
		},
		{
			name: "deployed contracts",
			served: &rpcv10.StateDiff{
				DeployedContracts: []rpcv10.DeployedContract{{Address: *address, ClassHash: *classHash}},
			},
			expected: func(diff *core.StateDiff) {
				diff.DeployedContracts[*address] = classHash
			},
		},
		{
			name:   "deprecated declared classes",
			served: &rpcv10.StateDiff{DeprecatedDeclaredClasses: []*felt.Felt{classHash}},
			expected: func(diff *core.StateDiff) {
				diff.DeclaredV0Classes = []*felt.Felt{classHash}
			},
		},
		{
			name: "declared classes",
			served: &rpcv10.StateDiff{
				DeclaredClasses: []rpcv10.DeclaredClass{{
					ClassHash:         *classHash,
					CompiledClassHash: *compiledClassHash,
				}},
			},
			expected: func(diff *core.StateDiff) {
				diff.DeclaredV1Classes[*classHash] = compiledClassHash
			},
		},
		{
			name: "replaced classes",
			served: &rpcv10.StateDiff{
				ReplacedClasses: []rpcv10.ReplacedClass{{ContractAddress: *address, ClassHash: *classHash}},
			},
			expected: func(diff *core.StateDiff) {
				diff.ReplacedClasses[*address] = classHash
			},
		},
		{
			name: "migrated compiled classes",
			served: &rpcv10.StateDiff{MigratedCompiledClasses: []rpcv10.MigratedCompiledClass{{
				ClassHash:         felt.SierraClassHash(*classHash),
				CompiledClassHash: felt.CasmClassHash(*compiledClassHash),
			}}},
			expected: func(diff *core.StateDiff) {
				diff.MigratedClasses[felt.SierraClassHash(*classHash)] = felt.CasmClassHash(*compiledClassHash)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			served := &rpcv10.StateUpdate{
				BlockHash: &felt.One,
				NewRoot:   &felt.One,
				OldRoot:   &felt.Zero,
				StateDiff: test.served,
			}
			expected := core.EmptyStateDiff()
			// The deprecated class list is taken as served, and a nil one is served as null.
			expected.DeclaredV0Classes = nil
			test.expected(&expected)

			adapted, err := rpc2core.AdaptStateUpdate(overWire(t, served))
			require.NoError(t, err)
			assert.Equal(t, &core.StateUpdate{
				BlockHash: &felt.One,
				NewRoot:   &felt.One,
				OldRoot:   &felt.Zero,
				StateDiff: &expected,
			}, adapted)
		})
	}
}

func TestAdaptStateUpdateErrors(t *testing.T) {
	tests := []struct {
		name   string
		served *rpcv10.StateUpdate
		err    string
	}{
		{
			name:   "nil",
			served: nil,
			err:    "nil state update",
		},
		{
			name: "missing block_hash",
			served: &rpcv10.StateUpdate{
				NewRoot:   &felt.One,
				OldRoot:   &felt.Zero,
				StateDiff: &rpcv10.StateDiff{},
			},
			err: "state update is missing block_hash",
		},
		{
			name: "missing new_root",
			served: &rpcv10.StateUpdate{
				BlockHash: &felt.One,
				OldRoot:   &felt.Zero,
				StateDiff: &rpcv10.StateDiff{},
			},
			err: "state update for block 0x1 is missing new_root",
		},
		{
			name: "missing old_root",
			served: &rpcv10.StateUpdate{
				BlockHash: &felt.One,
				NewRoot:   &felt.One,
				StateDiff: &rpcv10.StateDiff{},
			},
			err: "state update for block 0x1 is missing old_root",
		},
		{
			name:   "missing state_diff",
			served: &rpcv10.StateUpdate{BlockHash: &felt.One, NewRoot: &felt.One, OldRoot: &felt.Zero},
			err:    "state update for block 0x1 is missing state_diff",
		},
		{
			name: "null deprecated declared class",
			served: &rpcv10.StateUpdate{
				BlockHash: &felt.One,
				NewRoot:   &felt.One,
				OldRoot:   &felt.Zero,
				StateDiff: &rpcv10.StateDiff{DeprecatedDeclaredClasses: []*felt.Felt{&felt.One, nil}},
			},
			err: "state update for block 0x1 has a null deprecated declared class",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapted, err := rpc2core.AdaptStateUpdate(test.served)
			require.EqualError(t, err, test.err)
			assert.Nil(t, adapted)
		})
	}
}
