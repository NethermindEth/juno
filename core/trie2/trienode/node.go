package trienode

import (
	"github.com/NethermindEth/juno/core/felt"
)

// Represents a raw trie node, which contains the encoded blob and the hash of the node.
type Node struct {
	blob []byte
	hash felt.Felt
}

func (r *Node) IsDeleted() bool {
	return len(r.blob) == 0
}

func (r *Node) Blob() []byte {
	return r.blob
}

func (r *Node) Hash() felt.Felt {
	return r.hash
}

func NewNode(hash felt.Felt, blob []byte) *Node {
	return &Node{hash: hash, blob: blob}
}

func NewDeleted() *Node {
	return &Node{hash: felt.Felt{}, blob: nil}
}
