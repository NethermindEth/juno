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

	Update(blockNum uint64,
		update *core.StateUpdate,
		declaredClasses map[felt.Felt]core.Class,
		skipVerifyNewRoot bool,
		flushChanges bool,
	) error
	Revert(blockNum uint64, update *core.StateUpdate) error
	Commitment() (felt.Felt, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*core.DeclaredClass, error)

	ClassTrie() (commontrie.Trie, error)
	ContractTrie() (commontrie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error)
}

// StateAdapter wraps state.State to implement CommonState
type StateAdapter struct {
	*state.State
}

func NewStateAdapter(s *state.State) *StateAdapter {
	return &StateAdapter{State: s}
}

func (sa *StateAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := sa.State.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (sa *StateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := sa.State.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (sa *StateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := sa.State.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

type StateReaderAdapter struct {
	state.StateReader
}

func NewStateReaderAdapter(s state.StateReader) *StateReaderAdapter {
	return &StateReaderAdapter{StateReader: s}
}

func (sha *StateReaderAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return sha.StateReader.Class(classHash)
}

func (sha *StateReaderAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return sha.StateReader.ContractClassHash(addr)
}

func (sha *StateReaderAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return sha.StateReader.ContractNonce(addr)
}

func (sha *StateReaderAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return sha.StateReader.ContractStorage(addr, key)
}

func (sha *StateReaderAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := sha.StateReader.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (sha *StateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := sha.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (sha *StateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := sha.StateReader.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

type StateFactory struct {
	UseNewState bool
	triedb      *triedb.Database
	stateDB     *state.StateDB
}

func NewStateFactory(newState bool, triedb *triedb.Database, stateDB *state.StateDB) (*StateFactory, error) {
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
		return NewDeprecatedStateAdapter(deprecatedState), nil
	}

	stateState, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateAdapter(stateState), nil
}

func (sf *StateFactory) NewStateReader(stateRoot *felt.Felt, txn db.IndexedBatch, blockNumber uint64) (StateReader, error) {
	if !sf.UseNewState {
		deprecatedState := core.NewState(txn)
		snapshot := core.NewStateSnapshot(deprecatedState, blockNumber)
		return NewDeprecatedStateReaderAdapter(snapshot), nil
	}

	history, err := state.NewStateHistory(blockNumber, stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateReaderAdapter(&history), nil
}

func (sf *StateFactory) EmptyState() (StateReader, error) {
	if !sf.UseNewState {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewState(txn)
		return NewDeprecatedStateReaderAdapter(emptyState), nil
	}
	state, err := state.New(&felt.Zero, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateReaderAdapter(state), nil
}
