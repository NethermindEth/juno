package state_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractAccessors(t *testing.T) {
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")

	// storeContract stores a contract with the given values and fails the test on error.
	storeContract := func(
		t *testing.T, disk db.KeyValueWriter, nonce, classHash string, height uint64,
	) {
		t.Helper()
		require.NoError(t, state.WriteContract(
			disk, addr,
			felt.UnsafeFromString[felt.Felt](nonce),
			felt.UnsafeFromString[felt.Felt](classHash),
			height,
		))
	}

	t.Run("absent before write", func(t *testing.T) {
		disk := memory.New()

		exists, err := state.HasContract(disk, addr)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = state.GetContract(disk, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("write then read round-trip", func(t *testing.T) {
		disk := memory.New()
		storeContract(t, disk, "0x1", "0xabc", 42)

		exists, err := state.HasContract(disk, addr)
		require.NoError(t, err)
		assert.True(t, exists)

		contract, err := state.GetContract(disk, addr)
		require.NoError(t, err)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0x1"), contract.Nonce)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0xabc"), contract.ClassHash)
		assert.Equal(t, uint64(42), contract.DeployedHeight)
	})

	t.Run("overwrite reflects latest values", func(t *testing.T) {
		disk := memory.New()
		storeContract(t, disk, "0x1", "0xabc", 42)
		storeContract(t, disk, "0x2", "0xdef", 99)

		contract, err := state.GetContract(disk, addr)
		require.NoError(t, err)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0x2"), contract.Nonce)
		assert.Equal(t, felt.UnsafeFromString[felt.Felt]("0xdef"), contract.ClassHash)
		assert.Equal(t, uint64(99), contract.DeployedHeight)
	})

	t.Run("delete removes the contract", func(t *testing.T) {
		disk := memory.New()
		storeContract(t, disk, "0x1", "0xabc", 42)

		require.NoError(t, state.DeleteContract(disk, addr))

		exists, err := state.HasContract(disk, addr)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = state.GetContract(disk, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestClassAccessors(t *testing.T) {
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xabc")
	newClass := func() *core.DeclaredClassDefinition {
		return &core.DeclaredClassDefinition{
			At: 12,
			Class: &core.SierraClass{
				SemanticVersion: "0.1.0",
				Program:         []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")},
			},
		}
	}

	t.Run("absent before write", func(t *testing.T) {
		disk := memory.New()

		exists, err := state.HasClass(disk, classHash)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = state.GetClass(disk, classHash)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("write then read round-trips the encoded class", func(t *testing.T) {
		disk := memory.New()
		class := newClass()
		require.NoError(t, state.WriteClass(disk, classHash, class))

		exists, err := state.HasClass(disk, classHash)
		require.NoError(t, err)
		assert.True(t, exists)

		got, err := state.GetClass(disk, classHash)
		require.NoError(t, err)
		assert.Equal(t, class.At, got.At)
		sierra, ok := got.Class.(*core.SierraClass)
		require.True(t, ok)
		assert.Equal(t, "0.1.0", sierra.SemanticVersion)
		assert.Equal(t, []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x1")}, sierra.Program)
	})

	t.Run("delete removes the class", func(t *testing.T) {
		disk := memory.New()
		require.NoError(t, state.WriteClass(disk, classHash, newClass()))

		require.NoError(t, state.DeleteClass(disk, classHash))

		exists, err := state.HasClass(disk, classHash)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = state.GetClass(disk, classHash)
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
		readAt   func(r *state.StateReader, blockNum uint64) (felt.Felt, error)
		valueOne *felt.Felt
		valueTwo *felt.Felt
	}

	kinds := []historyKind{
		{
			name: "nonce",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return state.WriteNonceHistory(w, addr, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return state.DeleteNonceHistory(w, addr, blockNum)
			},
			readAt: func(r *state.StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractNonceAt(addr, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0x7"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0x2a"),
		},
		{
			name: "class hash",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return state.WriteClassHashHistory(w, addr, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return state.DeleteClassHashHistory(w, addr, blockNum)
			},
			readAt: func(r *state.StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractClassHashAt(addr, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0xaaa"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0xbbb"),
		},
		{
			name: "storage",
			write: func(w db.KeyValueWriter, blockNum uint64, value *felt.Felt) error {
				return state.WriteStorageHistory(w, addr, storageKey, blockNum, value)
			},
			del: func(w db.KeyValueWriter, blockNum uint64) error {
				return state.DeleteStorageHistory(w, addr, storageKey, blockNum)
			},
			readAt: func(r *state.StateReader, blockNum uint64) (felt.Felt, error) {
				return r.ContractStorageAt(addr, storageKey, blockNum)
			},
			valueOne: felt.NewUnsafeFromString[felt.Felt]("0xdef"),
			valueTwo: felt.NewUnsafeFromString[felt.Felt]("0xfed"),
		},
	}

	const blockOne, blockTwo = uint64(7), uint64(15)

	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			// newReader returns a fresh disk store and a reader over it, so each subtest
			// starts from an empty history and is independent of execution order.
			newReader := func(t *testing.T) (db.KeyValueStore, *state.StateReader) {
				t.Helper()
				disk := memory.New()
				stateDB := state.NewStateDB(disk, triedb.New(disk, nil))
				reader, err := state.NewStateReader(&felt.Zero, stateDB)
				require.NoError(t, err)
				return disk, reader
			}

			t.Run("reads the written value at its block", func(t *testing.T) {
				disk, reader := newReader(t)
				require.NoError(t, k.write(disk, blockOne, k.valueOne))

				got, err := k.readAt(reader, blockOne)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, got)
			})

			t.Run("steps back to the last entry for a later block", func(t *testing.T) {
				disk, reader := newReader(t)
				require.NoError(t, k.write(disk, blockOne, k.valueOne))

				got, err := k.readAt(reader, blockOne+3)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, got)
			})

			t.Run("reads zero before the first entry", func(t *testing.T) {
				disk, reader := newReader(t)
				require.NoError(t, k.write(disk, blockOne, k.valueOne))

				got, err := k.readAt(reader, blockOne-1)
				require.NoError(t, err)
				assert.Equal(t, felt.Zero, got)
			})

			t.Run("honours block boundaries across two entries", func(t *testing.T) {
				disk, reader := newReader(t)
				require.NoError(t, k.write(disk, blockOne, k.valueOne))
				require.NoError(t, k.write(disk, blockTwo, k.valueTwo))

				between, err := k.readAt(reader, blockTwo-1)
				require.NoError(t, err)
				assert.Equal(t, *k.valueOne, between)

				after, err := k.readAt(reader, blockTwo)
				require.NoError(t, err)
				assert.Equal(t, *k.valueTwo, after)
			})

			t.Run("delete drops the entry", func(t *testing.T) {
				disk, reader := newReader(t)
				require.NoError(t, k.write(disk, blockOne, k.valueOne))
				require.NoError(t, k.write(disk, blockTwo, k.valueTwo))

				require.NoError(t, k.del(disk, blockOne))
				require.NoError(t, k.del(disk, blockTwo))

				got, err := k.readAt(reader, blockTwo+1)
				require.NoError(t, err)
				assert.Equal(t, felt.Zero, got)
			})
		})
	}
}

func TestGetStateObject(t *testing.T) {
	addr := felt.NewUnsafeFromString[felt.Felt]("0x123")

	t.Run("propagates not-found for a missing contract", func(t *testing.T) {
		disk := memory.New()

		_, err := state.GetStateObject(disk, nil, addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}
