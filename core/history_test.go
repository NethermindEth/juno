package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageValueAt(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
		require.NoError(t, testDB.Close())
	})

	history := core.NewHistory(txn)

	contractAddress := new(felt.Felt).SetUint64(123)
	location := new(felt.Felt).SetUint64(456)

	t.Run("no history", func(t *testing.T) {
		_, err := history.ContractStorageAt(contractAddress, location, 1)
		assert.EqualError(t, err, "check head state")
	})

	value := new(felt.Felt).SetUint64(789)

	t.Run("log value changed at height 5 and 10", func(t *testing.T) {
		assert.NoError(t, history.LogContractStorage(contractAddress, location, &felt.Zero, 5))
		assert.NoError(t, history.LogContractStorage(contractAddress, location, value, 10))
	})

	t.Run("get value before height 5", func(t *testing.T) {
		oldValue, err := history.ContractStorageAt(contractAddress, location, 1)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, oldValue)
	})

	t.Run("get value between height 5-10 ", func(t *testing.T) {
		oldValue, err := history.ContractStorageAt(contractAddress, location, 7)
		require.NoError(t, err)
		assert.Equal(t, value, oldValue)
	})

	t.Run("get value on height that change happened ", func(t *testing.T) {
		oldValue, err := history.ContractStorageAt(contractAddress, location, 5)
		require.NoError(t, err)
		assert.Equal(t, value, oldValue)

		_, err = history.ContractStorageAt(contractAddress, location, 10)
		assert.EqualError(t, err, "check head state")
	})

	t.Run("get value after height 10 ", func(t *testing.T) {
		_, err := history.ContractStorageAt(contractAddress, location, 13)
		assert.EqualError(t, err, "check head state")
	})

	t.Run("get a random location ", func(t *testing.T) {
		_, err := history.ContractStorageAt(contractAddress, new(felt.Felt).SetUint64(37), 13)
		assert.EqualError(t, err, "check head state")
	})
}

func TestDeleteContractStorageLog(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
		require.NoError(t, testDB.Close())
	})

	history := core.NewHistory(txn)

	contractAddress := new(felt.Felt).SetUint64(123)
	location := new(felt.Felt).SetUint64(456)
	assert.NoError(t, history.LogContractStorage(contractAddress, location, &felt.Zero, 5))

	t.Run("get before delete", func(t *testing.T) {
		oldValue, err := history.ContractStorageAt(contractAddress, location, 2)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, oldValue)
	})

	require.NoError(t, history.DeleteContractStorageLog(contractAddress, location, 5))

	t.Run("get after delete", func(t *testing.T) {
		_, err := history.ContractStorageAt(contractAddress, location, 2)
		assert.EqualError(t, err, "check head state")
	})
}
