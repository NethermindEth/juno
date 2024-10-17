package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sort"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/pool"
)

const globalTrieHeight = 251

var (
	stateVersion      = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion       = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
	ErrCheckHeadState = errors.New("check head state")
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
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{
		txn: txn,
	}
}

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

func (s *State) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	return s.contractValueAt(classHashLogKey, addr, blockNumber)
}

func (s *State) ContractStorageAt(addr, loc *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	return s.contractValueAt(func(a *felt.Felt) []byte { return storageLogKey(a, loc) }, addr, blockNumber)
}

func (s *State) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	return s.contractValueAt(nonceLogKey, addr, blockNumber)
}

func (s *State) deleteLog(key []byte, height uint64) error {
	return s.txn.Delete(logDBKey(key, height))
}

func (s *State) DeleteContractStorageLog(contractAddress, storageLocation *felt.Felt, height uint64) error {
	return s.deleteLog(storageLogKey(contractAddress, storageLocation), height)
}

func (s *State) DeleteContractNonceLog(contractAddress *felt.Felt, height uint64) error {
	return s.deleteLog(nonceLogKey(contractAddress), height)
}

func (s *State) DeleteContractClassHashLog(contractAddress *felt.Felt, height uint64) error {
	return s.deleteLog(classHashLogKey(contractAddress), height)
}

func (s *State) contractValueAt(keyFunc func(*felt.Felt) []byte, addr *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	key := keyFunc(addr)
	value, err := s.valueAt(key, blockNumber)
	if err != nil {
		return nil, err
	}

	return new(felt.Felt).SetBytes(value), nil
}

func (s *State) valueAt(key []byte, height uint64) ([]byte, error) {
	it, err := s.txn.NewIterator()
	if err != nil {
		return nil, err
	}

	for it.Seek(logDBKey(key, height)); it.Valid(); it.Next() {
		seekedKey := it.Key()
		// seekedKey size should be `len(key) + sizeof(uint64)` and seekedKey should match key prefix
		if len(seekedKey) != len(key)+8 || !bytes.HasPrefix(seekedKey, key) {
			break
		}

		seekedHeight := binary.BigEndian.Uint64(seekedKey[len(key):])
		if seekedHeight < height {
			// last change happened before the height we are looking for
			// check head state
			break
		} else if seekedHeight == height {
			// a log exists for the height we are looking for, so the old value in this log entry is not useful.
			// advance the iterator and see we can use the next entry. If not, ErrCheckHeadState will be returned
			continue
		}

		val, itErr := it.Value()
		if err = utils.RunAndWrapOnError(it.Close, itErr); err != nil {
			return nil, err
		}
		// seekedHeight > height
		return val, nil
	}

	return nil, utils.RunAndWrapOnError(it.Close, ErrCheckHeadState)
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

		contracts[addr] = NewStateContract(&addr, classHash, &felt.Zero, blockNumber)
	}

	if err = s.updateContracts(blockNumber, update.StateDiff, true, contracts); err != nil {
		return err
	}

	if err = s.Commit(stateTrie, contracts, true, blockNumber); err != nil {
		return fmt.Errorf("state commit: %v", err)
	}

	if err = storageCloser(); err != nil {
		return err
	}

	return s.verifyStateUpdateRoot(update.NewRoot)
}

func (s *State) GetReverseStateDiff(blockNumber uint64, diff *StateDiff) (*StateDiff, error) {
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
		reversed.ReplacedClasses[addr] = classHash
	}

	return &reversed, nil
}

var (
	noClassContractsClassHash = new(felt.Felt).SetUint64(0)

	noClassContracts = map[felt.Felt]struct{}{
		*new(felt.Felt).SetUint64(1): {},
	}
)

// Commit updates the state by committing the dirty contracts to the database.
func (s *State) Commit(
	stateTrie *trie.Trie,
	contracts map[felt.Felt]*StateContract,
	logChanges bool,
	blockNumber uint64,
) error {
	type bufferedTransactionWithAddress struct {
		txn  *db.BufferedTransaction
		addr *felt.Felt
	}

	// // sort the contracts in descending storage diff order
	keys := slices.SortedStableFunc(maps.Keys(contracts), func(a, b felt.Felt) int {
		return len(contracts[a].dirtyStorage) - len(contracts[b].dirtyStorage)
	})

	contractPools := pool.NewWithResults[*bufferedTransactionWithAddress]().WithErrors().WithMaxGoroutines(runtime.GOMAXPROCS(0))
	for _, addr := range keys {
		contract := contracts[addr]
		contractPools.Go(func() (*bufferedTransactionWithAddress, error) {
			txn, err := contract.BufferedCommit(s.txn, logChanges, blockNumber)
			if err != nil {
				return nil, err
			}

			return &bufferedTransactionWithAddress{
				txn:  txn,
				addr: contract.Address,
			}, nil
		})
	}

	bufferedTxns, err := contractPools.Wait()
	if err != nil {
		return err
	}

	// we sort bufferedTxns in ascending contract address order to achieve an additional speedup
	sort.Slice(bufferedTxns, func(i, j int) bool {
		return bufferedTxns[i].addr.Cmp(bufferedTxns[j].addr) < 0
	})

	for _, bufferedTxn := range bufferedTxns {
		if err := bufferedTxn.txn.Flush(); err != nil {
			return err
		}
	}

	for _, contract := range contracts {
		if err := s.updateContractCommitment(stateTrie, contract); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) updateContracts(blockNumber uint64, diff *StateDiff, logChanges bool, contracts map[felt.Felt]*StateContract) error {
	if contracts == nil {
		return fmt.Errorf("contracts is nil")
	}

	if err := s.updateContractClasses(blockNumber, diff.ReplacedClasses, logChanges, contracts); err != nil {
		return err
	}

	if err := s.updateContractNonces(blockNumber, diff.Nonces, logChanges, contracts); err != nil {
		return err
	}

	return s.updateContractStorages(blockNumber, diff.StorageDiffs, contracts)
}

func (s *State) updateContractClasses(
	blockNumber uint64,
	replacedClasses map[felt.Felt]*felt.Felt,
	logChanges bool,
	contracts map[felt.Felt]*StateContract,
) error {
	for addr, classHash := range replacedClasses {
		contract, err := s.getOrCreateContract(addr, contracts)
		if err != nil {
			return err
		}

		if logChanges {
			if err := contract.LogClassHash(blockNumber, s.txn); err != nil {
				return err
			}
		}

		contract.ClassHash = classHash
	}
	return nil
}

func (s *State) updateContractNonces(
	blockNumber uint64,
	nonces map[felt.Felt]*felt.Felt,
	logChanges bool,
	contracts map[felt.Felt]*StateContract,
) error {
	for addr, nonce := range nonces {
		contract, err := s.getOrCreateContract(addr, contracts)
		if err != nil {
			return err
		}

		if logChanges {
			if err := contract.LogNonce(blockNumber, s.txn); err != nil {
				return err
			}
		}

		contract.Nonce = nonce
	}
	return nil
}

func (s *State) updateContractStorages(
	blockNumber uint64,
	storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt,
	contracts map[felt.Felt]*StateContract,
) error {
	for addr, diff := range storageDiffs {
		contract, err := s.getOrCreateContract(addr, contracts)
		if err != nil {
			if _, ok := noClassContracts[addr]; ok && errors.Is(err, ErrContractNotDeployed) {
				contract = NewStateContract(&addr, noClassContractsClassHash, &felt.Zero, blockNumber)
				contracts[addr] = contract
			} else {
				return err
			}
		}

		contract.dirtyStorage = diff
	}
	return nil
}

func (s *State) getOrCreateContract(addr felt.Felt, contracts map[felt.Felt]*StateContract) (*StateContract, error) {
	contract, ok := contracts[addr]
	if !ok {
		var err error
		contract, err = GetContract(&addr, s.txn)
		if err != nil {
			return nil, err
		}
		contracts[addr] = contract
	}
	return contract, nil
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

	reversedDiff, err := s.GetReverseStateDiff(blockNumber, update.StateDiff)
	if err != nil {
		return fmt.Errorf("build reverse diff: %v", err)
	}

	if err = s.performStateDeletions(blockNumber, reversedDiff); err != nil {
		return fmt.Errorf("perform state deletions: %v", err)
	}

	stateTrie, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	contracts := make(map[felt.Felt]*StateContract)
	if err = s.updateContracts(blockNumber, reversedDiff, false, contracts); err != nil {
		return fmt.Errorf("update contracts: %v", err)
	}

	if err = s.Commit(stateTrie, contracts, false, blockNumber); err != nil {
		return fmt.Errorf("state commit: %v", err)
	}

	// purge deployed contracts
	for addr := range update.StateDiff.DeployedContracts {
		if err = s.purgeContract(stateTrie, &addr); err != nil {
			return fmt.Errorf("purge contract: %v", err)
		}
	}

	if err = s.purgeNoClassContracts(stateTrie); err != nil {
		return fmt.Errorf("purge no class contract: %v", err)
	}

	if err = storageCloser(); err != nil {
		return err
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

func (s *State) purgeContract(stateTrie *trie.Trie, addr *felt.Felt) error {
	contract, err := GetContract(addr, s.txn)
	if err != nil {
		return err
	}

	if _, err = stateTrie.Put(contract.Address, &felt.Zero); err != nil {
		return err
	}

	if err = contract.Purge(s.txn); err != nil {
		return err
	}

	return nil
}

func (s *State) performStateDeletions(blockNumber uint64, diff *StateDiff) error {
	// storage diffs
	for addr, storageDiffs := range diff.StorageDiffs {
		for key := range storageDiffs {
			if err := s.DeleteContractStorageLog(&addr, &key, blockNumber); err != nil {
				return err
			}
		}
	}

	// nonces
	for addr := range diff.Nonces {
		if err := s.DeleteContractNonceLog(&addr, blockNumber); err != nil {
			return err
		}
	}

	// replaced classes
	for addr := range diff.ReplacedClasses {
		if err := s.DeleteContractClassHashLog(&addr, blockNumber); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) purgeNoClassContracts(stateTrie *trie.Trie) error {
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
			if err = s.purgeContract(stateTrie, &addr); err != nil {
				return fmt.Errorf("purge contract: %v", err)
			}
		}
	}

	return nil
}

func logDBKey(key []byte, height uint64) []byte {
	return binary.BigEndian.AppendUint64(key, height)
}
