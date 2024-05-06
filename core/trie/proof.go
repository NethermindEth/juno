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

func (pn *ProofNode) Hash() *felt.Felt {
	switch {
	case pn.Binary != nil:
		return crypto.Pedersen(pn.Binary.LeftHash, pn.Binary.RightHash)
	case pn.Edge != nil:
		length := make([]byte, 32)
		length[31] = pn.Edge.Path.len
		pathFelt := pn.Edge.Path.Felt()
		lengthFelt := new(felt.Felt).SetBytes(length)
		return new(felt.Felt).Add(crypto.Pedersen(pn.Edge.Child, &pathFelt), lengthFelt)
	default:
		return nil
	}
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
	Path  *Key
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
		if sNode.node.Right != nil {
			if sNode.node.Right.len-sNodeLen > 1 {
				return true
			}
		}

		if sNode.node.Left != nil {
			if sNode.node.Left.len-sNodeLen > 1 {
				return true
			}
		}
		return false
	}

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

			epf := edgePath.Felt()
			fmt.Println("edgePathFelt.String()", epf.String())
			fmt.Println("childHash.String()", tri.hash(leftHash, rightHash).String())

			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					Path:  &edgePath, // Path from that node to the leaf
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
func VerifyProof(root *felt.Felt, key *Key, value *felt.Felt, proofs []ProofNode) bool {

	if key.Len() != 251 {
		return false
	}

	expectedHash := root
	remainingPath := key

	for _, proofNode := range proofs {
		if !proofNode.Hash().Equal(expectedHash) {
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
			// The next "proofNode.Edge.len" bits must match
			if !proofNode.Edge.Path.Equal(remainingPath.SubKey(proofNode.Edge.Path.Len())) {
				return false
			}
			expectedHash = proofNode.Edge.Child
			remainingPath.Truncate(proofNode.Edge.Path.Len())
		}
	}
	return expectedHash.Equal(value)
}
