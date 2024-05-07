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

// Note: does not work for leaves
func (pn *ProofNode) Hash() *felt.Felt {
	switch {
	case pn.Binary != nil:
		return crypto.Pedersen(pn.Binary.LeftHash, pn.Binary.RightHash)
	case pn.Edge != nil:
		length := make([]byte, len(pn.Edge.Path.bitset))
		length[len(pn.Edge.Path.bitset)-1] = pn.Edge.Path.len
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

func isEdge(parentKey *Key, sNode storageNode) bool {
	sNodeLen := sNode.key.len
	if parentKey == nil { // Root
		return sNodeLen != 1
	}
	if sNodeLen-parentKey.len > 1 {
		return true
	}

	// leftKey := sNode.node.Left.len
	// rightKey := sNode.node.Right.len
	// if (leftKey-sNodeLen > 1) || (rightKey-sNodeLen > 1) {
	// 	return true
	// }
	return false
}

// The binary node uses the hash of children. If the child is an edge, we first need to represent it
// as an edge node, and then take its hash.
func getChildHash(tri *Trie, parentKey *Key, childKey *Key) (*felt.Felt, error) {
	childNode, err := tri.GetNodeFromKey(childKey)
	if err != nil {
		return nil, err
	}

	childIsEdgeBool := isEdge(parentKey, storageNode{node: childNode, key: childKey})
	if childIsEdgeBool {
		fmt.Println("childKey", childKey)
		fmt.Println("childNode.Value", childNode.Value)
		edgeNode := ProofNode{Edge: &Edge{
			Path:  childKey,
			Child: childNode.Value,
		}}
		return edgeNode.Hash(), nil
	}
	return childNode.Value, nil
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

	// 1. If it's an edge-node in pathfinders impl, we need to expand the node into an edge + binary
	// -> Child should be internal node (len<251). Distance between child and parent should be > 1.
	// 2. If it's a binary-node, we store binary
	// -> Child should be internal node (len<251). Distance between child and parent should be 1.
	// 3. If it's a binary leaf, we store binary leaf
	// -> Child should be leaf (len=251). Distance between child and parent should be 1.
	// 4. If it's an edge leaf, we store an edge leaf
	// -> Child should be leaf (len=251). Distance between child and parent should be > 1.

	for i, sNode := range nodesExcludingLeaf {

		leftHash, err := getChildHash(tri, sNode.key, sNode.node.Left)
		if err != nil {
			return nil, err
		}
		rightHash, err := getChildHash(tri, sNode.key, sNode.node.Right)
		if err != nil {
			return nil, err
		}

		childIsInternal := nodesToLeaf[i+1].key.len < tri.height
		var isEdgeBool bool
		if i == 0 {
			isEdgeBool = isEdge(nil, sNode)
		} else {
			isEdgeBool = isEdge(nodesToLeaf[i-1].key, sNode)
		}

		if (childIsInternal && isEdgeBool) || (isEdgeBool && i == 0) { // Internal Edge
			// Juno node is split into an edge + binary.
			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					Path:  sNode.key,
					Child: sNode.node.Value,
					// Value: value, // Todo: ??
				},
			},
				ProofNode{
					Binary: &Binary{
						LeftHash:  leftHash,
						RightHash: rightHash,
					},
				})
		} else if childIsInternal && !isEdgeBool { // Internal Binary
			proofNodes = append(proofNodes, ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			})
		} else if !childIsInternal && isEdgeBool { // Leaf Edge
			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					Child: sNode.node.Value,
					// Value: value, // Todo: ??
				},
			})
		} else if !childIsInternal && !isEdgeBool { // Leaf binary
			proofNodes = append(proofNodes, ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			})
		} else {
			return nil, errors.New("unexpected error in GetProof")
		}
	}
	return proofNodes, nil
}

// verifyProof checks if `leafPath` leads from `root` to `leafHash` along the `proofNodes`
// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2006
func VerifyProof(root *felt.Felt, key *Key, value *felt.Felt, proofs []ProofNode) bool {
	if key.Len() != key.len {
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
			// Todo: Isn't edge.path from root? and remaining from edge to leaf??
			if !proofNode.Edge.Path.Equal(remainingPath.SubKey(proofNode.Edge.Path.Len())) {
				return false
			}
			expectedHash = proofNode.Edge.Child
			remainingPath.Truncate(proofNode.Edge.Path.Len())
		}
	}
	return expectedHash.Equal(value)
}
