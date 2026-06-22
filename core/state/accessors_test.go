package state

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractAccessors(t *testing.T) {
	stateDB := newTestStateDB()
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")

	t.Run("absent before write", func(t *testing.T) {
		exists, err := HasContract(stateDB.disk, addr)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = GetContract(stateDB.disk, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("write then read round-trip", func(t *testing.T) {
		require.NoError(t, WriteContract(
			stateDB.disk, addr,
			felt.UnsafeFromString[felt.Felt]("0x1"),   // nonce
			felt.UnsafeFromString[felt.Felt]("0xabc"), // classHash
			42,                                        // deployHeight
		))

		exists, err := HasContract(stateDB.disk, addr)
		require.NoError(t, err)
		assert.True(t, exists)

		contract, err := GetContract(stateDB.disk, addr)
		require.NoError(t, err)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0x1"), contract.Nonce)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0xabc"), contract.ClassHash)
		assert.Equal(t, uint64(42), contract.DeployedHeight)
	})

	t.Run("overwrite reflects latest values", func(t *testing.T) {
		require.NoError(t, WriteContract(
			stateDB.disk, addr,
			felt.UnsafeFromString[felt.Felt]("0x2"),
			felt.UnsafeFromString[felt.Felt]("0xdef"),
			99,
		))

		contract, err := GetContract(stateDB.disk, addr)
		require.NoError(t, err)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0x2"), contract.Nonce)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0xdef"), contract.ClassHash)
		assert.Equal(t, uint64(99), contract.DeployedHeight)
	})

	t.Run("delete removes the contract", func(t *testing.T) {
		require.NoError(t, DeleteContract(stateDB.disk, addr))

		exists, err := HasContract(stateDB.disk, addr)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = GetContract(stateDB.disk, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestClassAccessors(t *testing.T) {
	stateDB := newTestStateDB()
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xabc")
	class := &core.DeclaredClassDefinition{
		At: 12,
		Class: &core.SierraClass{
			SemanticVersion: "0.1.0",
			Program:         []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")},
		},
	}

	t.Run("absent before write", func(t *testing.T) {
		exists, err := HasClass(stateDB.disk, classHash)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = GetClass(stateDB.disk, classHash)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("write then read round-trips the encoded class", func(t *testing.T) {
		require.NoError(t, WriteClass(stateDB.disk, classHash, class))

		exists, err := HasClass(stateDB.disk, classHash)
		require.NoError(t, err)
		assert.True(t, exists)

		got, err := GetClass(stateDB.disk, classHash)
		require.NoError(t, err)
		assert.Equal(t, class.At, got.At)
		sierra, ok := got.Class.(*core.SierraClass)
		require.True(t, ok)
		assert.Equal(t, "0.1.0", sierra.SemanticVersion)
		assert.Equal(t, []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")}, sierra.Program)
	})

	t.Run("delete removes the class", func(t *testing.T) {
		require.NoError(t, DeleteClass(stateDB.disk, classHash))

		exists, err := HasClass(stateDB.disk, classHash)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = GetClass(stateDB.disk, classHash)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

// Reads values back through the StateReader history readers, which use a different key path
// than the accessors, so the round-trip tests behavior rather than the accessor's own keys.
func TestHistoryAccessors(t *testing.T) {
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")
	storageKey := felt.NewUnsafeFromString[felt.Felt]("0x456")

	type historyKind struct {
		name     string
		write    func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error
		del      func(w db.KeyValueWriter, blockNum uint64) error
		readAt   func(r *StateReader, blockNum uint64) (felt.Felt, error)
		valueOne *felt.Felt
		valueTwo *felt.Felt
	}

	kinds := []historyKind{
		{
			name: "nonce",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return WriteNonceHistory(w, addr, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return DeleteNonceHistory(w, addr, blockNum)
			},
			readAt: func(r *StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractNonceAt(addr, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0x7"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0x2a"),
		},
		{
			name: "class hash",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return WriteClassHashHistory(w, addr, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return DeleteClassHashHistory(w, addr, blockNum)
			},
			readAt: func(r *StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractClassHashAt(addr, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0xaaa"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0xbbb"),
		},
		{
			name: "storage",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return WriteStorageHistory(w, addr, storageKey, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return DeleteStorageHistory(w, addr, storageKey, blockNum)
			},
			readAt: func(r *StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractStorageAt(addr, storageKey, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0xdef"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0xfed"),
		},
	}

	const blockOne, blockTwo = uint64(7), uint64(15)

	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			stateDB := newTestStateDB()
			reader, err := NewStateReader(&felt.Zero, stateDB)
			require.NoError(t, err)

			require.NoError(t, k.write(stateDB.disk, blockOne, k.valueOne))

			t.Run("reads the written value at its block", func(t *testing.T) {
				got, err := k.readAt(reader, blockOne)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, got)
			})

			t.Run("steps back to the last entry for a later block", func(t *testing.T) {
				got, err := k.readAt(reader, blockOne+3)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, got)
			})

			t.Run("reads zero before the first entry", func(t *testing.T) {
				got, err := k.readAt(reader, blockOne-1)
				require.NoError(t, err)
				assert.Equal(t, felt.Zero, got)
			})

			t.Run("honours block boundaries across two entries", func(t *testing.T) {
				require.NoError(t, k.write(stateDB.disk, blockTwo, k.valueTwo))

				between, err := k.readAt(reader, blockTwo-1)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, between)

				after, err := k.readAt(reader, blockTwo)
				require.NoError(t, err)
				assert.Equal(t, *k.valueTwo, after)
			})

			t.Run("delete drops the entry", func(t *testing.T) {
				require.NoError(t, k.del(stateDB.disk, blockOne))
				require.NoError(t, k.del(stateDB.disk, blockTwo))

				got, err := k.readAt(reader, blockTwo+1)
				require.NoError(t, err)
				assert.Equal(t, felt.Zero, got)
			})
		})
	}
}

func TestGetStateObject(t *testing.T) {
	stateDB := newTestStateDB()
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")

	t.Run("propagates not-found for a missing contract", func(t *testing.T) {
		_, err := GetStateObject(stateDB.disk, nil, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("wraps the stored contract on success", func(t *testing.T) {
		require.NoError(t, WriteContract(stateDB.disk, addr,
			felt.UnsafeFromString[felt.Felt]("0x1"),
			felt.UnsafeFromString[felt.Felt]("0xabc"),
			42,
		))

		obj, err := GetStateObject(stateDB.disk, nil, addr)
		require.NoError(t, err)
		assert.Equal(t, *addr, obj.addr)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0x1"), obj.contract.Nonce)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0xabc"), obj.contract.ClassHash)
		assert.Equal(t, uint64(42), obj.contract.DeployedHeight)
	})
}
