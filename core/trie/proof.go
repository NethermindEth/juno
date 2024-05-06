package trie

import (
	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starknet-io/starknet-p2p-specs/blob/main/p2p/proto/snapshot.proto#L6
type ProofNode struct {
	Binary *Binary
	Edge   *Edge
}

type Binary struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

type Edge struct {
	Child *felt.Felt
	Path  *felt.Felt
	Value *felt.Felt
}

// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L514
func GetProof(leaf *felt.Felt, tri *Trie) ([]ProofNode, error) {
	leafKey := tri.feltToKey(leaf)
	nodesToLeaf, err := tri.nodesFromRoot(&leafKey)
	if err != nil {
		return nil, err
	}
	nodesExcludingLeaf := nodesToLeaf[:len(nodesToLeaf)-1]
	proofNodes := make([]ProofNode, len(nodesExcludingLeaf))

	// Edge nodes are defined as having a child with len greater than 1 from the parent
	isEdge := func(sNode *storageNode) bool {
		sNodeLen := sNode.key.len
		lNodeLen := sNode.node.Right.len
		rNodeLen := sNode.node.Left.len
		return (lNodeLen-sNodeLen > 1) || (rNodeLen-sNodeLen > 1)
	}

	// The child key may be an edge node. If so, we only take the "edge" section of the path.
	getChildKey := func(childKey *Key) (*felt.Felt, error) {
		childNode, err := tri.GetNodeFromKey(childKey)
		if err != nil {
			return nil, err
		}
		childSNode := storageNode{
			key:  childKey,
			node: childNode,
		}
		childKeyFelt := childKey.Felt()
		if isEdge(&childSNode) {
			childEdgePath := NewKey(childSNode.key.len, childSNode.key.bitset[:])
			childEdgePath.RemoveLastBit()
			childKeyFelt = childEdgePath.Felt()
		}
		return &childKeyFelt, nil
	}

	height := uint8(0)
	for i, sNode := range nodesExcludingLeaf {
		height += uint8(sNode.key.len)

		if isEdge(&sNode) { // Split into Edge + Binary
			edgePath := NewKey(sNode.key.len, sNode.key.bitset[:])
			edgePath.RemoveLastBit() // Todo: make sure we remove it from the correct side
			edgePathFelt := edgePath.Felt()

			childKey := sNode.key.Felt()

			proofNodes[i] = ProofNode{
				Edge: &Edge{
					Path:  &edgePathFelt,
					Child: &childKey,
					// Value: value, // Todo: ??
				},
			}
		}
		leftHash, err := getChildKey(sNode.node.Left)
		if err != nil {
			return nil, err
		}
		rightHash, err := getChildKey(sNode.node.Right)
		if err != nil {
			return nil, err
		}
		proofNodes[i] = ProofNode{
			Binary: &Binary{
				LeftHash:  leftHash,
				RightHash: rightHash,
			},
		}
	}

	return proofNodes, nil
}

// verifyProof checks if `leafPath` leads from `root` to `leafHash` along the `proofNodes`
// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2006
// func VerifyProof(root *felt.Felt, leafPath *Key, leafHash felt.Felt, proofNodes []ProofNode, hashFunc hashFunc) error {
// 	expectedHash := root

// 	for i, pNode := range proofNodes {
// 		pNodeHash := hashFunc(pNode.LeftHash, pNode.RightHash)
// 		if !expectedHash.Equal(pNodeHash) {
// 			return errors.New("proof node does not have the expected hash")
// 		}

// 		if leafPath.Test(leafPath.Len() - uint8(i) - 1) {
// 			expectedHash = pNode.RightHash
// 		} else {
// 			expectedHash = pNode.LeftHash
// 		}

// 	}

// 	if !expectedHash.Equal(&leafHash) {
// 		return errors.New("leafHash does not have the expected hash")
// 	}

// 	return nil
// }
