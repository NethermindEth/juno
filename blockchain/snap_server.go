package blockchain

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

type AddressRangeResult struct {
	Paths  []*felt.Felt
	Leaves []*AddressRangeLeaf

	Proofs []*trie.ProofNode
}

type AddressRangeLeaf struct {
	ContractStorageRoot *felt.Felt
	ClassHash           *felt.Felt
	Nonce               *felt.Felt
}

type SnapServer interface {
	GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (*AddressRangeResult, error)
}

const maxNodePerRequest = 1024 * 1024 // I just want it to process faster
func determineMaxNodes(specifiedMaxNodes uint64) uint64 {
	if specifiedMaxNodes == 0 {
		maxNodes := 64
		return uint64(maxNodes)
	}

	if specifiedMaxNodes < maxNodePerRequest {
		return specifiedMaxNodes
	}
	return maxNodePerRequest
}

func iterateWithLimit(
	srcTrie *trie.Trie,
	startAddr *felt.Felt,
	limitAddr *felt.Felt,
	maxNode uint64,
	consumer func(key, value *felt.Felt) error,
) ([]*trie.ProofNode, error) {
	pathes := make([]*felt.Felt, 0)
	hashes := make([]*felt.Felt, 0)

	// TODO: Verify class trie
	var startPath *felt.Felt
	var endPath *felt.Felt
	count := uint64(0)
	neverStopped, err := srcTrie.Iterate(startAddr, func(key *felt.Felt, value *felt.Felt) (bool, error) {
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

	if neverStopped && startAddr.Equal(&felt.Zero) {
		return nil, nil // No need for proof
	}
	if startPath == nil {
		return nil, nil // No need for proof
	}

	return trie.RangeProof(srcTrie, startAddr, endPath)
}

func (b *Blockchain) findSnapshotMatching(filter func(record *snapshotRecord) bool) (*snapshotRecord, error) {
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

func (b *Blockchain) GetAddressRange(rootHash, startAddr, limitAddr *felt.Felt, maxNodes uint64) (*AddressRangeResult, error) {
	if rootHash == nil {
		return nil, fmt.Errorf("root hash is nil")
	}
	if maxNodes == 0 {
		return nil, fmt.Errorf("maxNodes cannot be 0")
	}

	snapshot, err := b.findSnapshotMatching(func(record *snapshotRecord) bool {
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

	defer func() {
		if cerr := scloser(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	response := &AddressRangeResult{
		Paths:  nil,
		Leaves: nil,
		Proofs: nil,
	}

	response.Proofs, err = iterateWithLimit(strie, startAddr, limitAddr, determineMaxNodes(maxNodes), func(key, value *felt.Felt) error {
		response.Paths = append(response.Paths, key)

		classHash, errProof := s.ContractClassHash(key)
		if errProof != nil {
			return errProof
		}

		nonce, errProof := s.ContractNonce(key)
		if errProof != nil {
			return errProof
		}

		croot, errProof := core.ContractRoot(key, snapshot.txn)
		if errProof != nil {
			return errProof
		}

		leaf := &AddressRangeLeaf{
			ContractStorageRoot: croot,
			ClassHash:           classHash,
			Nonce:               nonce,
		}

		response.Leaves = append(response.Leaves, leaf)
		return nil
	})

	return response, err
}
