package rpcv10

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeInitialReadsPreservesOriginalValues(t *testing.T) {
	address1 := felt.FromUint64[felt.Address](1)
	address2 := felt.FromUint64[felt.Address](2)
	key1 := felt.FromUint64[felt.Felt](3)
	key2 := felt.FromUint64[felt.Felt](4)
	class1 := felt.FromUint64[felt.ClassHash](5)
	class2 := felt.FromUint64[felt.ClassHash](6)

	existing := &InitialReads{
		Storage: []StorageEntry{{
			ContractAddress: address1, Key: key1, Value: felt.FromUint64[felt.Felt](10),
		}},
		Nonces: []NonceEntry{{ContractAddress: address1, Nonce: felt.FromUint64[felt.Felt](11)}},
		ClassHashes: []ClassHashEntry{{
			ContractAddress: address1, ClassHash: class1,
		}},
		DeclaredContracts: []DeclaredContractEntry{{ClassHash: class1, IsDeclared: false}},
	}
	incoming := &InitialReads{
		Storage: []StorageEntry{
			{ContractAddress: address1, Key: key1, Value: felt.FromUint64[felt.Felt](100)},
			{ContractAddress: address2, Key: key2, Value: felt.FromUint64[felt.Felt](20)},
		},
		Nonces: []NonceEntry{
			{ContractAddress: address1, Nonce: felt.FromUint64[felt.Felt](101)},
			{ContractAddress: address2, Nonce: felt.FromUint64[felt.Felt](21)},
		},
		ClassHashes: []ClassHashEntry{
			{ContractAddress: address1, ClassHash: class2},
			{ContractAddress: address2, ClassHash: class2},
		},
		DeclaredContracts: []DeclaredContractEntry{
			{ClassHash: class1, IsDeclared: true},
			{ClassHash: class2, IsDeclared: false},
		},
	}

	merged := mergeInitialReads(existing, incoming)
	require.Same(t, existing, merged)
	require.Len(t, merged.Storage, 2)
	assert.Equal(t, uint64(10), merged.Storage[0].Value.Uint64())
	assert.Equal(t, address2, merged.Storage[1].ContractAddress)
	require.Len(t, merged.Nonces, 2)
	assert.Equal(t, uint64(11), merged.Nonces[0].Nonce.Uint64())
	require.Len(t, merged.ClassHashes, 2)
	assert.Equal(t, class1, merged.ClassHashes[0].ClassHash)
	require.Len(t, merged.DeclaredContracts, 2)
	assert.False(t, merged.DeclaredContracts[0].IsDeclared)
}
