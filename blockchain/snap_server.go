package blockchain

import (
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/pkg/errors"
)

type TrieRootInfo struct {
	StorageRoot *felt.Felt
	ClassRoot   *felt.Felt
}

type ClassRangeResult struct {
	Paths       []*felt.Felt
	ClassHashes []*felt.Felt
	Classes     []core.Class

	Proofs []*trie.ProofNode
}

type AddressRangeResult struct {
	Paths  []*felt.Felt
	Hashes []*felt.Felt
	Leaves []*AddressRangeLeaf

	Proofs []*trie.ProofNode
}

type AddressRangeLeaf struct {
	ContractStorageRoot *felt.Felt
	ClassHash           *felt.Felt
	Nonce               *felt.Felt
}

type StorageRangeRequest struct {
	Path      *felt.Felt
	Hash      *felt.Felt
	StartAddr *felt.Felt
	LimitAddr *felt.Felt
}

type StorageRangeResult struct {
	Paths  []*felt.Felt
	Values []*felt.Felt

	Proofs []*trie.ProofNode
}

type SnapServer interface {
	GetTrieRootAt(blockHash *felt.Felt) (*TrieRootInfo, error)
	GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*ClassRangeResult, error)
	GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*AddressRangeResult, error)
	GetContractRange(storageTrieRootHash *felt.Felt, requests []*StorageRangeRequest) ([]*StorageRangeResult, error)
}

var _ SnapServer = &Blockchain{}

const maxNodePerRequest = 1024 * 1024 // I just want it to process faster

func (b *Blockchain) FindSnapshotMatching(filter func(record *snapshotRecord) bool) (*snapshotRecord, error) {
	var snapshot *snapshotRecord
	for _, record := range b.snapshots {
		if filter(record) {
			snapshot = record
			break
		}
	}

	if snapshot == nil {
		return nil, ErrMissingSnapshot
	}

	return snapshot, nil
}

func (b *Blockchain) GetTrieRootAt(blockHash *felt.Felt) (*TrieRootInfo, error) {
	snapshot, err := b.FindSnapshotMatching(func(record *snapshotRecord) bool {
		return record.blockHash.Equal(blockHash)
	})

	if err != nil {
		return nil, err
	}

	return &TrieRootInfo{
		StorageRoot: snapshot.stateRoot,
		ClassRoot:   snapshot.classRoot,
	}, nil
}

func iterateWithLimit(
	srcTrie *trie.Trie,
	startAddr *felt.Felt,
	limitAddr *felt.Felt,
	maxNode int,
	consumer func(key, value *felt.Felt) error,
	hashFunc trie.HashFunc) ([]*trie.ProofNode, error) {
	pathes := make([]*felt.Felt, 0)
	hashes := make([]*felt.Felt, 0)

	// TODO: Verify class trie
	var startPath *felt.Felt
	var endPath *felt.Felt
	count := 0
	err := srcTrie.Iterate(startAddr, func(key *felt.Felt, value *felt.Felt) (bool, error) {
		// Need at least one.
		if limitAddr != nil && key.Cmp(limitAddr) > 1 && count > 0 {
			return false, nil
		}

		if startPath == nil {
			startPath = key
		}

		pathes = append(pathes, key)
		hashes = append(hashes, value)

		err := consumer(key, value)
		if err != nil {
			return false, err
		}

		endPath = key
		count++
		if count >= maxNode {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	if count == 1 {
		return srcTrie.ProofTo(startPath)
	} else if count > 1 {
		leftProof, err := srcTrie.ProofTo(startPath)
		if err != nil {
			return nil, err
		}
		rightProof, err := srcTrie.ProofTo(endPath)
		if err != nil {
			return nil, err
		}

		skippedcount := 0
		proofs := leftProof
		for _, proof := range rightProof {
			alreadyExist := false
			for _, node := range proofs {
				if node.Key.Equal(proof.Key) {
					alreadyExist = true
					break
				}
			}
			if alreadyExist {
				skippedcount += 1
				continue
			}

			proofs = append(proofs, proof)
		}

		root, err := srcTrie.Root()
		if err != nil {
			return nil, err
		}

		_, err = trie.VerifyTrie(root, pathes, hashes, proofs, hashFunc)
		if err != nil {
			return nil, errors.Wrap(err, "error double checking root")
		}

		return proofs, nil
	}

	return nil, nil
}

func (b *Blockchain) GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*ClassRangeResult, error) {
	snapshot, err := b.FindSnapshotMatching(func(record *snapshotRecord) bool {
		return record.classRoot.Equal(classTrieRootHash)
	})
	if err != nil {
		return nil, err
	}

	s := core.NewState(snapshot.txn)

	// TODO: Verify class trie
	ctrie, classCloser, err := s.ClassTrie()
	if err != nil {
		return nil, err
	}
	defer classCloser()

	response := &ClassRangeResult{
		Paths:       nil,
		ClassHashes: nil,
		Classes:     nil,
		Proofs:      nil,
	}

	response.Proofs, err = iterateWithLimit(ctrie, startAddr, limitAddr, maxNodePerRequest, func(key, value *felt.Felt) error {
		response.Paths = append(response.Paths, key)
		response.ClassHashes = append(response.ClassHashes, value)

		class, err := s.Class(key)
		if err != nil {
			return err
		}

		response.Classes = append(response.Classes, class.Class)
		return nil
	}, crypto.Poseidon)

	return response, err
}

func (b *Blockchain) GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*AddressRangeResult, error) {
	if rootHash == nil {
		return nil, fmt.Errorf("root hash is nil")
	}
	snapshot, err := b.FindSnapshotMatching(func(record *snapshotRecord) bool {
		return record.stateRoot.Equal(rootHash)
	})
	if err != nil {
		return nil, err
	}

	s := core.NewState(snapshot.txn)

	// TODO: Verify class trie
	strie, scloser, err := s.StorageTrie()
	if err != nil {
		return nil, err
	}
	defer scloser()

	response := &AddressRangeResult{
		Paths:  nil,
		Hashes: nil,
		Leaves: nil,
		Proofs: nil,
	}

	response.Proofs, err = iterateWithLimit(strie, startAddr, limitAddr, maxNodePerRequest, func(key, value *felt.Felt) error {
		response.Paths = append(response.Paths, key)
		response.Hashes = append(response.Hashes, value)

		classHash, err := s.ContractClassHash(key)
		if err != nil {
			return err
		}

		nonce, err := s.ContractNonce(key)
		if err != nil {
			return err
		}

		ctrk, err := s.Contract(key)
		if err != nil {
			return err
		}

		croot, err := ctrk.Root()
		if err != nil {
			return err
		}

		leaf := &AddressRangeLeaf{
			ContractStorageRoot: croot,
			ClassHash:           classHash,
			Nonce:               nonce,
		}

		response.Leaves = append(response.Leaves, leaf)
		return nil
	}, crypto.Pedersen)

	return response, err
}

func (b *Blockchain) GetContractRange(storageTrieRootHash *felt.Felt, requests []*StorageRangeRequest) ([]*StorageRangeResult, error) {
	snapshot, err := b.FindSnapshotMatching(func(record *snapshotRecord) bool {
		return record.stateRoot.Equal(storageTrieRootHash)
	})
	if err != nil {
		return nil, err
	}

	s := core.NewState(snapshot.txn)

	curNodeLimit := maxNodePerRequest

	responses := make([]*StorageRangeResult, 0)

	for _, request := range requests {
		response, err := b.handleStorageRangeRequest(s, request, curNodeLimit)
		if err != nil {
			return nil, err
		}

		responses = append(responses, response)
		curNodeLimit -= len(response.Paths)

		if curNodeLimit <= 0 {
			break
		}
	}

	return responses, nil
}

func (b *Blockchain) handleStorageRangeRequest(s *core.State, request *StorageRangeRequest, nodeLimit int) (*StorageRangeResult, error) {
	if request.Hash == nil {
		return nil, errors.New("request hash is nil")
	}

	contract, err := s.Contract(request.Path)
	if err != nil {
		return nil, err
	}

	strie, err := contract.StorageTrie()
	if err != nil {
		return nil, err
	}

	sroot, err := strie.Root()
	if err != nil {
		return nil, err
	}

	if !sroot.Equal(request.Hash) {
		return nil, fmt.Errorf("storage root hash mismatch %s vs %s", sroot.String(), request.Hash.String())
	}

	response := &StorageRangeResult{
		Paths:  nil,
		Values: nil,
		Proofs: nil,
	}

	response.Proofs, err = iterateWithLimit(strie, request.StartAddr, request.LimitAddr, nodeLimit, func(key, value *felt.Felt) error {
		response.Paths = append(response.Paths, key)
		response.Values = append(response.Values, value)
		return nil
	}, crypto.Pedersen)

	return response, err
}
