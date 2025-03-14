package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db"
	"github.com/sourcegraph/conc/pool"
)

const (
	ClassTrieHeight    = 251
	ContractTrieHeight = 251
)

var (
	stateVersion0             = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion0              = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
	noClassContractsClassHash = new(felt.Felt).SetUint64(0)
	noClassContracts          = map[felt.Felt]struct{}{
		*new(felt.Felt).SetUint64(1): {},
	}

	ErrNoHistoryValue = errors.New("no history value found")
)

type State struct {
	txn db.Transaction

	contractTrie *trie2.Trie
	classTrie    *trie2.Trie

	// holds the contract objects which are being updated in the current state update.
	dirtyContracts map[felt.Felt]*StateContract
}

func New(txn db.Transaction) (*State, error) {
	state := &State{
		txn:            txn,
		dirtyContracts: make(map[felt.Felt]*StateContract),
	}

	//TODO(MaksymMalicki): handle for both db schemes, update when condition for
	// triedb scheme added
	if true {
		lastStateHash, err := state.GetLastStateHash()
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		if lastStateHash != nil {
			classRoot, contractRoot, err := state.GetStateHash(lastStateHash)
			if err != nil {
				return nil, err
			}

			state.classTrie, err = trie2.NewWithRootHash(trie2.NewClassTrieID(), ClassTrieHeight, crypto.Poseidon, txn, *classRoot)
			if err != nil {
				return nil, err
			}

			state.contractTrie, err = trie2.NewWithRootHash(trie2.NewContractTrieID(), ContractTrieHeight, crypto.Pedersen, txn, *contractRoot)
			if err != nil {
				return nil, err
			}

			return state, nil
		}
	}

	contractTrie, err := trie2.New(trie2.NewContractTrieID(), ContractTrieHeight, crypto.Pedersen, txn)
	if err != nil {
		return nil, err
	}

	classTrie, err := trie2.New(trie2.NewClassTrieID(), ClassTrieHeight, crypto.Poseidon, txn)
	if err != nil {
		return nil, err
	}

	state.contractTrie = contractTrie
	state.classTrie = classTrie

	return state, nil
}

// Returns the class hash of a contract.
func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := s.getContract(*addr)
	if err != nil {
		return nil, err
	}

	return contract.ClassHash, nil
}

// Returns the nonce of a contract.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := s.getContract(*addr)
	if err != nil {
		return nil, err
	}

	return contract.Nonce, nil
}

// Returns the storage value of a contract at a given storage key.
func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	contract, err := s.getContract(*addr)
	if err != nil {
		return nil, err
	}

	return contract.GetStorage(key, s.txn)
}

// Returns true if the contract was deployed at or before the given block number.
func (s *State) ContractDeployedAt(addr felt.Felt, blockNum uint64) (bool, error) {
	contract, err := s.getContract(addr)
	if err != nil {
		if errors.Is(err, ErrContractNotDeployed) {
			return false, nil
		}
		return false, err
	}

	return contract.DeployHeight <= blockNum, nil
}

// Returns the class of a contract.
func (s *State) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	classKey := classKey(classHash)

	var class core.DeclaredClass
	err := s.txn.Get(classKey, class.UnmarshalBinary)
	if err != nil {
		return nil, err
	}

	return &class, nil
}

// Returns the class trie.
func (s *State) ClassTrie() (*trie2.Trie, error) {
	return s.classTrie, nil
}

// Returns the contract trie.
func (s *State) ContractTrie() (*trie2.Trie, error) {
	return s.contractTrie, nil
}

// TODO: add tests for this
func (s *State) ContractStorageTrie(addr *felt.Felt) (*trie2.Trie, error) {
	contract, err := s.getContract(*addr)
	if err != nil {
		return nil, err
	}

	return contract.getTrie(s.txn)
}

// Applies a state update to a given state. If any error is encountered, state is not updated.
// After a state update is applied, the root of the state must match the given new root in the state update.
func (s *State) Update(blockNum uint64, update *core.StateUpdate, declaredClasses map[felt.Felt]core.Class) error {
	if err := s.verifyRoot(update.OldRoot); err != nil {
		return err
	}

	// Register the declared classes
	for hash, class := range declaredClasses {
		if err := s.putClass(&hash, class, blockNum); err != nil {
			return err
		}
	}

	if err := s.updateClassTrie(update.StateDiff.DeclaredV1Classes, declaredClasses); err != nil {
		return err
	}

	// Register deployed contracts
	for addr, classHash := range update.StateDiff.DeployedContracts {
		_, err := GetContract(&addr, s.txn)
		if err == nil {
			return ErrContractAlreadyDeployed
		}

		if !errors.Is(err, ErrContractNotDeployed) {
			return err
		}

		s.dirtyContracts[addr] = NewStateContract(&addr, classHash, &felt.Zero, blockNum)
	}

	if err := s.updateContracts(blockNum, update.StateDiff); err != nil {
		return err
	}

	newRoot, err := s.Commit(true, blockNum)
	if err != nil {
		return err
	}

	if !newRoot.Equal(update.NewRoot) {
		return fmt.Errorf("root mismatch: %s (expected) != %s (current)", update.NewRoot.String(), newRoot.String())
	}

	return s.StoreStateHash(newRoot)
}

// Reverts a state update to a given state at a given block number.
func (s *State) Revert(blockNum uint64, update *core.StateUpdate) error {
	// Ensure the current root is the same as the new root
	if err := s.verifyRoot(update.NewRoot); err != nil {
		return err
	}

	reverseDiff, err := s.GetReverseStateDiff(blockNum, update.StateDiff)
	if err != nil {
		return fmt.Errorf("get reverse state diff: %v", err)
	}

	if err := s.removeDeclaredClasses(blockNum, update.StateDiff.DeclaredV0Classes, update.StateDiff.DeclaredV1Classes); err != nil {
		return fmt.Errorf("remove declared classes: %v", err)
	}

	if err := s.deleteHistory(blockNum, update.StateDiff); err != nil {
		return fmt.Errorf("delete history: %v", err)
	}

	if err := s.updateContracts(blockNum, reverseDiff); err != nil {
		return fmt.Errorf("update contracts: %v", err)
	}

	for addr := range update.StateDiff.DeployedContracts {
		s.dirtyContracts[addr] = nil // mark as deleted
	}
	revertRoot, err := s.Commit(false, blockNum)
	if err != nil {
		return fmt.Errorf("commit: %v", err)
	}
	// Ensure the reverted root is the same as the old root
	if !revertRoot.Equal(update.OldRoot) {
		return fmt.Errorf("root mismatch: %s (expected) != %s (current)", update.OldRoot.String(), revertRoot.String())
	}

	//TODO(MaksymMalicki): handle for both db scheme
	if true {
		if blockNum == 0 {
			if err := s.txn.Delete(db.LastStateHash.Key()); err != nil {
				return fmt.Errorf("remove last state hash for block 0: %v", err)
			}
		} else {
			if err := s.StoreLastStateHash(update.OldRoot); err != nil {
				return fmt.Errorf("store last state hash: %v", err)
			}
		}

		if err := s.RemoveLastStateHash(update.NewRoot); err != nil {
			return fmt.Errorf("remove last state hash: %v", err)
		}
	}

	return nil
}

func (s *State) Commit(storeHistory bool, blockNum uint64) (*felt.Felt, error) {
	// Sort in descending order of the number of storage changes
	// so that we start with the heaviest update first
	keys := slices.SortedStableFunc(maps.Keys(s.dirtyContracts), s.compareContracts)

	// Commit contracts in parallel in a buffered transaction
	p := pool.New().WithMaxGoroutines(runtime.GOMAXPROCS(0)).WithErrors()
	bufTxn := db.NewBufferedTransaction(s.txn)
	comms := make([]*felt.Felt, len(keys))
	for i, addr := range keys {
		contract := s.dirtyContracts[addr]

		p.Go(func() error {
			// Contract is marked as deleted
			if contract == nil {
				if err := s.deleteContract(bufTxn, addr); err != nil {
					return err
				}
				comms[i] = &felt.Zero
				return nil
			}

			// Otherwise, commit the contract changes
			if err := contract.Commit(bufTxn, storeHistory, blockNum); err != nil {
				return err
			}
			comms[i] = contract.Commitment()
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return nil, err
	}

	if err := bufTxn.Flush(); err != nil {
		return nil, err
	}

	// Update the contract trie with the new commitments
	for i, addr := range keys {
		if err := s.contractTrie.Update(&addr, comms[i]); err != nil {
			return nil, err
		}

		// As noClassContracts are not in StateDiff.DeployedContracts we can only purge them if their storage no longer exists.
		// Updating contracts with reverse diff will eventually lead to the deletion of noClassContract's storage key from db. Thus,
		// we can use the lack of key's existence as reason for purging noClassContracts.
		for nAddr := range noClassContracts {
			if addr.Equal(&nAddr) {
				contract := s.dirtyContracts[addr]
				root, err := contract.GetStorageRoot(s.txn)
				if err != nil {
					return nil, err
				}

				if root.IsZero() {
					if err := s.contractTrie.Update(&nAddr, &felt.Zero); err != nil {
						return nil, err
					}

					if err := s.txn.Delete(contractKey(&nAddr)); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	classRoot, err := s.classTrie.Commit()
	if err != nil {
		return nil, err
	}

	contractRoot, err := s.contractTrie.Commit()
	if err != nil {
		return nil, err
	}

	return stateCommitment(&contractRoot, &classRoot), nil
}

// Retrieves the root hash of the state.
func (s *State) Root() (*felt.Felt, error) {
	contractRoot := s.contractTrie.Hash()
	classRoot := s.classTrie.Hash()
	return stateCommitment(&contractRoot, &classRoot), nil
}

func (s *State) GetReverseStateDiff(blockNum uint64, diff *core.StateDiff) (*core.StateDiff, error) {
	reverse := &core.StateDiff{
		StorageDiffs:    make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs)),
		Nonces:          make(map[felt.Felt]*felt.Felt, len(diff.Nonces)),
		ReplacedClasses: make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses)),
	}

	for addr, stDiffs := range diff.StorageDiffs {
		reverse.StorageDiffs[addr] = make(map[felt.Felt]*felt.Felt, len(stDiffs))
		for key := range stDiffs {
			value := &felt.Zero
			if blockNum > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNum-1)
				if err != nil {
					return nil, err
				}
				value = oldValue
			}
			reverse.StorageDiffs[addr][key] = value
		}
	}

	for addr := range diff.Nonces {
		oldNonce := &felt.Zero
		if blockNum > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNum-1)
			if err != nil {
				return nil, err
			}
		}
		reverse.Nonces[addr] = oldNonce
	}

	for addr := range diff.ReplacedClasses {
		oldCh := &felt.Zero
		if blockNum > 0 {
			var err error
			oldCh, err = s.ContractClassHashAt(&addr, blockNum-1)
			if err != nil {
				return nil, err
			}
		}
		reverse.ReplacedClasses[addr] = oldCh
	}

	return reverse, nil
}

// Returns the storage value of a contract at a given storage key at a given block number.
func (s *State) ContractStorageAt(addr, key *felt.Felt, blockNum uint64) (*felt.Felt, error) {
	prefix := db.ContractStorageHistory.Key(addr.Marshal(), key.Marshal())
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the nonce of a contract at a given block number.
func (s *State) ContractNonceAt(addr *felt.Felt, blockNum uint64) (*felt.Felt, error) {
	prefix := db.ContractNonceHistory.Key(addr.Marshal())
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the class hash of a contract at a given block number.
func (s *State) ContractClassHashAt(addr *felt.Felt, blockNum uint64) (*felt.Felt, error) {
	prefix := db.ContractClassHashHistory.Key(addr.Marshal())
	return s.getHistoricalValue(prefix, blockNum)
}

func (s *State) deleteHistory(blockNum uint64, diff *core.StateDiff) error {
	for addr := range diff.StorageDiffs {
		for key := range diff.StorageDiffs[addr] {
			if err := s.txn.Delete(contractHistoryStorageKey(&addr, &key, blockNum)); err != nil {
				return err
			}
		}
	}

	for addr := range diff.Nonces {
		if err := s.txn.Delete(contractHistoryNonceKey(&addr, blockNum)); err != nil {
			return err
		}
	}

	for addr := range diff.ReplacedClasses {
		if err := s.txn.Delete(contractHistoryClassHashKey(&addr, blockNum)); err != nil {
			return err
		}
	}

	for addr := range diff.DeployedContracts {
		if err := s.txn.Delete(contractHistoryNonceKey(&addr, blockNum)); err != nil {
			return err
		}
		if err := s.txn.Delete(contractHistoryClassHashKey(&addr, blockNum)); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) deleteContract(txn db.Transaction, addr felt.Felt) error {
	if err := txn.Delete(contractKey(&addr)); err != nil {
		return err
	}

	// Create a temporary contract with zero values so that we can delete the storage trie
	tempContract := NewStateContract(&addr, &felt.Zero, &felt.Zero, 0)
	err := tempContract.deleteStorageTrie(txn)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) getHistoricalValue(prefix []byte, blockNum uint64) (*felt.Felt, error) {
	val, err := s.valueAt(prefix, blockNum)
	if err != nil {
		if errors.Is(err, ErrNoHistoryValue) {
			return &felt.Zero, nil
		}
		return nil, err
	}
	return new(felt.Felt).SetBytes(val), nil
}

func (s *State) valueAt(prefix []byte, blockNum uint64) ([]byte, error) {
	it, err := s.txn.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	seekKey := binary.BigEndian.AppendUint64(prefix, blockNum)
	if !it.Seek(seekKey) {
		return nil, ErrNoHistoryValue
	}

	key := it.Key()
	keyBlockNum := binary.BigEndian.Uint64(key[len(prefix):])
	if keyBlockNum == blockNum {
		// Found the value
		return it.Value()
	}

	// Otherwise, we move the iterator backwards
	if !it.Prev() {
		// Moving iterator backwards is invalid, this means we were already at the first key
		// No values will be found beyond the first key
		return nil, ErrNoHistoryValue
	}

	// At this point we already know that the block number is less than the target block number
	// So we just return the old value
	return it.Value()
}

// Retrieves a given contract from the state.
func (s *State) getContract(addr felt.Felt) (*StateContract, error) {
	contract, ok := s.dirtyContracts[addr]
	if ok {
		return contract, nil
	}

	contract, err := GetContract(&addr, s.txn)
	if err != nil {
		return nil, err
	}

	s.dirtyContracts[addr] = contract
	return contract, nil
}

func (s *State) putClass(classHash *felt.Felt, class core.Class, declaredAt uint64) error {
	classKey := classKey(classHash)

	err := s.txn.Get(classKey, func(val []byte) error { return nil }) // check if class already exists
	if errors.Is(err, db.ErrKeyNotFound) {
		dc := core.DeclaredClass{
			At:    declaredAt,
			Class: class,
		}

		encoded, err := dc.MarshalBinary()
		if err != nil {
			return err
		}

		return s.txn.Set(classKey, encoded)
	}
	return err
}

func (s *State) updateClassTrie(declaredClasses map[felt.Felt]*felt.Felt, classDefs map[felt.Felt]core.Class) error {
	for classHash, compiledClassHash := range declaredClasses {
		if _, found := classDefs[classHash]; !found {
			continue
		}

		leafVal := crypto.Poseidon(leafVersion0, compiledClassHash)
		if err := s.classTrie.Update(&classHash, leafVal); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) updateContracts(blockNum uint64, diff *core.StateDiff) error {
	if err := s.updateContractClasses(diff.ReplacedClasses); err != nil {
		return err
	}

	if err := s.updateContractNonces(diff.Nonces); err != nil {
		return err
	}

	if err := s.updateContractStorages(blockNum, diff.StorageDiffs); err != nil {
		return err
	}
	return nil
}

func (s *State) updateContractClasses(classes map[felt.Felt]*felt.Felt) error {
	for addr, classHash := range classes {
		contract, err := s.getContract(addr)
		if err != nil {
			return err
		}

		contract.ClassHash = classHash
	}

	return nil
}

func (s *State) updateContractNonces(nonces map[felt.Felt]*felt.Felt) error {
	for addr, nonce := range nonces {
		contract, err := s.getContract(addr)
		if err != nil {
			return err
		}

		contract.Nonce = nonce
	}

	return nil
}

func (s *State) updateContractStorages(blockNum uint64, storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt) error {
	for addr, diff := range storageDiffs {
		contract, err := s.getContract(addr)
		if err != nil {
			if _, ok := noClassContracts[addr]; ok && errors.Is(err, ErrContractNotDeployed) {
				contract = NewStateContract(&addr, noClassContractsClassHash, &felt.Zero, blockNum)
				s.dirtyContracts[addr] = contract
			} else {
				return err
			}
		}

		contract.dirtyStorage = diff
	}
	return nil
}

func (s *State) verifyRoot(root *felt.Felt) error {
	curRoot, err := s.Root()
	if err != nil {
		return err
	}

	if !root.Equal(curRoot) {
		return fmt.Errorf("root mismatch: %s (expected) != %s (current)", root.String(), curRoot.String())
	}

	return nil
}

func (s *State) removeDeclaredClasses(blockNum uint64, v0Classes []*felt.Felt, v1Classes map[felt.Felt]*felt.Felt) error {
	// Gather the class hashes
	totalCapacity := len(v0Classes) + len(v1Classes)
	classHashes := make([]*felt.Felt, 0, totalCapacity)
	classHashes = append(classHashes, v0Classes...)
	for classHash := range v1Classes {
		classHashes = append(classHashes, classHash.Clone())
	}

	for _, cHash := range classHashes {
		declaredClass, err := s.Class(cHash)
		if err != nil {
			return err
		}

		// We only want to remove classes that were declared at the given block number
		if declaredClass.At != blockNum {
			continue
		}

		if err := s.txn.Delete(classKey(cHash)); err != nil {
			return err
		}

		// For cairo1 classes, we update the class trie
		if declaredClass.Class.Version() == 1 {
			if err := s.classTrie.Update(cHash, &felt.Zero); err != nil {
				return err
			}
		}
	}

	return nil
}

// Calculate the commitment of the state
func stateCommitment(contractRoot, classRoot *felt.Felt) *felt.Felt {
	if classRoot.IsZero() {
		return contractRoot
	}

	return crypto.PoseidonArray(stateVersion0, contractRoot, classRoot)
}

func classKey(classHash *felt.Felt) []byte {
	return db.Class.Key(classHash.Marshal())
}

func (s *State) compareContracts(a, b felt.Felt) int {
	contractA, contractB := s.dirtyContracts[a], s.dirtyContracts[b]

	switch {
	case contractA == nil && contractB == nil:
		return 0
	case contractA == nil:
		return 1 // Move nil contracts to end
	case contractB == nil:
		return -1 // Keep non-nil contracts first
	}

	return len(contractB.dirtyStorage) - len(contractA.dirtyStorage)
}

func (s *State) GetStateHash(stateHash *felt.Felt) (*felt.Felt, *felt.Felt, error) {
	key := db.StateHashToClassAndContract.Key(stateHash.Marshal())
	var classRoot, contractRoot felt.Felt
	err := s.txn.Get(key, func(val []byte) error {
		if len(val) != 2*felt.Bytes {
			return fmt.Errorf("invalid state hash value length")
		}
		classRoot.SetBytes(val[:felt.Bytes])
		contractRoot.SetBytes(val[felt.Bytes:])
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &classRoot, &contractRoot, nil
}

func (s *State) StoreStateHash(stateHash *felt.Felt) error {
	contractRoot := s.contractTrie.Hash()
	classRoot := s.classTrie.Hash()
	key := db.StateHashToClassAndContract.Key(stateHash.Marshal())
	value := append(classRoot.Marshal(), contractRoot.Marshal()...)
	if err := s.txn.Set(key, value); err != nil {
		return err
	}
	return s.StoreLastStateHash(stateHash)
}

func (s *State) StoreLastStateHash(stateHash *felt.Felt) error {
	key := db.LastStateHash.Key()
	return s.txn.Set(key, stateHash.Marshal())
}

func (s *State) RemoveLastStateHash(stateHash *felt.Felt) error {
	key := db.StateHashToClassAndContract.Key(stateHash.Marshal())
	if err := s.txn.Delete(key); err != nil {
		return fmt.Errorf("delete state hash: %v", err)
	}
	return nil
}

func (s *State) GetLastStateHash() (*felt.Felt, error) {
	key := db.LastStateHash.Key()
	var stateHash felt.Felt
	err := s.txn.Get(key, func(val []byte) error {
		stateHash.SetBytes(val)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &stateHash, nil
}
