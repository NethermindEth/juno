package state

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db"
)

var ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")

type StateHistory struct {
	blockNum uint64
	state    *State
}

func NewStateHistory(txn db.Transaction, blockNum uint64) (*StateHistory, error) {
	state, err := New(txn)
	if err != nil {
		return nil, err
	}

	return &StateHistory{
		blockNum: blockNum,
		state:    state,
	}, nil
}

func (s *StateHistory) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractClassHashAt(addr, s.blockNum)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (s *StateHistory) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractNonceAt(addr, s.blockNum)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (s *StateHistory) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractStorageAt(addr, key, s.blockNum)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (s *StateHistory) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractDeployedAt(*addr, s.blockNum)
	if err != nil {
		return err
	}

	if !isDeployed {
		return db.ErrKeyNotFound
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
