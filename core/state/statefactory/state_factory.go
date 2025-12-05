package statefactory

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

type StateFactory struct {
	useNewState bool
	triedb      database.TrieDB
	stateDB     *state.StateDB
}

func NewStateFactory(
	newState bool,
	triedb database.TrieDB,
	stateDB *state.StateDB,
) *StateFactory {
	if !newState {
		return &StateFactory{useNewState: false}
	}

	return &StateFactory{
		useNewState: true,
		triedb:      triedb,
		stateDB:     stateDB,
	}
}

func (sf *StateFactory) NewState(
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
	batch db.Batch,
) (core.State, error) {
	if !sf.useNewState {
		deprecatedState := core.NewState(txn)
		return deprecatedState, nil
	}

	state, err := state.New(stateRoot, sf.stateDB, batch)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (sf *StateFactory) NewStateReader(
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
	blockNumber uint64,
) (core.StateReader, error) {
	if !sf.useNewState {
		deprecatedState := core.NewState(txn)
		history := core.NewDeprecatedStateHistory(deprecatedState, blockNumber)
		return history, nil
	}

	history, err := state.NewStateHistory(blockNumber, stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func (sf *StateFactory) EmptyState() (core.StateReader, error) {
	if !sf.useNewState {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewState(txn)
		return emptyState, nil
	}
	state, err := state.New(&felt.Zero, sf.stateDB, nil)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (sf *StateFactory) UseNewState() bool {
	return sf.useNewState
}
