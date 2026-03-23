package statefactory

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb/rawdb"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFactory(t *testing.T, useNewState bool) *StateFactory {
	t.Helper()
	if !useNewState {
		sf := NewStateFactory(false, nil, nil)
		require.NotNil(t, sf)
		return sf
	}
	memDB := memory.New()
	trieDB := rawdb.New(memDB)
	stateDB := state.NewStateDB(memDB, trieDB)
	sf := NewStateFactory(true, trieDB, stateDB)
	require.NotNil(t, sf)
	return sf
}

func TestStateFactory_NewState(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		sf := newTestFactory(t, false)
		txn := memory.New().NewIndexedBatch()

		st, err := sf.NewState(&felt.Zero, txn, nil)
		require.NoError(t, err)
		assert.NotNil(t, st)
	})

	t.Run("new impl", func(t *testing.T) {
		sf := newTestFactory(t, true)
		batch := memory.New().NewBatch()

		st, err := sf.NewState(&felt.Zero, nil, batch)
		require.NoError(t, err)
		assert.NotNil(t, st)
	})
}

func TestStateFactory_NewStateHistory(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		sf := newTestFactory(t, false)
		txn := memory.New().NewIndexedBatch()

		reader, err := sf.NewStateHistory(&felt.Zero, txn, 0)
		require.NoError(t, err)
		assert.NotNil(t, reader)
	})

	t.Run("new impl", func(t *testing.T) {
		sf := newTestFactory(t, true)

		reader, err := sf.NewStateHistory(&felt.Zero, nil, 0)
		require.NoError(t, err)
		assert.NotNil(t, reader)
	})
}

func TestStateFactory_NewStateReader(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		sf := newTestFactory(t, false)
		txn := memory.New().NewIndexedBatch()

		reader, err := sf.NewStateReader(&felt.Zero, txn)
		require.NoError(t, err)
		assert.NotNil(t, reader)
	})

	t.Run("new impl", func(t *testing.T) {
		sf := newTestFactory(t, true)

		reader, err := sf.NewStateReader(&felt.Zero, nil)
		require.NoError(t, err)
		assert.NotNil(t, reader)
	})
}
