package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
	"github.com/NethermindEth/juno/db"
)

var ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")

type deprecatedStateHistory struct {
	blockNumber uint64
	state       StateHistoryReader
}

func NewDeprecatedStateHistory(
	state StateHistoryReader,
	blockNumber uint64,
) *deprecatedStateHistory {
	return &deprecatedStateHistory{
		blockNumber: blockNumber,
		state:       state,
	}
}

func (s *deprecatedStateHistory) ChainHeight() (uint64, error) {
	return s.blockNumber, nil
}

func (s *deprecatedStateHistory) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
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

func (s *deprecatedStateHistory) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
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

func (s *deprecatedStateHistory) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
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

func (s *deprecatedStateHistory) checkDeployed(addr *felt.Felt) error {
	isDeployed, err := s.state.ContractDeployedAt(addr, s.blockNumber)
	if err != nil {
		return err
	}

	if !isDeployed {
		return db.ErrKeyNotFound
	}
	return nil
}

func (s *deprecatedStateHistory) Class(classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	declaredClass, err := s.state.Class(classHash)
	if err != nil {
		return nil, err
	}

	if s.blockNumber < declaredClass.At {
		return nil, db.ErrKeyNotFound
	}
	return declaredClass, nil
}

func (s *deprecatedStateHistory) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	return s.state.CompiledClassHash(classHash)
}

func (s *deprecatedStateHistory) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	return s.state.CompiledClassHashV2(classHash)
}

func (s *deprecatedStateHistory) ClassTrie() (commontrie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *deprecatedStateHistory) ContractTrie() (commontrie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (s *deprecatedStateHistory) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	return nil, ErrHistoricalTrieNotSupported
}
