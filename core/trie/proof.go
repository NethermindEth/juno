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
		fmt.Printf("    Value: %v\n", pn.Edge.Value)
	}
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

type Edge struct {
	Child *felt.Felt // child hash
	Path  *Key       // path from parent to child
	Value *felt.Felt // this nodes hash
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

// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L514
func GetProof(key *Key, tri *Trie) ([]ProofNode, error) { // todo: Get proof should always be for leafs??
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

			// If we are verifying the key doesn't exist, then we should
			// update subKey to point in the other direction
			if value == nil && i == len(proofs)-1 {
				return true // todo: hack for non-set proof nodes
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
// and therefore it's hash won't match the expected root
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

			proofPaths[i], err = ProofToPath(proofs[i], proofKeys[i], proofValues[i], hash) // Todo: incorrect for both paths :(
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

	// Todo: remove. Just inspection. Correct structure, and contains left/right hashes.
	rootNode, _ := tmpTrie.GetNodeFromKey(tmpTrie.rootKey)
	l, _ := tmpTrie.GetNodeFromKey(rootNode.Left)
	r, _ := tmpTrie.GetNodeFromKey(rootNode.Right)
	ll, _ := tmpTrie.GetNodeFromKey(l.Left)
	lr, _ := tmpTrie.GetNodeFromKey(l.Right)
	lll, _ := tmpTrie.GetNodeFromKey(ll.Left)
	llr, _ := tmpTrie.GetNodeFromKey(ll.Right)
	fmt.Println(l, r, lr, lll, llr)

	// Verify that the recomputed root hash matches the provided root hash
	recomputedRoot, err := tmpTrie.Root() // Todo: key not found
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

// shouldSquish determines if the node needs compressed, and if so, the len needed to arrive at the next key
func shouldSquish(idx int, proofNodes []ProofNode, hashF hashFunc) (int, uint8, *felt.Felt, *felt.Felt) {
	if idx > len(proofNodes)-1 {
		return 0, 1, nil, nil
	}
	parent := &proofNodes[idx]
	var child *ProofNode
	// The child is nil of the current node is a leaf
	if idx != len(proofNodes)-1 {
		child = &proofNodes[idx+1]
	}

	if child == nil {
		return 0, 1, nil, nil
	}

	if parent.Edge != nil && child.Binary != nil {
		return 1, parent.Edge.Path.len, child.Binary.LeftHash, child.Binary.RightHash
	}

	if parent.Binary != nil && child.Edge != nil {
		childHash := child.Hash(hashF)
		if parent.Binary.LeftHash.Equal(childHash) {
			return 1, child.Edge.Path.len, child.Edge.Value, parent.Binary.RightHash
		} else if parent.Binary.RightHash.Equal(childHash) {
			return 1, child.Edge.Path.len, parent.Binary.LeftHash, child.Edge.Value
		} else {
			panic("can't determine the child hash from the parent and child") // Todo: pass error up
		}

	}

	if parent.Binary != nil && child.Binary != nil {
		return 0, 1, parent.Binary.LeftHash, parent.Binary.RightHash
	}

	return 0, 1, nil, nil
}

func assignChild(crntNode *Node, nilKey, childKey *Key, isRight bool) {
	if isRight {
		crntNode.Right = childKey
		crntNode.Left = nilKey
	} else {
		crntNode.Right = nilKey
		crntNode.Left = childKey
	}
}

// ProofToPath returns the set of storage nodes along the proofNodes towards the leaf.
// Note that only the nodes and children along this path will be set correctly.
func ProofToPath(proofNodes []ProofNode, leafKey *Key, leafValue *felt.Felt, hashF hashFunc) ([]StorageNode, error) {
	pathNodes := []StorageNode{}

	// Hack: this allows us to store a right without an existing left node.
	zeroFeltBytes := new(felt.Felt).Bytes()
	nilKey := NewKey(0, zeroFeltBytes[:])

	for i, pNode := range proofNodes {
		// Skip this node if it has been squished
		if i != 0 {
			lastNode := pathNodes[len(pathNodes)-1].node
			noLeftMatch, noRightMatch := false, false
			if lastNode.LeftHash != nil && !pNode.Hash(hashF).Equal(lastNode.LeftHash) {
				noLeftMatch = true
			}
			if lastNode.RightHash != nil && !pNode.Hash(hashF).Equal(lastNode.RightHash) {
				noRightMatch = true
			}
			if noLeftMatch && noRightMatch {
				continue
			}
		}

		var crntKey *Key
		crntNode := Node{}

		height := getHeight(i, pathNodes, proofNodes)

		// Set the key of the current node
		var err error
		squishedParent, squishParentOffset, leftHash, rightHash := shouldSquish(i, proofNodes, hashF)
		if pNode.Binary != nil {
			crntKey, err = leafKey.SubKey(height)
		} else {
			crntKey, err = leafKey.SubKey(height + squishParentOffset)
		}
		if err != nil {
			return nil, err
		}

		// Set the value,left hash, and right hash of the current node
		crntNode.Value = pNode.Hash(hashF)
		crntNode.LeftHash = leftHash
		crntNode.RightHash = rightHash

		// If current key is a leaf then don't set the children
		if crntKey.len == 251 {
			break
		}

		// Set the child key of the current node.
		childIdx := i + squishedParent + 1
		_, squishChildOffset, _, _ := shouldSquish(childIdx, proofNodes, hashF)
		childKey, err := leafKey.SubKey(crntKey.len + squishChildOffset)
		if err != nil {
			return nil, err
		}
		childIsRight := leafKey.Test(leafKey.len - crntKey.len - 1)
		assignChild(&crntNode, &nilKey, childKey, childIsRight)

		pathNodes = append(pathNodes, StorageNode{key: crntKey, node: &crntNode})
	}
	// If the proof node is unset, then ensure that the child keys are also
	// unset (they are erroneously set above)
	if leafValue == nil {
		pathNodes[len(pathNodes)-1].node.Left = nil
		pathNodes[len(pathNodes)-1].node.Right = nil
	}
	pathNodes = addLeafNode(proofNodes, pathNodes, leafKey)
	return pathNodes, nil
}

// getHeight returns the height of the current node, which depends on the previous
// height and whether the current proofnode is edge or binary
func getHeight(idx int, pathNodes []StorageNode, proofNodes []ProofNode) uint8 {
	if len(pathNodes) > 0 {
		if proofNodes[idx].Edge != nil {
			return pathNodes[len(pathNodes)-1].key.len + proofNodes[idx].Edge.Path.len
		} else {
			return pathNodes[len(pathNodes)-1].key.len + 1
		}
	} else {
		return 0
	}
}

// addLeafNode appends the leaf node, if the final node in pathNodes points to a leaf.
func addLeafNode(proofNodes []ProofNode, pathNodes []StorageNode, leafKey *Key) []StorageNode {
	lastNode := pathNodes[len(pathNodes)-1].node
	lastProof := proofNodes[len(proofNodes)-1]
	if lastNode.Left.Equal(leafKey) || lastNode.Right.Equal(leafKey) {
		leafNode := Node{}
		if lastProof.Edge != nil {
			leafNode.Value = lastProof.Edge.Child
		} else if lastNode.Left.Equal(leafKey) {
			leafNode.Value = lastProof.Binary.LeftHash
		} else {
			leafNode.Value = lastProof.Binary.RightHash
		}
		pathNodes = append(pathNodes, StorageNode{key: leafKey, node: &leafNode})
	}
	return pathNodes
}

// BuildTrie builds a trie using the proof paths (including inner nodes), and then sets all the keys-values (leaves)
func BuildTrie(leftProofPath, rightProofPath []StorageNode, keys, values []*felt.Felt) (*Trie, error) {
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
		// Can't store nil keys (reached end of line for proof)
		if sNode.node.Left == nil || sNode.node.Right == nil {
			break
		}
		_, err := tempTrie.PutInner(sNode.key, sNode.node)
		if err != nil {
			return nil, err
		}
	}

	for _, sNode := range rightProofPath {
		// Can't store nil keys (reached end of line for proof)
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
