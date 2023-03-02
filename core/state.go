package core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bitset"
)

var (
	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
)

const (
	globalTrieHeight = 251
	// Fields of state metadata table
	stateRootKey   = "rootKey"
	classesRootKey = "classesRootKey"
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
	return fmt.Sprintf("mismatched %s root: want %s, got %s", newOld, e.Want.String(), e.Got.String())
}

type State struct {
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{txn: txn}
}

func CalculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	commitment := crypto.Pedersen(classHash, storageRoot)
	commitment = crypto.Pedersen(commitment, nonce)
	return crypto.Pedersen(commitment, &felt.Zero)
}

// putNewContract creates a contract storage instance in the state and
// stores the relation between contract address and class hash to be
// queried later on with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt) error {
	contract := NewContract(addr, s.txn)
	if err := contract.Deploy(classHash); err != nil {
		return err
	} else {
		return s.updateContractCommitment(contract)
	}
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(addr, classHash *felt.Felt) error {
	contract := NewContract(addr, s.txn)
	if err := contract.Replace(classHash); err != nil {
		return err
	} else {
		return s.updateContractCommitment(contract)
	}
}

// ContractClass returns class hash of a contract at a given address.
func (s *State) ContractClass(addr *felt.Felt) (*felt.Felt, error) {
	return NewContract(addr, s.txn).ClassHash()
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	return NewContract(addr, s.txn).Nonce()
}

// Root returns the state commitment.
func (s *State) Root() (*felt.Felt, error) {
	if storage, err := s.stateStorage(); err != nil {
		return nil, err
	} else if classes, err := s.classTrie(); err != nil {
		return nil, err
	} else if classesRoot, err := classes.Root(); err != nil {
		return nil, err
	} else if storageRoot, err := storage.Root(); err != nil {
		return nil, err
	} else if classesRoot.IsZero() {
		return storageRoot, nil
	} else {
		return crypto.PoseidonArray(stateVersion, storageRoot, classesRoot), nil
	}
}

// stateStorage returns a [core.Trie] that represents the Starknet
// global state in the given Txn context
func (s *State) stateStorage() (*trie.Trie, error) {
	return s.globalTrie(db.StateTrie, stateRootKey, trie.NewTrie)
}

// getClassTrie returns a [core.Trie] that represents the Starknet
// classes trie in the given Txn context
func (s *State) classTrie() (*trie.Trie, error) {
	return s.globalTrie(db.ClassesTrie, classesRootKey, trie.NewTriePoseidon)
}

func (s *State) globalTrie(bucket db.Bucket, rootDbKey string, NewTrie trie.NewTrieFunc) (*trie.Trie, error) {
	tTxn := NewTransactionStorage(s.txn, []byte{byte(bucket)})

	var rootKey *bitset.BitSet
	if err := s.txn.Get(db.State.Key([]byte(rootDbKey)), func(val []byte) error {
		rootKey = new(bitset.BitSet)
		return rootKey.UnmarshalBinary(val)
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	return NewTrie(tTxn, globalTrieHeight, rootKey), nil
}

// putStateStorage updates the fields related to the state trie root in
// the given Txn context.
func (s *State) putStateStorage(state *trie.Trie) error {
	return s.putGlobalTrie(state, stateRootKey)
}

// putClassesTrie updates the fields related to the classes trie root in
// the given Txn context.
func (s *State) putClassesTrie(classes *trie.Trie) error {
	return s.putGlobalTrie(classes, classesRootKey)
}

func (s *State) putGlobalTrie(trie *trie.Trie, rootDbKey string) error {
	rootKeyDbKey := db.State.Key([]byte(rootDbKey))
	if rootKey := trie.RootKey(); rootKey != nil {
		if rootKeyBytes, err := rootKey.MarshalBinary(); err != nil {
			return err
		} else if err = s.txn.Set(rootKeyDbKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err := s.txn.Delete(rootKeyDbKey); err != nil {
		return err
	}

	return nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
	currentRoot, err := s.Root()
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

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	for classHash, class := range declaredClasses {
		classKey := db.Class.Key(classHash.Marshal())

		if err := s.txn.Get(classKey, func(val []byte) error {
			return nil
		}); errors.Is(err, db.ErrKeyNotFound) {
			classEncoded, err := encoder.Marshal(class)
			if err != nil {
				return err
			}

			if err := s.txn.Set(classKey, classEncoded); err != nil {
				return err
			}
		}
	}

	if err = s.updateDeclaredClasses(update.StateDiff.DeclaredV1Classes); err != nil {
		return err
	}

	// register deployed contracts
	for _, contract := range update.StateDiff.DeployedContracts {
		if err = s.putNewContract(contract.Address, contract.ClassHash); err != nil {
			return err
		}
	}

	// replace contract instances
	for _, replace := range update.StateDiff.ReplacedClasses {
		if err = s.replaceContract(replace.Address, replace.ClassHash); err != nil {
			return err
		}
	}

	// update contract nonces
	for addr, nonce := range update.StateDiff.Nonces {
		if err = s.updateContractNonce(&addr, nonce); err != nil {
			return err
		}
	}

	// update contract storages
	for addr, diff := range update.StateDiff.StorageDiffs {
		if err != nil {
			return err
		}
		if err = s.updateContractStorage(&addr, diff); err != nil {
			return err
		}
	}

	newRoot, err := s.Root()
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
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(addr *felt.Felt, diff []StorageDiff) error {
	contract := NewContract(addr, s.txn)
	if err := contract.UpdateStorage(diff); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(addr, nonce *felt.Felt) error {
	contract := NewContract(addr, s.txn)
	if err := contract.UpdateNonce(nonce); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(contract *Contract) error {
	if storageRoot, err := contract.StorageRoot(); err != nil {
		return err
	} else if classHash, err := contract.ClassHash(); err != nil {
		return err
	} else if nonce, err := contract.Nonce(); err != nil {
		return err
	} else {
		commitment := CalculateContractCommitment(storageRoot, classHash, nonce)
		// todo: eliminate repeated calls to stateStorage and putStateStorage, move to Update()
		state, err := s.stateStorage()
		if err != nil {
			return err
		}

		if _, err = state.Put(contract.Address, commitment); err != nil {
			return err
		}

		return s.putStateStorage(state)
	}
}

func (s *State) updateDeclaredClasses(declaredClasses []DeclaredV1Class) error {
	classesTrie, err := s.classTrie()
	if err != nil {
		return err
	}

	for _, declaredClass := range declaredClasses {
		// https://docs.starknet.io/documentation/starknet_versions/upcoming_versions/#commitment
		leafValue := crypto.Poseidon(leafVersion, declaredClass.CompiledClassHash)
		if _, err = classesTrie.Put(declaredClass.ClassHash, leafValue); err != nil {
			return err
		}
	}

	return s.putClassesTrie(classesTrie)
}
