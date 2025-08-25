package commonstate

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateAdapter(t *testing.T) {
	memDB := memory.New()
	db, err := triedb.New(memDB, nil)
	if err != nil {
		panic(err)
	}
	stateDB := state.NewStateDB(memDB, db)
	state, err := state.New(&felt.Zero, stateDB)
	require.NoError(t, err)

	stateAdapter := NewStateAdapter(state)
	assert.NotNil(t, stateAdapter)
}

func TestStateReaderAdapter(t *testing.T) {
	memDB := memory.New()
	db, err := triedb.New(memDB, nil)
	if err != nil {
		panic(err)
	}
	stateDB := state.NewStateDB(memDB, db)
	state, err := state.New(&felt.Zero, stateDB)
	require.NoError(t, err)

	stateReaderAdapter := NewStateReaderAdapter(state)
	assert.NotNil(t, stateReaderAdapter)
}
