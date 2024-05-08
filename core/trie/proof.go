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
		return sNodeLen != 0
	}
	if sNodeLen-parentKey.len > 1 {
		return true
	}
	return false
}

// transformNode takes a node and splits it into an edge+binary if it's an edge node
func transformNode(tri *Trie, parentKey *Key, sNode storageNode) (*Edge, *Binary, error) {
	// Internal Edge
	isEdgeBool := isEdge(parentKey, sNode)

	var edge *Edge
	if isEdgeBool {
		edge = &Edge{
			Path:  sNode.key,
			Child: sNode.node.Value,
		}
		if sNode.key.len == tri.height {
			return edge, nil, nil
		}
	}
	if sNode.key.len == tri.height {
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
	if isEdge(sNode.key, storageNode{node: rNode, key: sNode.node.Right}) {
		edge := ProofNode{Edge: &Edge{
			Path:  sNode.node.Right,
			Child: rNode.Value,
		}}
		rightHash = edge.Hash()
	}
	leftHash := lNode.Value
	if isEdge(sNode.key, storageNode{node: lNode, key: sNode.node.Left}) {
		edge := ProofNode{Edge: &Edge{
			Path:  sNode.node.Left,
			Child: lNode.Value,
		}}
		leftHash = edge.Hash()
	}
	binary := &Binary{
		LeftHash:  leftHash,
		RightHash: rightHash,
	}

	return edge, binary, nil
}

// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L514
// Note: Juno nodes are Edge AND Binary, whereas pathfinders are Edge XOR Binary, so
// we need to perform a transformation as we progress along Junos Trie.
func GetProof(leaf *felt.Felt, tri *Trie) ([]ProofNode, error) {
	leafKey := tri.feltToKey(leaf)
	nodesToLeaf, err := tri.nodesFromRoot(&leafKey)
	if err != nil {
		return nil, err
	}
	proofNodes := []ProofNode{}

	var parentKey *Key

	for i := 0; i < len(nodesToLeaf); i++ {
		if i != 0 {
			parentKey = nodesToLeaf[i-1].key
		}
		sNode := nodesToLeaf[i]
		sNodeEdge, sNodeBinary, err := transformNode(tri, parentKey, sNode)
		if err != nil {
			return nil, err
		}
		if sNodeEdge == nil && sNodeBinary == nil {
			break
		}

		isLeaf := sNode.key.len == tri.height
		if sNodeEdge != nil && !isLeaf { // Internal Edge
			proofNodes = append(proofNodes, []ProofNode{{Edge: sNodeEdge}, {Binary: sNodeBinary}}...)
		} else if sNodeEdge == nil && !isLeaf { // Internal Binary
			proofNodes = append(proofNodes, []ProofNode{{Binary: sNodeBinary}}...)
		} else if sNodeEdge != nil && isLeaf { // pre-leaf Edge
			proofNodes = append(proofNodes, []ProofNode{{Edge: sNodeEdge}}...)
		} else if sNodeEdge == nil && isLeaf { // pre-leaf binary
			proofNodes = append(proofNodes, []ProofNode{{Binary: sNodeBinary}}...)
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
