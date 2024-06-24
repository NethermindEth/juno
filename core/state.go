package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/sourcegraph/conc/pool"
	"runtime"
	"sort"
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
func (s *State) putNewContract(stateTrie *trie.Trie, addr, classHash *felt.Felt, blockNumber uint64) error {
	contract, err := DeployContract(addr, classHash, s.txn)
	if err != nil {
		return err
	}

	numBytes := MarshalBlockNumber(blockNumber)
	if err = s.txn.Set(db.ContractDeploymentHeight.Key(addr.Marshal()), numBytes); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
}

// ContractClassHash returns class hash of a contract at a given address.
func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	return ContractClassHash(addr, s.txn)
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	return ContractNonce(addr, s.txn)
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	return ContractStorage(addr, key, s.txn)
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

func (s *State) StateAndClassRoot() (*felt.Felt, *felt.Felt, error) {
	var storageRoot, classesRoot *felt.Felt

	sStorage, closer, err := s.storage()
	if err != nil {
		return nil, nil, err
	}

	if storageRoot, err = sStorage.Root(); err != nil {
		return nil, nil, err
	}

	if err = closer(); err != nil {
		return nil, nil, err
	}

	classes, closer, err := s.classesTrie()
	if err != nil {
		return nil, nil, err
	}

	if classesRoot, err = classes.Root(); err != nil {
		return nil, nil, err
	}

	if err = closer(); err != nil {
		return nil, nil, err
	}

	return storageRoot, classesRoot, nil
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func (s *State) storage() (*trie.Trie, func() error, error) {
	return s.globalTrie(db.StateTrie, trie.NewTriePedersen)
}

func (s *State) classesTrie() (*trie.Trie, func() error, error) {
	return s.globalTrie(db.ClassesTrie, trie.NewTriePoseidon)
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func (s *State) StorageTrie() (*trie.Trie, func() error, error) {
	return s.storage()
}

func (s *State) ClassTrie() (*trie.Trie, func() error, error) {
	return s.classesTrie()
}

func (s *State) StorageTrieForAddr(addr *felt.Felt) (*trie.Trie, error) {
	return storage(addr, s.txn)
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
	if err != nil && !errors.Is(db.ErrKeyNotFound, err) {
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

	if err = s.UpdateNoVerify(blockNumber, update.StateDiff, declaredClasses); err != nil {
		return err
	}

	return s.verifyStateUpdateRoot(update.NewRoot)
}

func (s *State) UpdateNoVerify(blockNumber uint64, update *StateDiff, declaredClasses map[felt.Felt]Class) error {
	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	for cHash, class := range declaredClasses {
		if err := s.putClass(&cHash, class, blockNumber); err != nil {
			return err
		}
	}

	if err := s.updateDeclaredClassesTrie(update.DeclaredV1Classes, declaredClasses); err != nil {
		return err
	}

	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	// register deployed contracts
	for addr, classHash := range update.DeployedContracts {
		if err = s.putNewContract(stateTrie, &addr, classHash, blockNumber); err != nil {
			return err
		}
	}

	if err = s.updateContracts(stateTrie, blockNumber, update, true); err != nil {
		return err
	}

	if err = storageCloser(); err != nil {
		return err
	}

	return nil
}

var (
	noClassContractsClassHash = new(felt.Felt).SetUint64(0)

	noClassContracts = map[felt.Felt]struct{}{
		*new(felt.Felt).SetUint64(1): {},
	}
)

func (s *State) updateContracts(stateTrie *trie.Trie, blockNumber uint64, diff *StateDiff, logChanges bool) error {
	// replace contract instances
	for addr, classHash := range diff.ReplacedClasses {
		oldClassHash, err := s.replaceContract(stateTrie, &addr, classHash)
		if err != nil {
			return err
		}

		if logChanges {
			if err = s.LogContractClassHash(&addr, oldClassHash, blockNumber); err != nil {
				return err
			}
		}
	}

	// update contract nonces
	for addr, nonce := range diff.Nonces {
		oldNonce, err := s.updateContractNonce(stateTrie, &addr, nonce)
		if err != nil {
			return err
		}

		if logChanges {
			if err = s.LogContractNonce(&addr, oldNonce, blockNumber); err != nil {
				return err
			}
		}
	}

	// update contract storages
	return s.updateContractStorages(stateTrie, diff.StorageDiffs, blockNumber, logChanges)
}

func (s *State) UpdateContractNoLog(paths, nonces, classes []*felt.Felt) error {
	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	for i, path := range paths {
		nonce := nonces[i]
		class := classes[i]

		contract, err := NewContractUpdater(path, s.txn)
		if err != nil && !errors.Is(err, ErrContractNotDeployed) {
			return err
		}
		if errors.Is(err, ErrContractNotDeployed) {
			contract, err = DeployContract(path, class, s.txn)
			if err != nil {
				return err
			}

		}

		err = contract.Replace(class)
		if err != nil {
			return err
		}

		err = contract.UpdateNonce(nonce)
		if err != nil {
			return err
		}

		err = s.updateContractCommitment(stateTrie, contract)
		if err != nil {
			return err
		}
	}

	if err = storageCloser(); err != nil {
		return err
	}
	return nil
}

func (s *State) UpdateContractStorages(storages map[felt.Felt]map[felt.Felt]*felt.Felt) error {
	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	err = s.updateContractStorages(stateTrie, storages, 0, false)
	if err != nil {
		return err
	}

	return storageCloser()
}

func (s *State) UpdateContractStoragesKV(storages map[felt.Felt][]FeltKV) error {
	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	err = s.updateContractStoragesKV(stateTrie, storages, 0, false)
	if err != nil {
		return err
	}

	return storageCloser()
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(stateTrie *trie.Trie, addr, classHash *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContractUpdater(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldClassHash, err := ContractClassHash(addr, s.txn)
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

// StartsWith checks if the byte array 'a' starts with the byte array 'b'
func StartsWith(a, b []byte) bool {
	// If b is longer than a, it can't be a prefix
	if len(b) > len(a) {
		return false
	}

	// Compare the elements of a and b
	for i := 0; i < len(b); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func (s *State) PrintIt() (*DeclaredClass, error) {
	classKey := db.Class.Key(nil)

	it, err := s.txn.NewIterator()
	if err != nil {
		return nil, err
	}

	it.Seek(classKey)
	idx := 0
	printed := 0
	for it.Valid() && StartsWith(it.Key(), classKey) {
		value, err := it.Value()
		if err != nil {
			return nil, err
		}

		var class DeclaredClass
		err = encoder.Unmarshal(value, &class)
		if err != nil {
			return nil, err
		}

		if class.Class.Version() == 0 {
			fmt.Printf("%d %x %d %d\n", idx, it.Key(), class.Class.Version(), len(value))
			if class.Class.Version() == 0 && len(value) < 20000 {
				fmt.Printf("%x\n", value)
				printed++
				if printed >= 2 {
					return nil, nil
				}
			}
		}

		idx++
		it.Next()
	}

	return nil, nil
}

func (s *State) updateStorageBuffered(contractAddr *felt.Felt, updateDiff map[felt.Felt]*felt.Felt, blockNumber uint64, logChanges bool) (
	*db.BufferedTransaction, error,
) {
	// to avoid multiple transactions writing to s.txn, create a buffered transaction and use that in the worker goroutine
	bufferedTxn := db.NewBufferedTransaction(s.txn)
	bufferedState := NewState(bufferedTxn)
	bufferedContract, err := NewContractUpdater(contractAddr, bufferedTxn)
	if err != nil {
		return nil, err
	}

	onValueChanged := func(location, oldValue *felt.Felt) error {
		if logChanges {
			return bufferedState.LogContractStorage(contractAddr, location, oldValue, blockNumber)
		}
		return nil
	}

	if err = bufferedContract.UpdateStorage(updateDiff, onValueChanged); err != nil {
		return nil, err
	}

	return bufferedTxn, nil
}

func (s *State) updateStorageBufferedKV(contractAddr *felt.Felt, updateDiff []FeltKV, blockNumber uint64, logChanges bool) (
	*db.BufferedTransaction, error,
) {
	// to avoid multiple transactions writing to s.txn, create a buffered transaction and use that in the worker goroutine
	bufferedTxn := db.NewBufferedTransaction(s.txn)
	bufferedState := NewState(bufferedTxn)
	bufferedContract, err := NewContractUpdater(contractAddr, bufferedTxn)
	if err != nil {
		return nil, err
	}

	onValueChanged := func(location, oldValue *felt.Felt) error {
		if logChanges {
			return bufferedState.LogContractStorage(contractAddr, location, oldValue, blockNumber)
		}
		return nil
	}

	if err = bufferedContract.UpdateStorageKV(updateDiff, onValueChanged); err != nil {
		return nil, err
	}

	return bufferedTxn, nil
}

type FeltKV struct {
	Key   *felt.Felt
	Value *felt.Felt
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStoragesKV(stateTrie *trie.Trie, diffs map[felt.Felt][]FeltKV,
	blockNumber uint64, logChanges bool,
) error {
	// make sure all noClassContracts are deployed
	for addr := range diffs {
		if _, ok := noClassContracts[addr]; !ok {
			continue
		}

		_, err := NewContractUpdater(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			// Deploy noClassContract
			err = s.putNewContract(stateTrie, &addr, noClassContractsClassHash, blockNumber)
			if err != nil {
				return err
			}
		}
	}

	// sort the contracts in decending diff size order
	// so we start with the heaviest update first
	keys := make([]felt.Felt, 0, len(diffs))
	for key := range diffs {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return len(diffs[keys[i]]) > len(diffs[keys[j]])
	})

	// update per-contract storage Tries concurrently
	contractUpdaters := pool.NewWithResults[*db.BufferedTransaction]().WithErrors().WithMaxGoroutines(runtime.GOMAXPROCS(0))
	for _, key := range keys {
		conractAddr := key
		updateDiff := diffs[conractAddr]
		contractUpdaters.Go(func() (*db.BufferedTransaction, error) {
			return s.updateStorageBufferedKV(&conractAddr, updateDiff, blockNumber, logChanges)
		})
	}

	bufferedTxns, err := contractUpdaters.Wait()
	if err != nil {
		return err
	}

	// flush buffered txns
	for _, bufferedTxn := range bufferedTxns {
		if err = bufferedTxn.Flush(); err != nil {
			return err
		}
	}

	for addr := range diffs {
		contract, err := NewContractUpdater(&addr, s.txn)
		if err != nil {
			return err
		}

		if err = s.updateContractCommitment(stateTrie, contract); err != nil {
			return err
		}
	}

	return nil
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorages(stateTrie *trie.Trie, diffs map[felt.Felt]map[felt.Felt]*felt.Felt,
	blockNumber uint64, logChanges bool,
) error {
	// make sure all noClassContracts are deployed
	for addr := range diffs {
		if _, ok := noClassContracts[addr]; !ok {
			continue
		}

		_, err := NewContractUpdater(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			// Deploy noClassContract
			err = s.putNewContract(stateTrie, &addr, noClassContractsClassHash, blockNumber)
			if err != nil {
				return err
			}
		}
	}

	// sort the contracts in decending diff size order
	// so we start with the heaviest update first
	keys := make([]felt.Felt, 0, len(diffs))
	for key := range diffs {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return len(diffs[keys[i]]) > len(diffs[keys[j]])
	})

	// update per-contract storage Tries concurrently
	contractUpdaters := pool.NewWithResults[*db.BufferedTransaction]().WithErrors().WithMaxGoroutines(runtime.GOMAXPROCS(0))
	for _, key := range keys {
		conractAddr := key
		updateDiff := diffs[conractAddr]
		contractUpdaters.Go(func() (*db.BufferedTransaction, error) {
			return s.updateStorageBuffered(&conractAddr, updateDiff, blockNumber, logChanges)
		})
	}

	bufferedTxns, err := contractUpdaters.Wait()
	if err != nil {
		return err
	}

	// flush buffered txns
	for _, bufferedTxn := range bufferedTxns {
		if err = bufferedTxn.Flush(); err != nil {
			return err
		}
	}

	for addr := range diffs {
		contract, err := NewContractUpdater(&addr, s.txn)
		if err != nil {
			return err
		}

		if err = s.updateContractCommitment(stateTrie, contract); err != nil {
			return err
		}
	}

	return nil
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(stateTrie *trie.Trie, addr, nonce *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContractUpdater(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldNonce, err := ContractNonce(addr, s.txn)
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
func (s *State) updateContractCommitment(stateTrie *trie.Trie, contract *ContractUpdater) error {
	root, err := ContractRoot(contract.Address, s.txn)
	if err != nil {
		return err
	}

	cHash, err := ContractClassHash(contract.Address, s.txn)
	if err != nil {
		return err
	}

	nonce, err := ContractNonce(contract.Address, s.txn)
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

func (s *State) updateDeclaredClassesTrie(declaredClasses map[felt.Felt]*felt.Felt, classDefinitions map[felt.Felt]Class) error {
	classesTrie, classesCloser, err := s.classesTrie()
	if err != nil {
		return err
	}

	for classHash, compiledClassHash := range declaredClasses {
		if _, found := classDefinitions[classHash]; !found {
			continue
		}

		// https://docs.starknet.io/documentation/starknet_versions/upcoming_versions/#commitment
		leafValue := crypto.Poseidon(leafVersion, compiledClassHash)
		if _, err = classesTrie.Put(&classHash, leafValue); err != nil {
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

	if err = s.updateContracts(stateTrie, blockNumber, reversedDiff, false); err != nil {
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
		noClassC, err := NewContractUpdater(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			continue
		}

		r, err := ContractRoot(noClassC.Address, s.txn)
		if err != nil {
			return fmt.Errorf("contract root: %v", err)
		}

		if r.Equal(&felt.Zero) {
			if err = s.purgeContract(&addr); err != nil {
				return fmt.Errorf("purge contract: %v", err)
			}
		}
	}

	return s.verifyStateUpdateRoot(update.OldRoot)
}

func (s *State) removeDeclaredClasses(blockNumber uint64, v0Classes []*felt.Felt, v1Classes map[felt.Felt]*felt.Felt) error {
	var classHashes []*felt.Felt
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
	contract, err := NewContractUpdater(addr, s.txn)
	if err != nil {
		return err
	}

	state, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	if err = s.txn.Delete(db.ContractDeploymentHeight.Key(addr.Marshal())); err != nil {
		return err
	}

	if _, err = state.Put(contract.Address, &felt.Zero); err != nil {
		return err
	}

	if err = contract.Purge(); err != nil {
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
