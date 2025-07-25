package state

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db"
)

var _ StateReader = (*StateHistory)(nil)

// StateHistory represents a snapshot of the blockchain state at a specific block number.
type StateHistory struct {
	blockNum uint64
	state    *State
}

func NewStateHistory(blockNum uint64, stateRoot *felt.Felt, db *StateDB) (StateHistory, error) {
	state, err := New(stateRoot, db)
	if err != nil {
		return StateHistory{}, err
	}

	return StateHistory{
		blockNum: blockNum,
		state:    state,
	}, nil
}

func (s *StateHistory) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}
	ret, err := s.state.ContractClassHashAt(addr, s.blockNum)
	if err != nil {
		return felt.Felt{}, err
	}
	return ret, nil
}

func (s *StateHistory) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}
	ret, err := s.state.ContractNonceAt(addr, s.blockNum)
	if err != nil {
		return felt.Felt{}, err
	}
	return ret, nil
}

func (s *StateHistory) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}
	ret, err := s.state.ContractStorageAt(addr, key, s.blockNum)
	if err != nil {
		return felt.Felt{}, err
	}
	return ret, nil
}

// Checks if the contract is deployed at the given block number.
func (s *StateHistory) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractDeployedAt(addr, s.blockNum)
	if err != nil {
		return err
	}

	if !isDeployed {
		// TODO(weiihann): previously this was db.ErrKeyNotFound
		// remember to handle it in the rpc
		return ErrContractNotDeployed
	}

	return nil
}

func (s *StateHistory) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	declaredClass, err := s.state.Class(classHash)
	if err != nil {
		return nil, err
	}

	if s.blockNum < declaredClass.At {
		return nil, db.ErrKeyNotFound
	}

	return declaredClass, nil
}

func (s *StateHistory) ClassTrie() (*trie2.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *StateHistory) ContractTrie() (*trie2.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *StateHistory) ContractStorageTrie(addr *felt.Felt) (*trie2.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}
