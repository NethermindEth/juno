package statefactory

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

type StateFactory struct {
	UseNewState bool
	triedb      *triedb.Database
	stateDB     *state.StateDB
}

func NewStateFactory(
	newState bool,
	triedb *triedb.Database,
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
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
) (core.CommonState, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewState(txn)
		return deprecatedState, nil
	}

	state, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (sf *StateFactory) NewStateReader(
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
	blockNumber uint64,
) (core.CommonStateReader, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewState(txn)
		snapshot := core.NewStateSnapshot(deprecatedState, blockNumber)
		return snapshot, nil
	}

	history, err := state.NewStateHistory(blockNumber, stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func (sf *StateFactory) EmptyState() (core.CommonStateReader, error) {
	if !sf.UseNewState {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewState(txn)
		return emptyState, nil
	}
	state, err := state.New(&felt.Zero, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return state, nil
}
