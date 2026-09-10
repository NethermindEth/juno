package trie

import (
	"errors"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, ProofNode]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, ProofNode]()
}

type ProofNode interface {
	Hash(hash crypto.HashFn) felt.Felt
	Len() uint8
	String() string
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

func (b *Binary) Hash(hash crypto.HashFn) felt.Felt {
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

func (e *Edge) Hash(hash crypto.HashFn) felt.Felt {
	var length [32]byte
	length[31] = e.Path.len
	pathFelt := e.Path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length[:])

	hashPath := hash(e.Child, &pathFelt)
	hashPath.Add(&hashPath, lengthFelt)
	return hashPath
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
// Proof hashes come from stored node values: call Hash after the last write.
func (t *Trie) Prove(key *felt.Felt, proof *ProofNodeSet) error {
	if err := t.ensureNoUnhashedWrites(); err != nil {
		return err
	}

	k := t.FeltToKey(key)

	nodesFromRoot, err := t.nodesFromRoot(&k)
	if err != nil {
		return err
	}

	var parentKey *BitArray
	// Hash of the current node as seen by its parent; nil for the root.
	var carriedHash *felt.Felt

	// Proof entries alias node felts, so the nodes cannot go back to nodePool.
	for i, sNode := range nodesFromRoot {
		isLeaf := sNode.key.len == t.height

		var onPathChild *StorageNode
		if !isLeaf && i+1 < len(nodesFromRoot) {
			onPathChild = &nodesFromRoot[i+1]
		}
		sNodeBinary, err := t.addProofNode(parentKey, sNode, carriedHash, proof, onPathChild)
		if err != nil {
			return err
		}

		if isLeaf {
			break // binary leaf; nothing to add
		}

		// Carry the on-path child's hash; a nil carry only costs a recomputation.
		carriedHash = nil
		switch {
		case onPathChild == nil:
		case onPathChild.key.Equal(sNode.node.Left):
			carriedHash = sNodeBinary.LeftHash
		case onPathChild.key.Equal(sNode.node.Right):
			carriedHash = sNodeBinary.RightHash
		}
		parentKey = sNode.key
	}
	return nil
}

// ProveMulti generates Merkle proofs for multiple keys with a shared trie traversal.
// Keys are sorted by trie path internally so shared prefixes are read once.
func (t *Trie) ProveMulti(keys []felt.Felt, proof *ProofNodeSet) error {
	if err := t.ensureNoUnhashedWrites(); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	trieKeys := make([]BitArray, len(keys))
	for i := range keys {
		trieKeys[i] = t.FeltToKey(&keys[i])
	}

	// FeltToKey returns the trie path, so this order groups keys with shared
	// prefixes and lets proveMultiFrom split each subtree as a contiguous range.
	slices.SortFunc(trieKeys, func(a, b BitArray) int {
		return a.Cmp(&b)
	})

	dedupedKeys := trieKeys[:0]
	for i := range trieKeys {
		if len(dedupedKeys) == 0 || !trieKeys[i].Equal(&dedupedKeys[len(dedupedKeys)-1]) {
			dedupedKeys = append(dedupedKeys, trieKeys[i])
		}
	}

	return t.proveMultiFrom(t.rootKey, nil, dedupedKeys, nil, nil, proof)
}

func (t *Trie) ensureNoUnhashedWrites() error {
	if t.rootKeyIsDirty || len(t.dirtyNodes) > 0 {
		return errors.New("cannot prove a trie with unhashed writes")
	}
	return nil
}

func (t *Trie) proveMultiFrom(
	cur, parentKey *BitArray,
	keys []BitArray,
	carriedHash *felt.Felt,
	knownNode *Node,
	proof *ProofNodeSet,
) error {
	if shouldSkipMultiProofNode(cur, parentKey, keys) {
		return nil
	}

	node, err := t.proofNode(cur, knownNode)
	if err != nil {
		return err
	}

	continuingKeys := multiProofKeysForNode(cur, keys)
	leftKeys, rightKeys := splitKeysByBit(continuingKeys, cur.Len())

	knownChildren, leftNode, rightNode, err := t.readKnownProofChildren(node, leftKeys, rightKeys)
	if err != nil {
		return err
	}

	binary, err := t.addProofNode(
		parentKey,
		StorageNode{key: cur, node: node},
		carriedHash,
		proof,
		knownChildren...,
	)
	if err != nil {
		return err
	}

	if cur.Len() >= t.height || len(continuingKeys) == 0 {
		return nil
	}

	var leftHash *felt.Felt
	if binary != nil {
		leftHash = binary.LeftHash
	}
	if err := t.proveMultiFrom(node.Left, cur, leftKeys, leftHash, leftNode, proof); err != nil {
		return err
	}

	var rightHash *felt.Felt
	if binary != nil {
		rightHash = binary.RightHash
	}
	return t.proveMultiFrom(node.Right, cur, rightKeys, rightHash, rightNode, proof)
}

func shouldSkipMultiProofNode(cur, parentKey *BitArray, keys []BitArray) bool {
	if cur == nil || len(keys) == 0 {
		return true
	}
	// Proof nodes set "nil" nodes to zero. This mirrors nodesFromRoot for non-root children.
	return parentKey != nil && cur.len == 0
}

func (t *Trie) proofNode(cur *BitArray, knownNode *Node) (*Node, error) {
	if knownNode != nil {
		return knownNode, nil
	}
	return t.readStorage.Get(cur)
}

func multiProofKeysForNode(cur *BitArray, keys []BitArray) []BitArray {
	continuingKeys := keys[:0]
	for i := range keys {
		if cur.Len() < keys[i].Len() && keys[i].EqualMSBs(cur) {
			continuingKeys = append(continuingKeys, keys[i])
		}
	}
	return continuingKeys
}

func splitKeysByBit(keys []BitArray, bitIndex uint8) ([]BitArray, []BitArray) {
	rightStart := len(keys)
	for i := range keys {
		if keys[i].IsBitSet(bitIndex) {
			rightStart = i
			break
		}
	}
	return keys[:rightStart], keys[rightStart:]
}

func (t *Trie) readKnownProofChildren(
	node *Node,
	leftKeys, rightKeys []BitArray,
) ([]*StorageNode, *Node, *Node, error) {
	var knownChildren []*StorageNode

	leftNode, leftChild, err := t.readKnownProofChild(leftKeys, node.Left)
	if err != nil {
		return nil, nil, nil, err
	}
	if leftChild != nil {
		knownChildren = append(knownChildren, leftChild)
	}

	rightNode, rightChild, err := t.readKnownProofChild(rightKeys, node.Right)
	if err != nil {
		return nil, nil, nil, err
	}
	if rightChild != nil {
		knownChildren = append(knownChildren, rightChild)
	}

	return knownChildren, leftNode, rightNode, nil
}

func (t *Trie) readKnownProofChild(
	keys []BitArray,
	childKey *BitArray,
) (*Node, *StorageNode, error) {
	if len(keys) == 0 || childKey == nil || childKey.len == 0 {
		return nil, nil, nil
	}

	node, err := t.readStorage.Get(childKey)
	if err != nil {
		return nil, nil, err
	}

	return node, &StorageNode{key: childKey, node: node}, nil
}

func (t *Trie) addProofNode(
	parentKey *BitArray,
	sNode StorageNode,
	carriedHash *felt.Felt,
	proof *ProofNodeSet,
	knownChildren ...*StorageNode,
) (*Binary, error) {
	var edge *Edge
	if isEdge(parentKey, sNode.key) {
		edgePath := path(sNode.key, parentKey)
		edge = &Edge{
			Path:  &edgePath,
			Child: sNode.node.Value,
		}
	}
	if sNode.key.len == t.height {
		if edge != nil { // Leaf Edge
			proof.Put(edgeHash(edge, carriedHash, t.hash), edge)
		}
		return nil, nil
	}

	binary, err := binaryProofNode(t, sNode, knownChildren...)
	if err != nil {
		return nil, err
	}

	if edge != nil { // Internal Edge
		proof.Put(edgeHash(edge, carriedHash, t.hash), edge)
	}
	proof.Put(*sNode.node.Value, binary)

	return binary, nil
}

// edgeHash returns the parent-facing hash of an edge node, from carried when set.
func edgeHash(edge *Edge, carried *felt.Felt, hash crypto.HashFn) felt.Felt {
	if carried != nil {
		return *carried
	}
	return edge.Hash(hash)
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
func VerifyProof(
	root,
	keyFelt *felt.Felt,
	proof *ProofNodeSet,
	hash crypto.HashFn,
) (felt.Felt, error) {
	var keyBits BitArray
	keyBits.SetFelt(globalTrieHeight, keyFelt)
	expectedHash := root

	var curPos uint8
	for {
		proofNode, ok := proof.Get(*expectedHash)
		if !ok {
			return felt.Felt{}, fmt.Errorf(
				"proof node not found, expected hash: %s",
				expectedHash.String(),
			)
		}

		// Verify the hash matches
		proofNodeHash := proofNode.Hash(hash)
		if !proofNodeHash.Equal(expectedHash) {
			return felt.Felt{},
				fmt.Errorf(
					"proof node hash mismatch, expected hash: %s, got hash: %s",
					expectedHash.String(),
					proofNodeHash.String(),
				)
		}

		switch node := proofNode.(type) {
		case *Binary: // Binary nodes represent left/right choices
			if keyBits.Len() <= curPos {
				return felt.Felt{},
					fmt.Errorf(
						"key length less than current position, key length: %d, current position: %d",
						keyBits.Len(),
						curPos,
					)
			}
			// Determine the next node to traverse based on the next bit position
			expectedHash = node.LeftHash
			if keyBits.IsBitSet(curPos) {
				expectedHash = node.RightHash
			}
			curPos++
		case *Edge: // Edge nodes represent paths between binary nodes
			if !verifyEdgePath(&keyBits, node.Path, curPos) {
				return felt.Zero, nil
			}

			// Move to the immediate child node
			curPos += node.Path.Len()
			expectedHash = node.Child
		}

		// We've consumed all bits in our path
		if curPos >= keyBits.Len() {
			return *expectedHash, nil
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

// isEdge reports whether the path between parentKey and childKey is longer
// than the branching bit, which the root does not have.
func isEdge(parentKey, childKey *BitArray) bool {
	if parentKey == nil { // Root
		return childKey.len != 0
	}
	return childKey.len-parentKey.len > 1
}

// binaryProofNode builds the Binary proof node of an internal StorageNode.
// Juno trie nodes are Binary AND Edge; the protocol requires Binary XOR Edge.
// knownChildren were already read by the traversal, so they don't cost another
// database lookup.
func binaryProofNode(
	tri *Trie, sNode StorageNode, knownChildren ...*StorageNode,
) (*Binary, error) {
	childHash := func(childKey *BitArray) (*felt.Felt, error) {
		var child *Node
		for _, knownChild := range knownChildren {
			if knownChild != nil && childKey.Equal(knownChild.key) {
				child = knownChild.node
				break
			}
		}
		if child == nil {
			var err error
			if child, err = tri.GetNodeFromKey(childKey); err != nil {
				return nil, err
			}
		}

		// Node.HashFromParent would cost an allocation per non-edge child.
		if isEdge(sNode.key, childKey) {
			edgePath := path(childKey, sNode.key)
			wrapped := child.Hash(&edgePath, tri.hash)
			return &wrapped, nil
		}
		return child.Value, nil
	}

	leftHash, err := childHash(sNode.node.Left)
	if err != nil {
		return nil, err
	}
	rightHash, err := childHash(sNode.node.Right)
	if err != nil {
		return nil, err
	}

	return &Binary{LeftHash: leftHash, RightHash: rightHash}, nil
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
	memoryDB := memory.New()
	txn := memoryDB.NewIndexedBatch()
	tr, err := NewTriePedersen(txn, nil, height)
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
