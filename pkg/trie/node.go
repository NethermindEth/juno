package trie

import (
	"math/big"

	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

// constants
var EmptyNode = &leafNode{&types.Felt0}

type TrieNode interface {
	Path() *collections.BitSet
	Bottom() *types.Felt
	Hash() *types.Felt
	Data() []byte
}

type BinaryNode struct {
	bottom *types.Felt

	LeftH  *types.Felt
	RightH *types.Felt
}

func (n *BinaryNode) Path() *collections.BitSet {
	return collections.EmptyBitSet
}

func (n *BinaryNode) Bottom() *types.Felt {
	if n.bottom != nil {
		return n.bottom
	}

	bottom := types.BigToFelt(pedersen.Digest(n.LeftH.Big(), n.RightH.Big()))
	n.bottom = &bottom
	return n.bottom
}

func (n *BinaryNode) Hash() *types.Felt {
	return n.Bottom()
}

func (n *BinaryNode) Data() []byte {
	b := make([]byte, 2*types.FeltLength)
	copy(b[:types.FeltLength], n.LeftH.Bytes())
	copy(b[types.FeltLength:], n.RightH.Bytes())
	return b
}

type EdgeNode struct {
	hash *types.Felt

	path   *collections.BitSet
	bottom *types.Felt
}

func NewEdgeNode(path *collections.BitSet, bottom *types.Felt) *EdgeNode {
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

func (n *EdgeNode) Bottom() *types.Felt {
	return n.bottom
}

func (n *EdgeNode) Hash() *types.Felt {
	if n.hash != nil {
		return n.hash
	}

	pathBig := new(big.Int).SetBytes(n.path.Bytes())
	pedersen := types.BigToFelt(pedersen.Digest(n.bottom.Big(), pathBig))
	lenBig := new(big.Int).SetUint64(uint64(n.path.Len()))
	lenFelt := types.BigToFelt(lenBig)
	hash := pedersen.Add(&lenFelt)
	n.hash = &hash
	return n.hash
}

func (n *EdgeNode) Data() []byte {
	b := make([]byte, 2*types.FeltLength+1)
	copy(b[:types.FeltLength], n.bottom.Bytes())
	copy(b[types.FeltLength:2*types.FeltLength], types.BytesToFelt(n.path.Bytes()).Bytes())
	b[2*types.FeltLength] = uint8(n.path.Len())
	return b
}

type leafNode struct {
	value *types.Felt
}

func (n *leafNode) Path() *collections.BitSet {
	return collections.EmptyBitSet
}

func (n *leafNode) Bottom() *types.Felt {
	return n.value
}

func (n *leafNode) Hash() *types.Felt {
	return n.value
}

func (n *leafNode) Data() []byte {
	return n.value.Bytes()
}
