package trie

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

func (t *Trie) IterateAndGenerateProof(startValue *felt.Felt, consumer func(key, value *felt.Felt) (bool, error),
) ([]ProofNode, bool, error) {
	var lastKey *felt.Felt

	finished, err := t.Iterate(startValue, func(key, value *felt.Felt) (bool, error) {
		lastKey = key

		return consumer(key, value)
	})
	if err != nil {
		return nil, finished, err
	}

	proofset := map[felt.Felt]ProofNode{}

	// If start value is null && finished, you dont need to provide any proof at all
	if !finished || startValue != nil {
		feltBts := startValue.Bytes()
		startKey := NewKey(t.height, feltBts[:])
		// Yes, the left proof is actually for the start query, not the actual leaf. Very confusing, yea I know. Need to
		// actually check that the server did not skip leafs.
		leftProof, err := GetProof(&startKey, t)
		if err != nil {
			return nil, finished, err
		}
		for _, proof := range leftProof {
			// Well.. using the trie hash here is kinda slow... but I just need it to work right now.
			proofset[*proof.Hash(t.hash)] = proof
		}
	}

	if !finished && lastKey != nil {
		feltBts := lastKey.Bytes()
		lastKey := NewKey(t.height, feltBts[:])
		rightProof, err := GetProof(&lastKey, t)
		if err != nil {
			return nil, finished, err
		}

		for _, proof := range rightProof {
			proofset[*proof.Hash(t.hash)] = proof
		}
	}

	proofs := make([]ProofNode, 0, len(proofset))
	for _, node := range proofset {
		proofs = append(proofs, node)
	}

	return proofs, finished, nil
}

func (t *Trie) IterateWithLimit(
	startAddr *felt.Felt,
	limitAddr *felt.Felt,
	maxNodes uint32,
	// TODO: remove the logger - and move to the tree
	logger utils.SimpleLogger,
	consumer func(key, value *felt.Felt) error,
) ([]ProofNode, bool, error) {
	pathes := make([]*felt.Felt, 0)
	hashes := make([]*felt.Felt, 0)

	count := uint32(0)
	proof, finished, err := t.IterateAndGenerateProof(startAddr, func(key *felt.Felt, value *felt.Felt) (bool, error) {
		// Need at least one.
		if limitAddr != nil && key.Cmp(limitAddr) > 0 {
			return true, nil
		}

		pathes = append(pathes, key)
		hashes = append(hashes, value)

		err := consumer(key, value)
		if err != nil {
			logger.Errorw("error from consumer function", "err", err)
			return false, err
		}

		count++
		if count >= maxNodes {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logger.Errorw("IterateAndGenerateProof", "err", err, "finished", finished)
		return nil, finished, err
	}

	return proof, finished, err
}

func VerifyRange(root, startKey *felt.Felt, keys, values []*felt.Felt, proofs []ProofNode, hash hashFunc,
	treeHeight uint8,
) (hasMore, valid bool, oerr error) {
	proofMap := map[felt.Felt]ProofNode{}
	for _, proof := range proofs {
		proofHash := proof.Hash(hash)
		proofMap[*proofHash] = proof
	}

	if len(proofMap) == 0 && startKey == nil {
		// Special case where the whole trie is sent in one go.
		// We just need to completely reconstruct the trie.

		tempTrie, err := newTrie(newMemStorage(), treeHeight, hash)
		if err != nil {
			return false, false, err
		}

		for i, key := range keys {
			_, err = tempTrie.Put(key, values[i])
			if err != nil {
				return false, false, err
			}
		}

		recalculatedRoot, err := tempTrie.Root()
		if err != nil {
			return false, false, err
		}

		if !root.Equal(recalculatedRoot) {
			return false, false, nil
		}

		return false, true, nil
	}

	if _, ok := proofMap[*root]; !ok {
		// Verification failure, root not included in proof.
		return false, false, nil
	}

	proofKeys := map[felt.Felt]Key{}
	err := buildKeys(NewKey(0, []byte{}), root, proofMap, proofKeys, 0)
	if err != nil {
		return false, false, err
	}

	// TODO: Verify here proof here

	hasMoreKeyCheck := startKey
	if len(keys) > 0 {
		hasMoreKeyCheck = keys[len(keys)-1]
	}

	feltBytes := hasMoreKeyCheck.Bytes()
	hasMoreKeyCheckKey := NewKey(treeHeight, feltBytes[:])

	// does this actually work on all case?
	hasMore = false
	for _, key := range proofKeys {
		comparison := key.CmpAligned(&hasMoreKeyCheckKey)
		if comparison > 0 {
			hasMore = true
		}
	}

	return hasMore, true, nil
}

func buildKeys(currentKey Key, currentNode *felt.Felt, proofMap map[felt.Felt]ProofNode, keys map[felt.Felt]Key, depth int) error {
	keys[*currentNode] = currentKey
	proofNode, ok := proofMap[*currentNode]
	if !ok {
		return nil
	}

	switch node := proofNode.(type) {
	case *Edge:
		chKey := currentKey.Append(node.Path)
		ch := node.Child
		err := buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}
	case *Binary:
		chKey := currentKey.AppendBit(false)
		ch := node.LeftHash
		err := buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}

		chKey = currentKey.AppendBit(true)
		ch = node.RightHash
		err = buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}
	}

	return nil
}
