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

const globalTrieHeight = 251

var (
	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
)

type State struct {
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{txn: txn}
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt) error {
	contract, err := DeployContract(addr, classHash, s.txn)
	if err != nil {
		return err
	}
	return s.updateContractCommitment(contract)
}

// ContractClassHash returns class hash of a contract at a given address.
func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}
	return contract.ClassHash()
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}
	return contract.Nonce()
}

// Root returns the state commitment.
func (s *State) Root() (*felt.Felt, error) {
	var storageRoot, classesRoot *felt.Felt

	sStorage, closer, err := s.storage()
	if err != nil {
		return nil, err
	}

	if storageRoot, err = sStorage.Root(); err != nil {
		return nil, err
	}

	if err = closer(); err != nil {
		return nil, err
	}

	classes, closer, err := s.classesTrie()
	if err != nil {
		return nil, err
	}

	if classesRoot, err = classes.Root(); err != nil {
		return nil, err
	}

	if err = closer(); err != nil {
		return nil, err
	}

	if classesRoot.IsZero() {
		return storageRoot, nil
	}

	return crypto.PoseidonArray(stateVersion, storageRoot, classesRoot), nil
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func (s *State) storage() (*trie.Trie, func() error, error) {
	return s.globalTrie(db.StateTrie, trie.NewTriePedersen)
}

func (s *State) classesTrie() (*trie.Trie, func() error, error) {
	return s.globalTrie(db.ClassesTrie, trie.NewTriePoseidon)
}

func (s *State) globalTrie(bucket db.Bucket, newTrie trie.NewTrieFunc) (*trie.Trie, func() error, error) {
	dbPrefix := bucket.Key()
	tTxn := NewTransactionStorage(s.txn, dbPrefix)

	// fetch root key
	rootKeyDBKey := dbPrefix
	var rootKey *bitset.BitSet
	err := s.txn.Get(rootKeyDBKey, func(val []byte) error {
		rootKey = new(bitset.BitSet)
		return rootKey.UnmarshalBinary(val)
	})

	// if some error other than "not found"
	if err != nil && !errors.Is(db.ErrKeyNotFound, err) {
		return nil, nil, err
	}

	gTrie, err := newTrie(tTxn, globalTrieHeight, rootKey)
	if err != nil {
		return nil, nil, err
	}

	// prep closer
	closer := func() error {
		resultingRootKey := gTrie.RootKey()
		// no updates on the trie, short circuit and return
		if resultingRootKey.Equal(rootKey) {
			return nil
		}

		if resultingRootKey != nil {
			rootKeyBytes, marshalErr := resultingRootKey.MarshalBinary()
			if marshalErr != nil {
				return marshalErr
			}

			return s.txn.Set(rootKeyDBKey, rootKeyBytes)
		}
		return s.txn.Delete(rootKeyDBKey)
	}

	return gTrie, closer, nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
	currentRoot, err := s.Root()
	if err != nil {
		return err
	} else if !update.OldRoot.Equal(currentRoot) {
		return fmt.Errorf("state's current root: %s does not match state update's old root: %s", currentRoot, update.OldRoot)
	}

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	// Todo: add test cases for retrieving Classes when State struct is extended to return Classes.
	for classHash, class := range declaredClasses {
		if err = s.putClass(&classHash, class); err != nil {
			return err
		}
	}

	if err = s.updateDeclaredClasses(update.StateDiff.DeclaredV1Classes); err != nil {
		return err
	}

	if err = s.updateContracts(update.StateDiff); err != nil {
		return err
	}

	newRoot, err := s.Root()
	if err != nil {
		return err
	} else if !update.NewRoot.Equal(newRoot) {
		return fmt.Errorf("state's new root: %s does not match state update's new root: %s", newRoot, update.NewRoot)
	}
	return nil
}

func (s *State) updateContracts(diff *StateDiff) error {
	// register deployed contracts
	for _, contract := range diff.DeployedContracts {
		if err := s.putNewContract(contract.Address, contract.ClassHash); err != nil {
			return err
		}
	}

	// replace contract instances
	for _, replace := range diff.ReplacedClasses {
		if err := s.replaceContract(replace.Address, replace.ClassHash); err != nil {
			return err
		}
	}

	// update contract nonces
	for addr, nonce := range diff.Nonces {
		if err := s.updateContractNonce(&addr, nonce); err != nil {
			return err
		}
	}

	// update contract storages
	for addr, storageDiff := range diff.StorageDiffs {
		if err := s.updateContractStorage(&addr, storageDiff); err != nil {
			return err
		}
	}

	return nil
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(addr, classHash *felt.Felt) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err = contract.Replace(classHash); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

func (s *State) putClass(classHash *felt.Felt, class Class) error {
	classKey := db.Class.Key(classHash.Marshal())

	err := s.txn.Get(classKey, func(val []byte) error {
		return nil
	})

	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(class)
		if encErr != nil {
			return encErr
		}

		return s.txn.Set(classKey, classEncoded)
	}
	return err
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(addr *felt.Felt, diff []StorageDiff) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err := contract.UpdateStorage(diff); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(addr, nonce *felt.Felt) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err := contract.UpdateNonce(nonce); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(contract *Contract) error {
	root, err := contract.Root()
	if err != nil {
		return err
	}

	cHash, err := contract.ClassHash()
	if err != nil {
		return err
	}

	nonce, err := contract.Nonce()
	if err != nil {
		return err
	}

	commitment := calculateContractCommitment(root, cHash, nonce)

	state, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	if _, err = state.Put(contract.Address, commitment); err != nil {
		return err
	}

	return storageCloser()
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(classHash, storageRoot), nonce), &felt.Zero)
}

func (s *State) updateDeclaredClasses(declaredClasses []DeclaredV1Class) error {
	classesTrie, classesCloser, err := s.classesTrie()
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

	return classesCloser()
}
