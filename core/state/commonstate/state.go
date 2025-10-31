package commonstate

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/state/commontrie"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

//go:generate mockgen -destination=../../mocks/mock_commonstate_reader.go -package=mocks github.com/NethermindEth/juno/core/state/commonstate StateReader
type State interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)

	Update(
		blockNum uint64,
		update *core.StateUpdate,
		declaredClasses map[felt.Felt]core.ClassDefinition,
		skipVerifyNewRoot bool,
	) error
	Revert(blockNum uint64, update *core.StateUpdate) error
	Commitment() (felt.Felt, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*core.DeclaredClassDefinition, error)

	ClassTrie() (commontrie.Trie, error)
	ContractTrie() (commontrie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error)
}

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

func (sf *StateFactory) NewState(stateRoot *felt.Felt, txn db.IndexedBatch) (State, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewState(txn)
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
) (StateReader, error) {
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

func (sf *StateFactory) EmptyState() (StateReader, error) {
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
