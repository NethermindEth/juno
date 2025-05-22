package trienode

import (
	"github.com/NethermindEth/juno/core/felt"
)

var (
	_ TrieNode = &NonLeafNode{}
	_ TrieNode = &LeafNode{}
	_ TrieNode = &DeletedNode{}
)

type TrieNode interface {
	Blob() []byte
	Hash() felt.Felt
	IsLeaf() bool
}

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
	hash felt.Felt
}

func NewLeaf(hash felt.Felt, blob []byte) *LeafNode {
	return &LeafNode{blob: blob, hash: hash}
}

func (r *LeafNode) Blob() []byte    { return r.blob }
func (r *LeafNode) Hash() felt.Felt { return r.hash }
func (r *LeafNode) IsLeaf() bool    { return true }

type DeletedNode struct {
	isLeaf bool
	hash   *felt.Felt
}

func NewDeleted(isLeaf bool, hash *felt.Felt) *DeletedNode {
	return &DeletedNode{isLeaf: isLeaf, hash: hash}
}

func (r *DeletedNode) Blob() []byte { return nil }
func (r *DeletedNode) Hash() felt.Felt {
	if r.hash == nil {
		return felt.Zero
	}
	return *r.hash
}
func (r *DeletedNode) IsLeaf() bool { return r.isLeaf }
