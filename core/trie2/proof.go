package trie2

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, Node]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, Node]()
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
		nodes  []Node
		prefix = new(Path)
		rn     = t.root
	)

	for k.Len() > 0 && rn != nil {
		switch n := rn.(type) {
		case *EdgeNode:
			if !n.pathMatches(k) {
				rn = nil // Trie doesn't contain the key
			} else {
				rn = n.Child
				prefix.Append(prefix, n.Path)
				k.LSBs(k, n.Path.Len())
			}
			nodes = append(nodes, n)
		case *BinaryNode:
			bit := k.MSB()
			rn = n.Children[bit]
			prefix.AppendBit(prefix, bit)
			k.LSBs(k, 1)
			nodes = append(nodes, n)
		case *HashNode:
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
		var hn Node
		n, hn = hasher.proofHash(n)
		if hash, ok := hn.(*HashNode); ok || i == 0 {
			if !ok {
				hash = &HashNode{Felt: n.Hash(hasher.hashFn)}
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
		if !nHash.(*HashNode).Felt.Equal(&expected) {
			return felt.Zero, fmt.Errorf("proof node hash mismatch, expected hash: %s, got hash: %s", expected.String(), nHash.String())
		}

		child := get(node, keyBits, false)
		switch cld := child.(type) {
		case nil:
			return felt.Zero, nil
		case *HashNode:
			// TODO(weiihann):
			// There's a case where the leaf node is defined as a hash node instead of a value node
			// Ideally, this should not occur.
			if keyBits.Len() == 0 {
				return cld.Felt, nil
			}
			expected = cld.Felt
		case *ValueNode:
			return cld.Felt, nil
		case *EdgeNode, *BinaryNode:
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
func proofToPath(rootHash *felt.Felt, root Node, keyBits Path, proof *ProofNodeSet, allowNonExistent bool) (Node, *felt.Felt, error) {
	// Retrieves the node from the proof node set given the node hash
	retrieveNode := func(hash felt.Felt) (Node, error) {
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
		child, parent Node
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
		case *EdgeNode:
			parent = child
			continue
		case *BinaryNode:
			parent = child
			continue
		case *HashNode:
			child, err = retrieveNode(n.Felt)
			if err != nil {
				return nil, nil, err
			}
		case *ValueNode:
			val = &n.Felt
		}
		// Link the parent and child
		switch p := parent.(type) {
		case *EdgeNode:
			p.Child = child
		case *BinaryNode:
			p.Children[msb] = child
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

// unsetInternal removes all internal node references (HashNode, embedded node).
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
func unsetInternal(n Node, left, right Path) (bool, error) {
	// Step down to the fork point. There are two scenarios that can happen:
	// - the fork point is an EdgeNode: either the key of left proof or
	//   right proof doesn't match with the edge node's path
	// - the fork point is a BinaryNode: both two edge proofs are allowed
	//   to point to a non-existent key
	var (
		pos    uint8
		parent Node

		// fork indicator for edge nodes
		edgeForkLeft, edgeForkRight int
	)

findFork:
	for {
		switch rn := n.(type) {
		case *EdgeNode:
			rn.flags = newFlag()

			if left.Len()-pos < rn.Path.Len() {
				edgeForkLeft = new(Path).LSBs(&left, pos).Cmp(rn.Path)
			} else {
				subKey := new(Path).Subset(&left, pos, pos+rn.Path.Len())
				edgeForkLeft = subKey.Cmp(rn.Path)
			}

			if right.Len()-pos < rn.Path.Len() {
				edgeForkRight = new(Path).LSBs(&right, pos).Cmp(rn.Path)
			} else {
				subKey := new(Path).Subset(&right, pos, pos+rn.Path.Len())
				edgeForkRight = subKey.Cmp(rn.Path)
			}

			if edgeForkLeft != 0 || edgeForkRight != 0 {
				break findFork
			}

			parent = n
			n = rn.Child
			pos += rn.Path.Len()
		case *BinaryNode:
			rn.flags = newFlag()

			leftnode, rightnode := rn.Children[left.Bit(pos)], rn.Children[right.Bit(pos)]
			if leftnode == nil || rightnode == nil || leftnode != rightnode {
				break findFork
			}
			parent = n
			n = rn.Children[left.Bit(pos)]
			pos++
		default:
			panic(fmt.Sprintf("%T: invalid node: %v", n, n))
		}
	}

	switch rn := n.(type) {
	case *EdgeNode:
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
			parent.(*BinaryNode).Children[left.Bit(pos-1)] = nil
			return false, nil
		}
		// Only one proof points to non-existent key
		if edgeForkRight != 0 {
			if _, ok := rn.Child.(*ValueNode); ok {
				// The fork point is root node, unset the entire trie
				if parent == nil {
					return true, nil
				}
				parent.(*BinaryNode).Children[left.Bit(pos-1)] = nil
				return false, nil
			}
			return false, unset(rn, rn.Child, new(Path).LSBs(&left, pos), rn.Path.Len(), false)
		}
		if edgeForkLeft != 0 {
			if _, ok := rn.Child.(*ValueNode); ok {
				// The fork point is root node, unset the entire trie
				if parent == nil {
					return true, nil
				}
				parent.(*BinaryNode).Children[right.Bit(pos-1)] = nil
				return false, nil
			}
			return false, unset(rn, rn.Child, new(Path).LSBs(&right, pos), rn.Path.Len(), true)
		}
		return false, nil
	case *BinaryNode:
		leftBit := left.Bit(pos)
		rightBit := right.Bit(pos)
		if leftBit == 0 && rightBit == 0 {
			rn.Children[1] = nil
		}
		if leftBit == 1 && rightBit == 1 {
			rn.Children[0] = nil
		}
		if err := unset(rn, rn.Children[leftBit], new(Path).LSBs(&left, pos), 1, false); err != nil {
			return false, err
		}
		if err := unset(rn, rn.Children[rightBit], new(Path).LSBs(&right, pos), 1, true); err != nil {
			return false, err
		}
		return false, nil
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// unset removes all internal node references either the left most or right most.
func unset(parent, child Node, key *Path, pos uint8, removeLeft bool) error {
	switch cld := child.(type) {
	case *BinaryNode:
		keyBit := key.Bit(pos)
		cld.flags = newFlag() // Mark dirty
		if removeLeft {
			// Remove left child if we're removing left side
			if keyBit == 1 {
				cld.Children[0] = nil
			}
		} else {
			// Remove right child if we're removing right side
			if keyBit == 0 {
				cld.Children[1] = nil
			}
		}
		return unset(cld, cld.Children[key.Bit(pos)], key, pos+1, removeLeft)

	case *EdgeNode:
		keyPos := new(Path).LSBs(key, pos)
		keyBit := key.Bit(pos - 1)
		if !cld.pathMatches(keyPos) {
			// We append zeros to the path to match the length of the remaining key
			// The key length is guaranteed to be >= path length
			edgePath := new(Path).AppendZeros(cld.Path, keyPos.Len()-cld.Path.Len())
			// Found fork point, non-existent branch
			if removeLeft {
				if edgePath.Cmp(keyPos) < 0 {
					// Edge node path is in range, unset entire branch
					parent.(*BinaryNode).Children[keyBit] = nil
				}
			} else {
				if edgePath.Cmp(keyPos) > 0 {
					parent.(*BinaryNode).Children[keyBit] = nil
				}
			}
			return nil
		}
		if _, ok := cld.Child.(*ValueNode); ok {
			parent.(*BinaryNode).Children[keyBit] = nil
			return nil
		}
		cld.flags = newFlag()
		return unset(cld, cld.Child, key, pos+cld.Path.Len(), removeLeft)

	case nil, *HashNode, *ValueNode:
		// Child is nil, nothing to unset
		return nil
	default:
		panic("it shouldn't happen") // HashNode, ValueNode
	}
}

// hasRightElement checks if there is a right sibling for the given key in the trie.
// This function assumes that the entire path has been resolved.
func hasRightElement(node Node, key Path) bool {
	for node != nil {
		switch n := node.(type) {
		case *BinaryNode:
			bit := key.MSB()
			if bit == 0 && n.Children[1] != nil {
				// right sibling exists
				return true
			}
			node = n.Children[bit]
			key.LSBs(&key, 1)
		case *EdgeNode:
			if !n.pathMatches(&key) {
				// There's a divergence in the path, check if the node path is greater than the key
				// If so, that means that this node comes after the search key, which indicates that
				// there are elements with larger values
				var edgePath *Path
				if key.Len() > n.Path.Len() {
					edgePath = new(Path).AppendZeros(n.Path, key.Len()-n.Path.Len())
				} else {
					edgePath = n.Path
				}
				return edgePath.Cmp(&key) > 0
			}
			node = n.Child
			key.LSBs(&key, n.Path.Len())
		case *ValueNode:
			return false // resolved the whole path
		default:
			panic(fmt.Sprintf("unknown node type: %T", n))
		}
	}
	return false
}

// Resolves the whole path of the given key and node.
// If skipResolved is true, it will only return the immediate child node of the current node
func get(rn Node, key *Path, skipResolved bool) Node {
	for {
		switch n := rn.(type) {
		case *EdgeNode:
			if !n.pathMatches(key) {
				return nil
			}
			rn = n.Child
			key.LSBs(key, n.Path.Len())
		case *BinaryNode:
			bit := key.MSB()
			rn = n.Children[bit]
			key.LSBs(key, 1)
		case *HashNode:
			return n
		case *ValueNode:
			return n
		case nil:
			return nil
		}

		if !skipResolved {
			return rn
		}
	}
}
