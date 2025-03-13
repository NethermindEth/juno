package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

// var ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")

type stateSnapshot2 struct {
	blockNumber uint64
	state       StateHistoryReader2
}

func NewStateSnapshot2(state StateHistoryReader2, blockNumber uint64) StateReader2 {
	return &stateSnapshot2{
		blockNumber: blockNumber,
		state:       state,
	}
}

func (s *stateSnapshot2) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractClassHashAt(addr, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractClassHash(addr)
		}
		return nil, err
	}
	return val, nil
}

func (s *stateSnapshot2) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractNonceAt(addr, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractNonce(addr)
		}
		return nil, err
	}
	return val, nil
}

func (s *stateSnapshot2) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if err := s.checkDeployed(addr); err != nil {
		return nil, err
	}

	val, err := s.state.ContractStorageAt(addr, key, s.blockNumber)
	if err != nil {
		if errors.Is(err, ErrCheckHeadState) {
			return s.state.ContractStorage(addr, key)
		}
		return nil, err
	}
	return val, nil
}

func (s *stateSnapshot2) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractIsAlreadyDeployedAt(addr, s.blockNumber)
	if err != nil {
		return err
	}

	if !isDeployed {
		return db.ErrKeyNotFound
	}
	return nil
}

func (s *stateSnapshot2) Class(classHash *felt.Felt) (*DeclaredClass, error) {
	declaredClass, err := s.state.Class(classHash)
	if err != nil {
		return nil, err
	}

	if s.blockNumber < declaredClass.At {
		return nil, db.ErrKeyNotFound
	}
	return declaredClass, nil
}

func (s *stateSnapshot2) ClassTrie() (*trie.Trie2, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *stateSnapshot2) ContractTrie() (*trie.Trie2, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *stateSnapshot2) ContractStorageTrie(addr *felt.Felt) (*trie.Trie2, error) {
	return nil, ErrHistoricalTrieNotSupported
}
