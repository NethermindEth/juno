package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
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

func TestContractAddress(t *testing.T) {
	tests := []struct {
		callerAddress       *felt.Felt
		classHash           *felt.Felt
		salt                *felt.Felt
		constructorCallData []*felt.Felt
		want                *felt.Felt
	}{
		{
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0
			callerAddress: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000"),
			classHash: utils.HexToFelt(
				t, "0x46f844ea1a3b3668f81d38b5c1bd55e816e0373802aefe732138628f0133486"),
			salt: utils.HexToFelt(
				t, "0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b"),
			constructorCallData: []*felt.Felt{
				utils.HexToFelt(t, "0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4"),
				utils.HexToFelt(t, "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
				utils.HexToFelt(t, "0x2"),
				utils.HexToFelt(t, "0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6"),
				utils.HexToFelt(t, "0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328"),
			},
			want: utils.HexToFelt(t, "0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
		},
	}

	for _, tt := range tests {
		t.Run("Address", func(t *testing.T) {
			address := ContractAddress(tt.callerAddress, tt.classHash, tt.salt, tt.constructorCallData)
			if !address.Equal(tt.want) {
				t.Errorf("wrong address: got %s, want %s", address.String(), tt.want.String())
			}
		})
	}
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
