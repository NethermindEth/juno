package trienode

import (
	"github.com/NethermindEth/juno/core/felt"
)

type Node struct {
	blob []byte
	hash felt.Felt
}

func (r *Node) IsDeleted() bool {
	return len(r.blob) == 0
}

func NewNode(hash felt.Felt, blob []byte) *Node {
	return &Node{hash: hash, blob: blob}
}

func NewDeleted() *Node {
	return &Node{hash: felt.Felt{}, blob: nil}
}
