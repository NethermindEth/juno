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
	UseNewState bool
	triedb      database.TrieDB
	stateDB     *state.StateDB
}

func NewStateFactory(
	newState bool,
	triedb database.TrieDB,
	stateDB *state.StateDB,
) (*StateFactory, error) {
	if !newState {
		return &StateFactory{UseNewState: false}, nil
	}

	return &StateFactory{
		UseNewState: true,
		triedb:      triedb,
		stateDB:     stateDB,
	}, nil
}

func (sf *StateFactory) NewState(
	// todo: this should be *felt.StateRootHash
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
) (core.State, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewDeprecatedState(txn)
		return deprecatedState, nil
	}

	stateState, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return stateState, nil
}

func (sf *StateFactory) NewStateReader(
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
	blockNumber uint64,
) (core.StateReader, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewDeprecatedState(txn)
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
	if !sf.UseNewState {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewDeprecatedState(txn)
		return emptyState, nil
	}
	state, err := state.New(&felt.Zero, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return state, nil
}
