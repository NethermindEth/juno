package core

import (
	"encoding/binary"
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

var _ StateHistoryReader = (*State)(nil)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
type StateHistoryReader interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClass, error)
}

type State struct {
	*History
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{
		History: NewHistory(txn),
		txn:     txn,
	}
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State) putNewContract(stateTrie *trie.Trie, addr, classHash *felt.Felt, blockNumber uint64) error {
	contract, err := DeployContract(addr, classHash, s.txn)
	if err != nil {
		return err
	}

	var numBytes [8]byte
	binary.BigEndian.PutUint64(numBytes[:], blockNumber)
	if err = s.txn.Set(db.ContractDeploymentHeight.Key(addr.Marshal()), numBytes[:]); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
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

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	return contract.Storage(key)
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
	tTxn := trie.NewTransactionStorage(s.txn, dbPrefix)

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
		if err = gTrie.Commit(); err != nil {
			return err
		}

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
func (s *State) Update(blockNumber uint64, update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
	currentRoot, err := s.Root()
	if err != nil {
		return err
	} else if !update.OldRoot.Equal(currentRoot) {
		return fmt.Errorf("state's current root: %s does not match state update's old root: %s", currentRoot, update.OldRoot)
	}

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	// Todo: add test cases for retrieving Classes when State struct is extended to return Classes.
	for classHash, class := range declaredClasses {
		if err = s.putClass(&classHash, class, blockNumber); err != nil {
			return err
		}
	}

	if err = s.updateDeclaredClasses(update.StateDiff.DeclaredV1Classes); err != nil {
		return err
	}

	if err = s.updateContracts(blockNumber, update.StateDiff); err != nil {
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

func (s *State) updateContracts(blockNumber uint64, diff *StateDiff) error {
	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	// register deployed contracts
	for _, contract := range diff.DeployedContracts {
		if err := s.putNewContract(stateTrie, contract.Address, contract.ClassHash, blockNumber); err != nil {
			return err
		}
	}

	// replace contract instances
	for _, replace := range diff.ReplacedClasses {
		oldClassHash, err := s.replaceContract(stateTrie, replace.Address, replace.ClassHash)
		if err != nil {
			return err
		}

		if err = s.LogContractClassHash(replace.Address, oldClassHash, blockNumber); err != nil {
			return err
		}
	}

	// update contract nonces
	for addr, nonce := range diff.Nonces {
		oldNonce, err := s.updateContractNonce(stateTrie, &addr, nonce)
		if err != nil {
			return err
		}

		if err = s.LogContractNonce(&addr, oldNonce, blockNumber); err != nil {
			return err
		}
	}

	// update contract storages
	for addr, storageDiff := range diff.StorageDiffs {
		onValueChanged := func(location, oldValue *felt.Felt) error {
			return s.LogContractStorage(&addr, location, oldValue, blockNumber)
		}

		if err := s.updateContractStorage(stateTrie, &addr, storageDiff, onValueChanged); err != nil {
			return err
		}
	}

	return storageCloser()
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(stateTrie *trie.Trie, addr, classHash *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldClassHash, err := contract.ClassHash()
	if err != nil {
		return nil, err
	}

	if err = contract.Replace(classHash); err != nil {
		return nil, err
	}

	if err = s.updateContractCommitment(stateTrie, contract); err != nil {
		return nil, err
	}

	return oldClassHash, nil
}

type DeclaredClass struct {
	At    uint64
	Class Class
}

func (s *State) putClass(classHash *felt.Felt, class Class, declaredAt uint64) error {
	classKey := db.Class.Key(classHash.Marshal())

	err := s.txn.Get(classKey, func(val []byte) error {
		return nil
	})

	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(DeclaredClass{
			At:    declaredAt,
			Class: class,
		})
		if encErr != nil {
			return encErr
		}

		return s.txn.Set(classKey, classEncoded)
	}
	return err
}

// Class returns the class object corresponding to the given classHash
func (s *State) Class(classHash *felt.Felt) (*DeclaredClass, error) {
	classKey := db.Class.Key(classHash.Marshal())

	var class DeclaredClass
	err := s.txn.Get(classKey, func(val []byte) error {
		return encoder.Unmarshal(val, &class)
	})
	if err != nil {
		return nil, err
	}
	return &class, nil
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(stateTrie *trie.Trie, addr *felt.Felt, diff []StorageDiff, onChanged OnValueChanged) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err = contract.UpdateStorage(diff, onChanged); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(stateTrie *trie.Trie, addr, nonce *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldNonce, err := contract.Nonce()
	if err != nil {
		return nil, err
	}

	if err = contract.UpdateNonce(nonce); err != nil {
		return nil, err
	}

	if err = s.updateContractCommitment(stateTrie, contract); err != nil {
		return nil, err
	}

	return oldNonce, nil
}

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(stateTrie *trie.Trie, contract *Contract) error {
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

	_, err = stateTrie.Put(contract.Address, commitment)
	return err
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

// ContractIsAlreadyDeployedAt returns if contract at given addr was deployed at blockNumber
func (s *State) ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	var deployedAt uint64
	if err := s.txn.Get(db.ContractDeploymentHeight.Key(addr.Marshal()), func(bytes []byte) error {
		deployedAt = binary.BigEndian.Uint64(bytes)
		return nil
	}); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return deployedAt <= blockNumber, nil
}
