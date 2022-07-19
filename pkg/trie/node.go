package trie

import (
	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/felt"
)

var EmptyNode = &leafNode{&felt.Felt{}}

type TrieNode interface {
	Path() *collections.BitSet
	Bottom() *felt.Felt
	Hash() *felt.Felt
}

type BinaryNode struct {
	bottom *felt.Felt

	LeftH  *felt.Felt
	RightH *felt.Felt
}

func (n *BinaryNode) Path() *collections.BitSet {
	return collections.EmptyBitSet
}

func (n *BinaryNode) Bottom() *felt.Felt {
	if n.bottom != nil {
		return n.bottom
	}

	bottom := pedersen.Digest(n.LeftH, n.RightH)
	n.bottom = bottom
	return n.bottom
}

func (n *BinaryNode) Hash() *felt.Felt {
	return n.Bottom()
}

type EdgeNode struct {
	hash *felt.Felt

	path   *collections.BitSet
	bottom *felt.Felt
}

func NewEdgeNode(path *collections.BitSet, bottom *felt.Felt) *EdgeNode {
	// TODO: we can remove this constructor if we change parh and bottom to
	// be public fields in EdgeNode, but now we can't because colide with
	// Path() and Bottom() methods
	return &EdgeNode{
		path:   path,
		bottom: bottom,
	}
}

func (n *EdgeNode) Path() *collections.BitSet {
	return n.path
}

func (n *EdgeNode) Bottom() *felt.Felt {
	return n.bottom
}

func (n *EdgeNode) Hash() *felt.Felt {
	if n.hash != nil {
		return n.hash
	}

	path := new(felt.Felt).SetBytes(n.path.Bytes())
	pedersen := pedersen.Digest(n.bottom, path)
	lenFelt := new(felt.Felt).SetInt64(int64(n.path.Len()))
	n.hash = new(felt.Felt).Add(pedersen, lenFelt)
	return n.hash
}

type leafNode struct {
	value *felt.Felt
}

func (n *leafNode) Path() *collections.BitSet {
	return collections.EmptyBitSet
}

func (n *leafNode) Bottom() *felt.Felt {
	return n.value
}

func (n *leafNode) Hash() *felt.Felt {
	return n.value
}

func (n *leafNode) Data() []byte {
	return n.value.ByteSlice()
}
