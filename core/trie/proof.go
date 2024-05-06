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

	getValue := func(key *Key) (*felt.Felt, error) {
		node, err := tri.GetNodeFromKey(key)
		if err != nil {
			return nil, err
		}
		return node.Value, nil
		// return node.Hash(key, crypto.Pedersen), nil
	}

	height := uint8(0)

	// 1. If it's an edge-node in pathfinders impl, we need to expand the node into an edge + binary
	// -> Child should be internal node (len<251). Distance between child and parent should be > 1.
	// 2. If it's a binary-node, we store binary
	// -> Child should be internal node (len<251). Distance between child and parent should be 1.
	// 3. If it's a binary leaf, we store binary leaf
	// -> Child should be leaf (len=251). Distance between child and parent should be 1.
	// 4. If it's an edge leaf, we store an edge leaf
	// -> Child should be leaf (len=251). Distance between child and parent should be > 1.

	for i, sNode := range nodesExcludingLeaf {
		height += uint8(sNode.key.len)

		leftHash, err := getValue(sNode.node.Left)
		if err != nil {
			return nil, err
		}

		rightHash, err := getValue(sNode.node.Right)
		if err != nil {
			return nil, err
		}
		fmt.Println("LeftHash", leftHash.String())
		fmt.Println("rightHash", rightHash.String())

		child := nodesToLeaf[i+1]
		parentChildDistance := child.key.len - sNode.key.len

		if child.key.len < 251 && parentChildDistance == 1 { // Internal Binary
			proofNodes = append(proofNodes, ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			})
			height++
		} else if child.key.len < 251 && parentChildDistance > 1 { // Internal Edge
			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					Path:  sNode.key, // Todo: Path from that node to the leaf?
					Child: sNode.node.Value,
					// Value: value, // Todo: ??
				},
			})
			height += sNode.key.len
			proofNodes = append(proofNodes, ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			})
			height++
		} else if child.key.len == 251 && parentChildDistance == 1 { // Leaf binary
			proofNodes = append(proofNodes, ProofNode{
				Binary: &Binary{
					LeftHash:  leftHash,
					RightHash: rightHash,
				},
			})
			height++
		} else if child.key.len == 251 && parentChildDistance > 1 { // lead Edge
			proofNodes = append(proofNodes, ProofNode{
				Edge: &Edge{
					// Path:  sNode.key, // Todo: Path from that node to the leaf?
					Child: sNode.node.Value,
					// Value: value, // Todo: ??
				},
			})
			height += sNode.key.len
		} else {
			return nil, errors.New("unexpected error in GetProof")
		}

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
			if !proofNode.Edge.Path.Equal(remainingPath.SubKey(proofNode.Edge.Path.Len())) { // Todo: Isn't edge.path from root? and remaining from edge to leaf??
				return false
			}
			expectedHash = proofNode.Edge.Child
			remainingPath.Truncate(proofNode.Edge.Path.Len())
		}
	}
	return expectedHash.Equal(value)
}
