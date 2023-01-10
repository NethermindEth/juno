package state

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
)

const (
	stateTrieHeight           = 251
	contractStorageTrieHeight = 251
	// fields of state metadata table
	stateRootKey = "rootPath"
)

type ErrMismatchedRoot struct {
	Want  *felt.Felt
	Got   *felt.Felt
	IsOld bool
}

func (e *ErrMismatchedRoot) Error() string {
	newOld := " new "
	if e.IsOld {
		newOld = " old "
	}
	return fmt.Sprintf("mismatched %s root: want %s, got %s", newOld, e.Want.Text(16), e.Got.Text(16))
}

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) *State {
	state := &State{
		db: db,
	}
	return state
}

func CalculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	commitment := crypto.Pedersen(classHash, storageRoot)
	commitment = crypto.Pedersen(commitment, nonce)
	return crypto.Pedersen(commitment, &felt.Zero)
}

// putNewContract creates a contract storage instance in the state and
// stores the relation between contract address and class hash to be
// queried later on with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt, txn *badger.Txn) error {
	addrBytes := addr.Marshal()
	classHashKey := db.ContractClassHash.Key(addrBytes)
	nonceKey := db.ContractNonce.Key(addrBytes)
	if _, err := txn.Get(classHashKey); err == nil {
		// Should not happen.
		return errors.New("existing contract")
	} else if err = txn.Set(classHashKey, classHash.Marshal()); err != nil {
		return err
	} else if err = txn.Set(nonceKey, felt.Zero.Marshal()); err != nil {
		return err
	} else {
		commitment := CalculateContractCommitment(&felt.Zero, classHash, &felt.Zero)
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

// GetContractNonce returns nonce of a contract at a given address.
func (s *State) GetContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	var nonce *felt.Felt

	return nonce, s.db.View(func(txn *badger.Txn) error {
		var err error
		nonce, err = s.getContractNonce(addr, txn)
		return err
	})
}

// getContractNonce returns nonce of a contract at a given address
// in the given Txn context.
func (s *State) getContractNonce(addr *felt.Felt, txn *badger.Txn) (*felt.Felt, error) {
	var nonce *felt.Felt

	key := db.ContractNonce.Key(addr.Marshal())
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return nonce, item.Value(func(val []byte) error {
		nonce = new(felt.Felt).SetBytes(val)
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

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(update *core.StateUpdate) error {
	return s.db.Update(func(txn *badger.Txn) error {
		currentRoot, err := s.root(txn)
		if err != nil {
			return err
		}
		if !update.OldRoot.Equal(currentRoot) {
			return &ErrMismatchedRoot{
				Want:  update.OldRoot,
				Got:   currentRoot,
				IsOld: true,
			}
		}

		// register deployed contracts
		for _, contract := range update.StateDiff.DeployedContracts {
			if err := s.putNewContract(contract.Address, contract.ClassHash, txn); err != nil {
				return err
			}
		}

		// update contract nonces
		for addr, nonce := range update.StateDiff.Nonces {
			if err != nil {
				return err
			}
			if err = s.updateContractNonce(&addr, nonce, txn); err != nil {
				return err
			}
		}

		// update contract storages
		for addr, diff := range update.StateDiff.StorageDiffs {
			if err != nil {
				return err
			}
			if err = s.updateContractStorage(&addr, diff, txn); err != nil {
				return err
			}
		}

		newRoot, err := s.root(txn)
		if err != nil {
			return err
		}
		if !update.NewRoot.Equal(newRoot) {
			return &ErrMismatchedRoot{
				Want:  update.NewRoot,
				Got:   newRoot,
				IsOld: false,
			}
		}
		return nil
	})
}

// getContractStorage returns the [core.Trie] that represents the
// storage of the contract at the given address in the given Txn
// context.
func (s *State) getContractStorage(addr *felt.Felt, txn *badger.Txn) (*trie.Trie, error) {
	addrBytes := addr.Marshal()
	var contractRootKey *bitset.BitSet

	if item, err := txn.Get(db.ContractRootPath.Key(addrBytes)); err == nil {
		if err = item.Value(func(val []byte) error {
			contractRootKey = new(bitset.BitSet)
			return contractRootKey.UnmarshalBinary(val)
		}); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, badger.ErrKeyNotFound) {
		// Don't continue normal operation with arbitrary
		// database error.
		return nil, err
	}
	trieTxn := trie.NewTrieBadgerTxn(txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTrie(trieTxn, contractStorageTrieHeight, contractRootKey), nil
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(addr *felt.Felt, diff []core.StorageDiff, txn *badger.Txn) error {
	classHash, err := s.getContractClass(addr, txn)
	if err != nil {
		return err
	}

	storage, err := s.getContractStorage(addr, txn)
	if err != nil {
		return err
	}

	nonce, err := s.getContractNonce(addr, txn)
	if err != nil {
		return err
	}

	// apply the diff
	for _, pair := range diff {
		if err = storage.Put(pair.Key, pair.Value); err != nil {
			return err
		}
	}

	// update contract storage root in the database
	rootKeyDbKey := db.ContractRootPath.Key(addr.Marshal())
	if rootKey := storage.RootKey(); rootKey != nil {
		if rootKeyBytes, err := storage.RootKey().MarshalBinary(); err != nil {
			return err
		} else if err = txn.Set(rootKeyDbKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err = txn.Delete(rootKeyDbKey); err != nil {
		return err
	}

	// recalculate commitment to be put in the global state
	storageRoot, err := storage.Root()
	if err != nil {
		return err
	}

	commitment := CalculateContractCommitment(storageRoot, classHash, nonce)
	state, err := s.getStateStorage(txn)
	if err != nil {
		return err
	}

	if err = state.Put(addr, commitment); err != nil {
		return err
	}

	return s.putStateStorage(state, txn)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(addr, nonce *felt.Felt, txn *badger.Txn) error {
	classHash, err := s.getContractClass(addr, txn)
	if err != nil {
		return err
	}

	storage, err := s.getContractStorage(addr, txn)
	if err != nil {
		return err
	}

	// update nonce in the database
	if err = txn.Set(db.ContractNonce.Key(addr.Marshal()), nonce.Marshal()); err != nil {
		return err
	}

	// calculate commitment to be put in the global state
	storageRoot, err := storage.Root()
	if err != nil {
		return err
	}

	commitment := CalculateContractCommitment(storageRoot, classHash, nonce)
	state, err := s.getStateStorage(txn)
	if err != nil {
		return err
	}

	if err = state.Put(addr, commitment); err != nil {
		return err
	}

	return s.putStateStorage(state, txn)
}
