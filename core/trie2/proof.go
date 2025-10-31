package trie2

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, trienode.Node]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, trienode.Node]()
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

	var (
		nodes    []trienode.Node
		prefix   = new(Path)
		rootNode = t.root
	)

	for path.Len() > 0 && rootNode != nil {
		switch n := rootNode.(type) {
		case *trienode.EdgeNode:
			if !n.PathMatches(&path) {
				rootNode = nil // Trie doesn't contain the key
			} else {
				rootNode = n.Child
				prefix.Append(prefix, n.Path)
				(&path).LSBs(&path, n.Path.Len())
			}
			nodes = append(nodes, n)
		case *trienode.BinaryNode:
			bit := (&path).MSB()
			rootNode = n.Children[bit]
			prefix.AppendBit(prefix, bit)
			(&path).LSBs(&path, 1)
			nodes = append(nodes, n)
		case *trienode.HashNode:
			resolved, err := t.resolveNode(n, *prefix)
			if err != nil {
				return err
			}
			rootNode = resolved
		default:
			panic(fmt.Sprintf("key: %s, unknown node type: %T", key.String(), n))
		}
	}

	hasher := newHasher(t.hashFn, false)
	for i, n := range nodes {
		var hn trienode.Node
		n, hn = hasher.proofHash(n)
		if hash, ok := hn.(*trienode.HashNode); ok || i == 0 {
			if !ok {
				hashVal := n.Hash(hasher.hashFn)
				hash = (*trienode.HashNode)(&hashVal)
			}
			proof.Put(felt.Felt(*hash), n)
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
		hashVal := felt.Felt(*nHash.(*trienode.HashNode))
		if !hashVal.Equal(&expected) {
			return felt.Zero, fmt.Errorf("proof node hash mismatch, expected hash: %s, got hash: %s", expected.String(), nHash.String())
		}

		child := get(node, keyBits, false)
		switch cld := child.(type) {
		case nil:
			return felt.Zero, nil
		case *trienode.HashNode:
			// TODO(weiihann):
			// There's a case where the leaf node is defined as a hash node instead of a value node
			// Ideally, this should not occur.
			if keyBits.Len() == 0 {
				return felt.Felt(*cld), nil
			}
			expected = felt.Felt(*cld)
		case *trienode.ValueNode:
			return felt.Felt(*cld), nil
		case *trienode.EdgeNode, *trienode.BinaryNode:
			if hash, _ := cld.Cache(); hash != nil {
				expected = felt.Felt(*hash)
			}
		}
	}
}

// verifyProofData validates the consistency of keys and values
func verifyProofData(keys, values []*felt.Felt) error {
	if len(keys) != len(values) {
		return fmt.Errorf("inconsistent length of proof data, keys: %d, values: %d", len(keys), len(values))
	}

	for i := range keys {
		if i < len(keys)-1 && keys[i].Cmp(keys[i+1]) > 0 {
			return errors.New("keys are not monotonic increasing")
		}

		if values[i] == nil || values[i].Equal(&felt.Zero) {
			return errors.New("range contains empty leaf")
		}
	}
	return nil
}

// verifyEmptyRangeProof handles the case when there are no key-value pairs
func verifyEmptyRangeProof(rootHash, first *felt.Felt, proof *ProofNodeSet) (bool, error) {
	var firstKey Path
	firstKey.SetFelt(contractClassTrieHeight, first)

	rootKey, val, err := proofToPath(rootHash, nil, firstKey, proof, true)
	if err != nil {
		return false, err
	}

	if val != nil || hasRightElement(rootKey, &firstKey) {
		return false, errors.New("more entries available")
	}

	return false, nil
}

// verifySingleElementProof handles the case when there is only one element
func verifySingleElementProof(rootHash, key, value *felt.Felt, proof *ProofNodeSet) (bool, error) {
	var keyPath Path
	keyPath.SetFelt(contractClassTrieHeight, key)

	root, val, err := proofToPath(rootHash, nil, keyPath, proof, false)
	if err != nil {
		return false, err
	}

	itemKey := new(Path).SetFelt(contractClassTrieHeight, key)
	if !keyPath.Equal(itemKey) {
		return false, errors.New("correct proof but invalid key")
	}

	if val == nil || !value.Equal(val) {
		return false, errors.New("correct proof but invalid value")
	}

	return hasRightElement(root, &keyPath), nil
}

// verifyRangeWithProof handles the general case with multiple elements
func verifyRangeWithProof(rootHash, first, last *felt.Felt, keys, values []*felt.Felt, proof *ProofNodeSet) (bool, error) {
	var firstKey, lastKey Path
	firstKey.SetFelt(contractClassTrieHeight, first)
	lastKey.SetFelt(contractClassTrieHeight, last)

	// Build the trie with the left edge proof
	root, _, err := proofToPath(rootHash, nil, firstKey, proof, true)
	if err != nil {
		return false, err
	}

	// Add the right edge proof to the existing trie built with the left edge proof
	root, _, err = proofToPath(rootHash, root, lastKey, proof, true)
	if err != nil {
		return false, err
	}

	empty, err := unsetInternal(root, &firstKey, &lastKey)
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

	newRoot, _ := tr.Hash()
	if !newRoot.Equal(rootHash) {
		return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", rootHash.String(), newRoot.String())
	}

	return hasRightElement(root, &lastKey), nil
}

func VerifyRangeProof(rootHash, first *felt.Felt, keys, values []*felt.Felt, proof *ProofNodeSet) (bool, error) {
	if err := verifyProofData(keys, values); err != nil {
		return false, err
	}

	// Special case: no edge proof provided
	if proof == nil {
		tr := NewEmpty(contractClassTrieHeight, crypto.Pedersen)
		for i, key := range keys {
			if err := tr.Update(key, values[i]); err != nil {
				return false, err
			}
		}

		recomputedRoot, _ := tr.Hash()
		if !recomputedRoot.Equal(rootHash) {
			return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", rootHash.String(), recomputedRoot.String())
		}

		return false, nil
	}

	// Special case: empty range
	if len(keys) == 0 {
		return verifyEmptyRangeProof(rootHash, first, proof)
	}

	last := keys[len(keys)-1]

	// Special case: single element
	if len(keys) == 1 && first.Equal(last) {
		return verifySingleElementProof(rootHash, keys[0], values[0], proof)
	}

	// General case: multiple elements
	if last.Cmp(first) <= 0 {
		return false, errors.New("last key is less than first key")
	}

	return verifyRangeWithProof(rootHash, first, last, keys, values, proof)
}

// proofToPath converts a Merkle proof to trie node path. All necessary nodes will be resolved and leave the remaining
// as hashes. The given edge proof can be existent or non-existent.
func proofToPath(
	rootHash *felt.Felt,
	root trienode.Node,
	keyBits Path,
	proof *ProofNodeSet,
	allowNonExistent bool,
) (trienode.Node, *felt.Felt, error) {
	// Retrieves the node from the proof node set given the node hash
	retrieveNode := func(hash *felt.Felt) (trienode.Node, error) {
		n, ok := proof.Get(*hash)
		if !ok {
			return nil, fmt.Errorf("proof node not found, expected hash: %s", hash.String())
		}
		return n, nil
	}

	// Must resolve the root node first if it's not provided
	if root == nil {
		n, err := retrieveNode(rootHash)
		if err != nil {
			return nil, nil, err
		}
		root = n
	}

	parent := root
	for {
		var (
			err   error
			child trienode.Node
			val   *felt.Felt
		)
		msb := keyBits.MSB() // key gets modified in get(), we need the current msb to get the correct child during linking
		child = get(parent, &keyBits, false)
		switch n := child.(type) {
		case nil:
			if allowNonExistent {
				return root, nil, nil
			}
			return nil, nil, errors.New("the node is not in the trie")
		case *trienode.EdgeNode:
			parent = child
			continue
		case *trienode.BinaryNode:
			parent = child
			continue
		case *trienode.HashNode:
			child, err = retrieveNode((*felt.Felt)(n))
			if err != nil {
				return nil, nil, err
			}
		case *trienode.ValueNode:
			val = (*felt.Felt)(n)
		}
		// Link the parent and child
		switch p := parent.(type) {
		case *trienode.EdgeNode:
			p.Child = child
		case *trienode.BinaryNode:
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
func unsetInternal(n trienode.Node, left, right *Path) (bool, error) {
	// Step down to the fork point. There are two scenarios that can happen:
	// - the fork point is an EdgeNode: either the key of left proof or
	//   right proof doesn't match with the edge node's path
	// - the fork point is a BinaryNode: both two edge proofs are allowed
	//   to point to a non-existent key
	var (
		pos    uint8
		parent trienode.Node

		// fork indicator for edge nodes
		edgeForkLeft, edgeForkRight int
	)

	for {
		switch rn := n.(type) {
		case *trienode.EdgeNode:
			rn.Flags = trienode.NewNodeFlag()

			if left.Len()-pos < rn.Path.Len() {
				edgeForkLeft = new(Path).LSBs(left, pos).Cmp(rn.Path)
			} else {
				subKey := new(Path).Subset(left, pos, pos+rn.Path.Len())
				edgeForkLeft = subKey.Cmp(rn.Path)
			}

			if right.Len()-pos < rn.Path.Len() {
				edgeForkRight = new(Path).LSBs(right, pos).Cmp(rn.Path)
			} else {
				subKey := new(Path).Subset(right, pos, pos+rn.Path.Len())
				edgeForkRight = subKey.Cmp(rn.Path)
			}

			if edgeForkLeft != 0 || edgeForkRight != 0 {
				return handleEdgeFork(rn, parent, left, right, pos, edgeForkLeft, edgeForkRight)
			}

			parent = n
			n = rn.Child
			pos += rn.Path.Len()
		case *trienode.BinaryNode:
			rn.Flags = trienode.NewNodeFlag()

			leftnode, rightnode := rn.Children[left.Bit(pos)], rn.Children[right.Bit(pos)]
			if leftnode == nil || rightnode == nil || leftnode != rightnode {
				return handleBinaryFork(rn, left, right, pos)
			}
			parent = n
			n = rn.Children[left.Bit(pos)]
			pos++
		default:
			panic(fmt.Sprintf("%T: invalid node: %v", n, n))
		}
	}
}

// handleEdgeFork processes the fork point when it's an EdgeNode
func handleEdgeFork(
	n *trienode.EdgeNode,
	parent trienode.Node,
	left, right *Path,
	pos uint8,
	edgeForkLeft, edgeForkRight int,
) (bool, error) {
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
		parent.(*trienode.BinaryNode).Children[left.Bit(pos-1)] = nil
		return false, nil
	}
	// Only one proof points to non-existent key
	if edgeForkRight != 0 {
		if _, ok := n.Child.(*trienode.ValueNode); ok {
			// The fork point is root node, unset the entire trie
			if parent == nil {
				return true, nil
			}
			parent.(*trienode.BinaryNode).Children[left.Bit(pos-1)] = nil
			return false, nil
		}
		return false, unset(n, n.Child, new(Path).LSBs(left, pos), n.Path.Len(), false)
	}
	if edgeForkLeft != 0 {
		if _, ok := n.Child.(*trienode.ValueNode); ok {
			// The fork point is root node, unset the entire trie
			if parent == nil {
				return true, nil
			}
			parent.(*trienode.BinaryNode).Children[right.Bit(pos-1)] = nil
			return false, nil
		}
		return false, unset(n, n.Child, new(Path).LSBs(right, pos), n.Path.Len(), true)
	}
	return false, nil
}

// handleBinaryFork processes the fork point when it's a BinaryNode
func handleBinaryFork(n *trienode.BinaryNode, left, right *Path, pos uint8) (bool, error) {
	leftBit := left.Bit(pos)
	rightBit := right.Bit(pos)
	if leftBit == 0 && rightBit == 0 {
		n.Children[1] = nil
	}
	if leftBit == 1 && rightBit == 1 {
		n.Children[0] = nil
	}
	if err := unset(n, n.Children[leftBit], new(Path).LSBs(left, pos), 1, false); err != nil {
		return false, err
	}
	if err := unset(n, n.Children[rightBit], new(Path).LSBs(right, pos), 1, true); err != nil {
		return false, err
	}
	return false, nil
}

// unset removes all internal node references either the left most or right most.
func unset(parent, child trienode.Node, key *Path, pos uint8, removeLeft bool) error {
	switch cld := child.(type) {
	case *trienode.BinaryNode:
		keyBit := key.Bit(pos)
		cld.Flags = trienode.NewNodeFlag() // Mark dirty
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

	case *trienode.EdgeNode:
		keyPos := new(Path).LSBs(key, pos)
		keyBit := key.Bit(pos - 1)
		if !cld.PathMatches(keyPos) {
			// We append zeros to the path to match the length of the remaining key
			// The key length is guaranteed to be >= path length
			edgePath := new(Path).AppendZeros(cld.Path, keyPos.Len()-cld.Path.Len())
			// Found fork point, non-existent branch
			if removeLeft {
				if edgePath.Cmp(keyPos) < 0 {
					// Edge node path is in range, unset entire branch
					parent.(*trienode.BinaryNode).Children[keyBit] = nil
				}
			} else {
				if edgePath.Cmp(keyPos) > 0 {
					parent.(*trienode.BinaryNode).Children[keyBit] = nil
				}
			}
			return nil
		}
		if _, ok := cld.Child.(*trienode.ValueNode); ok {
			parent.(*trienode.BinaryNode).Children[keyBit] = nil
			return nil
		}
		cld.Flags = trienode.NewNodeFlag()
		return unset(cld, cld.Child, key, pos+cld.Path.Len(), removeLeft)

	case nil, *trienode.HashNode, *trienode.ValueNode:
		// Child is nil, nothing to unset
		return nil
	default:
		panic("it shouldn't happen") // HashNode, ValueNode
	}
}

// hasRightElement checks if there is a right sibling for the given key in the trie.
// This function assumes that the entire path has been resolved.
func hasRightElement(node trienode.Node, key *Path) bool {
	for node != nil {
		switch n := node.(type) {
		case *trienode.BinaryNode:
			bit := key.MSB()
			if bit == 0 && n.Right() != nil {
				// right sibling exists
				return true
			}
			node = n.Children[bit]
			key.LSBs(key, 1)
		case *trienode.EdgeNode:
			if !n.PathMatches(key) {
				// There's a divergence in the path, check if the node path is greater than the key
				// If so, that means that this node comes after the search key, which indicates that
				// there are elements with larger values
				var edgePath *Path
				if key.Len() > n.Path.Len() {
					edgePath = new(Path).AppendZeros(n.Path, key.Len()-n.Path.Len())
				} else {
					edgePath = n.Path
				}
				return edgePath.Cmp(key) > 0
			}
			node = n.Child
			key.LSBs(key, n.Path.Len())
		case *trienode.ValueNode:
			return false // resolved the whole path
		default:
			panic(fmt.Sprintf("unknown node type: %T", n))
		}
	}
	return false
}

// Resolves the whole path of the given key and node.
// If skipResolved is true, it will only return the immediate child node of the current node
func get(rn trienode.Node, key *Path, skipResolved bool) trienode.Node {
	for {
		switch n := rn.(type) {
		case *trienode.EdgeNode:
			if !n.PathMatches(key) {
				return nil
			}
			rn = n.Child
			key.LSBs(key, n.Path.Len())
		case *trienode.BinaryNode:
			bit := key.MSB()
			rn = n.Children[bit]
			key.LSBs(key, 1)
		case *trienode.HashNode:
			return n
		case *trienode.ValueNode:
			return n
		case nil:
			return nil
		}

		if !skipResolved {
			return rn
		}
	}
}
