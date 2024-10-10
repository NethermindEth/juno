package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

type stateSnapshot struct {
	blockNumber uint64
	state       StateHistoryReader
}

func NewStateSnapshot(state StateHistoryReader, blockNumber uint64) StateReader {
	return &stateSnapshot{
		blockNumber: blockNumber,
		state:       state,
	}
}

func (s *stateSnapshot) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
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

func (s *stateSnapshot) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
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

func (s *stateSnapshot) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
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

func (s *stateSnapshot) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractIsAlreadyDeployedAt(addr, s.blockNumber)
	if err != nil {
		return err
	}

	if !isDeployed {
		return db.ErrKeyNotFound
	}
	return nil
}

func (s *stateSnapshot) Class(classHash *felt.Felt) (*DeclaredClass, error) {
	declaredClass, err := s.state.Class(classHash)
	if err != nil {
		return nil, err
	}

	if s.blockNumber < declaredClass.At {
		return nil, db.ErrKeyNotFound
	}
	return declaredClass, nil
}

// Note[pnowosie]: Maybe extending StateReader with the following methods was not a good idea?
func (s *stateSnapshot) ClassTrie() (*trie.Trie, func() error, error) {
	return nil, nopCloser, errFeatureNotImplemented
}

func (s *stateSnapshot) StorageTrie() (*trie.Trie, func() error, error) {
	return nil, nopCloser, errFeatureNotImplemented
}

func (s *stateSnapshot) StorageTrieForAddr(*felt.Felt) (*trie.Trie, error) {
	return nil, errFeatureNotImplemented
}

func (s *stateSnapshot) StateAndClassRoot() (*felt.Felt, *felt.Felt, error) {
	return nil, nil, errFeatureNotImplemented
}

var (
	errFeatureNotImplemented = errors.New("feature not implemented for a historical state")
	nopCloser                = func() error { return nil }
)
