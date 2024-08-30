package trie

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starknet-io/starknet-p2p-specs/blob/main/p2p/proto/snapshot.proto#L6
type ProofNode struct {
	Binary *Binary
	Edge   *Edge
}

// Note: does not work for leaves
func (pn *ProofNode) Hash(hash hashFunc) *felt.Felt {
	switch {
	case pn.Binary != nil:
		return hash(pn.Binary.LeftHash, pn.Binary.RightHash)
	case pn.Edge != nil:
		length := make([]byte, len(pn.Edge.Path.bitset))
		length[len(pn.Edge.Path.bitset)-1] = pn.Edge.Path.len
		pathFelt := pn.Edge.Path.Felt()
		lengthFelt := new(felt.Felt).SetBytes(length)
		return new(felt.Felt).Add(hash(pn.Edge.Child, &pathFelt), lengthFelt)
	default:
		return nil
	}
}

func (pn *ProofNode) Len() uint8 {
	if pn.Binary != nil {
		return 1
	}
	return pn.Edge.Path.len
}

func (pn *ProofNode) PrettyPrint() {
	if pn.Binary != nil {
		fmt.Printf("  Binary:\n")
		fmt.Printf("    LeftHash: %v\n", pn.Binary.LeftHash)
		fmt.Printf("    RightHash: %v\n", pn.Binary.RightHash)
	}
	if pn.Edge != nil {
		fmt.Printf("  Edge:\n")
		fmt.Printf("    Child: %v\n", pn.Edge.Child)
		fmt.Printf("    Path: %v\n", pn.Edge.Path)
	}
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

type Edge struct {
	Child *felt.Felt // child hash
	Path  *Key       // path from parent to child
}

func GetBoundaryProofs(leftBoundary, rightBoundary *Key, tri *Trie) ([2][]ProofNode, error) {
	proofs := [2][]ProofNode{}
	leftProof, err := GetProof(leftBoundary, tri)
	if err != nil {
		return proofs, err
	}
	rightProof, err := GetProof(rightBoundary, tri)
	if err != nil {
		return proofs, err
	}
	proofs[0] = leftProof
	proofs[1] = rightProof
	return proofs, nil
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
		rEdge := ProofNode{Edge: &Edge{
			Path:  &edgePath,
			Child: rNode.Value,
		}}
		rightHash = rEdge.Hash(tri.hash)
	}
	leftHash := lNode.Value
	if isEdge(sNode.key, StorageNode{node: lNode, key: sNode.node.Left}) {
		edgePath := path(sNode.node.Left, sNode.key)
		lEdge := ProofNode{Edge: &Edge{
			Path:  &edgePath,
			Child: lNode.Value,
		}}
		leftHash = lEdge.Hash(tri.hash)
	}
	binary := &Binary{
		LeftHash:  leftHash,
		RightHash: rightHash,
	}

	return edge, binary, nil
}

// Begins from the root node and traverses the merged proof path
// Until it finds a split node, adds nodes to commonPath
// Then continues with traversing left and right paths separetaly
// Assumes there is no circular paths and the split happens at most once
func traverseNodes(currNode *ProofNode, path, leftPath, rightPath *[]ProofNode, nodeHashes map[felt.Felt]ProofNode) {
	*path = append(*path, *currNode)

	if currNode.Binary != nil {
		expectedLeftHash := currNode.Binary.LeftHash
		expectedRightHash := currNode.Binary.RightHash

		nodeLeft, leftExist := nodeHashes[*expectedLeftHash]
		nodeRight, rightExist := nodeHashes[*expectedRightHash]
		if leftExist && rightExist {
			traverseNodes(&nodeLeft, leftPath, nil, nil, nodeHashes)
			traverseNodes(&nodeRight, rightPath, nil, nil, nodeHashes)
		} else if leftExist {
			traverseNodes(&nodeLeft, path, leftPath, rightPath, nodeHashes)
		} else if rightExist {
			traverseNodes(&nodeRight, path, leftPath, rightPath, nodeHashes)
		}
	} else if currNode.Edge != nil {
		edgeNode, exist := nodeHashes[*currNode.Edge.Child]
		if !exist {
			return
		}
		traverseNodes(&edgeNode, path, leftPath, rightPath, nodeHashes)
	}
}

// Remove duplicates and merges proof paths into a single path
func MergeProofPaths(leftPath, rightPath []ProofNode, hash hashFunc) ([]ProofNode, *felt.Felt, error) {
	merged := []ProofNode{}
	minLen := min(len(leftPath), len(rightPath))

	if len(leftPath) == 0 || len(rightPath) == 0 {
		return merged, nil, errors.New("empty proof paths")
	}

	if !leftPath[0].Hash(hash).Equal(rightPath[0].Hash(hash)) {
		return merged, nil, errors.New("roots of the proof paths are different")
	}

	rootHash := leftPath[0].Hash(hash)

	// Get duplicates and insert by one
	i := 0
	for i = 0; i < minLen; i++ {
		leftNode := leftPath[i]
		rightNode := rightPath[i]

		if leftNode.Hash(hash).Equal(rightNode.Hash(hash)) {
			merged = append(merged, leftNode)
		} else {
			break
		}
	}

	// Add rest of the nodes one from left and one from right
	// until we reach the end of the shortest path
	for ; i < minLen; i++ {
		merged = append(merged, leftPath[i], rightPath[i])
	}

	// Add the rest of the nodes from the longest path
	if len(leftPath) > minLen {
		merged = append(merged, leftPath[i:]...)
	} else if len(rightPath) > minLen {
		merged = append(merged, rightPath[i:]...)
	}

	return merged, rootHash, nil
}

// Splits the merged proof path into two paths
// First validates that the merged path is not circular and the split happens at most once
// Then calls traverseNodes to split the path
func SplitProofPath(mergedPath []ProofNode, rootHash *felt.Felt, hash hashFunc) ([]ProofNode, []ProofNode, error) {
	commonPath := []ProofNode{}
	leftPath := []ProofNode{}
	rightPath := []ProofNode{}

	if len(mergedPath) == 0 {
		return leftPath, rightPath, nil
	}

	nodeHashes := make(map[felt.Felt]ProofNode)

	// validates the merged path is not circular
	for _, node := range mergedPath {
		nodeHash := node.Hash(hash)
		_, nodeExists := nodeHashes[*nodeHash]

		if nodeExists {
			return leftPath, rightPath, errors.New("duplicate node in the merged path")
		}

		nodeHashes[*nodeHash] = node
	}

	// validates that the split happens at most once
	splitHappened := false
	for _, node := range mergedPath {
		if node.Edge != nil {
			continue
		}

		leftHash := node.Binary.LeftHash
		rightHash := node.Binary.RightHash

		_, leftExists := nodeHashes[*leftHash]
		_, rightExists := nodeHashes[*rightHash]

		if leftExists && rightExists {
			if splitHappened {
				return leftPath, rightPath, errors.New("split happened more than once")
			}

			splitHappened = true
		}
	}

	// checks if the root hash exists in the merged path
	currNode, rootExists := nodeHashes[*rootHash]
	if !rootExists {
		return leftPath, rightPath, errors.New("root hash not found in the merged path")
	}

	traverseNodes(&currNode, &commonPath, &leftPath, &rightPath, nodeHashes)

	leftResult := make([]ProofNode, len(commonPath)+len(leftPath))
	copy(leftResult, commonPath)
	copy(leftResult[len(commonPath):], leftPath)

	rightResult := make([]ProofNode, len(commonPath)+len(rightPath))
	copy(rightResult, commonPath)
	copy(rightResult[len(commonPath):], rightPath)

	return leftResult, rightResult, nil
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
			proofNodes = append(proofNodes, []ProofNode{{Edge: sNodeEdge}, {Binary: sNodeBinary}}...)
		} else if sNodeEdge == nil && !isLeaf { // Internal Binary
			proofNodes = append(proofNodes, []ProofNode{{Binary: sNodeBinary}}...)
		} else if sNodeEdge != nil && isLeaf { // Leaf Edge
			proofNodes = append(proofNodes, []ProofNode{{Edge: sNodeEdge}}...)
		} else if sNodeEdge == nil && sNodeBinary == nil { // sNode is a binary leaf
			break
		}
		parentKey = nodesFromRoot[i].key
	}
	return proofNodes, nil
}

// verifyProof checks if `leafPath` leads from `root` to `leafHash` along the `proofNodes`
// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2006
func VerifyProof(root *felt.Felt, key *Key, value *felt.Felt, proofs []ProofNode, hash hashFunc) bool {
	expectedHash := root
	remainingPath := NewKey(key.len, key.bitset[:])
	for i, proofNode := range proofs {
		if !proofNode.Hash(hash).Equal(expectedHash) {
			return false
		}

		switch {
		case proofNode.Binary != nil:
			if remainingPath.Test(remainingPath.Len() - 1) {
				expectedHash = proofNode.Binary.RightHash
			} else {
				expectedHash = proofNode.Binary.LeftHash
			}
			remainingPath.RemoveLastBit()
		case proofNode.Edge != nil:
			subKey, err := remainingPath.SubKey(proofNode.Edge.Path.Len())
			if err != nil {
				return false
			}

			// Todo:
			// If we are verifying the key doesn't exist, then we should
			// update subKey to point in the other direction
			if value == nil && i == len(proofs)-1 {
				return true
			}

			if !proofNode.Edge.Path.Equal(subKey) {
				return false
			}
			expectedHash = proofNode.Edge.Child
			remainingPath.Truncate(251 - proofNode.Edge.Path.Len()) //nolint:gomnd
		}
	}

	return expectedHash.Equal(value)
}

// VerifyRangeProof verifies the range proof for the given range of keys.
// This is achieved by constructing a trie from the boundary proofs, and the supplied key-values.
// If the root of the reconstructed trie matches the supplied root, then the verification passes.
// If the trie is constructed incorrectly then the root will have an incorrect key(len,path), and value,
// and therefore it's hash won't match the expected root.
// ref: https://github.com/ethereum/go-ethereum/blob/v1.14.3/trie/proof.go#L484
func VerifyRangeProof(root *felt.Felt, keys, values []*felt.Felt, proofKeys [2]*Key, proofValues [2]*felt.Felt,
	proofs [2][]ProofNode, hash hashFunc,
) (bool, error) {
	// Step 0: checks
	if len(keys) != len(values) {
		return false, fmt.Errorf("inconsistent proof data, number of keys: %d, number of values: %d", len(keys), len(values))
	}

	// Ensure all keys are monotonic increasing
	if err := ensureMonotonicIncreasing(proofKeys, keys); err != nil {
		return false, err
	}

	// Ensure the inner values contain no deletions
	for _, value := range values {
		if value.Equal(&felt.Zero) {
			return false, errors.New("range contains deletion")
		}
	}

	// Step 1: Verify proofs, and get proof paths
	var proofPaths [2][]StorageNode
	var err error
	for i := 0; i < 2; i++ {
		if proofs[i] != nil {
			if !VerifyProof(root, proofKeys[i], proofValues[i], proofs[i], hash) {
				return false, fmt.Errorf("invalid proof for key %x", proofKeys[i].String())
			}

			proofPaths[i], err = ProofToPath(proofs[i], proofKeys[i], hash)
			if err != nil {
				return false, err
			}
		}
	}

	// Step 2: Build trie from proofPaths and keys
	tmpTrie, err := BuildTrie(proofPaths[0], proofPaths[1], keys, values)
	if err != nil {
		return false, err
	}

	// Verify that the recomputed root hash matches the provided root hash
	recomputedRoot, err := tmpTrie.Root()
	if err != nil {
		return false, err
	}
	if !recomputedRoot.Equal(root) {
		return false, errors.New("root hash mismatch")
	}

	return true, nil
}

func ensureMonotonicIncreasing(proofKeys [2]*Key, keys []*felt.Felt) error {
	if proofKeys[0] != nil {
		leftProofFelt := proofKeys[0].Felt()
		if leftProofFelt.Cmp(keys[0]) >= 0 {
			return errors.New("range is not monotonically increasing")
		}
	}
	if proofKeys[1] != nil {
		rightProofFelt := proofKeys[1].Felt()
		if keys[len(keys)-1].Cmp(&rightProofFelt) >= 0 {
			return errors.New("range is not monotonically increasing")
		}
	}
	if len(keys) >= 2 {
		for i := 0; i < len(keys)-1; i++ {
			if keys[i].Cmp(keys[i+1]) >= 0 {
				return errors.New("range is not monotonically increasing")
			}
		}
	}
	return nil
}

// compressNode determines if the node needs compressed, and if so, the len needed to arrive at the next key
func compressNode(idx int, proofNodes []ProofNode, hashF hashFunc) (int, uint8, error) {
	parent := &proofNodes[idx]

	if idx == len(proofNodes)-1 {
		if parent.Edge != nil {
			return 1, parent.Len(), nil
		}
		return 0, parent.Len(), nil
	}

	child := &proofNodes[idx+1]

	switch {
	case parent.Edge != nil && child.Binary != nil:
		return 1, parent.Edge.Path.len, nil
	case parent.Binary != nil && child.Edge != nil:
		childHash := child.Hash(hashF)
		if parent.Binary.LeftHash.Equal(childHash) || parent.Binary.RightHash.Equal(childHash) {
			return 1, child.Edge.Path.len, nil
		} else {
			return 0, 0, errors.New("can't determine the child hash from the parent and child")
		}
	}

	return 0, 1, nil
}

func assignChild(i, compressedParent int, parentNode *Node,
	nilKey, leafKey, parentKey *Key, proofNodes []ProofNode, hashF hashFunc,
) (*Key, error) {
	childInd := i + compressedParent + 1
	childKey, err := getChildKey(childInd, parentKey, leafKey, nilKey, proofNodes, hashF)
	if err != nil {
		return nil, err
	}
	if leafKey.Test(leafKey.len - parentKey.len - 1) {
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
func ProofToPath(proofNodes []ProofNode, leafKey *Key, hashF hashFunc) ([]StorageNode, error) {
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
		if parentKey.len == 251 { //nolint:gomnd
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

func skipNode(pNode ProofNode, pathNodes []StorageNode, hashF hashFunc) bool {
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
	parent := &proofNodes[parentInd]
	if parent.Binary == nil {
		if parentInd+1 > len(proofNodes)-1 {
			return nil, nil, errors.New("cant get hash of children from proof node, out of range")
		}
		parent = &proofNodes[parentInd+1]
	}
	return parent.Binary.LeftHash, parent.Binary.RightHash, nil
}

func getParentKey(idx int, compressedParentOffset uint8, leafKey *Key,
	pNode ProofNode, pathNodes []StorageNode, proofNodes []ProofNode,
) (*Key, error) {
	var crntKey *Key
	var err error

	var height uint8
	if len(pathNodes) > 0 {
		if proofNodes[idx].Edge != nil {
			height = pathNodes[len(pathNodes)-1].key.len + proofNodes[idx].Edge.Path.len
		} else {
			height = pathNodes[len(pathNodes)-1].key.len + 1
		}
	} else {
		height = 0
	}

	if pNode.Binary != nil {
		crntKey, err = leafKey.SubKey(height)
	} else {
		crntKey, err = leafKey.SubKey(height + compressedParentOffset)
	}
	return crntKey, err
}

func getChildKey(childIdx int, crntKey, leafKey, nilKey *Key, proofNodes []ProofNode, hashF hashFunc) (*Key, error) {
	if childIdx > len(proofNodes)-1 {
		return nilKey, nil
	}

	compressChild, compressChildOffset, err := compressNode(childIdx, proofNodes, hashF)
	if err != nil {
		return nil, err
	}

	if crntKey.len+uint8(compressChild)+compressChildOffset == 251 { //nolint:gomnd
		return nilKey, nil
	}

	return leafKey.SubKey(crntKey.len + uint8(compressChild) + compressChildOffset)
}

// BuildTrie builds a trie using the proof paths (including inner nodes), and then sets all the keys-values (leaves)
func BuildTrie(leftProofPath, rightProofPath []StorageNode, keys, values []*felt.Felt) (*Trie, error) { //nolint:gocyclo
	tempTrie, err := NewTriePedersen(newMemStorage(), 251) //nolint:gomnd
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
