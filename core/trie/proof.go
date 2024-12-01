package trie

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

var (
	ErrUnknownProofNode  = errors.New("unknown proof node")
	ErrChildHashNotFound = errors.New("can't determine the child hash from the parent and child")
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, ProofNode]

type ProofNode interface {
	Hash(hash hashFunc) *felt.Felt
	Len() uint8
	String() string
}

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, ProofNode]()
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

func (b *Binary) Hash(hash hashFunc) *felt.Felt {
	return hash(b.LeftHash, b.RightHash)
}

func (b *Binary) Len() uint8 {
	return 1
}

func (b *Binary) String() string {
	return fmt.Sprintf("Binary: %v:\n\tLeftHash: %v\n\tRightHash: %v\n", b.Hash(crypto.Pedersen), b.LeftHash, b.RightHash)
}

type Edge struct {
	Child *felt.Felt // child hash
	Path  *Key       // path from parent to child
}

func (e *Edge) Hash(hash hashFunc) *felt.Felt {
	length := make([]byte, len(e.Path.bitset))
	length[len(e.Path.bitset)-1] = e.Path.len
	pathFelt := e.Path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length)
	return new(felt.Felt).Add(hash(e.Child, &pathFelt), lengthFelt)
}

func (e *Edge) Len() uint8 {
	return e.Path.Len()
}

func (e *Edge) String() string {
	return fmt.Sprintf("Edge: %v:\n\tChild: %v\n\tPath: %v\n", e.Hash(crypto.Pedersen), e.Child, e.Path)
}

func (t *Trie) Prove(key *felt.Felt, proof *ProofNodeSet) error {
	k := t.FeltToKey(key)

	nodesFromRoot, err := t.nodesFromRoot(&k)
	if err != nil {
		return err
	}

	var parentKey *Key

	for i, sNode := range nodesFromRoot {
		sNodeEdge, sNodeBinary, err := storageNodeToProofNode(t, parentKey, sNode)
		if err != nil {
			return err
		}
		isLeaf := sNode.key.len == t.height

		if sNodeEdge != nil && !isLeaf { // Internal Edge
			proof.Put(*sNodeEdge.Hash(t.hash), sNodeEdge)
			proof.Put(*sNodeBinary.Hash(t.hash), sNodeBinary)
		} else if sNodeEdge == nil && !isLeaf { // Internal Binary
			proof.Put(*sNodeBinary.Hash(t.hash), sNodeBinary)
		} else if sNodeEdge != nil && isLeaf { // Leaf Edge
			proof.Put(*sNodeEdge.Hash(t.hash), sNodeEdge)
		} else if sNodeEdge == nil && sNodeBinary == nil { // sNode is a binary leaf
			break
		}
		parentKey = nodesFromRoot[i].key
	}
	return nil
}

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

func isEdge(parentKey *Key, sNode StorageNode) bool {
	sNodeLen := sNode.key.len
	if parentKey == nil { // Root
		return sNodeLen != 0
	}
	return sNodeLen-parentKey.len > 1
}

// Note: we need to account for the fact that Junos Trie has nodes that are Binary AND Edge,
// whereas the protocol requires nodes that are Binary XOR Edge
func storageNodeToProofNode(tri *Trie, parentKey *Key, sNode StorageNode) (*Edge, *Binary, error) {
	isEdgeBool := isEdge(parentKey, sNode)

	var edge *Edge
	if isEdgeBool {
		edgePath := path(sNode.key, parentKey)
		edge = &Edge{
			Path:  &edgePath,
			Child: sNode.node.Value,
		}
	}
	if sNode.key.len == tri.height { // Leaf
		return edge, nil, nil
	}
	lNode, err := tri.GetNodeFromKey(sNode.node.Left)
	if err != nil {
		return nil, nil, err
	}
	rNode, err := tri.GetNodeFromKey(sNode.node.Right)
	if err != nil {
		return nil, nil, err
	}

	rightHash := rNode.Value
	if isEdge(sNode.key, StorageNode{node: rNode, key: sNode.node.Right}) {
		edgePath := path(sNode.node.Right, sNode.key)
		rEdge := &Edge{
			Path:  &edgePath,
			Child: rNode.Value,
		}
		rightHash = rEdge.Hash(tri.hash)
	}
	leftHash := lNode.Value
	if isEdge(sNode.key, StorageNode{node: lNode, key: sNode.node.Left}) {
		edgePath := path(sNode.node.Left, sNode.key)
		lEdge := &Edge{
			Path:  &edgePath,
			Child: lNode.Value,
		}
		leftHash = lEdge.Hash(tri.hash)
	}
	binary := &Binary{
		LeftHash:  leftHash,
		RightHash: rightHash,
	}

	return edge, binary, nil
}

// VerifyProof verifies that a proof path is valid for a given key in a binary trie.
// It walks through the proof nodes, verifying each step matches the expected path to reach the key.
//
// The verification process:
// 1. Starts at the root hash and retrieves the corresponding proof node
// 2. For each proof node:
//   - Verifies the node's computed hash matches the expected hash
//   - For Binary nodes:
//     -- Uses the next unprocessed bit in the key to choose left/right path
//     -- If key bit is 0, takes left path; if 1, takes right path
//   - For Edge nodes:
//     -- Verifies the compressed path matches the corresponding bits in the key
//     -- Moves to the child node if paths match
//
// 3. Continues until all bits in the key are processed
//
// The proof is considered invalid if:
//   - Any proof node is missing from the OrderedSet
//   - Any node's computed hash doesn't match its expected hash
//   - The path bits don't match the key bits
//   - The proof ends before processing all key bits
func VerifyProof(root *felt.Felt, key *Key, proof *ProofNodeSet, hash hashFunc) (*felt.Felt, error) {
	expectedHash := root
	keyLen := key.Len()
	var processedBits uint8

	for {
		proofNode, ok := proof.Get(*expectedHash)
		if !ok {
			return nil, fmt.Errorf("proof node not found, expected hash: %s", expectedHash.String())
		}

		// Verify the hash matches
		if !proofNode.Hash(hash).Equal(expectedHash) {
			return nil, fmt.Errorf("proof node hash mismatch, expected hash: %s, got hash: %s", expectedHash.String(), proofNode.Hash(hash).String())
		}

		switch node := proofNode.(type) {
		case *Binary: // Binary nodes represent left/right choices
			if key.Len() <= processedBits {
				return nil, fmt.Errorf("key length less than processed bits, key length: %d, processed bits: %d", key.Len(), processedBits)
			}
			// Check the bit at parent's position
			expectedHash = node.LeftHash
			if key.IsBitSet(keyLen - processedBits - 1) {
				expectedHash = node.RightHash
			}
			processedBits++
		case *Edge: // Edge nodes represent paths between binary nodes
			nodeLen := node.Path.Len()

			if key.Len() < processedBits+nodeLen {
				// Key is shorter than the path - this proves non-membership
				return &felt.Zero, nil
			}

			// Ensure the bits between segment of the key and the node path match
			start := keyLen - processedBits - nodeLen
			end := keyLen - processedBits
			for i := start; i < end; i++ { // check if the bits match
				if key.IsBitSet(i) != node.Path.IsBitSet(i-start) {
					return &felt.Zero, nil // paths diverge - this proves non-membership
				}
			}

			processedBits += nodeLen
			expectedHash = node.Child
		}

		// We've consumed all bits in our path
		if processedBits >= keyLen {
			return expectedHash, nil
		}
	}
}

// VerifyRangeProof verifies the range proof for the given range of keys.
// This is achieved by constructing a trie from the boundary proofs, and the supplied key-values.
// If the root of the reconstructed trie matches the supplied root, then the verification passes.
// If the trie is constructed incorrectly then the root will have an incorrect key(len,path), and value,
// and therefore its hash won't match the expected root.
// ref: https://github.com/ethereum/go-ethereum/blob/v1.14.3/trie/proof.go#L484
func VerifyRangeProof(root *felt.Felt, first *felt.Felt, keys, values []*felt.Felt, proof *ProofNodeSet) (bool, error) {
	// Ensure the number of keys and values are the same
	if len(keys) != len(values) {
		return false, fmt.Errorf("inconsistent proof data, number of keys: %d, number of values: %d", len(keys), len(values))
	}

	// Ensure all keys are monotonic increasing and values contain no deletions
	for i := 0; i < len(keys); i++ {
		if i < len(keys)-1 && keys[i].Cmp(keys[i+1]) > 0 {
			return false, errors.New("keys are not monotonic increasing")
		}

		if values[i].Equal(&felt.Zero) {
			return false, errors.New("range contains deletion")
		}
	}

	// Special case: no edge proof at all, given range is the whole leaf set in the trie
	if proof == nil {
		tr, err := BuildTrie(globalTrieHeight, nil, nil, keys, values)
		if err != nil {
			return false, err
		}

		recomputedRoot, err := tr.Root()
		if err != nil {
			return false, err
		}

		if !recomputedRoot.Equal(root) {
			return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", root.String(), recomputedRoot.String())
		}

		return false, nil // no more elements available
	}

	nodes := NewStorageNodeSet()
	firstKey := FeltToKey(globalTrieHeight, first)

	// Special case: there is a provided proof but no key-value pairs, make sure regenerated trie has no more values
	if len(keys) == 0 {
		rootKey, val, err := proofToPath(root, &firstKey, proof, nodes)
		if err != nil {
			return false, err
		}

		if val != nil || hasRightElement(rootKey, &firstKey, nodes) {
			return false, errors.New("more entries available")
		}

		return false, nil
	}

	last := keys[len(keys)-1]
	lastKey := FeltToKey(globalTrieHeight, last)

	// Special case: there is only one element and two edge keys are the same
	if len(keys) == 1 && firstKey.Equal(&lastKey) {
		rootKey, val, err := proofToPath(root, &firstKey, proof, nodes)
		if err != nil {
			return false, err
		}

		elementKey := FeltToKey(globalTrieHeight, keys[0])
		if !firstKey.Equal(&elementKey) {
			return false, errors.New("correct proof but invalid key")
		}

		if val == nil || !values[0].Equal(val) {
			return false, errors.New("correct proof but invalid value")
		}

		return hasRightElement(rootKey, &firstKey, nodes), nil
	}

	// In all other cases, we require two edge paths available.
	// First, ensure that the last key is greater than the first key
	if last.Cmp(first) <= 0 {
		return false, errors.New("last key is less than first key")
	}

	rootKey, _, err := proofToPath(root, &firstKey, proof, nodes)
	if err != nil {
		return false, err
	}

	// TODO(weiihann): handle comments
	// Gapped range proof succeeds if we have binary leafs.
	// Given a binary leaf, we store both the left and right child hashes, where they could be the leafs.
	// So if a key is gapped but is present in the proof as a binary leaf, the proof would still be valid.
	// So we need to check if a binary leaf exists, and check if the next/previous key exists.

	lastRootKey, _, err := proofToPath(root, &lastKey, proof, nodes)
	if err != nil {
		return false, err
	}

	if !rootKey.Equal(lastRootKey) {
		return false, errors.New("first and last root keys do not match")
	}

	// Build the trie from the proof paths
	tr, err := BuildTrie(globalTrieHeight, rootKey, nodes.List(), keys, values)
	if err != nil {
		return false, err
	}

	// Verify that the recomputed root hash matches the provided root hash
	recomputedRoot, err := tr.Root()
	if err != nil {
		return false, err
	}

	if !recomputedRoot.Equal(root) {
		return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", root.String(), recomputedRoot.String())
	}

	return hasRightElement(rootKey, &lastKey, nodes), nil
}

func proofToPath(root *felt.Felt, key *Key, proof *ProofNodeSet, nodes *StorageNodeSet) (*Key, *felt.Felt, error) {
	rootKey, val, err := buildPath(root, key, 0, nil, proof, nodes)
	if err != nil {
		return nil, nil, err
	}

	// Special case: non-existent key at the root
	// We still need to include the root node in the node set.
	// It's guaranteed that we will only get the following two cases:
	// 1. The root node is an edge node only where path.len == key.len (single key trie)
	// 2. The root node is an edge node + binary node
	if nodes.Size() == 0 {
		proofNode, ok := proof.Get(*root)
		if !ok {
			return nil, nil, fmt.Errorf("proof node (hash: %s) not found", root.String())
		}

		edge, ok := proofNode.(*Edge)
		if !ok {
			return nil, nil, fmt.Errorf("proof node (hash: %s) is not an edge", root.String())
		}

		if edge.Path.Len() == key.Len() {
			if err := nodes.Put(*edge.Path, &StorageNode{
				key: edge.Path,
				node: &Node{
					Value:     edge.Child,
					Left:      NilKey,
					Right:     NilKey,
					LeftHash:  nil,
					RightHash: nil,
				},
			}); err != nil {
				return nil, nil, err
			}
			return edge.Path, nil, nil
		}

		child, ok := proof.Get(*edge.Child)
		if !ok {
			return nil, nil, fmt.Errorf("proof node (hash: %s) not found", edge.Child.String())
		}

		binary, ok := child.(*Binary)
		if !ok {
			return nil, nil, fmt.Errorf("proof node's child (hash: %s) is not a binary", edge.Child.String())
		}

		if err := nodes.Put(*edge.Path, &StorageNode{
			key: edge.Path,
			node: &Node{
				Value:     edge.Child,
				Left:      NilKey,
				Right:     NilKey,
				LeftHash:  binary.LeftHash,
				RightHash: binary.RightHash,
			},
		}); err != nil {
			return nil, nil, err
		}
		rootKey = edge.Path

		// TODO(weiihann): handle binary leaf
	}

	return rootKey, val, nil
}

func buildPath(
	nodeHash *felt.Felt,
	key *Key,
	curPos uint8,
	curNode *StorageNode,
	proof *ProofNodeSet,
	nodes *StorageNodeSet,
) (*Key, *felt.Felt, error) {
	// We reached the leaf
	if curPos == key.Len() {
		leafKey := key.Copy()
		leafNode := &StorageNode{
			key: &leafKey,
			node: &Node{
				Left:      NilKey,
				Right:     NilKey,
				LeftHash:  nil,
				RightHash: nil,
				Value:     nodeHash,
			},
		}
		if err := nodes.Put(leafKey, leafNode); err != nil {
			return nil, nil, err
		}
		return &leafKey, nodeHash, nil
	}

	proofNode, ok := proof.Get(*nodeHash)
	if !ok { // non-existent proof node
		return NilKey, nil, nil
	}

	switch pn := proofNode.(type) {
	case *Binary:
		if curNode == nil {
			nodeKey, err := key.MostSignificantBits(curPos)
			if err != nil {
				return nil, nil, err
			}
			curNode = &StorageNode{
				key:  nodeKey,
				node: &Node{Value: nodeHash, Right: NilKey, Left: NilKey},
			}
		}
		curNode.node.LeftHash = pn.LeftHash
		curNode.node.RightHash = pn.RightHash

		// Calculate next position and determine path
		nextPos := curPos + 1
		nextBitIndex := key.Len() - nextPos
		isRightPath := key.IsBitSet(nextBitIndex)

		// Choose next hash based on path
		nextHash := pn.LeftHash
		if isRightPath {
			nextHash = pn.RightHash
		}

		// Recursively build the child path
		childKey, val, err := buildPath(nextHash, key, nextPos, nil, proof, nodes)
		if err != nil {
			return nil, nil, err
		}

		// TODO(weiihann): handle binary leaf and edge leaf cases

		// Set child reference and store node
		if isRightPath {
			curNode.node.Right = childKey
		} else {
			curNode.node.Left = childKey
		}

		// Store the node and return its key
		if err := nodes.Put(*curNode.key, curNode); err != nil {
			return nil, nil, err
		}
		return curNode.Key(), val, nil

	case *Edge:
		if curNode == nil {
			curNode = &StorageNode{node: &Node{Right: NilKey, Left: NilKey}}
		}
		curNode.node.Value = pn.Child

		nextPos := curPos + pn.Path.Len()
		if key.Len() < nextPos {
			return NilKey, nil, nil
		}

		// Ensure the bits between segment of the key and the node path match
		start := key.Len() - nextPos
		end := key.Len() - curPos
		for i := start; i < end; i++ {
			if key.IsBitSet(i) != pn.Path.IsBitSet(i-start) {
				return NilKey, nil, nil
			}
		}

		// If path reaches the key length, this is an edge leaf
		if nextPos == key.Len() {
			leafKey := key.Copy()
			curNode.key = &leafKey
			if err := nodes.Put(leafKey, curNode); err != nil {
				return nil, nil, err
			}
			return &leafKey, pn.Child, nil
		}

		// Set the current node's key
		nodeKey, err := key.MostSignificantBits(nextPos)
		if err != nil {
			return nil, nil, err
		}
		curNode.key = nodeKey

		// Recursively build the child path
		_, val, err := buildPath(pn.Child, key, nextPos, curNode, proof, nodes)
		if err != nil {
			return nil, nil, err
		}

		return curNode.key, val, nil
	}

	return nil, nil, nil
}

func BuildTrie(height uint8, rootKey *Key, nodes []*StorageNode, keys, values []*felt.Felt) (*Trie, error) {
	tr, err := NewTriePedersen(newMemStorage(), height)
	if err != nil {
		return nil, err
	}

	tr.setRootKey(rootKey)

	// Nodes are inserted in reverse order because the leaf nodes are placed at the front of the list.
	// We would want to insert root node first so the root key is set first.
	for i := len(nodes) - 1; i >= 0; i-- {
		if err := tr.PutInner(nodes[i].key, nodes[i].node); err != nil {
			return nil, err
		}
	}

	for index, key := range keys {
		_, err = tr.PutWithProof(key, values[index], nodes)
		if err != nil {
			return nil, err
		}
	}

	return tr, nil
}

func hasRightElement(rootKey, key *Key, nodes *StorageNodeSet) bool {
	cur := rootKey
	for cur != nil && !cur.Equal(NilKey) {
		sn, ok := nodes.Get(*cur)
		if !ok {
			return false
		}

		// We resolved the entire path, no more elements
		if key.Equal(cur) {
			return false
		}

		// If we're taking a left path and there's a right sibling,
		// then there are elements with larger values
		bitPos := key.Len() - cur.Len() - 1
		isLeft := !key.IsBitSet(bitPos)
		if isLeft && sn.node.RightHash != nil {
			return true
		}

		// Move to next node based on the path
		cur = sn.node.Right
		if isLeft {
			cur = sn.node.Left
		}
	}

	return false
}
