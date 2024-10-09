package core

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
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
	*history
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{
		history: &history{txn: txn},
		txn:     txn,
	}
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
// func (s *State) putNewContract(stateTrie *trie.Trie, addr, classHash *felt.Felt, blockNumber uint64) error {
// 	contract, err := DeployContract(addr, classHash, s.txn)
// 	if err != nil {
// 		return err
// 	}

// 	numBytes := MarshalBlockNumber(blockNumber)
// 	if err = s.txn.Set(db.ContractDeploymentHeight.Key(addr.Marshal()), numBytes); err != nil {
// 		return err
// 	}

// 	return s.updateContractCommitment(stateTrie, contract)
// }

// ContractClassHash returns class hash of a contract at a given address.
func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	return contract.ClassHash, nil
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	return contract.Nonce, nil
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	return contract.GetStorage(key, s.txn)
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
	tTxn := trie.NewStorage(s.txn, dbPrefix)

	// fetch root key
	rootKeyDBKey := dbPrefix
	var rootKey *trie.Key
	err := s.txn.Get(rootKeyDBKey, func(val []byte) error {
		rootKey = new(trie.Key)
		return rootKey.UnmarshalBinary(val)
	})

	// if some error other than "not found"
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil, err
	}

	gTrie, err := newTrie(tTxn, globalTrieHeight)
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
			var rootKeyBytes bytes.Buffer
			_, marshalErr := resultingRootKey.WriteTo(&rootKeyBytes)
			if marshalErr != nil {
				return marshalErr
			}

			return s.txn.Set(rootKeyDBKey, rootKeyBytes.Bytes())
		}
		return s.txn.Delete(rootKeyDBKey)
	}

	return gTrie, closer, nil
}

func (s *State) verifyStateUpdateRoot(root *felt.Felt) error {
	currentRoot, err := s.Root()
	if err != nil {
		return err
	}

	if !root.Equal(currentRoot) {
		return fmt.Errorf("state's current root: %s does not match the expected root: %s", currentRoot, root)
	}
	return nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(blockNumber uint64, update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
	err := s.verifyStateUpdateRoot(update.OldRoot)
	if err != nil {
		return err
	}

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	for cHash, class := range declaredClasses {
		if err = s.putClass(&cHash, class, blockNumber); err != nil {
			return err
		}
	}

	if err = s.updateDeclaredClassesTrie(update.StateDiff.DeclaredV1Classes, declaredClasses); err != nil {
		return err
	}

	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	contracts := make(map[felt.Felt]*StateContract)
	// register deployed contracts
	for addr, classHash := range update.StateDiff.DeployedContracts {
		// check if contract is already deployed
		_, err := GetContract(&addr, s.txn)
		if err == nil {
			return ErrContractAlreadyDeployed
		}

		if !errors.Is(err, ErrContractNotDeployed) {
			return err
		}

		contracts[addr] = NewStateContract(&addr, classHash, &felt.Zero, nil, blockNumber)
	}

	if err = s.updateContracts(blockNumber, update.StateDiff, true, contracts); err != nil {
		return err
	}

	// TODO(weiihann): handle history
	tempOnValChanged := func(location, oldValue *felt.Felt) error {
		return nil
	}

	// Commit all contract updates
	for _, contract := range contracts {
		if err = contract.Commit(s.txn, tempOnValChanged); err != nil {
			return err
		}

		if err := s.updateContractCommitment(stateTrie, contract); err != nil {
			return err
		}
	}

	if err = storageCloser(); err != nil {
		return err
	}

	return s.verifyStateUpdateRoot(update.NewRoot)
}

var (
	noClassContractsClassHash = new(felt.Felt).SetUint64(0)

	noClassContracts = map[felt.Felt]struct{}{
		*new(felt.Felt).SetUint64(1): {},
	}
)

func (s *State) updateContracts(blockNumber uint64, diff *StateDiff, logChanges bool, contracts map[felt.Felt]*StateContract) error {
	if contracts == nil {
		return fmt.Errorf("contracts is nil")
	}

	var err error

	// update contract class hashes
	for addr, classHash := range diff.ReplacedClasses {
		contract, ok := contracts[addr]
		if !ok {
			contract, err = GetContract(&addr, s.txn)
			if err != nil {
				return err
			}
			contracts[addr] = contract
		}

		oldClassHash := contract.ClassHash
		contract.ClassHash = classHash

		if logChanges {
			if err := s.LogContractClassHash(&addr, oldClassHash, blockNumber); err != nil {
				return err
			}
		}
	}

	// update contract nonces
	for addr, nonce := range diff.Nonces {
		contract, ok := contracts[addr]
		if !ok {
			contract, err = GetContract(&addr, s.txn)
			if err != nil {
				return err
			}
			contracts[addr] = contract
		}

		oldNonce := contract.Nonce
		contract.Nonce = nonce

		if logChanges {
			if err := s.LogContractNonce(&addr, oldNonce, blockNumber); err != nil {
				return err
			}
		}
	}

	// update contract storages
	for addr, diff := range diff.StorageDiffs {
		contract, ok := contracts[addr]
		if !ok {
			contract, err = GetContract(&addr, s.txn)
			if err != nil {
				// makes sure that all noClassContracts are deployed
				if _, ok := noClassContracts[addr]; ok && errors.Is(err, ErrContractNotDeployed) {
					contract = NewStateContract(&addr, noClassContractsClassHash, &felt.Zero, nil, blockNumber)
				} else {
					return err
				}
			}
			contracts[addr] = contract
		}

		contract.Storage = diff
	}

	return nil
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

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(stateTrie *trie.Trie, contract *StateContract) error {
	rootKey, err := contract.StorageRoot(s.txn)
	if err != nil {
		return err
	}

	commitment := calculateContractCommitment(
		rootKey,
		contract.ClassHash,
		contract.Nonce,
	)

	_, err = stateTrie.Put(contract.Address, commitment)
	return err
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(classHash, storageRoot), nonce), &felt.Zero)
}

func (s *State) updateDeclaredClassesTrie(declaredClasses map[felt.Felt]*felt.Felt, classDefinitions map[felt.Felt]Class) error {
	classesTrie, classesCloser, err := s.classesTrie()
	if err != nil {
		return err
	}

	for classHash, compiledClassHash := range declaredClasses {
		if _, found := classDefinitions[classHash]; !found {
			continue
		}

		leafValue := crypto.Poseidon(leafVersion, compiledClassHash)
		if _, err = classesTrie.Put(&classHash, leafValue); err != nil {
			return err
		}
	}

	return classesCloser()
}

// ContractIsAlreadyDeployedAt returns if contract at given addr was deployed at blockNumber
func (s *State) ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		if errors.Is(err, ErrContractNotDeployed) {
			return false, nil
		}
		return false, err
	}

	return contract.DeployHeight <= blockNumber, nil
}

func (s *State) Revert(blockNumber uint64, update *StateUpdate) error {
	err := s.verifyStateUpdateRoot(update.NewRoot)
	if err != nil {
		return fmt.Errorf("verify state update root: %v", err)
	}

	if err = s.removeDeclaredClasses(blockNumber, update.StateDiff.DeclaredV0Classes, update.StateDiff.DeclaredV1Classes); err != nil {
		return fmt.Errorf("remove declared classes: %v", err)
	}

	// update contracts
	reversedDiff, err := s.buildReverseDiff(blockNumber, update.StateDiff)
	if err != nil {
		return fmt.Errorf("build reverse diff: %v", err)
	}

	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	contracts := make(map[felt.Felt]*StateContract)
	if err = s.updateContracts(blockNumber, reversedDiff, false, contracts); err != nil {
		return fmt.Errorf("update contracts: %v", err)
	}

	if err = storageCloser(); err != nil {
		return err
	}

	// purge deployed contracts
	for addr := range update.StateDiff.DeployedContracts {
		if err = s.purgeContract(&addr); err != nil {
			return fmt.Errorf("purge contract: %v", err)
		}
	}

	// purge noClassContracts
	//
	// As noClassContracts are not in StateDiff.DeployedContracts we can only purge them if their storage no longer exists.
	// Updating contracts with reverse diff will eventually lead to the deletion of noClassContract's storage key from db. Thus,
	// we can use the lack of key's existence as reason for purging noClassContracts.
	for addr := range noClassContracts {
		contract, err := GetContract(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			continue
		}

		rootKey, err := contract.StorageRoot(s.txn)
		if err != nil {
			return fmt.Errorf("get root key: %v", err)
		}

		if rootKey.Equal(&felt.Zero) {
			if err = s.purgeContract(&addr); err != nil {
				return fmt.Errorf("purge contract: %v", err)
			}
		}
	}

	// TODO(weiihann): handle this
	tempOnValChanged := func(location, oldValue *felt.Felt) error {
		return nil
	}

	// TODO(weiihann): make concurrent
	// Commit the changes to the contracts and update their commitments
	for _, contract := range contracts {
		if err = contract.Commit(s.txn, tempOnValChanged); err != nil {
			return fmt.Errorf("commit contract: %v", err)
		}

		if err = s.updateContractCommitment(stateTrie, contract); err != nil {
			return fmt.Errorf("update contract commitment: %v", err)
		}
	}

	return s.verifyStateUpdateRoot(update.OldRoot)
}

func (s *State) removeDeclaredClasses(blockNumber uint64, v0Classes []*felt.Felt, v1Classes map[felt.Felt]*felt.Felt) error {
	totalCapacity := len(v0Classes) + len(v1Classes)
	classHashes := make([]*felt.Felt, 0, totalCapacity)
	classHashes = append(classHashes, v0Classes...)
	for classHash := range v1Classes {
		classHashes = append(classHashes, classHash.Clone())
	}

	classesTrie, classesCloser, err := s.classesTrie()
	if err != nil {
		return err
	}
	for _, cHash := range classHashes {
		declaredClass, err := s.Class(cHash)
		if err != nil {
			return fmt.Errorf("get class %s: %v", cHash, err)
		}
		if declaredClass.At != blockNumber {
			continue
		}

		if err = s.txn.Delete(db.Class.Key(cHash.Marshal())); err != nil {
			return fmt.Errorf("delete class: %v", err)
		}

		// cairo1 class, update the class commitment trie as well
		if declaredClass.Class.Version() == 1 {
			if _, err = classesTrie.Put(cHash, &felt.Zero); err != nil {
				return err
			}
		}
	}
	return classesCloser()
}

func (s *State) purgeContract(addr *felt.Felt) error {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		return err
	}

	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	if _, err = stateTrie.Put(contract.Address, &felt.Zero); err != nil {
		return err
	}

	if err = contract.Purge(s.txn); err != nil {
		return err
	}

	return storageCloser()
}

func (s *State) buildReverseDiff(blockNumber uint64, diff *StateDiff) (*StateDiff, error) {
	reversed := *diff

	// storage diffs
	reversed.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs))
	for addr, storageDiffs := range diff.StorageDiffs {
		reversedDiffs := make(map[felt.Felt]*felt.Felt, len(storageDiffs))
		for key := range storageDiffs {
			value := &felt.Zero
			if blockNumber > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNumber-1)
				if err != nil {
					return nil, err
				}
				value = oldValue
			}

			if err := s.DeleteContractStorageLog(&addr, &key, blockNumber); err != nil {
				return nil, err
			}
			reversedDiffs[key] = value
		}
		reversed.StorageDiffs[addr] = reversedDiffs
	}

	// nonces
	reversed.Nonces = make(map[felt.Felt]*felt.Felt, len(diff.Nonces))
	for addr := range diff.Nonces {
		oldNonce := &felt.Zero

		if blockNumber > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNumber-1)
			if err != nil {
				return nil, err
			}
		}

		if err := s.DeleteContractNonceLog(&addr, blockNumber); err != nil {
			return nil, err
		}
		reversed.Nonces[addr] = oldNonce
	}

	// replaced
	reversed.ReplacedClasses = make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses))
	for addr := range diff.ReplacedClasses {
		classHash := &felt.Zero
		if blockNumber > 0 {
			var err error
			classHash, err = s.ContractClassHashAt(&addr, blockNumber-1)
			if err != nil {
				return nil, err
			}
		}

		if err := s.DeleteContractClassHashLog(&addr, blockNumber); err != nil {
			return nil, err
		}
		reversed.ReplacedClasses[addr] = classHash
	}

	return &reversed, nil
}
