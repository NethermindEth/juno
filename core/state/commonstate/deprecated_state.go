package commonstate

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
)

// DeprecatedStateAdapter wraps core.State to implement CommonState
type DeprecatedStateAdapter core.State

func NewDeprecatedStateAdapter(s *core.State) *DeprecatedStateAdapter {
	return (*DeprecatedStateAdapter)(s)
}

func (s *DeprecatedStateAdapter) ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	return (*core.State)(s).ContractStorageAt(addr, key, blockNumber)
}

func (s *DeprecatedStateAdapter) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	return (*core.State)(s).ContractNonceAt(addr, blockNumber)
}

func (s *DeprecatedStateAdapter) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error) {
	return (*core.State)(s).ContractClassHashAt(addr, blockNumber)
}

func (s *DeprecatedStateAdapter) ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	return (*core.State)(s).ContractIsAlreadyDeployedAt(addr, blockNumber)
}

func (s *DeprecatedStateAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return (*core.State)(s).ContractClassHash(addr)
}

func (s *DeprecatedStateAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return (*core.State)(s).ContractNonce(addr)
}

func (s *DeprecatedStateAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return (*core.State)(s).ContractStorage(addr, key)
}

func (s *DeprecatedStateAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return (*core.State)(s).Class(classHash)
}

func (s *DeprecatedStateAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := (*core.State)(s).ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (s *DeprecatedStateAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := (*core.State)(s).ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (s *DeprecatedStateAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := (*core.State)(s).ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (s *DeprecatedStateAdapter) Commitment() (felt.Felt, error) {
	return (*core.State)(s).Root()
}

func (s *DeprecatedStateAdapter) Revert(blockNumber uint64, update *core.StateUpdate) error {
	return (*core.State)(s).Revert(blockNumber, update)
}

func (s *DeprecatedStateAdapter) Update(
	blockNumber uint64,
	update *core.StateUpdate,
	declaredClasses map[felt.Felt]core.Class,
	skipVerifyNewRoot bool,
	flushChanges bool,
) error {
	return (*core.State)(s).Update(blockNumber, update, declaredClasses, skipVerifyNewRoot, flushChanges)
}

type DeprecatedStateReaderAdapter struct {
	core.StateReader
}

func NewDeprecatedStateReaderAdapter(s core.StateReader) *DeprecatedStateReaderAdapter {
	return &DeprecatedStateReaderAdapter{StateReader: s}
}

func (s *DeprecatedStateReaderAdapter) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	return s.StateReader.Class(classHash)
}

func (s *DeprecatedStateReaderAdapter) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractClassHash(addr)
}

func (s *DeprecatedStateReaderAdapter) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractNonce(addr)
}

func (s *DeprecatedStateReaderAdapter) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return s.StateReader.ContractStorage(addr, key)
}

func (s *DeprecatedStateReaderAdapter) ClassTrie() (commontrie.Trie, error) {
	t, err := s.StateReader.ClassTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (s *DeprecatedStateReaderAdapter) ContractTrie() (commontrie.Trie, error) {
	t, err := s.StateReader.ContractTrie()
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}

func (s *DeprecatedStateReaderAdapter) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	t, err := s.StateReader.ContractStorageTrie(addr)
	if err != nil {
		return nil, err
	}
	return commontrie.NewDeprecatedTrieAdapter(t), nil
}
