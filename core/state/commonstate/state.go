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

	Update(blockNum uint64, update *core.StateUpdate, declaredClasses map[felt.Felt]core.Class, skipVerifyNewRoot bool) error
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

// CoreStateAdapter wraps core.State to implement CommonState
type CoreStateAdapter struct {
	*core.State
}

func NewCoreStateAdapter(s *core.State) *CoreStateAdapter {
	return &CoreStateAdapter{State: s}
}

func (csa *CoreStateAdapter) ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	value, err := csa.State.ContractStorageAt(addr, key, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (csa *CoreStateAdapter) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	nonce, err := csa.State.ContractNonceAt(addr, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (csa *CoreStateAdapter) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	classHash, err := csa.State.ContractClassHashAt(addr, blockNumber)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (csa *CoreStateAdapter) ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	return csa.State.ContractIsAlreadyDeployedAt(addr, blockNumber)
}

func (csa *CoreStateAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	classHash, err := csa.State.ContractClassHash(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (csa *CoreStateAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	nonce, err := csa.State.ContractNonce(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (csa *CoreStateAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	value, err := csa.State.ContractStorage(addr, key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (csa *CoreStateAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return csa.State.Class(classHash)
}

func (csa *CoreStateAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := csa.State.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (csa *CoreStateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := csa.State.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (csa *CoreStateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := csa.State.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (csa *CoreStateAdapter) Commitment() (felt.Felt, error) {
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
	return commontrie.NewTrie2Adapter(t), nil
}

func (sa *StateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := sa.State.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrie2Adapter(t), nil
}

func (sa *StateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := sa.State.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrie2Adapter(t), nil
}

type CoreStateReaderAdapter struct {
	core.StateReader
}

func NewCoreStateReaderAdapter(s core.StateReader) *CoreStateReaderAdapter {
	return &CoreStateReaderAdapter{StateReader: s}
}

func (ssa *CoreStateReaderAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return ssa.StateReader.Class(classHash)
}

func (ssa *CoreStateReaderAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	classHash, err := ssa.StateReader.ContractClassHash(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *classHash, nil
}

func (ssa *CoreStateReaderAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	nonce, err := ssa.StateReader.ContractNonce(addr)
	if err != nil {
		return felt.Zero, err
	}
	return *nonce, nil
}

func (ssa *CoreStateReaderAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	value, err := ssa.StateReader.ContractStorage(addr, key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (ssa *CoreStateReaderAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := ssa.StateReader.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (ssa *CoreStateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := ssa.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrieAdapter(t), nil
}

func (ssa *CoreStateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := ssa.StateReader.ContractStorageTrie(addr)
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
	return commontrie.NewTrie2Adapter(t), nil
}

func (sha *StateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := sha.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrie2Adapter(t), nil
}

func (sha *StateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := sha.StateReader.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewTrie2Adapter(t), nil
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
		return NewCoreStateAdapter(coreState), nil
	}

	stateState, err := state.New(stateRoot, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateAdapter(stateState), nil
}

func (sf *StateFactory) NewStateReader(stateRoot *felt.Felt, txn db.IndexedBatch, blockNumber uint64) (StateReader, error) {
	if !sf.UseNewState {
		coreState := core.NewState(txn)
		snapshot := core.NewStateSnapshot(coreState, blockNumber)
		return NewCoreStateReaderAdapter(snapshot), nil
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
		return NewCoreStateReaderAdapter(emptyState), nil
	}
	state, err := state.New(&felt.Zero, sf.stateDB)
	if err != nil {
		return nil, err
	}
	return NewStateReaderAdapter(state), nil
}
