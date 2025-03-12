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
	"github.com/sourcegraph/conc/pool"
)

// const globalTrieHeight = 251

// var (
// 	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
// 	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
// )

// var _ StateHistoryReader = (*State2)(nil)

// //go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
// type StateHistoryReader interface {
// 	StateReader

// 	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error)
// 	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
// 	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
// 	ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
// }

// type StateReader interface {
// 	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
// 	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
// 	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
// 	Class(classHash *felt.Felt) (*DeclaredClass, error)

// 	ClassTrie() (*trie.Trie, error)
// 	ContractTrie() (*trie.Trie, error)
// 	ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error)
// }

type State2 struct {
	*history2
	txn db.IndexedBatch
}

func NewState2(txn db.IndexedBatch) *State2 {
	return &State2{
		history2: &history2{txn: txn},
		txn:      txn,
	}
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State2) putNewContract(stateTrie *trie.Trie2, addr, classHash *felt.Felt, blockNumber uint64) error {
	contract, err := DeployContract2(addr, classHash, s.txn)
	if err != nil {
		return err
	}

	numBytes := MarshalBlockNumber(blockNumber)
	if err = s.txn.Put(db.ContractDeploymentHeightKey(addr), numBytes); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
}

// ContractClassHash returns class hash of a contract at a given address.
func (s *State2) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	return ContractClassHash2(addr, s.txn)
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State2) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	return ContractNonce2(addr, s.txn)
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State2) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	return ContractStorage2(addr, key, s.txn)
}

// Root returns the state commitment.
func (s *State2) Root() (*felt.Felt, error) {
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

// TODO(weiihann): handle the interface later...
func (s *State2) ClassTrie() (*trie.Trie2, error) {
	// We don't need to call the closer function here because we are only reading the trie
	tr, _, err := s.classesTrie()
	return tr, err
}

// TODO(weiihann): handle the interface later...
func (s *State2) ContractTrie() (*trie.Trie2, error) {
	tr, _, err := s.storage()
	return tr, err
}

// TODO(weiihann): handle the interface later...
func (s *State2) ContractStorageTrie(addr *felt.Felt) (*trie.Trie2, error) {
	return storage2(addr, s.txn)
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func (s *State2) storage() (*trie.Trie2, func() error, error) {
	return s.globalTrie(db.StateTrie, trie.NewTriePedersen2)
}

func (s *State2) classesTrie() (*trie.Trie2, func() error, error) {
	return s.globalTrie(db.ClassesTrie, trie.NewTriePoseidon2)
}

func (s *State2) globalTrie(bucket db.Bucket, newTrie trie.NewTrieFunc2) (*trie.Trie2, func() error, error) {
	dbPrefix := bucket.Key()
	tTxn := trie.NewStorage2(s.txn, dbPrefix)

	// fetch root key
	rootKeyDBKey := dbPrefix
	var rootKey *trie.BitArray // TODO: use value instead of pointer
	val, err := s.txn.Get2(rootKeyDBKey)
	if err != nil {
		return nil, nil, err
	}
	rootKey = new(trie.BitArray)
	if err := rootKey.UnmarshalBinary(val); err != nil {
		return nil, nil, err
	}

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
			_, marshalErr := resultingRootKey.Write(&rootKeyBytes)
			if marshalErr != nil {
				return marshalErr
			}

			return s.txn.Put(rootKeyDBKey, rootKeyBytes.Bytes())
		}
		return s.txn.Delete(rootKeyDBKey)
	}

	return gTrie, closer, nil
}

func (s *State2) verifyStateUpdateRoot(root *felt.Felt) error {
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
func (s *State2) Update(blockNumber uint64, update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
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

	// register deployed contracts
	for addr, classHash := range update.StateDiff.DeployedContracts {
		if err = s.putNewContract(stateTrie, &addr, classHash, blockNumber); err != nil {
			return err
		}
	}

	if err = s.updateContracts(stateTrie, blockNumber, update.StateDiff, true); err != nil {
		return err
	}

	if err = storageCloser(); err != nil {
		return err
	}

	return s.verifyStateUpdateRoot(update.NewRoot)
}

var (
	systemContractsClassHash = new(felt.Felt).SetUint64(0)

	systemContracts = map[felt.Felt]struct{}{
		*new(felt.Felt).SetUint64(1): {},
		*new(felt.Felt).SetUint64(2): {},
	}
)

func (s *State2) updateContracts(stateTrie *trie.Trie2, blockNumber uint64, diff *StateDiff, logChanges bool) error {
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

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State2) replaceContract(stateTrie *trie.Trie2, addr, classHash *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContractUpdater2(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldClassHash, err := ContractClassHash2(addr, s.txn)
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

func (s *State2) putClass(classHash *felt.Felt, class Class, declaredAt uint64) error {
	classKey := db.ClassKey(classHash)

	_, err := s.txn.Get2(classKey)
	if err != nil {
		return err
	}

	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(DeclaredClass{
			At:    declaredAt,
			Class: class,
		})
		if encErr != nil {
			return encErr
		}

		return s.txn.Put(classKey, classEncoded)
	}
	return err
}

// Class returns the class object corresponding to the given classHash
func (s *State2) Class(classHash *felt.Felt) (*DeclaredClass, error) {
	classKey := db.ClassKey(classHash)

	var class DeclaredClass
	val, err := s.txn.Get2(classKey)
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(val, &class)
	if err != nil {
		return nil, err
	}
	return &class, nil
}

func (s *State2) updateStorageBuffered(contractAddr *felt.Felt, updateDiff map[felt.Felt]*felt.Felt, blockNumber uint64, logChanges bool) (
	*db.BufferBatch, error,
) {
	// to avoid multiple transactions writing to s.txn, create a buffered transaction and use that in the worker goroutine
	bufferedTxn := db.NewBufferBatch(s.txn)
	bufferedState := NewState2(bufferedTxn)
	bufferedContract, err := NewContractUpdater2(contractAddr, bufferedTxn)
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

// updateContractStorages applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State2) updateContractStorages(stateTrie *trie.Trie2, diffs map[felt.Felt]map[felt.Felt]*felt.Felt,
	blockNumber uint64, logChanges bool,
) error {
	type bufferedTransactionWithAddress struct {
		txn  *db.BufferBatch
		addr *felt.Felt
	}

	// make sure all systemContracts are deployed
	for addr := range diffs {
		if _, ok := systemContracts[addr]; !ok {
			continue
		}

		_, err := NewContractUpdater2(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			// Deploy systemContract
			err = s.putNewContract(stateTrie, &addr, systemContractsClassHash, blockNumber)
			if err != nil {
				return err
			}
		}
	}

	// sort the contracts in decending diff size order
	// so we start with the heaviest update first
	keys := slices.SortedStableFunc(maps.Keys(diffs), func(a, b felt.Felt) int { return len(diffs[a]) - len(diffs[b]) })

	// update per-contract storage Tries concurrently
	contractUpdaters := pool.NewWithResults[*bufferedTransactionWithAddress]().WithErrors().WithMaxGoroutines(runtime.GOMAXPROCS(0))
	for _, key := range keys {
		contractAddr := key
		contractUpdaters.Go(func() (*bufferedTransactionWithAddress, error) {
			bufferedTxn, err := s.updateStorageBuffered(&contractAddr, diffs[contractAddr], blockNumber, logChanges)
			if err != nil {
				return nil, err
			}
			return &bufferedTransactionWithAddress{txn: bufferedTxn, addr: &contractAddr}, nil
		})
	}

	bufferedTxns, err := contractUpdaters.Wait()
	if err != nil {
		return err
	}

	// we sort bufferedTxns in ascending contract address order to achieve an additional speedup
	sort.Slice(bufferedTxns, func(i, j int) bool {
		return bufferedTxns[i].addr.Cmp(bufferedTxns[j].addr) < 0
	})

	// flush buffered txns
	for _, txnWithAddress := range bufferedTxns {
		if err := txnWithAddress.txn.Write(); err != nil {
			return err
		}
	}

	for addr := range diffs {
		contract, err := NewContractUpdater2(&addr, s.txn)
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
func (s *State2) updateContractNonce(stateTrie *trie.Trie2, addr, nonce *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContractUpdater2(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldNonce, err := ContractNonce2(addr, s.txn)
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
func (s *State2) updateContractCommitment(stateTrie *trie.Trie2, contract *ContractUpdater2) error {
	root, err := ContractRoot2(contract.Address, s.txn)
	if err != nil {
		return err
	}

	cHash, err := ContractClassHash2(contract.Address, s.txn)
	if err != nil {
		return err
	}

	nonce, err := ContractNonce2(contract.Address, s.txn)
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

func (s *State2) updateDeclaredClassesTrie(declaredClasses map[felt.Felt]*felt.Felt, classDefinitions map[felt.Felt]Class) error {
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
func (s *State2) ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	var deployedAt uint64
	val, err := s.txn.Get2(db.ContractDeploymentHeightKey(addr))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	deployedAt = binary.BigEndian.Uint64(val)
	return deployedAt <= blockNumber, nil
}

func (s *State2) Revert(blockNumber uint64, update *StateUpdate) error {
	err := s.verifyStateUpdateRoot(update.NewRoot)
	if err != nil {
		return fmt.Errorf("verify state update root: %v", err)
	}

	if err = s.removeDeclaredClasses(blockNumber, update.StateDiff.DeclaredV0Classes, update.StateDiff.DeclaredV1Classes); err != nil {
		return fmt.Errorf("remove declared classes: %v", err)
	}

	reversedDiff, err := s.GetReverseStateDiff(blockNumber, update.StateDiff)
	if err != nil {
		return fmt.Errorf("error getting reverse state diff: %v", err)
	}

	err = s.performStateDeletions(blockNumber, update.StateDiff)
	if err != nil {
		return fmt.Errorf("error performing state deletions: %v", err)
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

	if err = s.purgesystemContracts(); err != nil {
		return err
	}

	return s.verifyStateUpdateRoot(update.OldRoot)
}

func (s *State2) purgesystemContracts() error {
	// As systemContracts are not in StateDiff.DeployedContracts we can only purge them if their storage no longer exists.
	// Updating contracts with reverse diff will eventually lead to the deletion of noClassContract's storage key from db. Thus,
	// we can use the lack of key's existence as reason for purging systemContracts.
	for addr := range systemContracts {
		noClassC, err := NewContractUpdater2(&addr, s.txn)
		if err != nil {
			if !errors.Is(err, ErrContractNotDeployed) {
				return err
			}
			continue
		}

		r, err := ContractRoot2(noClassC.Address, s.txn)
		if err != nil {
			return fmt.Errorf("contract root: %v", err)
		}

		if r.Equal(&felt.Zero) {
			if err = s.purgeContract(&addr); err != nil {
				return fmt.Errorf("purge contract: %v", err)
			}
		}
	}
	return nil
}

func (s *State2) removeDeclaredClasses(blockNumber uint64, v0Classes []*felt.Felt, v1Classes map[felt.Felt]*felt.Felt) error {
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

		if err = s.txn.Delete(db.ClassKey(cHash)); err != nil {
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

func (s *State2) purgeContract(addr *felt.Felt) error {
	contract, err := NewContractUpdater2(addr, s.txn)
	if err != nil {
		return err
	}

	state, storageCloser, err := s.storage()
	if err != nil {
		return err
	}

	if err = s.txn.Delete(db.ContractDeploymentHeightKey(addr)); err != nil {
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

func (s *State2) GetReverseStateDiff(blockNumber uint64, diff *StateDiff) (*StateDiff, error) {
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

func (s *State2) performStateDeletions(blockNumber uint64, diff *StateDiff) error {
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
