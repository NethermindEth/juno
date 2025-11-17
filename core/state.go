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

const globalTrieHeight = 251

var (
	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
)

var _ StateHistoryReader = (*State)(nil)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
type StateHistoryReader interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
}

type StateReader interface {
	ChainHeight() (uint64, error)
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	ClassTrie() (*trie.Trie, error)
	ContractTrie() (*trie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error)
}

type State struct {
	*history
	txn db.IndexedBatch
}

func NewState(txn db.IndexedBatch) *State {
	return &State{
		history: &history{txn: txn},
		txn:     txn,
	}
}

func (s *State) ChainHeight() (uint64, error) {
	return GetChainHeight(s.txn)
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State) putNewContract(stateTrie *trie.Trie, addr, classHash *felt.Felt, blockNumber uint64) error {
	contract, err := DeployContract(addr, classHash, s.txn)
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
func (s *State) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return ContractClassHash(addr, s.txn)
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return ContractNonce(addr, s.txn)
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return ContractStorage(addr, key, s.txn)
}

// Root returns the state commitment.
func (s *State) Commitment() (felt.Felt, error) {
	var storageRoot, classesRoot felt.Felt

	sStorage, closer, err := s.storage()
	if err != nil {
		return felt.Felt{}, err
	}

	if storageRoot, err = sStorage.Hash(); err != nil {
		return felt.Felt{}, err
	}

	if err = closer(); err != nil {
		return felt.Felt{}, err
	}

	classes, closer, err := s.classesTrie()
	if err != nil {
		return felt.Felt{}, err
	}

	if classesRoot, err = classes.Hash(); err != nil {
		return felt.Felt{}, err
	}

	if err = closer(); err != nil {
		return felt.Felt{}, err
	}

	if classesRoot.IsZero() {
		return storageRoot, nil
	}

	root := crypto.PoseidonArray(stateVersion, &storageRoot, &classesRoot)
	return root, nil
}

func (s *State) ClassTrie() (*trie.Trie, error) {
	// We don't need to call the closer function here because we are only reading the trie
	tr, _, err := s.classesTrie()
	return tr, err
}

func (s *State) ContractTrie() (*trie.Trie, error) {
	tr, _, err := s.storage()
	return tr, err
}

func (s *State) ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error) {
	return storage(addr, s.txn)
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
	var val []byte
	err := s.txn.Get(rootKeyDBKey, func(data []byte) error {
		val = data
		return nil
	})
	// if some error other than "not found"
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil, err
	}

	rootKey := new(trie.BitArray)
	if len(val) > 0 {
		err = rootKey.UnmarshalBinary(val)
		if err != nil {
			return nil, nil, err
		}
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

func (s *State) verifyStateUpdateRoot(root *felt.Felt) error {
	currentRoot, err := s.Commitment()
	if err != nil {
		return err
	}
	if !root.Equal(&currentRoot) {
		return fmt.Errorf(
			"state's current root: %s does not match the expected root: %s",
			&currentRoot, root,
		)
	}
	return nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(
	blockNumber uint64,
	update *StateUpdate,
	declaredClasses map[felt.Felt]ClassDefinition,
	skipVerifyNewRoot bool,
) error {
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

	err = s.updateDeclaredClassesTrie(
		update.StateDiff.DeclaredV1Classes,
		declaredClasses,
		update.StateDiff.MigratedClasses,
	)
	if err != nil {
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

	// The following check isn't relevant for the centralised Juno sequencer
	if skipVerifyNewRoot {
		return nil
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

func (s *State) updateContracts(stateTrie *trie.Trie, blockNumber uint64, diff *StateDiff, logChanges bool) error {
	// replace contract instances
	for addr, classHash := range diff.ReplacedClasses {
		oldClassHash, err := s.replaceContract(stateTrie, &addr, classHash)
		if err != nil {
			return err
		}

		if logChanges {
			if err = s.LogContractClassHash(&addr, &oldClassHash, blockNumber); err != nil {
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
			if err = s.LogContractNonce(&addr, &oldNonce, blockNumber); err != nil {
				return err
			}
		}
	}

	// update contract storages
	return s.updateContractStorages(stateTrie, diff.StorageDiffs, blockNumber, logChanges)
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(
	stateTrie *trie.Trie,
	addr,
	classHash *felt.Felt,
) (felt.Felt, error) {
	return s.updateContract(stateTrie, addr, ContractClassHash, func(c *ContractUpdater) error {
		return c.Replace(classHash)
	})
}

func (s *State) putClass(classHash *felt.Felt, class ClassDefinition, declaredAt uint64) error {
	classKey := db.ClassKey(classHash)

	err := s.txn.Get(classKey, func(data []byte) error { return nil })
	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(DeclaredClassDefinition{
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
func (s *State) Class(classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	var class *DeclaredClassDefinition
	err := s.txn.Get(db.ClassKey(classHash), func(data []byte) error {
		return encoder.Unmarshal(data, &class)
	})
	return class, err
}

func (s *State) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	classTrie, closer, err := s.classesTrie()
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	defer func() {
		if closeErr := closer(); closeErr != nil {
			_ = closeErr
		}
	}()

	casmHash, err := classTrie.Get((*felt.Felt)(classHash))
	if err != nil {
		return felt.CasmClassHash{}, err
	}

	if casmHash.IsZero() {
		return felt.CasmClassHash{}, errors.New("casm hash not found")
	}

	return felt.CasmClassHash(casmHash), nil
}

func (s *State) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	casmHash, err := GetCasmClassHashV2(s.txn, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return casmHash, nil
}

func (s *State) updateStorageBuffered(contractAddr *felt.Felt, updateDiff map[felt.Felt]*felt.Felt, blockNumber uint64, logChanges bool) (
	*db.BufferBatch, error,
) {
	// to avoid multiple transactions writing to s.txn, create a buffered transaction and use that in the worker goroutine
	bufferedTxn := db.NewBufferBatch(s.txn)
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

// updateContractStorages applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorages(stateTrie *trie.Trie, diffs map[felt.Felt]map[felt.Felt]*felt.Felt,
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

		_, err := NewContractUpdater(&addr, s.txn)
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
		if err := txnWithAddress.txn.Flush(); err != nil {
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
func (s *State) updateContractNonce(
	stateTrie *trie.Trie,
	addr,
	nonce *felt.Felt,
) (felt.Felt, error) {
	return s.updateContract(stateTrie, addr, ContractNonce, func(c *ContractUpdater) error {
		return c.UpdateNonce(nonce)
	})
}

func (s *State) updateContract(
	stateTrie *trie.Trie,
	addr *felt.Felt,
	getOldValue func(*felt.Felt, db.IndexedBatch) (felt.Felt, error),
	updateValue func(*ContractUpdater) error,
) (felt.Felt, error) {
	contract, err := NewContractUpdater(addr, s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	oldVal, err := getOldValue(addr, s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	if err = updateValue(contract); err != nil {
		return felt.Felt{}, err
	}

	if err = s.updateContractCommitment(stateTrie, contract); err != nil {
		return felt.Felt{}, err
	}

	return oldVal, nil
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

	commitment := calculateContractCommitment(&root, &cHash, &nonce)

	_, err = stateTrie.Put(contract.Address, &commitment)
	return err
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) felt.Felt {
	h1 := crypto.Pedersen(classHash, storageRoot)
	h2 := crypto.Pedersen(&h1, nonce)
	return crypto.Pedersen(&h2, &felt.Zero)
}

func (s *State) updateDeclaredClassesTrie(
	declaredClasses map[felt.Felt]*felt.Felt,
	classDefinitions map[felt.Felt]ClassDefinition,
	migratedCasmClasses map[felt.SierraClassHash]felt.CasmClassHash,
) error {
	classesTrie, classesCloser, err := s.classesTrie()
	if err != nil {
		return err
	}

	for classHash, casmClassHash := range declaredClasses {
		if _, found := classDefinitions[classHash]; !found {
			continue
		}

		leafValue := crypto.Poseidon(leafVersion, casmClassHash)
		if _, err = classesTrie.Put(&classHash, &leafValue); err != nil {
			return err
		}
	}

	for classHash, casmClassHash := range migratedCasmClasses {
		classHashFelt := (*felt.Felt)(&classHash)

		leafValue := crypto.Poseidon(leafVersion, (*felt.Felt)(&casmClassHash))
		if _, err = classesTrie.Put(classHashFelt, &leafValue); err != nil {
			return err
		}
	}

	return classesCloser()
}

// ContractDeployedAt returns if contract at given addr was deployed at blockNumber
func (s *State) ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	var deployedAt uint64

	err := s.txn.Get(db.ContractDeploymentHeightKey(addr), func(data []byte) error {
		deployedAt = binary.BigEndian.Uint64(data)
		return nil
	})
	if err != nil {
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

	err = s.removeDeclaredClasses(
		blockNumber,
		update.StateDiff.DeclaredV0Classes,
		update.StateDiff.DeclaredV1Classes,
	)
	if err != nil {
		return fmt.Errorf("remove declared classes: %v", err)
	}

	err = s.revertMigratedCasmClasses(update.StateDiff.MigratedClasses)
	if err != nil {
		return fmt.Errorf("revert migrated casm classes: %v", err)
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

func (s *State) purgesystemContracts() error {
	// As systemContracts are not in StateDiff.DeployedContracts we can only purge them if their storage no longer exists.
	// Updating contracts with reverse diff will eventually lead to the deletion of noClassContract's storage key from db. Thus,
	// we can use the lack of key's existence as reason for purging systemContracts.
	for addr := range systemContracts {
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
	return nil
}

func (s *State) removeDeclaredClasses(
	blockNumber uint64,
	deprecatedClasses []*felt.Felt,
	sierraClasses map[felt.Felt]*felt.Felt,
) error {
	totalCapacity := len(deprecatedClasses) + len(sierraClasses)
	classHashes := make([]*felt.Felt, 0, totalCapacity)
	classHashes = append(classHashes, deprecatedClasses...)
	for classHash := range sierraClasses {
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

		if _, ok := declaredClass.Class.(*SierraClass); ok {
			if _, err = classesTrie.Put(cHash, &felt.Zero); err != nil {
				return err
			}

			sierraClassHash := felt.SierraClassHash(*cHash)
			if err = DeleteCasmClassHashV2(s.txn, &sierraClassHash); err != nil {
				return fmt.Errorf("delete CASM class hash V2 for class %s: %v", cHash, err)
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

// todo(rdr): return `StateDiff` by value
func (s *State) GetReverseStateDiff(blockNumber uint64, diff *StateDiff) (*StateDiff, error) {
	reversed := *diff

	reversed.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs))
	for addr, storageDiffs := range diff.StorageDiffs {
		reversedDiffs := make(map[felt.Felt]*felt.Felt, len(storageDiffs))
		for key := range storageDiffs {
			value := felt.Zero
			if blockNumber > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNumber-1)
				if err != nil {
					return nil, err
				}
				value = oldValue
			}
			reversedDiffs[key] = &value
		}
		reversed.StorageDiffs[addr] = reversedDiffs
	}

	reversed.Nonces = make(map[felt.Felt]*felt.Felt, len(diff.Nonces))
	for addr := range diff.Nonces {
		oldNonce := felt.Zero
		if blockNumber > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNumber-1)
			if err != nil {
				return nil, err
			}
		}
		reversed.Nonces[addr] = &oldNonce
	}

	reversed.ReplacedClasses = make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses))
	for addr := range diff.ReplacedClasses {
		classHash := felt.Zero
		if blockNumber > 0 {
			var err error
			classHash, err = s.ContractClassHashAt(&addr, blockNumber-1)
			if err != nil {
				return nil, err
			}
		}
		reversed.ReplacedClasses[addr] = &classHash
	}

	return &reversed, nil
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

func (s *State) revertMigratedCasmClasses(
	migratedCasmClasses map[felt.SierraClassHash]felt.CasmClassHash,
) error {
	classesTrie, classesCloser, err := s.classesTrie()
	if err != nil {
		return err
	}

	for classHash := range migratedCasmClasses {
		classHashFelt := (*felt.Felt)(&classHash)
		classDefinition, err := s.Class(classHashFelt)
		if err != nil {
			return err
		}

		stateUpdate, err := GetStateUpdateByBlockNum(s.txn, classDefinition.At)
		if err != nil {
			return err
		}
		deprecatedCasmHash := stateUpdate.StateDiff.DeclaredV1Classes[*classHashFelt]

		if _, err = classesTrie.Put(classHashFelt, deprecatedCasmHash); err != nil {
			return fmt.Errorf("revert class %s in trie: %w", classHashFelt, err)
		}
	}

	return classesCloser()
}
