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

// DeprecatedStateAdapter wraps core.State to implement CommonState
type DeprecatedStateAdapter struct {
	*core.State
}

func NewDeprecatedStateAdapter(s *core.State) *DeprecatedStateAdapter {
	return &DeprecatedStateAdapter{State: s}
}

func (csa *DeprecatedStateAdapter) ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	value, err := csa.State.ContractStorageAt(addr, key, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (csa *DeprecatedStateAdapter) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	nonce, err := csa.State.ContractNonceAt(addr, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (csa *DeprecatedStateAdapter) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	classHash, err := csa.State.ContractClassHashAt(addr, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (csa *DeprecatedStateAdapter) ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	return csa.State.ContractIsAlreadyDeployedAt(addr, blockNumber)
}

func (csa *DeprecatedStateAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	classHash, err := csa.State.ContractClassHash(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (csa *DeprecatedStateAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	nonce, err := csa.State.ContractNonce(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (csa *DeprecatedStateAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	value, err := csa.State.ContractStorage(addr, key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (csa *DeprecatedStateAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return csa.State.Class(classHash)
}

func (csa *DeprecatedStateAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := csa.State.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (csa *DeprecatedStateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := csa.State.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (csa *DeprecatedStateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := csa.State.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (csa *DeprecatedStateAdapter) Commitment() (felt.Felt, error) {
	root, err := csa.State.Root()
	if err != nil {
		return felt.Felt{}, err
	}
	return *root, nil
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

type DeprecatedStateReaderAdapter struct {
	core.StateReader
}

func NewDeprecatedStateReaderAdapter(s core.StateReader) *DeprecatedStateReaderAdapter {
	return &DeprecatedStateReaderAdapter{StateReader: s}
}

func (ssa *DeprecatedStateReaderAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return ssa.StateReader.Class(classHash)
}

func (ssa *DeprecatedStateReaderAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	classHash, err := ssa.StateReader.ContractClassHash(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (ssa *DeprecatedStateReaderAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	nonce, err := ssa.StateReader.ContractNonce(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (ssa *DeprecatedStateReaderAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	value, err := ssa.StateReader.ContractStorage(addr, key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (ssa *DeprecatedStateReaderAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := ssa.StateReader.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (ssa *DeprecatedStateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := ssa.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (ssa *DeprecatedStateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := ssa.StateReader.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
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
		coreState := core.NewState(txn)
		return NewDeprecatedStateAdapter(coreState), nil
	}

	stateState, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateAdapter(stateState), nil
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
