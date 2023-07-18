package core

import (
	"fmt"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

type TrieRootInfo struct {
	StorageRoot *felt.Felt
	ClassRoot   *felt.Felt
}

type ClassRangeResult struct {
	Paths       []*felt.Felt
	ClassHashes []*felt.Felt
	Classes     []Class

	Proofs []*trie.ProofNode
}

type AddressRangeResult struct {
	Paths  []*felt.Felt
	Hashes []*felt.Felt
	Leaves []*AddressRangeLeaf

	Proofs []*trie.ProofNode
}

type AddressRangeLeaf struct {
	StorageRoot *felt.Felt
	ClassHash   *felt.Felt
	Nonce       *felt.Felt
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

var _ SnapServer = &State{}

const maxNodePerRequest = 2

func (s *State) GetTrieRootAt(blockHash *felt.Felt) (*TrieRootInfo, error) {
	// TODO: check the block hash

	strie, stateCloser, err := s.storage()
	if err != nil {
		return nil, err
	}
	defer stateCloser()

	storageRoot, err := strie.Root()
	if err != nil {
		return nil, err
	}

	ctrie, classCloser, err := s.classesTrie()
	if err != nil {
		return nil, err
	}
	defer classCloser()

	classRoot, err := ctrie.Root()
	if err != nil {
		return nil, err
	}

	return &TrieRootInfo{
		StorageRoot: storageRoot,
		ClassRoot:   classRoot,
	}, nil
}

func iterateWithLimit(trie *trie.Trie, startAddr *felt.Felt, limitAddr *felt.Felt, maxNode int, consumer func(key, value *felt.Felt) error) ([]*trie.ProofNode, error) {
	// TODO: Verify class trie
	var startPath *felt.Felt
	var endPath *felt.Felt
	count := 0
	err := trie.Iterate(startAddr, func(key *felt.Felt, value *felt.Felt) (bool, error) {
		if startPath == nil {
			startPath = key
		}
		endPath = key

		// Need at least one.
		if limitAddr != nil && key.Cmp(limitAddr) > 1 && count > 0 {
			return false, nil
		}

		err := consumer(key, value)
		if err != nil {
			return false, err
		}

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
		return trie.ProofTo(startPath)
	} else if count > 1 {
		leftProof, err := trie.ProofTo(startPath)
		if err != nil {
			return nil, err
		}
		rightProof, err := trie.ProofTo(endPath)
		if err != nil {
			return nil, err
		}

		return append(leftProof, rightProof...), nil
	}

	return nil, nil
}

func (s *State) GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*ClassRangeResult, error) {
	// TODO: Verify class trie
	ctrie, classCloser, err := s.classesTrie()
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
	})

	return response, err
}

func (s *State) GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (*AddressRangeResult, error) {
	// TODO: Verify class trie
	strie, scloser, err := s.storage()
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
			StorageRoot: croot,
			ClassHash:   classHash,
			Nonce:       nonce,
		}

		response.Leaves = append(response.Leaves, leaf)
		return nil
	})

	return response, nil
}

func (s *State) GetContractRange(storageTrieRootHash *felt.Felt, requests []*StorageRangeRequest) ([]*StorageRangeResult, error) {
	curNodeLimit := maxNodePerRequest

	responses := make([]*StorageRangeResult, 0)

	for _, request := range requests {
		response, err := s.handleStorageRangeRequest(request, curNodeLimit)
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

func (s *State) handleStorageRangeRequest(request *StorageRangeRequest, nodeLimit int) (*StorageRangeResult, error) {
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
	})

	return response, err
}
