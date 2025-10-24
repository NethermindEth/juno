package trie

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, ProofNode]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, ProofNode]()
}

type ProofNode interface {
	Hash(hash crypto.HashFn) *felt.Felt
	Len() uint8
	String() string
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

func (b *Binary) Hash(hash crypto.HashFn) *felt.Felt {
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
	Path  *BitArray  // path from parent to child
}

func (e *Edge) Hash(hash crypto.HashFn) *felt.Felt {
	var length [32]byte
	length[31] = e.Path.len
	pathFelt := e.Path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length[:])
	// TODO: no need to return reference, just return value to avoid heap allocation
	return new(felt.Felt).Add(hash(e.Child, &pathFelt), lengthFelt)
}

func (e *Edge) Len() uint8 {
	return e.Path.Len()
}

func (e *Edge) String() string {
	return fmt.Sprintf("Edge: %v:\n\tChild: %v\n\tPath: %v\n", e.Hash(crypto.Pedersen), e.Child, e.Path)
}

// Prove generates a Merkle proof for a given key in the trie.
// The result contains the proof nodes on the path from the root to the leaf.
// The value is included in the proof if the key is present in the trie.
// If the key is not present, the proof will contain the nodes on the path to the closest ancestor.
func (t *Trie) Prove(key *felt.Felt, proof *ProofNodeSet) error {
	k := t.FeltToKey(key)

	nodesFromRoot, err := t.nodesFromRoot(&k)
	if err != nil {
		return err
	}

	var parentKey *BitArray

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
func VerifyProof(root, keyFelt *felt.Felt, proof *ProofNodeSet, hash crypto.HashFn) (*felt.Felt, error) {
	keyBits := new(BitArray).SetFelt(globalTrieHeight, keyFelt)
	expectedHash := root

	var curPos uint8
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
			if keyBits.Len() <= curPos {
				return nil, fmt.Errorf("key length less than current position, key length: %d, current position: %d", keyBits.Len(), curPos)
			}
			// Determine the next node to traverse based on the next bit position
			expectedHash = node.LeftHash
			if keyBits.IsBitSet(curPos) {
				expectedHash = node.RightHash
			}
			curPos++
		case *Edge: // Edge nodes represent paths between binary nodes
			if !verifyEdgePath(keyBits, node.Path, curPos) {
				return &felt.Zero, nil
			}

			// Move to the immediate child node
			curPos += node.Path.Len()
			expectedHash = node.Child
		}

		// We've consumed all bits in our path
		if curPos >= keyBits.Len() {
			return expectedHash, nil
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
//
// TODO(weiihann): Given a binary leaf and a left-sibling first key, if the right sibling is removed, the proof would still be valid.
// Conversely, given a binary leaf and a right-sibling last key, if the left sibling is removed, the proof would still be valid.
// Range proof should not be valid for both of these cases, but currently is, which is an attack vector.
// The problem probably lies in how we do root hash calculation.
func VerifyRangeProof(root, first *felt.Felt, keys, values []*felt.Felt, proof *ProofNodeSet) (bool, error) { //nolint:funlen,gocyclo
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
		tr, err := buildTrie(globalTrieHeight, nil, nil, keys, values)
		if err != nil {
			return false, err
		}

		recomputedRoot, err := tr.Hash()
		if err != nil {
			return false, err
		}

		if !recomputedRoot.Equal(root) {
			return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", root.String(), recomputedRoot.String())
		}

		return false, nil // no more elements available
	}

	nodes := NewStorageNodeSet()
	firstKey := new(BitArray).SetFelt(globalTrieHeight, first)

	// Special case: there is a provided proof but no key-value pairs, make sure regenerated trie has no more values
	// Empty range proof with more elements on the right is not accepted in this function.
	// This is due to snap sync specification detail, where the responder must send an existing key (if any) if the requested range is empty.
	if len(keys) == 0 {
		rootKey, val, err := proofToPath(root, firstKey, proof, nodes)
		if err != nil {
			return false, err
		}

		if val != nil || hasRightElement(rootKey, firstKey, nodes) {
			return false, errors.New("more entries available")
		}

		return false, nil
	}

	last := keys[len(keys)-1]
	lastKey := new(BitArray).SetFelt(globalTrieHeight, last)

	// Special case: there is only one element and two edge keys are the same
	if len(keys) == 1 && firstKey.Equal(lastKey) {
		rootKey, val, err := proofToPath(root, firstKey, proof, nodes)
		if err != nil {
			return false, err
		}

		elementKey := new(BitArray).SetFelt(globalTrieHeight, keys[0])
		if !firstKey.Equal(elementKey) {
			return false, errors.New("correct proof but invalid key")
		}

		if val == nil || !values[0].Equal(val) {
			return false, errors.New("correct proof but invalid value")
		}

		return hasRightElement(rootKey, firstKey, nodes), nil
	}

	// In all other cases, we require two edge paths available.
	// First, ensure that the last key is greater than the first key
	if last.Cmp(first) <= 0 {
		return false, errors.New("last key is less than first key")
	}

	rootKey, _, err := proofToPath(root, firstKey, proof, nodes)
	if err != nil {
		return false, err
	}

	lastRootKey, _, err := proofToPath(root, lastKey, proof, nodes)
	if err != nil {
		return false, err
	}

	if !rootKey.Equal(lastRootKey) {
		return false, errors.New("first and last root keys do not match")
	}

	// Build the trie from the proof paths
	tr, err := buildTrie(globalTrieHeight, rootKey, nodes.List(), keys, values)
	if err != nil {
		return false, err
	}

	// Verify that the recomputed root hash matches the provided root hash
	recomputedRoot, err := tr.Hash()
	if err != nil {
		return false, err
	}

	if !recomputedRoot.Equal(root) {
		return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", root.String(), recomputedRoot.String())
	}

	return hasRightElement(rootKey, lastKey, nodes), nil
}

// isEdge checks if the storage node is an edge node.
func isEdge(parentKey *BitArray, sNode StorageNode) bool {
	sNodeLen := sNode.key.len
	if parentKey == nil { // Root
		return sNodeLen != 0
	}
	return sNodeLen-parentKey.len > 1
}

// storageNodeToProofNode converts a StorageNode to the ProofNode(s).
// Juno's Trie has nodes that are Binary AND Edge, whereas the protocol requires nodes that are Binary XOR Edge.
// We need to convert the former to the latter for proof generation.
func storageNodeToProofNode(tri *Trie, parentKey *BitArray, sNode StorageNode) (*Edge, *Binary, error) {
	var edge *Edge
	if isEdge(parentKey, sNode) {
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

// proofToPath converts a Merkle proof to trie node path. All necessary nodes will be resolved and leave the remaining
// as hashes. The given edge proof can be existent or non-existent.
func proofToPath(root *felt.Felt, keyBits *BitArray, proof *ProofNodeSet, nodes *StorageNodeSet) (*BitArray, *felt.Felt, error) {
	rootKey, val, err := buildPath(root, keyBits, 0, nil, proof, nodes)
	if err != nil {
		return nil, nil, err
	}

	// Special case: non-existent key at the root
	// We must include the root node in the node set.
	// We will only get the following two cases:
	// 1. The root node is an edge node only where path.len == key.len (single key trie)
	// 2. The root node is an edge node + binary node (double key trie)
	if nodes.Size() == 0 {
		proofNode, ok := proof.Get(*root)
		if !ok {
			return nil, nil, fmt.Errorf("root proof node not found: %s", root)
		}

		edge, ok := proofNode.(*Edge)
		if !ok {
			return nil, nil, fmt.Errorf("expected edge node at root, got: %T", proofNode)
		}

		sn := NewPartialStorageNode(edge.Path, edge.Child)

		// Handle leaf edge case (single key trie)
		if edge.Path.Len() == keyBits.Len() {
			if err := nodes.Put(*sn.key, sn); err != nil {
				return nil, nil, fmt.Errorf("failed to store leaf edge: %w", err)
			}
			return sn.Key(), sn.Value(), nil
		}

		// Handle edge + binary case (double key trie)
		child, ok := proof.Get(*edge.Child)
		if !ok {
			return nil, nil, fmt.Errorf("edge child not found: %s", edge.Child)
		}

		binary, ok := child.(*Binary)
		if !ok {
			return nil, nil, fmt.Errorf("expected binary node as child, got: %T", child)
		}
		sn.node.LeftHash = binary.LeftHash
		sn.node.RightHash = binary.RightHash

		if err := nodes.Put(*sn.key, sn); err != nil {
			return nil, nil, fmt.Errorf("failed to store edge+binary: %w", err)
		}
		rootKey = sn.Key()
	}

	return rootKey, val, nil
}

// buildPath recursively builds the path for a given node hash, key, and current position.
// It returns the current node's key and any leaf value found along this path.
func buildPath(
	nodeHash *felt.Felt,
	key *BitArray,
	curPos uint8,
	curNode *StorageNode,
	proof *ProofNodeSet,
	nodes *StorageNodeSet,
) (*BitArray, *felt.Felt, error) {
	// We reached the leaf
	if curPos == key.Len() {
		leafKey := key.Copy()
		leafNode := NewPartialStorageNode(&leafKey, nodeHash)
		if err := nodes.Put(leafKey, leafNode); err != nil {
			return nil, nil, err
		}
		return leafNode.Key(), leafNode.Value(), nil
	}

	proofNode, ok := proof.Get(*nodeHash)
	if !ok { // non-existent proof node
		return emptyBitArray, nil, nil
	}

	switch pn := proofNode.(type) {
	case *Binary:
		return handleBinaryNode(pn, nodeHash, key, curPos, curNode, proof, nodes)
	case *Edge:
		return handleEdgeNode(pn, key, curPos, proof, nodes)
	}

	return nil, nil, nil
}

// handleBinaryNode processes a binary node in the proof path by creating/updating a storage node,
// setting its left/right hashes, and recursively building the path for the appropriate child direction.
// It returns the current node's key and any leaf value found along this path.
func handleBinaryNode(
	binary *Binary,
	nodeHash *felt.Felt,
	key *BitArray,
	curPos uint8,
	curNode *StorageNode,
	proof *ProofNodeSet,
	nodes *StorageNodeSet,
) (*BitArray, *felt.Felt, error) {
	// If curNode is nil, it means that this current binary node is the root node.
	// Or, it's an internal binary node and the parent is also a binary node.
	// A standalone binary proof node always corresponds to a single storage node.
	// If curNode is not nil, it means that the parent node is an edge node.
	// In this case, the key of the storage node is based on the parent edge node.
	if curNode == nil {
		curNode = NewPartialStorageNode(new(BitArray).MSBs(key, curPos), nodeHash)
	}
	curNode.node.LeftHash = binary.LeftHash
	curNode.node.RightHash = binary.RightHash

	// Calculate next position and determine to take left or right path
	nextPos := curPos + 1
	isRightPath := key.IsBitSet(curPos)
	nextHash := binary.LeftHash
	if isRightPath {
		nextHash = binary.RightHash
	}

	childKey, val, err := buildPath(nextHash, key, nextPos, nil, proof, nodes)
	if err != nil {
		return nil, nil, err
	}

	// Set child reference
	if isRightPath {
		curNode.node.Right = childKey
	} else {
		curNode.node.Left = childKey
	}

	if err := nodes.Put(*curNode.key, curNode); err != nil {
		return nil, nil, fmt.Errorf("failed to store binary node: %w", err)
	}

	return curNode.Key(), val, nil
}

// handleEdgeNode processes an edge node in the proof path by verifying the edge path matches
// the key path and either creating a leaf node or continuing to traverse the trie. It returns
// the current node's key and any leaf value found along this path.
func handleEdgeNode(
	edge *Edge,
	key *BitArray,
	curPos uint8,
	proof *ProofNodeSet,
	nodes *StorageNodeSet,
) (*BitArray, *felt.Felt, error) {
	// Verify the edge path matches the key path
	if !verifyEdgePath(key, edge.Path, curPos) {
		return emptyBitArray, nil, nil
	}

	// The next node position is the end of the edge path
	nextPos := curPos + edge.Path.Len()
	curNode := NewPartialStorageNode(new(BitArray).MSBs(key, nextPos), edge.Child)

	// This is an edge leaf, stop traversing the trie
	if nextPos == key.Len() {
		if err := nodes.Put(*curNode.key, curNode); err != nil {
			return nil, nil, fmt.Errorf("failed to store edge leaf: %w", err)
		}
		return curNode.Key(), curNode.Value(), nil
	}

	_, val, err := buildPath(edge.Child, key, nextPos, curNode, proof, nodes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build child path: %w", err)
	}

	if err := nodes.Put(*curNode.key, curNode); err != nil {
		return nil, nil, fmt.Errorf("failed to store internal edge: %w", err)
	}

	return curNode.Key(), val, nil
}

// verifyEdgePath checks if the edge path matches the key path at the current position.
func verifyEdgePath(key, edgePath *BitArray, curPos uint8) bool {
	return new(BitArray).LSBs(key, curPos).EqualMSBs(edgePath)
}

// buildTrie builds a trie from a list of storage nodes and a list of keys and values.
func buildTrie(height uint8, rootKey *BitArray, nodes []*StorageNode, keys, values []*felt.Felt) (*Trie, error) {
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

// hasRightElement checks if there is a right sibling for the given key in the trie.
// This function assumes that the entire path has been resolved.
func hasRightElement(rootKey, key *BitArray, nodes *StorageNodeSet) bool {
	cur := rootKey
	for cur != nil && !cur.Equal(emptyBitArray) {
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
		isLeft := !key.IsBitSet(cur.Len())
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
