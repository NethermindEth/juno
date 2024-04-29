package trie

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type Membership int

const (
	Member Membership = iota
	NonMember
)

type ProofNode struct {
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L514
func GetProof(leaf *felt.Felt, tri *Trie) ([]ProofNode, error) {
	leafKey := tri.feltToKey(leaf)
	nodesToLeaf, err := tri.nodesFromRoot(&leafKey)
	if err != nil {
		return nil, err
	}
	proofNodes := make([]ProofNode, len(nodesToLeaf)-1)

	getHash := func(tri *Trie, key *Key) (*felt.Felt, error) {
		keyFelt := key.Felt()
		node, err := tri.GetNode(&keyFelt)
		if err != nil {
			return nil, err
		}
		return node.Hash(key, crypto.Pedersen), nil
	}

	for i, sNode := range nodesToLeaf[:len(nodesToLeaf)-1] {
		leftHash, err := getHash(tri, sNode.node.Left)
		if err != nil {
			return nil, err
		}
		rightHash, err := getHash(tri, sNode.node.Right)
		if err != nil {
			return nil, err
		}
		proofNodes[i] = ProofNode{
			LeftHash:  leftHash,
			RightHash: rightHash,
		}
	}
	return proofNodes, nil
}

// verifyProof checks if `leafPath` leads from `root` to `leafHash` along the `proofNodes`
// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2006
func VerifyProof(root *felt.Felt, leafPath *Key, leafHash felt.Felt, proofNodes []ProofNode, hashFunc hashFunc) (Membership, error) {
	expectedHash := root

	for i, pNode := range proofNodes {
		pNodeHash := hashFunc(pNode.LeftHash, pNode.RightHash)
		if !expectedHash.Equal(pNodeHash) {
			return NonMember, errors.New("proof node does not have expected hash")
		}

		if leafPath.Test(leafPath.Len() - uint8(i) - 1) { // Todo: are we selecting the correct child here??
			expectedHash = pNode.RightHash
		} else {
			expectedHash = pNode.LeftHash
		}
	}

	if !expectedHash.Equal(&leafHash) {
		return NonMember, errors.New("value does not match")
	}

	return Member, nil
}
