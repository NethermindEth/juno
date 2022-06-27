package trie

import (
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	// constants
	EmptyNode = &leafNode{&types.Felt0}

	// errors
	ErrMarshalUnmarshal = errors.New("node marshal/unmarshal error")
)

type TrieNode interface {
	Path() *collections.BitSet
	Bottom() *types.Felt
	Hash() *types.Felt
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
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

func (n *BinaryNode) MarshalBinary() ([]byte, error) {
	b := make([]byte, 2*types.FeltLength)
	copy(b[:types.FeltLength], n.LeftH.Bytes())
	copy(b[types.FeltLength:], n.RightH.Bytes())
	return b, nil
}

func (n *BinaryNode) UnmarshalBinary(b []byte) error {
	if len(b) != 2*types.FeltLength {
		return ErrMarshalUnmarshal
	}
	leftFelt := types.BytesToFelt(b[:types.FeltLength])
	rightFelt := types.BytesToFelt(b[types.FeltLength:])
	n.LeftH, n.RightH = &leftFelt, &rightFelt
	return nil
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

func (n *EdgeNode) MarshalBinary() ([]byte, error) {
	b := make([]byte, 2*types.FeltLength+1)
	copy(b[:types.FeltLength], n.bottom.Bytes())
	copy(b[types.FeltLength:2*types.FeltLength], types.BytesToFelt(n.path.Bytes()).Bytes())
	b[2*types.FeltLength] = uint8(n.path.Len())
	return b, nil
}

func (n *EdgeNode) UnmarshalBinary(b []byte) error {
	if len(b) != 2*types.FeltLength+1 {
		return ErrMarshalUnmarshal
	}
	bottom := types.BytesToFelt(b[:types.FeltLength])
	length := int(b[2*types.FeltLength])
	path := collections.NewBitSet(length, b[types.FeltLength:2*types.FeltLength])
	n.bottom, n.path = &bottom, path
	return nil
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

func (n *leafNode) MarshalBinary() ([]byte, error) {
	return n.value.Bytes(), nil
}

func (n *leafNode) UnmarshalBinary(b []byte) error {
	if len(b) > types.FeltLength {
		return ErrMarshalUnmarshal
	}
	value := types.BytesToFelt(b)
	n.value = &value
	return nil
}
