package state

import (
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
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
	stateVersion0             = felt.NewFromBytes[felt.Felt]([]byte(`STARKNET_STATE_V0`))
	leafVersion0              = felt.NewFromBytes[felt.Felt]([]byte(`CONTRACT_CLASS_LEAF_V0`))
	noClassContractsClassHash = felt.Zero
	noClassContracts          = map[felt.Felt]struct{}{
		felt.FromUint64[felt.Felt](systemContract1Addr): {},
		felt.FromUint64[felt.Felt](systemContract2Addr): {},
	}
)

var (
	_ core.StateReader = &StateReader{}
	_ core.State       = &State{}
)

// State extends StateReader with mutation operations.
type State struct {
	StateReader

	stateObjects map[felt.Felt]*stateObject
	batch        db.Batch
}

// New creates a writable state at the given root. The caller must provide a non-nil batch.
// Should be used for operations, where state mutations are required. Read operations are
// also supported.
func New(stateRoot *felt.Felt, db *StateDB, batch db.Batch) (*State, error) {
	if batch == nil {
		return nil, errors.New("cannot create state, nil Batch received")
	}
	reader, err := NewStateReader(stateRoot, db)
	if err != nil {
		return nil, err
	}

	return &State{
		StateReader:  *reader,
		stateObjects: make(map[felt.Felt]*stateObject),
		batch:        batch,
	}, nil
}

// Applies a state update to a given state. If any error is encountered, state is not updated.
// After a state update is applied, the root of the state must match the given new root in the state update.
func (s *State) Update(
	header *core.Header,
	update *core.StateUpdate,
	declaredClasses map[felt.Felt]core.ClassDefinition,
	skipVerifyNewRoot bool,
) error {
	blockNum := header.Number
	protocolVersion := header.ProtocolVersion
	if err := s.verifyComm(update.OldRoot, protocolVersion); err != nil {
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
	err := s.updateClassTrie(
		update.StateDiff.DeclaredV1Classes,
		declaredClasses,
		update.StateDiff.MigratedClasses,
	)
	if err != nil {
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

	newComm, stateUpdate, err := s.commit(protocolVersion)
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

	return s.flush(blockNum, &stateUpdate, dirtyClasses, true)
}

// Revert a given state update. The block number is the block number of the state update.
//
//nolint:gocyclo
func (s *State) Revert(header *core.Header, update *core.StateUpdate) error {
	blockNum := header.Number
	protocolVersion := header.ProtocolVersion
	// Ensure the current root is the same as the new root
	if err := s.verifyComm(update.NewRoot, protocolVersion); err != nil {
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

	// Revert migrated class metadata: restore V1 casm hash in trie
	for classHash := range update.StateDiff.MigratedClasses {
		metadata, err := core.GetClassCasmHashMetadata(s.db.disk, &classHash)
		if err != nil {
			return fmt.Errorf("get casm metadata for class %s: %w", classHash.String(), err)
		}
		if err := metadata.Unmigrate(); err != nil {
			return fmt.Errorf("unmigrate class %s: %w", classHash.String(), err)
		}
		v1CasmHash := metadata.CasmHash() // returns V1 after Unmigrate
		leafVal := crypto.Poseidon(leafVersion0, (*felt.Felt)(&v1CasmHash))
		if err := s.classTrie.Update((*felt.Felt)(&classHash), &leafVal); err != nil {
			return fmt.Errorf("revert class %s in trie: %w", classHash.String(), err)
		}
	}

	if err := s.updateContracts(blockNum, &reverseDiff); err != nil {
		return fmt.Errorf("update contracts: %v", err)
	}

	for addr := range update.StateDiff.DeployedContracts {
		s.stateObjects[addr] = nil // mark for deletion
	}

	newComm, stateUpdate, err := s.commit(protocolVersion)
	if err != nil {
		return err
	}

	// Check if the new commitment matches the old one
	if !newComm.Equal(update.OldRoot) {
		return fmt.Errorf("state commitment mismatch: %v (expected) != %v (actual)", update.OldRoot, &newComm)
	}

	if err := s.flush(blockNum, &stateUpdate, dirtyClasses, false); err != nil {
		return err
	}
	return s.deleteHistory(blockNum, update.StateDiff)
}

//nolint:gocyclo
func (s *State) commit(protocolVersion string) (felt.Felt, stateUpdate, error) {
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
		if obj == nil {
			// Object is marked as delete
			continue
		}

		p.Go(func() error {
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
				root, err := obj.getStorageRoot()
				if err != nil {
					return felt.Zero, emptyStateUpdate, err
				}

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

	newComm := stateCommitment(&contractRoot, &classRoot, protocolVersion)

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
	err := s.db.triedb.Update(
		(*felt.StateRootHash)(&update.curComm),
		(*felt.StateRootHash)(&update.prevComm),
		blockNum,
		update.classNodes,
		update.contractNodes,
		s.batch,
	)
	if err != nil {
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
	migratedClasses map[felt.SierraClassHash]felt.CasmClassHash,
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

	for classHash, casmHash := range migratedClasses {
		leafVal := crypto.Poseidon(leafVersion0, (*felt.Felt)(&casmHash))
		if err := s.classTrie.Update((*felt.Felt)(&classHash), &leafVal); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) verifyComm(comm *felt.Felt, protocolVersion string) error {
	curComm, err := s.Commitment(protocolVersion)
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
			if _, ok := noClassContracts[addr]; ok && errors.Is(err, db.ErrKeyNotFound) {
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
