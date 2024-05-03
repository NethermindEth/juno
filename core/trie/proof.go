package trie

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
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

	getHash := func(key *Key) (*felt.Felt, error) {
		keyFelt := key.Felt()
		node, err := tri.GetNode(&keyFelt)
		if err != nil {
			return nil, err
		}
		return node.Hash(key, crypto.Pedersen), nil
	}

	for i, sNode := range nodesExcludingLeaf {
		// Determind if binary or edge
		sLeft := sNode.node.Left
		sRight := sNode.node.Right
		if sLeft != nil && sRight != nil { // sNode is (edge) Binary Node
			leftHash, err := getHash(sNode.node.Left)
			if err != nil {
				return nil, err
			}
			rightHash, err := getHash(sNode.node.Right)
			if err != nil {
				return nil, err
			}
			proofNodes[i] = ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			}

		} else { // sNode is Edge Node
			nxtNode := nodesToLeaf[i+1].node
			nxtKey := nodesToLeaf[i+1].key
			fmt.Println("sNode", sNode)
			fmt.Println("sNode", sNode.node)
			fmt.Println("sNode", sNode.node.Left)
			fmt.Println("sNode", sNode.node.Right)
			fmt.Println("nxtNode", nxtNode)

			fmt.Println("nxtNode", nxtNode.Left)
			fmt.Println("nxtNode", nxtNode.Right)

			// Juno doesn't have a notion of an edge node, so we construct it here
			edgePath, ok := findCommonKey(nxtKey, sNode.key)
			if !ok {
				return nil, errors.New("failed to get edge node path")
			}
			edgePathFelt := edgePath.Felt()

			var childKey felt.Felt
			if sNode.key.Test(sNode.key.len - nxtKey.len - 1) { // Todo: double -check
				childKey = sNode.node.Right.Felt()
			} else {
				childKey = sNode.node.Left.Felt()
			}

			proofNodes[i] = ProofNode{
				Edge: &Edge{
					Path:  &edgePathFelt,
					Child: &childKey,
					// Value: value, // Todo: ??
				},
			}
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
