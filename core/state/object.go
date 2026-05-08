package state

import (
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"golang.org/x/exp/maps"
)

type Storage map[felt.Felt]*felt.Felt

type stateObject struct {
	state *State // the state this object belongs to

	addr     felt.Felt      // address of the contract
	contract *stateContract // contract info

	dirtyStorage Storage     // storage changes
	storageTrie  *trie2.Trie // storage trie
}

func newStateObject(state *State, addr *felt.Felt, contract *stateContract) stateObject {
	return stateObject{
		state:        state,
		addr:         *addr,
		contract:     contract,
		dirtyStorage: make(Storage),
	}
}

func (s *stateObject) setClassHash(classHash *felt.Felt) {
	s.contract.ClassHash = *classHash
}

func (s *stateObject) setNonce(nonce *felt.Felt) {
	s.contract.Nonce = *nonce
}

func (s *stateObject) getStorageTrie() (*trie2.Trie, error) {
	if s.storageTrie != nil {
		return s.storageTrie, nil
	}

	storageTrie, err := s.state.db.ContractStorageTrie(
		&s.state.initRoot,
		&s.addr,
	)
	if err != nil {
		return nil, err
	}
	s.storageTrie = storageTrie

	return storageTrie, nil
}

func (s *stateObject) getStorageRoot() (felt.Felt, error) {
	// If the storage trie is loaded, it may be modified somewhere already.
	// Return the hash of the trie and update the contract's storage root.
	if s.storageTrie != nil {
		root, err := s.storageTrie.Hash()
		if err != nil {
			return felt.Zero, err
		}
		s.contract.StorageRoot = root
		return root, nil
	}

	// Otherwise, return the storage root from the contract.
	return s.contract.StorageRoot, nil
}

func (s *stateObject) commit() (*trienode.NodeSet, error) {
	tr, err := s.getStorageTrie()
	if err != nil {
		return nil, err
	}

	keys := maps.Keys(s.dirtyStorage)
	slices.SortFunc(keys, func(a, b felt.Felt) int {
		return b.Cmp(&a)
	})

	for _, key := range keys {
		val := s.dirtyStorage[key]
		if err := tr.Update(&key, val); err != nil {
			return nil, err
		}
	}

	root, nodes := tr.Commit()
	s.contract.StorageRoot = root
	return nodes, nil
}

func (s *stateObject) commitment() felt.Felt {
	return s.contract.commitment()
}
