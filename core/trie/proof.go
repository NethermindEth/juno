package trie

import (
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starknet-io/starknet-p2p-specs/blob/main/p2p/proto/snapshot.proto#L6
type ProofNode struct {
	Binary *Binary
	Edge   *Edge
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
	proofNodes := []ProofNode{}

	getHash := func(key *Key) (*felt.Felt, error) {
		node, err := tri.GetNodeFromKey(key)
		if err != nil {
			return nil, err
		}
		return node.Hash(key, crypto.Pedersen), nil
	}

	// Edge nodes are defined as having a child with len greater than 1 from the parent
	isEdge := func(sNode *storageNode) bool {
		sNodeLen := sNode.key.len
		rNodeLen := sNodeLen
		if sNode.node.Right != nil {
			rNodeLen = sNode.node.Right.len
		}
		lNodeLen := sNodeLen
		if sNode.node.Left != nil {
			lNodeLen = sNode.node.Left.len
		}
		return (lNodeLen-sNodeLen > 1) || (rNodeLen-sNodeLen > 1)
	}

	// The child key may be an edge node. If so, we only take the "edge" section of the path.
	// getChildKey := func(childKey *Key) (*felt.Felt, error) {
	// 	childNode, err := tri.GetNodeFromKey(childKey)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	childSNode := storageNode{
	// 		key:  childKey,
	// 		node: childNode,
	// 	}
	// 	childKeyFelt := childKey.Felt()
	// 	if isEdge(&childSNode) {
	// 		childEdgePath := NewKey(childSNode.key.len, childSNode.key.bitset[:])
	// 		childEdgePath.RemoveLastBit()
	// 		childKeyFelt = childEdgePath.Felt()
	// 	}
	// 	return &childKeyFelt, nil
	// }

	rootKeyFelt := tri.rootKey.Felt()
	height := uint8(0)
	for _, sNode := range nodesExcludingLeaf {
		height += uint8(sNode.key.len)

		leftHash, err := getHash(sNode.node.Left)
		if err != nil {
			return nil, err
		}

		rightHash, err := getHash(sNode.node.Right)
		if err != nil {
			return nil, err
		}

		sNodeFelt := sNode.key.Felt()
		if isEdge(&sNode) || sNodeFelt.Equal(&rootKeyFelt) { // Split into Edge + Binary // Todo: always split root??
			edgePath := NewKey(sNode.key.len, sNode.key.bitset[:])
			edgePath.RemoveLastBit() // Todo: make sure we remove it from the correct side
			edgePathFelt := edgePath.Felt()

			// Todo: get childs hash (should be H(H_l, H_r))
			fmt.Println("edgePathFelt.String()", edgePathFelt.String())
			fmt.Println("childHash.String()", tri.hash(leftHash, rightHash).String())

			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					Path:  &edgePathFelt,
					Child: tri.hash(leftHash, rightHash),
					// Value: value, // Todo: ??
				},
			})
		}

		fmt.Println("LeftHash", leftHash.String())
		fmt.Println("rightHash", rightHash.String())
		proofNodes = append(proofNodes, ProofNode{
			Binary: &Binary{
				LeftHash:  leftHash,
				RightHash: rightHash,
			},
		})
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
