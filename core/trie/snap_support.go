package trie

import (
	"github.com/NethermindEth/juno/core/felt"
)

func (t *Trie) IterateAndGenerateProof(startValue *felt.Felt, consumer func(key, value *felt.Felt) (bool, error),
) ([]ProofNode, bool, error) {
	var lastKey *felt.Felt

	finished, err := t.Iterate(startValue, func(key, value *felt.Felt) (bool, error) {
		lastKey = key

		return consumer(key, value)
	})
	if err != nil {
		return nil, false, err
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
			return nil, false, err
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
			return nil, false, err
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

// VerifyRange Verify range of keys and values given by IterateAndGenerateProof.
// Also returns a flag to indicate if there are more leaf from the tree, inferred from the proof.
// TODO: Actually verify proof in case when not the whole trie is sent.
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

		rootMatched := root.Equal(recalculatedRoot)

		return false, rootMatched, nil
	}

	if _, ok := proofMap[*root]; !ok {
		// Verification failure, root not included in proof.
		return false, false, nil
	}

	proofPathKeys := map[felt.Felt]Key{}
	err := buildKeys(NewKey(0, []byte{}), root, proofMap, proofPathKeys, 0)
	if err != nil {
		return false, false, err
	}

	// TODO: Verify here proof here

	rightMostKey := startKey
	if startKey == nil {
		rightMostKey = &felt.Zero
	}
	if len(keys) > 0 {
		rightMostKey = keys[len(keys)-1]
	}

	rightMostKeyBytes := rightMostKey.Bytes()
	hasMoreKeyCheckKey := NewKey(treeHeight, rightMostKeyBytes[:])

	// does this actually work on all case?
	hasMore = false
	for _, key := range proofPathKeys {
		comparison := key.CmpAligned(&hasMoreKeyCheckKey)
		if comparison > 0 {
			hasMore = true
		}
	}

	return hasMore, true, nil
}

// buildKeys regenerate the keys for each proof into `keys`. The proof on its own does not have complete path, it only
// points to its children by hash, which is given in `proofMap`.
func buildKeys(currentKey Key, currentNode *felt.Felt, proofMap map[felt.Felt]ProofNode, keys map[felt.Felt]Key, depth int) error {
	keys[*currentNode] = currentKey
	proofNode, ok := proofMap[*currentNode]
	if !ok {
		return nil
	}

	if proofNode.Edge != nil {
		chKey := currentKey.Append(proofNode.Edge.Path)
		ch := proofNode.Edge.Child
		err := buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}
	} else {
		binary := proofNode.Binary

		chKey := currentKey.AppendBit(false)
		ch := binary.LeftHash
		err := buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}

		chKey = currentKey.AppendBit(true)
		ch = binary.RightHash
		err = buildKeys(chKey, ch, proofMap, keys, depth+1)
		if err != nil {
			return err
		}
	}

	return nil
}
