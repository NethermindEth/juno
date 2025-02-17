package trie2

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, node]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, node]()
}

// Prove generates a Merkle proof for a given key in the trie.
// The result contains the proof nodes on the path from the root to the leaf.
// The value is included in the proof if the key is present in the trie.
// If the key is not present, the proof will contain the nodes on the path to the closest ancestor.
func (t *Trie) Prove(key *felt.Felt, proof *ProofNodeSet) error {
	if t.committed {
		return ErrCommitted
	}

	path := t.FeltToPath(key)
	k := &path

	var (
		nodes  []node
		prefix = new(Path)
		rn     = t.root
	)

	for k.Len() > 0 && rn != nil {
		switch n := rn.(type) {
		case *edgeNode:
			if !n.pathMatches(k) {
				rn = nil // Trie doesn't contain the key
			} else {
				rn = n.child
				prefix.Append(prefix, n.path)
				k.LSBs(k, n.path.Len())
			}
			nodes = append(nodes, n)
		case *binaryNode:
			bit := k.MSB()
			rn = n.children[bit]
			prefix.AppendBit(prefix, bit)
			k.LSBs(k, 1)
			nodes = append(nodes, n)
		case *hashNode:
			resolved, err := t.resolveNode(n, *prefix)
			if err != nil {
				return err
			}
			rn = resolved
		default:
			panic(fmt.Sprintf("key: %s, unknown node type: %T", key.String(), n))
		}
	}

	// TODO: ideally Hash() should be called before Prove() so that the hashes are cached
	// There should be a better way to do this
	hasher := newHasher(t.hashFn, false)
	for i, n := range nodes {
		var hn node
		n, hn = hasher.proofHash(n)
		if hash, ok := hn.(*hashNode); ok || i == 0 {
			if !ok {
				hash = &hashNode{Felt: *n.hash(hasher.hashFn)}
			}
			proof.Put(hash.Felt, n)
		}
	}

	return nil
}

// GetRangeProof generates a range proof for the given range of keys.
// The proof contains the proof nodes on the path from the root to the closest ancestor of the left and right keys.
func (t *Trie) GetRangeProof(leftKey, rightKey *felt.Felt, proofSet *ProofNodeSet) error {
	err := t.Prove(leftKey, proofSet)
	if err != nil {
		return err
	}

	// If they are the same key, don't need to generate the proof again
	if leftKey.Equal(rightKey) {
		return nil
	}

	err = t.Prove(rightKey, proofSet)
	if err != nil {
		return err
	}

	return nil
}

// VerifyProof verifies that a proof path is valid for a given key in a binary trie.
// It walks through the proof nodes, verifying each step matches the expected path to reach the key.
//
// The proof is considered invalid if:
//   - Any proof node is missing from the node set
//   - Any node's computed hash doesn't match its expected hash
//   - The path bits don't match the key bits
//   - The proof ends before processing all key bits
func VerifyProof(root, key *felt.Felt, proof *ProofNodeSet, hash crypto.HashFn) (felt.Felt, error) {
	keyBits := new(Path).SetFelt(contractClassTrieHeight, key)
	expected := *root
	h := newHasher(hash, false)

	for {
		node, ok := proof.Get(expected)
		if !ok {
			return felt.Zero, fmt.Errorf("proof node not found, expected hash: %s", expected.String())
		}

		nHash, _ := h.hash(node)

		// Verify the hash matches
		if !nHash.(*hashNode).Felt.Equal(&expected) {
			return felt.Zero, fmt.Errorf("proof node hash mismatch, expected hash: %s, got hash: %s", expected.String(), nHash.String())
		}

		child := get(node, keyBits, false)
		switch cld := child.(type) {
		case nil:
			return felt.Zero, nil
		case *hashNode:
			expected = cld.Felt
		case *valueNode:
			return cld.Felt, nil
		case *edgeNode, *binaryNode:
			if hash, _ := cld.cache(); hash != nil {
				expected = hash.Felt
			}
		}
	}
}

// VerifyRangeProof checks the validity of given key-value pairs and range proof against a provided root hash.
// The key-value pairs should be consecutive (no gaps) and monotonically increasing.
// The range proof contains two edge proofs: one for the first key and another for the last key.
// Both edge proofs can be for existent or non-existent keys.
// This function handles the following special cases:
//
//   - All elements proof: The proof can be nil if the range includes all leaves in the trie.
//   - Single element proof: Both left and right edge proofs are identical, and the range contains only one element.
//   - Zero element proof: A single edge proof suffices for verification. The proof is invalid if there are additional elements.
//
// The function returns a boolean indicating if there are more elements and an error if the range proof is invalid.
func VerifyRangeProof(rootHash, first *felt.Felt, keys, values []*felt.Felt, proof *ProofNodeSet) (bool, error) { //nolint:funlen,gocyclo
	// Ensure the number of keys and values are the same
	if len(keys) != len(values) {
		return false, fmt.Errorf("inconsistent length of proof data, keys: %d, values: %d", len(keys), len(values))
	}

	// Ensure all keys are monotonically increasing and values contain no deletions
	for i := range keys {
		if i < len(keys)-1 && keys[i].Cmp(keys[i+1]) > 0 {
			return false, errors.New("keys are not monotonic increasing")
		}

		if values[i] == nil || values[i].Equal(&felt.Zero) {
			return false, errors.New("range contains empty leaf")
		}
	}

	// Special case: no edge proof provided; the given range contains all leaves in the trie
	if proof == nil {
		tr := NewEmpty(contractClassTrieHeight, crypto.Pedersen)
		for i, key := range keys {
			if err := tr.Update(key, values[i]); err != nil {
				return false, err
			}
		}

		recomputedRoot := tr.Hash()
		if !recomputedRoot.Equal(rootHash) {
			return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", rootHash.String(), recomputedRoot.String())
		}

		return false, nil // no more elements available
	}

	var firstKey Path
	firstKey.SetFelt(contractClassTrieHeight, first)

	// Special case: there is a provided proof but no key-value pairs, make sure regenerated trie has no more values
	// Empty range proof with more elements on the right is not accepted in this function.
	// This is due to snap sync specification detail, where the responder must send an existing key (if any) if the requested range is empty.
	if len(keys) == 0 {
		rootKey, val, err := proofToPath(rootHash, nil, firstKey, proof, true)
		if err != nil {
			return false, err
		}

		if val != nil || hasRightElement(rootKey, firstKey) {
			return false, errors.New("more entries available")
		}

		return false, nil
	}

	last := keys[len(keys)-1]

	var lastKey Path
	lastKey.SetFelt(contractClassTrieHeight, last)

	// Special case: there is only one element and two edge keys are the same
	if len(keys) == 1 && firstKey.Equal(&lastKey) {
		root, val, err := proofToPath(rootHash, nil, firstKey, proof, false)
		if err != nil {
			return false, err
		}

		firstItemKey := new(Path).SetFelt(contractClassTrieHeight, keys[0])
		if !firstKey.Equal(firstItemKey) {
			return false, errors.New("correct proof but invalid key")
		}

		if val == nil || !values[0].Equal(val) {
			return false, errors.New("correct proof but invalid value")
		}

		return hasRightElement(root, firstKey), nil
	}

	// In all other cases, we require two edge paths available.
	// First, ensure that the last key is greater than the first key
	if last.Cmp(first) <= 0 {
		return false, errors.New("last key is less than first key")
	}

	root, _, err := proofToPath(rootHash, nil, firstKey, proof, true)
	if err != nil {
		return false, err
	}

	root, _, err = proofToPath(rootHash, root, lastKey, proof, true)
	if err != nil {
		return false, err
	}

	empty, err := unsetInternal(root, firstKey, lastKey)
	if err != nil {
		return false, err
	}

	tr := NewEmpty(contractClassTrieHeight, crypto.Pedersen)
	if !empty {
		tr.root = root
	}

	for i, key := range keys {
		if err := tr.Update(key, values[i]); err != nil {
			return false, err
		}
	}

	newRoot := tr.Hash()

	// Verify that the recomputed root hash matches the provided root hash
	if !newRoot.Equal(rootHash) {
		return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", rootHash.String(), newRoot.String())
	}

	return hasRightElement(root, lastKey), nil
}

// proofToPath converts a Merkle proof to trie node path. All necessary nodes will be resolved and leave the remaining
// as hashes. The given edge proof can be existent or non-existent.
func proofToPath(rootHash *felt.Felt, root node, keyBits Path, proof *ProofNodeSet, allowNonExistent bool) (node, *felt.Felt, error) {
	// Retrieves the node from the proof node set given the node hash
	retrieveNode := func(hash felt.Felt) (node, error) {
		n, _ := proof.Get(hash)
		if n == nil {
			return nil, fmt.Errorf("proof node not found, expected hash: %s", hash.String())
		}
		return n, nil
	}

	// Must resolve the root node first if it's not provided
	if root == nil {
		n, err := retrieveNode(*rootHash)
		if err != nil {
			return nil, nil, err
		}
		root = n
	}

	var (
		err           error
		child, parent node
		val           *felt.Felt
	)

	parent = root
	for {
		msb := keyBits.MSB() // key gets modified in get(), we need the current msb to get the correct child during linking
		child = get(parent, &keyBits, false)
		switch n := child.(type) {
		case nil:
			if allowNonExistent {
				return root, nil, nil
			}
			return nil, nil, errors.New("the node is not in the trie")
		case *edgeNode:
			parent = child
			continue
		case *binaryNode:
			parent = child
			continue
		case *hashNode:
			child, err = retrieveNode(n.Felt)
			if err != nil {
				return nil, nil, err
			}
		case *valueNode:
			val = &n.Felt
		}
		// Link the parent and child
		switch p := parent.(type) {
		case *edgeNode:
			p.child = child
		case *binaryNode:
			p.children[msb] = child
		default:
			panic(fmt.Sprintf("unknown parent node type: %T", p))
		}

		// We reached the leaf
		if val != nil {
			return root, val, nil
		}

		parent = child
	}
}

// unsetInternal removes all internal node references (hashNode, embedded node).
// It should be called after a trie is constructed with two edge paths. Also
// the given boundary keys must be the ones used to construct the edge paths.
//
// It's the key step for range proof. All visited nodes should be marked dirty
// since the node content might be modified.
//
// Note we have the assumption here the given boundary keys are different
// and right is larger than left.
//
//nolint:gocyclo
func unsetInternal(n node, left, right Path) (bool, error) {
	// Step down to the fork point. There are two scenarios that can happen:
	// - the fork point is an edgeNode: either the key of left proof or
	//   right proof doesn't match with the edge node's path
	// - the fork point is a binaryNode: both two edge proofs are allowed
	//   to point to a non-existent key
	var (
		pos    uint8
		parent node

		// fork indicator for edge nodes
		edgeForkLeft, edgeForkRight int
	)

findFork:
	for {
		switch rn := n.(type) {
		case *edgeNode:
			rn.flags = newFlag()

			if left.Len()-pos < rn.path.Len() {
				edgeForkLeft = new(Path).LSBs(&left, pos).Cmp(rn.path)
			} else {
				subKey := new(Path).Subset(&left, pos, pos+rn.path.Len())
				edgeForkLeft = subKey.Cmp(rn.path)
			}

			if right.Len()-pos < rn.path.Len() {
				edgeForkRight = new(Path).LSBs(&right, pos).Cmp(rn.path)
			} else {
				subKey := new(Path).Subset(&right, pos, pos+rn.path.Len())
				edgeForkRight = subKey.Cmp(rn.path)
			}

			if edgeForkLeft != 0 || edgeForkRight != 0 {
				break findFork
			}

			parent = n
			n = rn.child
			pos += rn.path.Len()
		case *binaryNode:
			rn.flags = newFlag()

			leftnode, rightnode := rn.children[left.Bit(pos)], rn.children[right.Bit(pos)]
			if leftnode == nil || rightnode == nil || leftnode != rightnode {
				break findFork
			}
			parent = n
			n = rn.children[left.Bit(pos)]
			pos++
		default:
			panic(fmt.Sprintf("%T: invalid node: %v", n, n))
		}
	}

	switch rn := n.(type) {
	case *edgeNode:
		// There can have these five scenarios:
		// - both proofs are less than the trie path => no valid range
		// - both proofs are greater than the trie path => no valid range
		// - left proof is less and right proof is greater => valid range, unset the shortnode entirely
		// - left proof points to the shortnode, but right proof is greater
		// - right proof points to the shortnode, but left proof is less
		if edgeForkLeft == -1 && edgeForkRight == -1 {
			return false, ErrEmptyRange
		}
		if edgeForkLeft == 1 && edgeForkRight == 1 {
			return false, ErrEmptyRange
		}
		if edgeForkLeft != 0 && edgeForkRight != 0 {
			// The fork point is root node, unset the entire trie
			if parent == nil {
				return true, nil
			}
			parent.(*binaryNode).children[left.Bit(pos-1)] = nil
			return false, nil
		}
		// Only one proof points to non-existent key
		if edgeForkRight != 0 {
			if _, ok := rn.child.(*valueNode); ok {
				// The fork point is root node, unset the entire trie
				if parent == nil {
					return true, nil
				}
				parent.(*binaryNode).children[left.Bit(pos-1)] = nil
				return false, nil
			}
			return false, unset(rn, rn.child, new(Path).LSBs(&left, pos), rn.path.Len(), false)
		}
		if edgeForkLeft != 0 {
			if _, ok := rn.child.(*valueNode); ok {
				// The fork point is root node, unset the entire trie
				if parent == nil {
					return true, nil
				}
				parent.(*binaryNode).children[right.Bit(pos-1)] = nil
				return false, nil
			}
			return false, unset(rn, rn.child, new(Path).LSBs(&right, pos), rn.path.Len(), true)
		}
		return false, nil
	case *binaryNode:
		leftBit := left.Bit(pos)
		rightBit := right.Bit(pos)
		if leftBit == 0 && rightBit == 0 {
			rn.children[1] = nil
		}
		if leftBit == 1 && rightBit == 1 {
			rn.children[0] = nil
		}
		if err := unset(rn, rn.children[leftBit], new(Path).LSBs(&left, pos), 1, false); err != nil {
			return false, err
		}
		if err := unset(rn, rn.children[rightBit], new(Path).LSBs(&right, pos), 1, true); err != nil {
			return false, err
		}
		return false, nil
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// unset removes all internal node references either the left most or right most.
func unset(parent, child node, key *Path, pos uint8, removeLeft bool) error {
	switch cld := child.(type) {
	case *binaryNode:
		keyBit := key.Bit(pos)
		cld.flags = newFlag() // Mark dirty
		if removeLeft {
			// Remove left child if we're removing left side
			if keyBit == 1 {
				cld.children[0] = nil
			}
		} else {
			// Remove right child if we're removing right side
			if keyBit == 0 {
				cld.children[1] = nil
			}
		}
		return unset(cld, cld.children[key.Bit(pos)], key, pos+1, removeLeft)

	case *edgeNode:
		keyPos := new(Path).LSBs(key, pos)
		keyBit := key.Bit(pos - 1)
		if !cld.pathMatches(keyPos) {
			// We append zeros to the path to match the length of the remaining key
			// The key length is guaranteed to be >= path length
			edgePath := new(Path).AppendZeros(cld.path, keyPos.Len()-cld.path.Len())
			// Found fork point, non-existent branch
			if removeLeft {
				if edgePath.Cmp(keyPos) < 0 {
					// Edge node path is in range, unset entire branch
					parent.(*binaryNode).children[keyBit] = nil
				}
			} else {
				if edgePath.Cmp(keyPos) > 0 {
					parent.(*binaryNode).children[keyBit] = nil
				}
			}
			return nil
		}
		if _, ok := cld.child.(*valueNode); ok {
			parent.(*binaryNode).children[keyBit] = nil
			return nil
		}
		cld.flags = newFlag()
		return unset(cld, cld.child, key, pos+cld.path.Len(), removeLeft)

	case nil, *hashNode, *valueNode:
		// Child is nil, nothing to unset
		return nil
	default:
		panic("it shouldn't happen") // hashNode, valueNode
	}
}

// hasRightElement checks if there is a right sibling for the given key in the trie.
// This function assumes that the entire path has been resolved.
func hasRightElement(node node, key Path) bool {
	for node != nil {
		switch n := node.(type) {
		case *binaryNode:
			bit := key.MSB()
			if bit == 0 && n.children[1] != nil {
				// right sibling exists
				return true
			}
			node = n.children[bit]
			key.LSBs(&key, 1)
		case *edgeNode:
			if !n.pathMatches(&key) {
				// There's a divergence in the path, check if the node path is greater than the key
				// If so, that means that this node comes after the search key, which indicates that
				// there are elements with larger values
				var edgePath *Path
				if key.Len() > n.path.Len() {
					edgePath = new(Path).AppendZeros(n.path, key.Len()-n.path.Len())
				} else {
					edgePath = n.path
				}
				return edgePath.Cmp(&key) > 0
			}
			node = n.child
			key.LSBs(&key, n.path.Len())
		case *valueNode:
			return false // resolved the whole path
		default:
			panic(fmt.Sprintf("unknown node type: %T", n))
		}
	}
	return false
}

// Resolves the whole path of the given key and node.
// If skipResolved is true, it will only return the immediate child node of the current node
func get(rn node, key *Path, skipResolved bool) node {
	for {
		switch n := rn.(type) {
		case *edgeNode:
			if !n.pathMatches(key) {
				return nil
			}
			rn = n.child
			key.LSBs(key, n.path.Len())
		case *binaryNode:
			bit := key.MSB()
			rn = n.children[bit]
			key.LSBs(key, 1)
		case *hashNode:
			return n
		case *valueNode:
			return n
		case nil:
			return nil
		}

		if !skipResolved {
			return rn
		}
	}
}
