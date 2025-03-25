package trienode

import (
	"github.com/NethermindEth/juno/core/felt"
)

// Represents a raw non-leaf trie node, which contains the encoded blob and the hash of the node.
type NonLeafNode struct {
	blob []byte
	hash felt.Felt
}

func NewNonLeaf(hash felt.Felt, blob []byte) *NonLeafNode {
	return &NonLeafNode{hash: hash, blob: blob}
}

func (r *NonLeafNode) Blob() []byte    { return r.blob }
func (r *NonLeafNode) Hash() felt.Felt { return r.hash }
func (r *NonLeafNode) IsLeaf() bool    { return false }

type LeafNode struct {
	blob []byte
}

func NewLeaf(blob []byte) *LeafNode {
	return &LeafNode{blob: blob}
}

func (r *LeafNode) Blob() []byte    { return r.blob }
func (r *LeafNode) Hash() felt.Felt { return felt.Felt{} }
func (r *LeafNode) IsLeaf() bool    { return true }

type DeletedNode struct {
	isLeaf bool
}

func NewDeleted(isLeaf bool) *DeletedNode {
	return &DeletedNode{isLeaf: isLeaf}
}

func (r *DeletedNode) Blob() []byte    { return nil }
func (r *DeletedNode) Hash() felt.Felt { return felt.Felt{} }
func (r *DeletedNode) IsLeaf() bool    { return r.isLeaf }
