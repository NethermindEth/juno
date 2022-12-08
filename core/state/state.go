package state

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/bits-and-blooms/bitset"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
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

func GetContractCommitment(storageRoot, classHash *felt.Felt) (*felt.Felt, error) {
	commitment, err := crypto.Pedersen(classHash, storageRoot)
	if err != nil {
		return nil, err
	}
	commitment, err = crypto.Pedersen(commitment, new(felt.Felt))
	if err != nil {
		return nil, err
	}
	return crypto.Pedersen(commitment, new(felt.Felt))
}

// putNewContractWithTxn creates a contract storage instance in the state and stores the relation
// between contract address and class hash to be queried later on with [GetContractClass]
func (s *State) putNewContract(addr, classHash *felt.Felt, txn *badger.Txn) error {
	key := db.Key(db.ContractClassHash, addr.Marshal())
	if _, err := txn.Get(key); err == nil {
		return errors.New("existing contract")
	} else if err = txn.Set(key, classHash.Marshal()); err != nil {
		return err
	} else if commitment, err := GetContractCommitment(new(felt.Felt), classHash); err != nil {
		return err
	} else if state, err := s.getStateStorage(txn); err != nil {
		return err
	} else if err = state.Put(addr, commitment); err != nil {
		return err
	}
	return nil
}

// GetContractClass returns class hash of a contract at a given address
func (s *State) GetContractClass(addr *felt.Felt) (*felt.Felt, error) {
	var classHash *felt.Felt

	return classHash, s.db.View(func(txn *badger.Txn) error {
		var err error
		classHash, err = s.getContractClass(addr, txn)
		return err
	})
}

// getContractClass returns class hash of a contract at a given address in the given Txn context
func (s *State) getContractClass(addr *felt.Felt, txn *badger.Txn) (*felt.Felt, error) {
	var classHash *felt.Felt

	key := db.Key(db.ContractClassHash, addr.Marshal())
	if item, err := txn.Get(key); err != nil {
		return classHash, err
	} else {
		return classHash, item.Value(func(val []byte) error {
			classHash = new(felt.Felt).SetBytes(val)
			return nil
		})
	}
}

// Root returns the state commitment
func (s *State) Root() (*felt.Felt, error) {
	var root *felt.Felt
	return root, s.db.View(func(txn *badger.Txn) error {
		read, err := s.root(txn)

		root = read
		return err
	})
}

// root returns the state commitment in the given Txn context
func (s *State) root(txn *badger.Txn) (*felt.Felt, error) {
	storage, err := s.getStateStorage(txn)
	if err != nil {
		return nil, err
	}
	return storage.Root()
}

// getStateStorage returns a [core.Trie] that represents the StarkNet global state in the given Txn context
func (s *State) getStateStorage(txn *badger.Txn) (*core.Trie, error) {
	tTxn := &TrieTxn{
		badgerTxn: txn,
		prefix:    []byte{db.StateTrie},
	}

	rootKey, err := s.rootKey(txn)
	if err != nil {
		rootKey = nil
	}

	return core.NewTrie(tTxn, stateTrieHeight, rootKey), nil
}

// rootKey returns key to the root node in the given Txn context
func (s *State) rootKey(txn *badger.Txn) (*bitset.BitSet, error) {
	key := new(bitset.BitSet)

	if item, err := txn.Get(db.Key(db.State, []byte(stateRootKey))); err != nil {
		return nil, err
	} else {
		return key, item.Value(func(val []byte) error {
			return key.UnmarshalBinary(val)
		})
	}
}

// putStateStorage updates the fields related to the state trie root in the given Txn context
func (s *State) putStateStorage(state *core.Trie, txn *badger.Txn) error {
	rootKey, err := state.RootKey().MarshalBinary()
	if err != nil {
		return err
	}

	if err = txn.Set(db.Key(db.State, []byte(stateRootKey)), rootKey); err != nil {
		return err
	}

	return nil
}
