package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalBinary(t *testing.T) {
	classHash := new(felt.Felt).SetBytes([]byte("class_hash"))
	nonce := new(felt.Felt).SetBytes([]byte("nonce"))
	deployHeight := uint64(123)
	storageRoot := new(felt.Felt).SetBytes([]byte("storage_root"))

	contract := &StateContract{
		ClassHash:    classHash,
		Nonce:        nonce,
		DeployHeight: deployHeight,
		StorageRoot:  storageRoot,
	}

	data, err := contract.MarshalBinary()
	require.NoError(t, err)

	var unmarshalled StateContract
	require.NoError(t, unmarshalled.UnmarshalBinary(data))

	assert.Equal(t, contract.ClassHash, unmarshalled.ClassHash)
	assert.Equal(t, contract.Nonce, unmarshalled.Nonce)
	assert.Equal(t, contract.DeployHeight, unmarshalled.DeployHeight)
	assert.Nil(t, unmarshalled.StorageRoot)
}

func TestNewContract(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	blockNumber := uint64(10)
	addr := new(felt.Felt).SetUint64(234)
	classHash := new(felt.Felt).SetBytes([]byte("class hash"))

	// Test initial state (contract not deployed)
	_, err = GetContract(addr, txn)
	require.ErrorIs(t, err, ErrContractNotDeployed)

	// Create and commit contract
	contract := NewStateContract(addr, classHash, &felt.Zero, blockNumber)
	require.NoError(t, contract.Commit(txn, true, blockNumber))

	// Retrieve and verify committed contract
	storedContract, err := GetContract(addr, txn)
	require.NoError(t, err)

	assert.Equal(t, addr, storedContract.Address)
	assert.Equal(t, classHash, storedContract.ClassHash)
	assert.Equal(t, &felt.Zero, storedContract.Nonce)
	assert.Equal(t, blockNumber, storedContract.DeployHeight)
}

func TestContractUpdate(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	blockNumber := uint64(10)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	// Initial contract setup
	contract := NewStateContract(addr, classHash, &felt.Zero, blockNumber)
	require.NoError(t, contract.Commit(txn, true, blockNumber))

	// Verify initial state
	contract, err = GetContract(addr, txn)
	require.NoError(t, err)
	require.Equal(t, &felt.Zero, contract.Nonce)
	require.Equal(t, classHash, contract.ClassHash)

	// Test nonce update
	newNonce := new(felt.Felt).SetUint64(1)
	contract.Nonce = newNonce
	require.NoError(t, contract.Commit(txn, true, blockNumber))

	contract, err = GetContract(addr, txn)
	require.NoError(t, err)
	require.Equal(t, newNonce, contract.Nonce)

	// Test class hash update
	newHash := new(felt.Felt).SetUint64(1)
	contract.ClassHash = newHash
	require.NoError(t, contract.Commit(txn, true, blockNumber))

	contract, err = GetContract(addr, txn)
	require.NoError(t, err)
	require.Equal(t, newHash, contract.ClassHash)
}

func TestContractStorage(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	blockNumber := uint64(10)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	contract := NewStateContract(addr, classHash, &felt.Zero, blockNumber)
	require.NoError(t, contract.Commit(txn, true, blockNumber))

	// Initial storage check
	contract, err = GetContract(addr, txn)
	require.NoError(t, err)

	gotValue, err := contract.GetStorage(addr, txn)
	require.NoError(t, err)
	assert.Equal(t, &felt.Zero, gotValue)

	// Storage update verification
	oldRoot, err := contract.GetStorageRoot(txn)
	require.NoError(t, err)

	newVal := new(felt.Felt).SetUint64(1)
	contract.UpdateStorage(addr, newVal)
	require.NoError(t, contract.Commit(txn, false, blockNumber))

	contract, err = GetContract(addr, txn)
	require.NoError(t, err)

	gotValue, err = contract.GetStorage(addr, txn)
	require.NoError(t, err)
	assert.Equal(t, newVal, gotValue)

	newRoot, err := contract.GetStorageRoot(txn)
	require.NoError(t, err)
	assert.NotEqual(t, oldRoot, newRoot)
}

func TestContractDelete(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)

	blockNumber := uint64(10)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	contract := NewStateContract(addr, classHash, &felt.Zero, blockNumber)
	require.NoError(t, contract.Commit(txn, false, blockNumber))

	require.NoError(t, contract.delete(txn))
	_, err = GetContract(addr, txn)
	assert.ErrorIs(t, err, ErrContractNotDeployed)
}
