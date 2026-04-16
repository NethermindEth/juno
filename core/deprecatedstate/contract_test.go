package deprecatedstate

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestNewContractUpdater(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()

	t.Run("contract not deployed returns ErrContractNotDeployed", func(t *testing.T) {
		addr := felt.NewRandom[felt.Felt]()

		_, err := NewContractUpdater(addr, txn)
		require.ErrorIs(t, err, ErrContractNotDeployed)
	})

	t.Run("deployed contract returns updater", func(t *testing.T) {
		addr := felt.NewRandom[felt.Felt]()
		classHash := felt.NewRandom[felt.Felt]()

		_, err := DeployContract(addr, classHash, txn)
		require.NoError(t, err)

		updater, err := NewContractUpdater(addr, txn)
		require.NoError(t, err)
		require.NotNil(t, updater)
		require.Equal(t, addr, updater.Address)
	})
}

func TestContractRoot(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()
	addr := felt.NewRandom[felt.Felt]()

	classHash := felt.NewRandom[felt.Felt]()
	_, err := DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	t.Run("returns zero root for empty storage", func(t *testing.T) {
		root, err := ContractRoot(addr, txn)
		require.NoError(t, err)
		require.True(t, root.IsZero(), "empty storage should have zero root")
	})

	updater, err := NewContractUpdater(addr, txn)
	require.NoError(t, err)

	key := felt.NewRandom[felt.Felt]()
	value := felt.NewRandom[felt.Felt]()
	err = updater.UpdateStorage(map[felt.Felt]*felt.Felt{
		*key: value,
	}, func(location, oldValue *felt.Felt) error { return nil })
	require.NoError(t, err)

	t.Run("returns non-zero root after storage update", func(t *testing.T) {
		root, err := ContractRoot(addr, txn)
		require.NoError(t, err)
		require.False(t, root.IsZero(), "non-empty storage should have non-zero root")
	})

	require.NoError(t, txn.Write())

	txn2 := testDB.NewIndexedBatch()
	addrBytes := addr.Marshal()
	storagePrefix := db.ContractStorage.Key(addrBytes)
	err = txn2.Put(storagePrefix, []byte{0xFF, 0xFF, 0xFF})
	require.NoError(t, err)
	require.NoError(t, txn2.Write())

	t.Run("returns error when Root() fails on corrupted trie data", func(t *testing.T) {
		txn3 := testDB.NewIndexedBatch()
		_, err := ContractRoot(addr, txn3)
		require.Error(t, err, "should fail on corrupted trie data")
	})
}

func TestContractStorage(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()
	addr := felt.NewRandom[felt.Felt]()
	key := felt.NewRandom[felt.Felt]()

	classHash := felt.NewRandom[felt.Felt]()
	_, err := DeployContract(addr, classHash, txn)
	require.NoError(t, err)

	t.Run("returns zero for non-existent key", func(t *testing.T) {
		value, err := ContractStorage(addr, key, txn)
		require.NoError(t, err)
		require.True(t, value.IsZero(), "non-existent key should return zero")
	})

	updater, err := NewContractUpdater(addr, txn)
	require.NoError(t, err)

	expectedValue := felt.NewRandom[felt.Felt]()
	err = updater.UpdateStorage(map[felt.Felt]*felt.Felt{
		*key: expectedValue,
	}, func(location, oldValue *felt.Felt) error { return nil })
	require.NoError(t, err)

	t.Run("returns stored value after update", func(t *testing.T) {
		value, err := ContractStorage(addr, key, txn)
		require.NoError(t, err)
		require.Equal(t, expectedValue, &value, "should return the stored value")
	})

	require.NoError(t, txn.Write())

	txn2 := testDB.NewIndexedBatch()
	addrBytes := addr.Marshal()
	storagePrefix := db.ContractStorage.Key(addrBytes)
	err = txn2.Put(storagePrefix, []byte{0xFF, 0xFF, 0xFF})
	require.NoError(t, err)
	require.NoError(t, txn2.Write())

	t.Run("returns error when Get() fails on corrupted trie data", func(t *testing.T) {
		txn3 := testDB.NewIndexedBatch()
		_, err := ContractStorage(addr, key, txn3)
		require.Error(t, err, "should fail on corrupted trie data")
	})
}
