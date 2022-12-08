package state

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
)

const (
	stateTrieHeight = 251

	// fields of state metadata table
	stateRootKey = "rootPath"
)

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) *State {
	state := &State{
		db: db,
	}
	return state
}

func CalculateContractCommitment(storageRoot, classHash *felt.Felt) *felt.Felt {
	commitment := crypto.Pedersen(classHash, storageRoot)
	commitment = crypto.Pedersen(commitment, new(felt.Felt))
	return crypto.Pedersen(commitment, new(felt.Felt))
}

// putNewContract creates a contract storage instance in the state and
// stores the relation between contract address and class hash to be
// queried later on with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt, txn *badger.Txn) error {
	key := db.ContractClassHash.Key(addr.Marshal())
	if _, err := txn.Get(key); err == nil {
		// Should not happen.
		return errors.New("existing contract")
	} else if err = txn.Set(key, classHash.Marshal()); err != nil {
		return err
	} else {
		commitment := CalculateContractCommitment(new(felt.Felt), classHash)
		if state, err := s.getStateStorage(txn); err != nil {
			return err
		} else if err = state.Put(addr, commitment); err != nil {
			return err
		} else {
			return s.putStateStorage(state, txn)
		}
	}
}

// GetContractClass returns class hash of a contract at a given address.
func (s *State) GetContractClass(addr *felt.Felt) (*felt.Felt, error) {
	var classHash *felt.Felt

	return classHash, s.db.View(func(txn *badger.Txn) error {
		var err error
		classHash, err = s.getContractClass(addr, txn)
		return err
	})
}

// getContractClass returns class hash of a contract at a given address
// in the given Txn context.
func (s *State) getContractClass(addr *felt.Felt, txn *badger.Txn) (*felt.Felt, error) {
	var classHash *felt.Felt

	key := db.ContractClassHash.Key(addr.Marshal())
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return classHash, item.Value(func(val []byte) error {
		classHash = new(felt.Felt).SetBytes(val)
		return nil
	})
}

// Root returns the state commitment.
func (s *State) Root() (*felt.Felt, error) {
	var root *felt.Felt
	return root, s.db.View(func(txn *badger.Txn) error {
		read, err := s.root(txn)

		root = read
		return err
	})
}

// root returns the state commitment in the given Txn context.
func (s *State) root(txn *badger.Txn) (*felt.Felt, error) {
	storage, err := s.getStateStorage(txn)
	if err != nil {
		return nil, err
	}
	return storage.Root()
}

// getStateStorage returns a [core.Trie] that represents the StarkNet
// global state in the given Txn context
func (s *State) getStateStorage(txn *badger.Txn) (*trie.Trie, error) {
	tTxn := trie.NewTrieBadgerTxn(txn, []byte{byte(db.StateTrie)})

	rootKey, err := s.rootKey(txn)
	if err != nil {
		rootKey = nil
	}

	return trie.NewTrie(tTxn, stateTrieHeight, rootKey), nil
}

// rootKey returns key to the root node in the given Txn context.
func (s *State) rootKey(txn *badger.Txn) (*bitset.BitSet, error) {
	key := new(bitset.BitSet)

	item, err := txn.Get(db.State.Key([]byte(stateRootKey)))
	if err != nil {
		return nil, err
	}

	return key, item.Value(func(val []byte) error {
		return key.UnmarshalBinary(val)
	})
}

// putStateStorage updates the fields related to the state trie root in
// the given Txn context.
func (s *State) putStateStorage(state *trie.Trie, txn *badger.Txn) error {
	rootKeyDbKey := db.State.Key([]byte(stateRootKey))
	if rootKey := state.RootKey(); rootKey != nil {
		if rootKeyBytes, err := rootKey.MarshalBinary(); err != nil {
			return err
		} else if err = txn.Set(rootKeyDbKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err := txn.Delete(rootKeyDbKey); err != nil {
		return err
	}

	return nil
}
