package migration

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/require"
)

func TestRevision0000(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	t.Run("empty DB", func(t *testing.T) {
		require.NoError(t, testDB.View(revision0000))
	})

	t.Run("non-empty DB", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set([]byte("asd"), []byte("123"))
		}))
		require.EqualError(t, testDB.View(revision0000), "initial DB should be empty")
	})
}

func TestRelocateContractStorageRootKeys(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)

	numberOfContracts := 5

	// Populate the database with entries in the old location.
	for i := 0; i < numberOfContracts; i++ {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()
		// Use exampleBytes for the key suffix (the contract address) and the value.
		err := txn.Set(db.Unused.Key(exampleBytes[:]), exampleBytes[:])
		require.NoError(t, err)
	}

	require.NoError(t, relocateContractStorageRootKeys(txn))

	// Each root-key entry should have been moved to its new location
	// and the old entry should not exist.
	for i := 0; i < numberOfContracts; i++ {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()

		// New entry exists.
		require.NoError(t, txn.Get(db.ContractStorage.Key(exampleBytes[:]), func(val []byte) error {
			require.Equal(t, exampleBytes[:], val, "the correct value was not transferred to the new location")
			return nil
		}))

		// Old entry does not exist.
		oldKey := db.Unused.Key(exampleBytes[:])
		err := txn.Get(oldKey, func(val []byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)
	}
}
