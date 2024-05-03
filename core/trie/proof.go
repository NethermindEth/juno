package trie

import (
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

	// The spec requires Edge nodes. However Juno doesn't have edge nodes.
	// if Len=250 and have left and right -> create a Binary
	// if len=250 and only have one child -> create edge
	// if len!=250, check if child-parent length = 1, if so ,create Binary
	// if len!=250, check if child-parent length = 1, if not ,create Edge
	for i, sNode := range nodesExcludingLeaf {
		// sLeft := sNode.node.Left
		// sRight := sNode.node.Right
		nxtNode := nodesToLeaf[i+1]

		if nxtNode.key.len-sNode.key.len > 1 { // split node into edge + child
			edgePath := NewKey(sNode.key.len, sNode.key.bitset[:])
			edgePath.RemoveLastBit() // Todo: make sure we remove it from the correct side
			edgePathFelt := edgePath.Felt()

			childKeyFelt := sNode.key.Felt()

			proofNodes[i] = ProofNode{
				Edge: &Edge{
					Path:  &edgePathFelt,
					Child: &childKeyFelt,
					// Value: value, // Todo: ??
				},
			}
		}
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
