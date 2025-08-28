package commonstate

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
)

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
