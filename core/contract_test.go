package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var NoopOnValueChanged = func(location, oldValue *felt.Felt) error {
	return nil
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
			address := core.ContractAddress(tt.callerAddress, tt.classHash, tt.salt, tt.constructorCallData)
			if !address.Equal(tt.want) {
				t.Errorf("wrong address: got %s, want %s", address.String(), tt.want.String())
			}
		})
	}
}

func TestNewContract(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})
	addr := new(felt.Felt).SetUint64(234)
	classHash := new(felt.Felt).SetBytes([]byte("class hash"))

	t.Run("cannot create Contract instance if un-deployed", func(t *testing.T) {
		_, err := core.NewContract(addr, txn)
		require.EqualError(t, err, core.ErrContractNotDeployed.Error())
	})

	contract, err := core.DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	t.Run("redeploy should fail", func(t *testing.T) {
		_, err := core.DeployContract(addr, classHash, txn)
		require.EqualError(t, err, core.ErrContractAlreadyDeployed.Error())
	})

	t.Run("instantiate previously deployed contract", func(t *testing.T) {
		t.Run("contract with empty storage", func(t *testing.T) {
			newContract, err := core.NewContract(addr, txn)
			require.NoError(t, err)

			root, err := newContract.Root()
			require.NoError(t, err)
			assert.Equal(t, &felt.Zero, root)
		})
		t.Run("contract with non-empty storage", func(t *testing.T) {
			oldRoot, err := contract.Root()
			require.NoError(t, err)

			require.NoError(t, contract.UpdateStorage([]core.StorageDiff{{Key: addr, Value: classHash}}, NoopOnValueChanged))

			newContract, err := core.NewContract(addr, txn)
			require.NoError(t, err)

			root, err := newContract.Root()
			require.NoError(t, err)
			assert.NotEqual(t, oldRoot, root)
		})
	})

	t.Run("a call to contract should fail with a committed txn", func(t *testing.T) {
		assert.NoError(t, txn.Commit())
		t.Run("ClassHash()", func(t *testing.T) {
			_, err := contract.ClassHash()
			assert.Error(t, err)
		})
		t.Run("Root()", func(t *testing.T) {
			_, err := contract.Root()
			assert.Error(t, err)
		})
		t.Run("Nonce()", func(t *testing.T) {
			_, err := contract.Nonce()
			assert.Error(t, err)
		})
		t.Run("Storage()", func(t *testing.T) {
			_, err := contract.Storage(classHash)
			assert.Error(t, err)
		})
		t.Run("UpdateNonce()", func(t *testing.T) {
			assert.Error(t, contract.UpdateNonce(&felt.Zero))
		})
		t.Run("UpdateStorage()", func(t *testing.T) {
			assert.Error(t, contract.UpdateStorage(nil, NoopOnValueChanged))
		})
	})
}

func TestNonceAndClassHash(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	contract, err := core.DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	t.Run("initial nonce should be 0", func(t *testing.T) {
		got, err := contract.Nonce()
		require.NoError(t, err)
		assert.Equal(t, new(felt.Felt), got)
	})
	t.Run("UpdateNonce()", func(t *testing.T) {
		require.NoError(t, contract.UpdateNonce(classHash))

		got, err := contract.Nonce()
		require.NoError(t, err)
		assert.Equal(t, classHash, got)
	})

	t.Run("ClassHash()", func(t *testing.T) {
		got, err := contract.ClassHash()
		require.NoError(t, err)
		assert.Equal(t, classHash, got)
	})

	t.Run("Replace()", func(t *testing.T) {
		replaceWith := utils.HexToFelt(t, "0xDEADBEEF")
		require.NoError(t, contract.Replace(replaceWith))
		got, err := contract.ClassHash()
		require.NoError(t, err)
		assert.Equal(t, replaceWith, got)
	})
}

func TestUpdateStorageAndStorage(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	contract, err := core.DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	t.Run("apply storage diff", func(t *testing.T) {
		oldRoot, err := contract.Root()
		require.NoError(t, err)

		require.NoError(t, contract.UpdateStorage([]core.StorageDiff{{Key: addr, Value: classHash}}, NoopOnValueChanged))

		gotValue, err := contract.Storage(addr)
		require.NoError(t, err)
		assert.Equal(t, classHash, gotValue)

		newRoot, err := contract.Root()
		require.NoError(t, err)
		assert.NotEqual(t, oldRoot, newRoot)
	})

	t.Run("delete key from storage with storage diff", func(t *testing.T) {
		require.NoError(t, contract.UpdateStorage([]core.StorageDiff{{Key: addr, Value: new(felt.Felt)}}, NoopOnValueChanged))

		val, err := contract.Storage(addr)
		require.NoError(t, err)
		require.Equal(t, &felt.Zero, val)

		sRoot, err := contract.Root()
		require.NoError(t, err)
		assert.Equal(t, new(felt.Felt), sRoot)
	})
}

func TestPurge(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)
	addr := new(felt.Felt).SetUint64(44)
	classHash := new(felt.Felt).SetUint64(37)

	contract, err := core.DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	require.NoError(t, contract.Purge())
	_, err = core.NewContract(addr, txn)
	assert.ErrorIs(t, err, core.ErrContractNotDeployed)
}
