package state

import (
	"errors"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/db"
	"golang.org/x/exp/maps"
)

// contract storage has fixed height at 251
const (
	ContractStorageTrieHeight = 251
)

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

type Storage map[felt.Felt]*felt.Felt

type stateObject struct {
	state *State // the state that this object belongs to

	addr     felt.Felt // address of starknet contract
	contract *stateContract

	dirtyStorage Storage     // Storage locations that have been updated
	tr           *trie2.Trie // The underlying storage trie
}

func newStateObject(state *State, addr felt.Felt, ct *stateContract) *stateObject {
	contract := &stateObject{
		state:        state,
		addr:         addr,
		contract:     ct,
		dirtyStorage: make(Storage),
	}

	return contract
}

func (s *stateObject) GetStorageRoot() (felt.Felt, error) {
	if s.StorageRoot != nil {
		return s.StorageRoot(), nil
	}

	tr, err := s.getTrie()
	if err != nil {
		return felt.Zero, err
	}

	root := tr.Hash()
	s.contract.StorageRoot = root

	return root, nil
}

func (s *stateObject) UpdateStorage(key, value felt.Felt) {
	if s.dirtyStorage == nil {
		s.dirtyStorage = make(Storage)
	}

	s.dirtyStorage[key] = value
}

func (s *stateObject) getStorage(key felt.Felt) (felt.Felt, error) {
	if s.dirtyStorage != nil {
		if val, ok := s.dirtyStorage[key]; ok {
			return val, nil
		}
	}

	var err error

	tr := s.tr
	if tr == nil {
		tr, err = s.getTrie()
		if err != nil {
			return felt.Zero, err
		}
	}

	// TODO(weiihann): don't need to resolve the whole path, just get directly
	val, err := tr.Get(&key)
	if err != nil {
		return felt.Zero, err
	}

	return val, nil
}

func (s *stateObject) commit(storeHistory bool, blockNum uint64) (*trienode.NodeSet, error) {
	var err error

	tr := s.tr
	if tr == nil {
		tr, err = s.getTrie()
		if err != nil {
			return err
		}
	}

	keys := maps.Keys(s.dirtyStorage)
	slices.SortFunc(keys, func(a, b felt.Felt) int {
		return b.Cmp(&a)
	})

	// Commit storage changes to the associated storage trie
	for _, key := range keys {
		val := s.dirtyStorage[key]
		if err := tr.Update(&key, &val); err != nil {
			return err
		}

		if storeHistory {
			if err := s.storeStorageHistory(w, blockNum, &key, &val); err != nil {
				return nil, err
			}
		}
	}

	root, nodes := tr.Commit()
	s.contract.StorageRoot = root

	if storeHistory {
		if err := s.storeNonceHistory(w, blockNum); err != nil {
			return nil, err
		}

		if err := s.storeClassHashHistory(w, blockNum); err != nil {
			return nil, err
		}
	}

	return nodes, nil
}

func (s *stateObject) Nonce() felt.Felt       { return s.contract.Nonce }
func (s *stateObject) ClassHash() felt.Felt   { return s.contract.ClassHash }
func (s *stateObject) StorageRoot() felt.Felt { return s.contract.StorageRoot }
func (s *stateObject) DeployHeight() uint64   { return s.contract.DeployHeight }

func (s *stateObject) storeNonceHistory(w db.KeyValueWriter, blockNum uint64) error {
	return WriteNonceHistory(w, &s.addr, blockNum, &s.contract.Nonce)
}

func (s *stateObject) storeClassHashHistory(w db.KeyValueWriter, blockNum uint64) error {
	return WriteClassHashHistory(w, &s.addr, blockNum, &s.contract.ClassHash)
}

func (s *stateObject) storeStorageHistory(w db.KeyValueWriter, blockNum uint64, key, value *felt.Felt) error {
	return WriteStorageHistory(w, &s.addr, key, blockNum, value)
}

func (s *stateObject) delete(w db.KeyValueWriter) error {
	return DeleteContract(w, &s.addr)
}

func (s *stateObject) deleteStorageTrie(txn db.Transaction) error {
	tr, err := s.getTrie(txn)
	if err != nil {
		return err
	}

	// TODO: Instead of using node iterator and delete each node one by one,
	// use the underlying DeleteRange from PebbleDB.
	it, err := tr.NodeIterator()
	if err != nil {
		return err
	}
	defer it.Close()

	for it.First(); it.Valid(); it.Next() {
		key := it.Key()
		if err := txn.Delete(key); err != nil {
			return err
		}
	}

	return nil
}

func (s *stateObject) getTrie() (*trie2.Trie, error) {
	if s.tr != nil {
		return s.tr, nil
	}

	tr, err := s.state.db.ContractStorageTrie(s.state.initRoot, s.addr)
	if err != nil {
		return nil, err
	}
	s.tr = tr

	return tr, nil
}
