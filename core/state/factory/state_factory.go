package factory

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/db"
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
	batch db.Batch,
) (core.State, error) {
	if !sf.UseNewState {
		deprecatedState := deprecatedstate.New(txn)
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
	if !sf.UseNewState {
		deprecatedState := deprecatedstate.New(txn)
		history := deprecatedstate.NewHistory(deprecatedState, blockNumber)
		return history, nil
	}

	history, err := state.NewStateHistory(blockNumber, stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return &history, nil
}
