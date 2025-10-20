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
type StateAdapter state.State

func NewStateAdapter(s *state.State) *StateAdapter {
	return (*StateAdapter)(s)
}

func (s *StateAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := (*state.State)(s).ClassTrie()
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
}

func (s *StateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := (*state.State)(s).ContractTrie()
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
}

func (s *StateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := (*state.State)(s).ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
}

func (s *StateAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return (*state.State)(s).Class(classHash)
}

func (s *StateAdapter) Commitment() (felt.Felt, error) {
	return (*state.State)(s).Commitment()
}

func (s *StateAdapter) Revert(blockNumber uint64, update *core.StateUpdate) error {
	return (*state.State)(s).Revert(blockNumber, update)
}

func (s *StateAdapter) Update(
	blockNumber uint64,
	update *core.StateUpdate,
	declaredClasses map[felt.Felt]core.Class,
	skipVerifyNewRoot bool,
	flushChanges bool,
) error {
	return (*state.State)(s).Update(
		blockNumber,
		update,
		declaredClasses,
		skipVerifyNewRoot,
		flushChanges,
	)
}

func (s *StateAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return (*state.State)(s).ContractClassHash(addr)
}

func (s *StateAdapter) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	return (*state.State)(s).ContractClassHashAt(addr, blockNumber)
}

func (s *StateAdapter) ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	return (*state.State)(s).ContractDeployedAt(addr, blockNumber)
}

func (s *StateAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return (*state.State)(s).ContractNonce(addr)
}

func (s *StateAdapter) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	return (*state.State)(s).ContractNonceAt(addr, blockNumber)
}

func (s *StateAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return (*state.State)(s).ContractStorage(addr, key)
}

func (s *StateAdapter) ContractStorageAt(
	addr,
	key *felt.Felt,
	blockNumber uint64,
) (felt.Felt, error) {
	return (*state.State)(s).ContractStorageAt(addr, key, blockNumber)
}

type StateReaderAdapter struct {
	state.StateReader
}

func NewStateReaderAdapter(s state.StateReader) *StateReaderAdapter {
	return &StateReaderAdapter{StateReader: s}
}

func (s *StateReaderAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return s.StateReader.Class(classHash)
}

func (s *StateReaderAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractClassHash(addr)
}

func (s *StateReaderAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractNonce(addr)
}

func (s *StateReaderAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractStorage(addr, key)
}

func (s *StateReaderAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := s.StateReader.ClassTrie()
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
}

func (s *StateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := s.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
}

func (s *StateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := s.StateReader.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return (*commontrie.TrieAdapter)(t), nil
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
		return NewDeprecatedStateAdapter(deprecatedState), nil
	}

	stateState, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateAdapter(stateState), nil
}

func (sf *StateFactory) NewStateReader(
	stateRoot *felt.Felt,
	txn db.IndexedBatch,
	blockNumber uint64,
) (StateReader, error) {
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
