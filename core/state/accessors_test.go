package state

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractAccessors(t *testing.T) {
	database := memory.New()
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")
	nonce := felt.UnsafeFromString[felt.Felt]("0x1")
	classHash := felt.UnsafeFromString[felt.Felt]("0xabc")

	exists, err := HasContract(database, addr)
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = GetContract(database, addr)
	assert.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, WriteContract(database, addr, nonce, classHash, 42))

	exists, err = HasContract(database, addr)
	require.NoError(t, err)
	assert.True(t, exists)

	contract, err := GetContract(database, addr)
	require.NoError(t, err)
	assert.Equal(t, nonce, contract.Nonce)
	assert.Equal(t, classHash, contract.ClassHash)
	assert.Equal(t, uint64(42), contract.DeployedHeight)

	require.NoError(t, DeleteContract(database, addr))

	exists, err = HasContract(database, addr)
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = GetContract(database, addr)
	assert.ErrorIs(t, err, db.ErrKeyNotFound)
}

func TestHistoryAccessors(t *testing.T) {
	database := memory.New()
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")
	storageKey := felt.NewUnsafeFromString[felt.Felt]("0x456")
	blockNum := uint64(7)
	nonce := felt.NewUnsafeFromString[felt.Felt]("0x1")
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xabc")
	storageValue := felt.NewUnsafeFromString[felt.Felt]("0xdef")

	require.NoError(t, WriteNonceHistory(database, addr, blockNum, nonce))
	require.NoError(t, WriteClassHashHistory(database, addr, blockNum, classHash))
	require.NoError(t, WriteStorageHistory(database, addr, storageKey, blockNum, storageValue))

	require.NoError(t, database.Get(db.ContractNonceHistoryAtBlockKey(addr, blockNum), func(value []byte) error {
		assert.Equal(t, *nonce, felt.FromBytes[felt.Felt](value))
		return nil
	}))
	require.NoError(t, database.Get(db.ContractClassHashHistoryAtBlockKey(addr, blockNum), func(value []byte) error {
		assert.Equal(t, *classHash, felt.FromBytes[felt.Felt](value))
		return nil
	}))
	require.NoError(t, database.Get(db.ContractStorageHistoryAtBlockKey(addr, storageKey, blockNum), func(value []byte) error {
		assert.Equal(t, *storageValue, felt.FromBytes[felt.Felt](value))
		return nil
	}))

	require.NoError(t, DeleteNonceHistory(database, addr, blockNum))
	require.NoError(t, DeleteClassHashHistory(database, addr, blockNum))
	require.NoError(t, DeleteStorageHistory(database, addr, storageKey, blockNum))

	assertKeyNotFound(t, database, db.ContractNonceHistoryAtBlockKey(addr, blockNum))
	assertKeyNotFound(t, database, db.ContractClassHashHistoryAtBlockKey(addr, blockNum))
	assertKeyNotFound(t, database, db.ContractStorageHistoryAtBlockKey(addr, storageKey, blockNum))
}

func TestClassAccessors(t *testing.T) {
	database := memory.New()
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xabc")
	class := &core.DeclaredClassDefinition{
		At: uint64(12),
		Class: &core.SierraClass{
			SemanticVersion: "0.1.0",
			Program:         []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")},
		},
	}

	exists, err := HasClass(database, classHash)
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = GetClass(database, classHash)
	assert.ErrorIs(t, err, db.ErrKeyNotFound)

	require.NoError(t, WriteClass(database, classHash, class))

	exists, err = HasClass(database, classHash)
	require.NoError(t, err)
	assert.True(t, exists)

	actual, err := GetClass(database, classHash)
	require.NoError(t, err)
	assert.Equal(t, class.At, actual.At)
	require.IsType(t, &core.SierraClass{}, actual.Class)
	actualClass := actual.Class.(*core.SierraClass)
	assert.Equal(t, "0.1.0", actualClass.SemanticVersion)
	assert.Equal(t, []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")}, actualClass.Program)

	require.NoError(t, DeleteClass(database, classHash))

	exists, err = HasClass(database, classHash)
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = GetClass(database, classHash)
	assert.ErrorIs(t, err, db.ErrKeyNotFound)
}

func assertKeyNotFound(t *testing.T, database db.KeyValueReader, key []byte) {
	t.Helper()

	err := database.Get(key, func([]byte) error {
		return nil
	})
	assert.ErrorIs(t, err, db.ErrKeyNotFound)
}
