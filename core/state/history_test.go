package state

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateHistory(t *testing.T) {
	stateDB := newTestStateDB()

	t.Run("successful creation", func(t *testing.T) {
		history, err := NewStateHistory(0, &felt.Zero, stateDB)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), history.blockNum)
		assert.NotNil(t, history.state)
	})

	t.Run("invalid state root", func(t *testing.T) {
		invalidRoot := new(felt.Felt).SetUint64(999) // Non-existent root
		_, err := NewStateHistory(1, invalidRoot, stateDB)
		assert.Error(t, err)
	})
}

func TestStateHistoryContractOperations(t *testing.T) {
	stateUpdates := []*core.StateUpdate{
		{
			OldRoot: &felt.Zero,
			NewRoot: utils.HexToFelt(
				t,
				"0x2e782bf13c68887b9f98c625aa284ba4d23237bd45fc1161442860d4a6576d8",
			),
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x1"): utils.HexToFelt(t, "0x1"),
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x1"): utils.HexToFelt(t, "0x1"),
				},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x1"): {
						*utils.HexToFelt(t, "0x1"): utils.HexToFelt(t, "0x1"),
						*utils.HexToFelt(t, "0x2"): utils.HexToFelt(t, "0x2"),
					},
				},
			},
		},
		{
			OldRoot: utils.HexToFelt(
				t,
				"0x2e782bf13c68887b9f98c625aa284ba4d23237bd45fc1161442860d4a6576d8",
			),
			NewRoot: utils.HexToFelt(
				t,
				"0x59aa7d6f2c197b91bffa600e4ba4d6d80990ed42a7321c5d01cbe06b45d95ee",
			),
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x2"): utils.HexToFelt(t, "0x2"),
					*utils.HexToFelt(t, "0x3"): utils.HexToFelt(t, "0x3"),
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x2"): utils.HexToFelt(t, "0x2"),
					*utils.HexToFelt(t, "0x3"): utils.HexToFelt(t, "0x3"),
				},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x2"): {
						*utils.HexToFelt(t, "0x1"): utils.HexToFelt(t, "0x3"),
						*utils.HexToFelt(t, "0x2"): utils.HexToFelt(t, "0x4"),
					},
					*utils.HexToFelt(t, "0x3"): {
						*utils.HexToFelt(t, "0x1"): utils.HexToFelt(t, "0x5"),
						*utils.HexToFelt(t, "0x2"): utils.HexToFelt(t, "0x6"),
					},
				},
			},
		},
	}
	stateDB := setupState(t, stateUpdates, 2)
	historyBlock0, err := NewStateHistory(0, &felt.Zero, stateDB)
	require.NoError(t, err)
	historyBlock1, err := NewStateHistory(1, &felt.Zero, stateDB)
	require.NoError(t, err)

	t.Run("ContractClassHash", func(t *testing.T) {
		hash, err := historyBlock0.ContractClassHash(utils.HexToFelt(t, "0x1"))
		require.NoError(t, err)
		assert.Equal(t, hash, *utils.HexToFelt(t, "0x1"))
		hash, err = historyBlock1.ContractClassHash(utils.HexToFelt(t, "0x2"))
		require.NoError(t, err)
		assert.Equal(t, hash, *utils.HexToFelt(t, "0x2"))
	})

	t.Run("ContractNonce", func(t *testing.T) {
		nonce, err := historyBlock0.ContractNonce(utils.HexToFelt(t, "0x1"))
		require.NoError(t, err)
		assert.Equal(t, nonce, *utils.HexToFelt(t, "0x1"))
		nonce, err = historyBlock1.ContractNonce(utils.HexToFelt(t, "0x2"))
		require.NoError(t, err)
		assert.Equal(t, nonce, *utils.HexToFelt(t, "0x2"))
	})

	t.Run("ContractStorage", func(t *testing.T) {
		value, err := historyBlock0.ContractStorage(
			utils.HexToFelt(t, "0x1"),
			utils.HexToFelt(t, "0x1"),
		)
		require.NoError(t, err)
		assert.Equal(t, value, *utils.HexToFelt(t, "0x1"))
		value, err = historyBlock1.ContractStorage(
			utils.HexToFelt(t, "0x2"),
			utils.HexToFelt(t, "0x1"),
		)
		require.NoError(t, err)
		assert.Equal(t, value, *utils.HexToFelt(t, "0x3"))
	})

	t.Run("NonExistentContract", func(t *testing.T) {
		nonExistentAddr := new(felt.Felt).SetUint64(999)
		_, err := historyBlock0.ContractClassHash(nonExistentAddr)
		assert.ErrorIs(t, err, ErrContractNotDeployed)
	})
}

func TestStateHistoryClassOperations(t *testing.T) {
	stateDB := newTestStateDB()

	class1Hash := *utils.HexToFelt(t, "0xDEADBEEF")
	class2Hash := *utils.HexToFelt(t, "0xDEADBEEF2")

	class1 := &core.Cairo1Class{}
	class2 := &core.Cairo1Class{}

	classes := map[felt.Felt]core.Class{
		class1Hash: class1,
	}
	stateUpdate := &core.StateUpdate{
		OldRoot:   &felt.Zero,
		NewRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}
	state, err := New(&felt.Zero, stateDB)
	require.NoError(t, err)
	err = state.Update(0, stateUpdate, classes)
	require.NoError(t, err)
	stateComm, err := state.Commitment()
	require.NoError(t, err)

	stateUpdate = &core.StateUpdate{
		OldRoot:   &stateComm,
		NewRoot:   &stateComm,
		StateDiff: &core.StateDiff{},
	}
	classes2 := map[felt.Felt]core.Class{
		class2Hash: class2,
	}

	state, err = New(&stateComm, stateDB)
	require.NoError(t, err)
	err = state.Update(1, stateUpdate, classes2)
	require.NoError(t, err)

	historyBlock0, err := NewStateHistory(0, &felt.Zero, stateDB)
	require.NoError(t, err)
	historyBlock1, err := NewStateHistory(1, &stateComm, stateDB)
	require.NoError(t, err)

	t.Run("Class retrieval at declaration block", func(t *testing.T) {
		retrievedClass, err := historyBlock0.Class(&class1Hash)
		require.NoError(t, err)
		assert.Equal(t, retrievedClass.Class, class1)
		assert.Equal(t, retrievedClass.At, uint64(0))

		retrievedClass, err = historyBlock1.Class(&class2Hash)
		require.NoError(t, err)
		assert.Equal(t, retrievedClass.Class, class2)
		assert.Equal(t, retrievedClass.At, uint64(1))
	})

	t.Run("NonExistentClass", func(t *testing.T) {
		nonExistentClass := utils.HexToFelt(t, "0xDEADBEEF3")
		_, err := historyBlock0.Class(nonExistentClass)
		assert.Error(t, err)
	})
}

func TestStateHistoryClassBeforeDeclaration(t *testing.T) {
	stateDB := newTestStateDB()
	history, err := NewStateHistory(0, &felt.Zero, stateDB)
	require.NoError(t, err)

	_, err = history.Class(utils.HexToFelt(t, "0xDEADBEEF"))
	assert.ErrorIs(t, err, db.ErrKeyNotFound)
}

func TestStateHistoryTrieOperations(t *testing.T) {
	stateDB := newTestStateDB()
	history, err := NewStateHistory(1, &felt.Zero, stateDB)
	require.NoError(t, err)

	t.Run("ClassTrie not supported", func(t *testing.T) {
		_, err := history.ClassTrie()
		assert.ErrorIs(t, err, ErrHistoricalTrieNotSupported)
	})

	t.Run("ContractTrie not supported", func(t *testing.T) {
		_, err := history.ContractTrie()
		assert.ErrorIs(t, err, ErrHistoricalTrieNotSupported)
	})

	t.Run("ContractStorageTrie not supported", func(t *testing.T) {
		addr := new(felt.Felt).SetUint64(1)
		_, err := history.ContractStorageTrie(addr)
		assert.ErrorIs(t, err, ErrHistoricalTrieNotSupported)
	})
}
