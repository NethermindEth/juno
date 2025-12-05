package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/sourcegraph/conc/pool"
)

const (
	systemContract1Addr = 1
	systemContract2Addr = 2
)

var (
	stateVersion0             = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion0              = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
	noClassContractsClassHash = felt.Zero
	noClassContracts          = map[felt.Felt]struct{}{
		felt.FromUint64[felt.Felt](systemContract1Addr): {},
		felt.FromUint64[felt.Felt](systemContract2Addr): {},
	}
)

var _ core.State = &State{}

type State struct {
	initRoot     felt.Felt
	db           *StateDB
	contractTrie *trie2.Trie
	classTrie    *trie2.Trie

	stateObjects map[felt.Felt]*stateObject

	batch db.Batch
}

func New(stateRoot *felt.Felt, db *StateDB, batch db.Batch) (*State, error) {
	contractTrie, err := db.ContractTrie(stateRoot)
	if err != nil {
		return nil, err
	}

	classTrie, err := db.ClassTrie(stateRoot)
	if err != nil {
		return nil, err
	}

	return &State{
		initRoot:     *stateRoot,
		db:           db,
		contractTrie: contractTrie,
		classTrie:    classTrie,
		stateObjects: make(map[felt.Felt]*stateObject),
		batch:        batch,
	}, nil
}

func (s *State) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	if classHash := s.db.stateCache.getReplacedClass(&s.initRoot, addr); classHash != nil {
		return *classHash, nil
	}

	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		return felt.Felt{}, err
	}
	return contract.ClassHash, nil
}

func (s *State) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	if nonce := s.db.stateCache.getNonce(&s.initRoot, addr); nonce != nil {
		return *nonce, nil
	}

	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		return felt.Felt{}, err
	}
	return contract.Nonce, nil
}

func (s *State) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	if storage := s.db.stateCache.getStorageDiff(&s.initRoot, addr, key); storage != nil {
		return *storage, nil
	}

	obj, err := s.getStateObject(addr)
	if err != nil {
		return felt.Felt{}, err
	}

	ret, err := obj.getStorage(key)
	if err != nil {
		return felt.Felt{}, err
	}

	return ret, nil
}

func (s *State) ContractDeployedAt(addr *felt.Felt, blockNum uint64) (bool, error) {
	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		if errors.Is(err, ErrContractNotDeployed) {
			return false, nil
		}
		return false, err
	}

	return contract.DeployedHeight <= blockNum, nil
}

func (s *State) Class(classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
	return GetClass(s.db.disk, classHash)
}

func (s *State) ClassTrie() (core.Trie, error) {
	return s.classTrie, nil
}

func (s *State) ContractTrie() (core.Trie, error) {
	return s.contractTrie, nil
}

func (s *State) ContractStorageTrie(addr *felt.Felt) (core.Trie, error) {
	return s.db.ContractStorageTrie(&s.initRoot, addr)
}

func (s *State) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHash(), nil
}

func (s *State) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashV2(), nil
}

// Returns the state commitment
func (s *State) Commitment() (felt.Felt, error) {
	contractRoot, err := s.contractTrie.Hash()
	if err != nil {
		return felt.Felt{}, err
	}
	classRoot, err := s.classTrie.Hash()
	if err != nil {
		return felt.Felt{}, err
	}
	return stateCommitment(&contractRoot, &classRoot), nil
}

// Applies a state update to a given state. If any error is encountered, state is not updated.
// After a state update is applied, the root of the state must match the given new root in the state update.
// TODO(weiihann): deal with flush atomicity
func (s *State) Update(
	blockNum uint64,
	update *core.StateUpdate,
	declaredClasses map[felt.Felt]core.ClassDefinition,
	skipVerifyNewRoot bool,
) error {
	if err := s.verifyComm(update.OldRoot); err != nil {
		return err
	}

	dirtyClasses := make(map[felt.Felt]core.ClassDefinition)

	// Register the declared classes
	for hash, class := range declaredClasses {
		hasClass, err := HasClass(s.db.disk, &hash)
		if err != nil {
			return err
		}

		if !hasClass {
			dirtyClasses[hash] = class
		}
	}

	// Update the class trie
	if err := s.updateClassTrie(update.StateDiff.DeclaredV1Classes, declaredClasses); err != nil {
		return err
	}

	// Register deployed contracts
	for addr, classHash := range update.StateDiff.DeployedContracts {
		exists, err := HasContract(s.db.disk, &addr)
		if err != nil {
			return err
		}

		if exists {
			return ErrContractAlreadyDeployed
		}

		contract := newContractDeployed(*classHash, blockNum)
		newObj := newStateObject(s, &addr, &contract)
		s.stateObjects[addr] = &newObj
	}

	// Update the contract fields
	if err := s.updateContracts(blockNum, update.StateDiff); err != nil {
		return err
	}

	newComm, stateUpdate, err := s.commit()
	if err != nil {
		return err
	}

	// Check if the new commitment matches the one in state diff
	// The following check isn't relevant for the centralised Juno sequencer
	if !skipVerifyNewRoot {
		if !newComm.Equal(update.NewRoot) {
			return fmt.Errorf(
				"state commitment mismatch: %v (expected) != %v (actual)",
				update.NewRoot,
				&newComm,
			)
		}
	}

	s.db.stateCache.PushLayer(&newComm, &stateUpdate.prevComm, &diffCache{
		storageDiffs:      update.StateDiff.StorageDiffs,
		nonces:            update.StateDiff.Nonces,
		deployedContracts: update.StateDiff.ReplacedClasses,
	})

	if s.batch != nil {
		return s.flush(blockNum, &stateUpdate, dirtyClasses, true)
	}

	return nil
}

// Revert a given state update. The block number is the block number of the state update.
//
//nolint:gocyclo
func (s *State) Revert(blockNum uint64, update *core.StateUpdate) error {
	// Ensure the current root is the same as the new root
	if err := s.verifyComm(update.NewRoot); err != nil {
		return err
	}

	reverseDiff, err := s.GetReverseStateDiff(blockNum, update.StateDiff)
	if err != nil {
		return fmt.Errorf("get reverse state diff: %v", err)
	}

	// Gather the classes to be reverted
	v0Classes := update.StateDiff.DeclaredV0Classes
	v1Classes := update.StateDiff.DeclaredV1Classes
	classHashes := make([]*felt.Felt, 0, len(v0Classes)+len(v1Classes))
	classHashes = append(classHashes, v0Classes...)
	for hash := range v1Classes {
		classHashes = append(classHashes, &hash)
	}

	// Revert the classes
	dirtyClasses := make(map[felt.Felt]core.ClassDefinition)
	for _, hash := range classHashes {
		dc, err := s.Class(hash)
		if err != nil {
			return err
		}

		if dc.At != blockNum {
			continue
		}

		if _, ok := dc.Class.(*core.SierraClass); ok {
			if err := s.classTrie.Update(hash, &felt.Zero); err != nil {
				return err
			}
		}

		dirtyClasses[*hash] = nil // mark for deletion
	}

	if err := s.updateContracts(blockNum, &reverseDiff); err != nil {
		return fmt.Errorf("update contracts: %v", err)
	}

	for addr := range update.StateDiff.DeployedContracts {
		s.stateObjects[addr] = nil // mark for deletion
	}

	newComm, stateUpdate, err := s.commit()
	if err != nil {
		return err
	}

	// Check if the new commitment matches the old one
	if !newComm.Equal(update.OldRoot) {
		return fmt.Errorf("state commitment mismatch: %v (expected) != %v (actual)", update.OldRoot, &newComm)
	}
	if s.batch != nil {
		if err := s.flush(blockNum, &stateUpdate, dirtyClasses, false); err != nil {
			return err
		}
		if err := s.deleteHistory(blockNum, update.StateDiff); err != nil {
			return err
		}
	}

	if err := s.db.stateCache.PopLayer(update.NewRoot, update.OldRoot); err != nil {
		return err
	}

	return nil
}

func (s *State) GetReverseStateDiff(blockNum uint64, diff *core.StateDiff) (core.StateDiff, error) {
	reverse := core.StateDiff{
		StorageDiffs:    make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs)),
		Nonces:          make(map[felt.Felt]*felt.Felt, len(diff.Nonces)),
		ReplacedClasses: make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses)),
	}

	for addr, stDiffs := range diff.StorageDiffs {
		reverse.StorageDiffs[addr] = make(map[felt.Felt]*felt.Felt, len(stDiffs))
		for key := range stDiffs {
			value := felt.Zero
			if blockNum > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNum-1)
				if err != nil {
					return core.StateDiff{}, err
				}
				value = oldValue
			}
			reverse.StorageDiffs[addr][key] = &value
		}
	}

	for addr := range diff.Nonces {
		oldNonce := felt.Zero
		if blockNum > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNum-1)
			if err != nil {
				return core.StateDiff{}, err
			}
		}
		reverse.Nonces[addr] = &oldNonce
	}

	for addr := range diff.ReplacedClasses {
		oldCh := felt.Zero
		if blockNum > 0 {
			var err error
			oldCh, err = s.ContractClassHashAt(&addr, blockNum-1)
			if err != nil {
				return core.StateDiff{}, err
			}
		}
		reverse.ReplacedClasses[addr] = &oldCh
	}

	return reverse, nil
}

//nolint:gocyclo
func (s *State) commit() (felt.Felt, stateUpdate, error) {
	// Sort in descending order of the number of storage changes
	// so that we start with the heaviest update first
	keys := slices.SortedStableFunc(maps.Keys(s.stateObjects), s.compareContracts)

	var (
		mergeLock           sync.Mutex
		mergedContractNodes = trienode.NewMergeNodeSet(nil)
		comms               = make([]felt.Felt, len(keys))
		p                   = pool.New().WithMaxGoroutines(runtime.GOMAXPROCS(0)).WithErrors()
	)

	merge := func(set *trienode.NodeSet) error {
		if set == nil {
			return nil
		}

		mergeLock.Lock()
		defer mergeLock.Unlock()

		return mergedContractNodes.Merge(set)
	}

	for i, addr := range keys {
		obj := s.stateObjects[addr]

		p.Go(func() error {
			// Object is marked as delete
			if obj == nil {
				return nil
			}

			nodes, err := obj.commit()
			if err != nil {
				return err
			}

			if err := merge(nodes); err != nil {
				return err
			}

			comms[i] = obj.commitment()
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return felt.Zero, emptyStateUpdate, err
	}

	for i, addr := range keys {
		if err := s.contractTrie.Update(&addr, &comms[i]); err != nil {
			return felt.Zero, emptyStateUpdate, err
		}

		// As noClassContracts are not in StateDiff.DeployedContracts we can only purge them if their storage no longer exists.
		// Updating contracts with reverse diff will eventually lead to the deletion of noClassContract's storage key from db. Thus,
		// we can use the lack of key's existence as reason for purging noClassContracts.
		for nAddr := range noClassContracts {
			if addr.Equal(&nAddr) {
				obj := s.stateObjects[addr]
				root := obj.getStorageRoot()

				if root.IsZero() {
					if err := s.contractTrie.Update(&nAddr, &felt.Zero); err != nil {
						return felt.Zero, emptyStateUpdate, err
					}

					s.stateObjects[nAddr] = nil // mark for deletion
				}
			}
		}
	}

	var (
		classRoot, contractRoot   felt.Felt
		classNodes, contractNodes *trienode.NodeSet
	)

	p = pool.New().WithMaxGoroutines(runtime.GOMAXPROCS(0)).WithErrors()
	p.Go(func() error {
		classRoot, classNodes = s.classTrie.Commit()
		return nil
	})
	p.Go(func() error {
		contractRoot, contractNodes = s.contractTrie.Commit()
		return nil
	})

	if err := p.Wait(); err != nil {
		return felt.Zero, emptyStateUpdate, err
	}

	if err := merge(contractNodes); err != nil {
		return felt.Zero, emptyStateUpdate, err
	}

	newComm := stateCommitment(&contractRoot, &classRoot)

	su := stateUpdate{
		prevComm:      s.initRoot,
		curComm:       newComm,
		contractNodes: mergedContractNodes,
	}

	if classNodes != nil {
		su.classNodes = trienode.NewMergeNodeSet(classNodes)
	}

	return newComm, su, nil
}

//nolint:gocyclo
func (s *State) flush(
	blockNum uint64,
	update *stateUpdate,
	classes map[felt.Felt]core.ClassDefinition,
	storeHistory bool,
) error {
	if err := s.db.triedb.Update(
		(*felt.StateRootHash)(&update.curComm),
		(*felt.StateRootHash)(&update.prevComm),
		blockNum,
		update.classNodes,
		update.contractNodes,
		s.batch,
	); err != nil {
		return err
	}

	for addr, obj := range s.stateObjects {
		if obj == nil { // marked as deleted
			if err := DeleteContract(s.batch, &addr); err != nil {
				return err
			}

			// TODO(weiihann): handle hash-based, and there should be better ways of doing this
			err := trieutils.DeleteStorageNodesByPath(s.batch, (*felt.Address)(&addr))
			if err != nil {
				return err
			}
		} else { // updated
			if err := WriteContract(s.batch, &addr, obj.contract); err != nil {
				return err
			}

			if storeHistory {
				for key, val := range obj.dirtyStorage {
					if err := WriteStorageHistory(s.batch, &addr, &key, blockNum, val); err != nil {
						return err
					}
				}

				if err := WriteNonceHistory(s.batch, &addr, blockNum, &obj.contract.Nonce); err != nil {
					return err
				}

				if err := WriteClassHashHistory(s.batch, &addr, blockNum, &obj.contract.ClassHash); err != nil {
					return err
				}
			}
		}
	}

	for classHash, class := range classes {
		if class == nil { // mark as deleted
			if err := DeleteClass(s.batch, &classHash); err != nil {
				return err
			}
		} else {
			if err := WriteClass(s.batch, &classHash, class, blockNum); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *State) updateClassTrie(
	declaredClasses map[felt.Felt]*felt.Felt,
	classDefs map[felt.Felt]core.ClassDefinition,
) error {
	for classHash, compiledClassHash := range declaredClasses {
		if _, found := classDefs[classHash]; !found {
			continue
		}

		leafVal := crypto.Poseidon(leafVersion0, compiledClassHash)
		if err := s.classTrie.Update(&classHash, &leafVal); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) verifyComm(comm *felt.Felt) error {
	curComm, err := s.Commitment()
	if err != nil {
		return err
	}

	if !curComm.Equal(comm) {
		return fmt.Errorf("state commitment mismatch: %v (expected) != %v (actual)", comm, &curComm)
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

	if err := s.updateContractStorage(blockNum, diff.StorageDiffs); err != nil {
		return err
	}

	return nil
}

func (s *State) updateContractClasses(classes map[felt.Felt]*felt.Felt) error {
	for addr, classHash := range classes {
		obj, err := s.getStateObject(&addr)
		if err != nil {
			return err
		}

		obj.setClassHash(classHash)
	}

	return nil
}

func (s *State) updateContractNonces(nonces map[felt.Felt]*felt.Felt) error {
	for addr, nonce := range nonces {
		obj, err := s.getStateObject(&addr)
		if err != nil {
			return err
		}

		obj.setNonce(nonce)
	}

	return nil
}

func (s *State) updateContractStorage(blockNum uint64, storage map[felt.Felt]map[felt.Felt]*felt.Felt) error {
	for addr, storage := range storage {
		obj, err := s.getStateObject(&addr)
		if err != nil {
			if _, ok := noClassContracts[addr]; ok && errors.Is(err, ErrContractNotDeployed) {
				contract := newContractDeployed(noClassContractsClassHash, blockNum)
				newObj := newStateObject(s, &addr, &contract)
				obj = &newObj
				s.stateObjects[addr] = obj
			} else {
				return err
			}
		}

		obj.dirtyStorage = storage
	}

	return nil
}

func (s *State) getStateObject(addr *felt.Felt) (*stateObject, error) {
	objPointer, ok := s.stateObjects[*addr]
	if ok {
		return objPointer, nil
	}

	obj, err := GetStateObject(s.db.disk, s, addr)
	if err != nil {
		return nil, err
	}

	s.stateObjects[*addr] = obj
	return obj, nil
}

// Returns the storage value of a contract at a given storage key at a given block number.
func (s *State) ContractStorageAt(addr, key *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractStorageHistoryKey(addr, key)
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the nonce of a contract at a given block number.
func (s *State) ContractNonceAt(addr *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractNonceHistoryKey(addr)
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the class hash of a contract at a given block number.
func (s *State) ContractClassHashAt(addr *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractClassHashHistoryKey(addr)
	return s.getHistoricalValue(prefix, blockNum)
}

func (s *State) CompiledClassHashAt(
	classHash *felt.SierraClassHash,
	blockNumber uint64,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashAt(blockNumber)
}

func (s *State) getHistoricalValue(prefix []byte, blockNum uint64) (felt.Felt, error) {
	var ret felt.Felt

	err := s.valueAt(prefix, blockNum, func(val []byte) error {
		ret.SetBytes(val)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNoHistoryValue) {
			return felt.Zero, nil
		}
		return felt.Zero, err
	}

	return ret, nil
}

// Returns the value at the given block number.
// First, it attempts to seek for the key at the given block number.
// If the key exists, it means that the key was modified at the given block number.
// Otherwise, it moves the iterator one step back.
// If a key-value pair exists, we have found the value.
func (s *State) valueAt(prefix []byte, blockNum uint64, cb func(val []byte) error) error {
	it, err := s.db.disk.NewIterator(prefix, true)
	if err != nil {
		return err
	}
	defer it.Close()

	seekKey := binary.BigEndian.AppendUint64(prefix, blockNum)
	if !it.Seek(seekKey) {
		return ErrNoHistoryValue
	}

	key := it.Key()
	keyBlockNum := binary.BigEndian.Uint64(key[len(prefix):])
	if keyBlockNum == blockNum {
		// Found the value
		val, err := it.Value()
		if err != nil {
			return err
		}

		return cb(val)
	}

	// Otherwise, move the iterator backwards
	if !it.Prev() {
		// Moving iterator backwards is invalid, this means we were already at the first key
		// No values will be found beyond the first key
		return ErrNoHistoryValue
	}

	val, err := it.Value()
	if err != nil {
		return err
	}

	return cb(val)
}

func (s *State) deleteHistory(blockNum uint64, diff *core.StateDiff) error {
	for addr, storage := range diff.StorageDiffs {
		for key := range storage {
			if err := DeleteStorageHistory(s.batch, &addr, &key, blockNum); err != nil {
				return err
			}
		}
	}

	for addr := range diff.Nonces {
		if err := DeleteNonceHistory(s.batch, &addr, blockNum); err != nil {
			return err
		}
	}

	for addr := range diff.ReplacedClasses {
		if err := DeleteClassHashHistory(s.batch, &addr, blockNum); err != nil {
			return err
		}
	}

	for addr := range diff.DeployedContracts {
		if err := DeleteNonceHistory(s.batch, &addr, blockNum); err != nil {
			return err
		}

		if err := DeleteClassHashHistory(s.batch, &addr, blockNum); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) compareContracts(a, b felt.Felt) int {
	contractA, contractB := s.stateObjects[a], s.stateObjects[b]

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

// Calculate the commitment of the state
func stateCommitment(contractRoot, classRoot *felt.Felt) felt.Felt {
	if classRoot.IsZero() {
		return *contractRoot
	}

	return crypto.PoseidonArray(stateVersion0, contractRoot, classRoot)
}
