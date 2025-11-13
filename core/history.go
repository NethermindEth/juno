package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

var ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")

type StateHistory struct {
	blockNumber uint64
	state       StateHistoryReader
}

func NewStateHistory(state StateHistoryReader, blockNumber uint64) StateHistory {
	return StateHistory{
		blockNumber: blockNumber,
		state:       state,
	}
}

func (s *StateHistory) ChainHeight() (uint64, error) {
	return s.blockNumber, nil
}

func (s *StateHistory) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}

	val, err := s.state.ContractClassHashAt(addr, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractClassHash(addr)
		}
		return felt.Felt{}, err
	}
	return val, nil
}

func (s *StateHistory) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}

	val, err := s.state.ContractNonceAt(addr, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractNonce(addr)
		}
		return felt.Felt{}, err
	}
	return val, nil
}

func (s *StateHistory) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return felt.Felt{}, err
	}

	val, err := s.state.ContractStorageAt(addr, key, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractStorage(addr, key)
		}
		return felt.Felt{}, err
	}
	return val, nil
}

func (s *StateHistory) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractDeployedAt(addr, s.blockNumber)
	if err != nil {
		return err
	}

	if !isDeployed {
		return db.ErrKeyNotFound
	}
	return nil
}

func (s *StateHistory) Class(classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	declaredClass, err := s.state.Class(classHash)
	if err != nil {
		return nil, err
	}

	if s.blockNumber < declaredClass.At {
		return nil, db.ErrKeyNotFound
	}
	return declaredClass, nil
}

func (s *StateHistory) ClassTrie() (*trie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *StateHistory) ContractTrie() (*trie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *StateHistory) ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}
