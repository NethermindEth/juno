package trie

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

var (
	ErrUnknownProofNode  = errors.New("unknown proof node")
	ErrChildHashNotFound = errors.New("can't determine the child hash from the parent and child")
)

type ProofNode interface {
	Hash(hash HashFunc) *felt.Felt
	Len() uint8
	PrettyPrint()
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

func (b *Binary) Hash(hash HashFunc) *felt.Felt {
	return hash(b.LeftHash, b.RightHash)
}

func (b *Binary) Len() uint8 {
	return 1
}

func (b *Binary) PrettyPrint() {
	fmt.Printf("  Binary:\n")
	fmt.Printf("    LeftHash: %v\n", b.LeftHash)
	fmt.Printf("    RightHash: %v\n", b.RightHash)
}

type Edge struct {
	Child *felt.Felt // child hash
	Path  *Key       // path from parent to child
}

func (e *Edge) Hash(hash HashFunc) *felt.Felt {
	length := make([]byte, len(e.Path.bitset))
	length[len(e.Path.bitset)-1] = e.Path.len
	pathFelt := e.Path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length)
	return new(felt.Felt).Add(hash(e.Child, &pathFelt), lengthFelt)
}

func (e *Edge) Len() uint8 {
	return e.Path.Len()
}

func (e *Edge) PrettyPrint() {
	fmt.Printf("  Edge:\n")
	fmt.Printf("    Child: %v\n", e.Child)
	fmt.Printf("    Path: %v\n", e.Path)
}

func (t *Trie) Prove(key *felt.Felt, proofSet *ProofSet) error {
	k := t.FeltToKey(key)

	nodesFromRoot, err := t.nodesFromRoot(&k)
	if err != nil {
		return err
	}

	var parentKey *Key

	for i, sNode := range nodesFromRoot {
		sNodeEdge, sNodeBinary, err := transformNode(t, parentKey, sNode)
		if err != nil {
			return err
		}
		isLeaf := sNode.key.len == t.height

		if sNodeEdge != nil && !isLeaf { // Internal Edge
			proofSet.Put(*sNodeEdge.Hash(t.hash), sNodeEdge)
			proofSet.Put(*sNodeBinary.Hash(t.hash), sNodeBinary)
		} else if sNodeEdge == nil && !isLeaf { // Internal Binary
			proofSet.Put(*sNodeBinary.Hash(t.hash), sNodeBinary)
		} else if sNodeEdge != nil && isLeaf { // Leaf Edge
			proofSet.Put(*sNodeEdge.Hash(t.hash), sNodeEdge)
		} else if sNodeEdge == nil && sNodeBinary == nil { // sNode is a binary leaf
			break
		}
		parentKey = nodesFromRoot[i].key
	}
	return nil
}

func (t *Trie) GetRangeProof(leftKey, rightKey *felt.Felt, proofSet *ProofSet) error {
	err := t.Prove(leftKey, proofSet)
	if err != nil {
		return err
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
func transformNode(tri *Trie, parentKey *Key, sNode StorageNode) (*Edge, *Binary, error) {
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

// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L514
// GetProof generates a set of proof nodes from the root to the leaf.
// The proof never contains the leaf node if it is set, as we already know it's hash.
func GetProof(key *Key, tri *Trie) ([]ProofNode, error) {
	nodesFromRoot, err := tri.nodesFromRoot(key)
	if err != nil {
		return nil, err
	}
	proofNodes := []ProofNode{}

	var parentKey *Key

	for i, sNode := range nodesFromRoot {
		sNodeEdge, sNodeBinary, err := transformNode(tri, parentKey, sNode)
		if err != nil {
			return nil, err
		}
		isLeaf := sNode.key.len == tri.height

		if sNodeEdge != nil && !isLeaf { // Internal Edge
			proofNodes = append(proofNodes, sNodeEdge, sNodeBinary)
		} else if sNodeEdge == nil && !isLeaf { // Internal Binary
			proofNodes = append(proofNodes, sNodeBinary)
		} else if sNodeEdge != nil && isLeaf { // Leaf Edge
			proofNodes = append(proofNodes, sNodeEdge)
		} else if sNodeEdge == nil && sNodeBinary == nil { // sNode is a binary leaf
			break
		}
		parentKey = nodesFromRoot[i].key
	}
	return proofNodes, nil
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
//   - Any proof node is missing from the proofSet
//   - Any node's computed hash doesn't match its expected hash
//   - The path bits don't match the key bits
//   - The proof ends before processing all key bits
func VerifyProof(root *felt.Felt, key *Key, proofSet *ProofSet, hash HashFunc) (*felt.Felt, error) {
	expectedHash := root
	keyLen := key.Len()
	var processedBits uint8

	for {
		proofNode, ok := proofSet.Get(*expectedHash)
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
//
//nolint:gocyclo
func VerifyRangeProof(root, firstKey *felt.Felt, keys, values []*felt.Felt, proofSet *ProofSet, hash HashFunc) (bool, error) {
	// Ensure the number of keys and values are the same
	if len(keys) != len(values) {
		return false, fmt.Errorf("inconsistent proof data, number of keys: %d, number of values: %d", len(keys), len(values))
	}

	// Ensure all keys are monotonic increasing
	for i := 0; i < len(keys)-1; i++ {
		if keys[i].Cmp(keys[i+1]) >= 0 {
			return false, errors.New("keys are not monotonic increasing")
		}
	}

	// Ensure the range contains no deletions
	for _, value := range values {
		if value.Equal(&felt.Zero) {
			return false, errors.New("range contains deletion")
		}
	}

	// Special case: no edge proof at all, given range is the whole leaf set in the trie
	if proofSet == nil {
		tr, err := NewTriePedersen(newMemStorage(), 251) //nolint:mnd
		if err != nil {
			return false, err
		}

		for index, key := range keys {
			_, err = tr.Put(key, values[index])
			if err != nil {
				return false, err
			}
		}

		recomputedRoot, err := tr.Root()
		if err != nil {
			return false, err
		}

		if !recomputedRoot.Equal(root) {
			return false, fmt.Errorf("root hash mismatch, expected: %s, got: %s", root.String(), recomputedRoot.String())
		}

		return true, nil
	}

	proofList := proofSet.List()
	lastKey := keys[len(keys)-1]

	// Construct the left proof path
	leftProofPath, err := ProofToPath(proofList, &Key{len: 251, bitset: firstKey.Bytes()}, hash)
	if err != nil {
		return false, err
	}

	// Construct the right proof path
	rightProofPath, err := ProofToPath(proofList, &Key{len: 251, bitset: lastKey.Bytes()}, hash)
	if err != nil {
		return false, err
	}

	// Build the trie from the proof paths and the key-value pairs
	tr, err := BuildTrie(leftProofPath, rightProofPath, keys, values)
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

	return true, nil
}

// compressNode determines if the node needs compressed, and if so, the len needed to arrive at the next key
func compressNode(idx int, proofNodes []ProofNode, hashF HashFunc) (int, uint8, error) {
	parent := proofNodes[idx]

	if idx == len(proofNodes)-1 {
		if _, ok := parent.(*Edge); ok {
			return 1, parent.Len(), nil
		}
		return 0, parent.Len(), nil
	}

	child := proofNodes[idx+1]
	_, isChildBinary := child.(*Binary)
	isChildEdge := !isChildBinary
	switch parent := parent.(type) {
	case *Edge:
		if isChildEdge {
			break
		}
		return 1, parent.Len(), nil
	case *Binary:
		if isChildBinary {
			break
		}
		childHash := child.Hash(hashF)
		if parent.LeftHash.Equal(childHash) || parent.RightHash.Equal(childHash) {
			return 1, child.Len(), nil
		}
		return 0, 0, ErrChildHashNotFound
	}

	return 0, 1, nil
}

func assignChild(i, compressedParent int, parentNode *Node,
	nilKey, leafKey, parentKey *Key, proofNodes []ProofNode, hashF HashFunc,
) (*Key, error) {
	childInd := i + compressedParent + 1
	childKey, err := getChildKey(childInd, parentKey, leafKey, nilKey, proofNodes, hashF)
	if err != nil {
		return nil, err
	}
	if leafKey.IsBitSet(leafKey.len - parentKey.len - 1) {
		parentNode.Right = childKey
		parentNode.Left = nilKey
	} else {
		parentNode.Right = nilKey
		parentNode.Left = childKey
	}
	return childKey, nil
}

// ProofToPath returns a set of storage nodes from the root to the end of the proof path.
// The storage nodes will have the hashes of the children, but only the key of the child
// along the path outlined by the proof.
func ProofToPath(proofNodes []ProofNode, leafKey *Key, hashF HashFunc) ([]StorageNode, error) {
	pathNodes := []StorageNode{}

	// Child keys that can't be derived are set to nilKey, so that we can store the node
	zeroFeltBytes := new(felt.Felt).Bytes()
	nilKey := NewKey(0, zeroFeltBytes[:])

	for i, pNode := range proofNodes {
		// Keep moving along the path (may need to skip nodes that were compressed into the last path node)
		if i != 0 {
			if skipNode(pNode, pathNodes, hashF) {
				continue
			}
		}

		var parentKey *Key
		parentNode := Node{}

		// Set the key of the current node
		compressParent, compressParentOffset, err := compressNode(i, proofNodes, hashF)
		if err != nil {
			return nil, err
		}
		parentKey, err = getParentKey(i, compressParentOffset, leafKey, pNode, pathNodes, proofNodes)
		if err != nil {
			return nil, err
		}

		// Don't store leafs along proof paths
		if parentKey.len == 251 { //nolint:mnd
			break
		}

		// Set the value of the current node
		parentNode.Value = pNode.Hash(hashF)

		// Set the child key of the current node.
		childKey, err := assignChild(i, compressParent, &parentNode, &nilKey, leafKey, parentKey, proofNodes, hashF)
		if err != nil {
			return nil, err
		}

		// Set the LeftHash and RightHash values
		parentNode.LeftHash, parentNode.RightHash, err = getLeftRightHash(i, proofNodes)
		if err != nil {
			return nil, err
		}
		pathNodes = append(pathNodes, StorageNode{key: parentKey, node: &parentNode})

		// break early since we don't store leafs along proof paths, or if no more nodes exist along the proof paths
		if childKey.len == 0 || childKey.len == 251 {
			break
		}
	}

	return pathNodes, nil
}

func skipNode(pNode ProofNode, pathNodes []StorageNode, hashF HashFunc) bool {
	lastNode := pathNodes[len(pathNodes)-1].node
	noLeftMatch, noRightMatch := false, false
	if lastNode.LeftHash != nil && !pNode.Hash(hashF).Equal(lastNode.LeftHash) {
		noLeftMatch = true
	}
	if lastNode.RightHash != nil && !pNode.Hash(hashF).Equal(lastNode.RightHash) {
		noRightMatch = true
	}
	if noLeftMatch && noRightMatch {
		return true
	}
	return false
}

func getLeftRightHash(parentInd int, proofNodes []ProofNode) (*felt.Felt, *felt.Felt, error) {
	parent := proofNodes[parentInd]

	switch parent := parent.(type) {
	case *Binary:
		return parent.LeftHash, parent.RightHash, nil
	case *Edge:
		if parentInd+1 > len(proofNodes)-1 {
			return nil, nil, errors.New("cant get hash of children from proof node, out of range")
		}
		parentBinary := proofNodes[parentInd+1].(*Binary)
		return parentBinary.LeftHash, parentBinary.RightHash, nil
	default:
		return nil, nil, fmt.Errorf("%w: %T", ErrUnknownProofNode, parent)
	}
}

func getParentKey(idx int, compressedParentOffset uint8, leafKey *Key,
	pNode ProofNode, pathNodes []StorageNode, proofNodes []ProofNode,
) (*Key, error) {
	var crntKey *Key
	var err error

	var height uint8
	if len(pathNodes) > 0 {
		if p, ok := proofNodes[idx].(*Edge); ok {
			height = pathNodes[len(pathNodes)-1].key.len + p.Path.len
		} else {
			height = pathNodes[len(pathNodes)-1].key.len + 1
		}
	}

	if _, ok := pNode.(*Binary); ok {
		crntKey, err = leafKey.SubKey(height)
	} else {
		crntKey, err = leafKey.SubKey(height + compressedParentOffset)
	}
	return crntKey, err
}

func getChildKey(childIdx int, crntKey, leafKey, nilKey *Key, proofNodes []ProofNode, hashF HashFunc) (*Key, error) {
	if childIdx > len(proofNodes)-1 {
		return nilKey, nil
	}

	compressChild, compressChildOffset, err := compressNode(childIdx, proofNodes, hashF)
	if err != nil {
		return nil, err
	}

	if crntKey.len+uint8(compressChild)+compressChildOffset == 251 { //nolint:mnd
		return nilKey, nil
	}

	return leafKey.MostSignificantBits(crntKey.len + uint8(compressChild) + compressChildOffset)
}

// BuildTrie builds a trie using the proof paths (including inner nodes), and then sets all the keys-values (leaves)
func BuildTrie(leftProofPath, rightProofPath []StorageNode, keys, values []*felt.Felt) (*Trie, error) { //nolint:gocyclo
	tempTrie, err := NewTriePedersen(newMemStorage(), 251) //nolint:mnd
	if err != nil {
		return nil, err
	}

	// merge proof paths
	for i := range min(len(leftProofPath), len(rightProofPath)) {
		// Can't store nil keys so stop merging
		if leftProofPath[i].node.Left == nil || leftProofPath[i].node.Right == nil ||
			rightProofPath[i].node.Left == nil || rightProofPath[i].node.Right == nil {
			break
		}
		if leftProofPath[i].key.Equal(rightProofPath[i].key) {
			leftProofPath[i].node.Right = rightProofPath[i].node.Right
			rightProofPath[i].node.Left = leftProofPath[i].node.Left
		} else {
			break
		}
	}

	for _, sNode := range leftProofPath {
		if sNode.node.Left == nil || sNode.node.Right == nil {
			break
		}
		_, err := tempTrie.PutInner(sNode.key, sNode.node)
		if err != nil {
			return nil, err
		}
	}

	for _, sNode := range rightProofPath {
		if sNode.node.Left == nil || sNode.node.Right == nil {
			break
		}
		_, err := tempTrie.PutInner(sNode.key, sNode.node)
		if err != nil {
			return nil, err
		}
	}

	for i := range len(keys) {
		_, err := tempTrie.PutWithProof(keys[i], values[i], leftProofPath, rightProofPath)
		if err != nil {
			return nil, err
		}
	}
	return tempTrie, nil
}
