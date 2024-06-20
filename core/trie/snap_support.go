package trie

import (
	"fmt"
	"github.com/NethermindEth/juno/core/felt"
)

func (t *Trie) IterateAndGenerateProof(startValue *felt.Felt, consumer func(key, value *felt.Felt) (bool, error)) ([]ProofNode, error) {
	var lastKey *felt.Felt

	finished, err := t.Iterate(startValue, func(key, value *felt.Felt) (bool, error) {
		lastKey = key

		return consumer(key, value)
	})
	if err != nil {
		return nil, err
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
			return nil, err
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
			return nil, err
		}

		for _, proof := range rightProof {
			proofset[*proof.Hash(t.hash)] = proof
		}
	}

	proofs := make([]ProofNode, 0, len(proofset))
	for _, node := range proofset {
		proofs = append(proofs, node)
	}

	return proofs, nil
}

func VerifyRange(root, startKey *felt.Felt, keys, values []*felt.Felt, proofs []ProofNode, hash hashFunc) (hasMore bool, valid bool, err error) {
	proofMap := map[felt.Felt]ProofNode{}
	for _, proof := range proofs {
		proofHash := proof.Hash(hash)
		proofMap[*proofHash] = proof
	}

	if len(proofMap) == 0 && startKey == nil {
		// Special case where the whole trie is sent in one go.
		// We just need to completely reconstruct the trie.

		tempTrie, err := newTrie(newMemStorage(), 251, hash) //nolint:gomnd
		if err != nil {
			return false, false, err
		}

		for i, key := range keys {
			_, err := tempTrie.Put(key, values[i])
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
		fmt.Printf("Its here %s %s\n", root, recalculatedRoot)

		return false, true, nil
	}

	if _, ok := proofMap[*root]; !ok {
		// Verification failure, root not included in proof.
		return false, false, nil
	}

	proofKeys := map[felt.Felt]Key{}
	err = buildKeys(NewKey(0, []byte{}), root, proofMap, proofKeys, 0)
	if err != nil {
		return false, false, err
	}

	// No idea how this work
	/*
		proofValuesArray := []*felt.Felt{}
		proofKeysArray := []*Key{}
		for f, key := range proofKeys {
			proofValuesArray = append(proofValuesArray, &f)
			proofKeysArray = append(proofKeysArray, &key)
		}

		VerifyRangeProof(root, keys, values, nil, nil, nil, hash)
	*/

	hasMoreKeyCheck := startKey
	if len(keys) > 0 {
		hasMoreKeyCheck = keys[len(keys)-1]
	}

	feltBytes := hasMoreKeyCheck.Bytes()
	hasMoreKeyCheckKey := NewKey(251, feltBytes[:])

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
