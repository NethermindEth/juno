package trienode

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

const (
	binaryNodeSize      = 2 * hashOrValueNodeSize                         // LeftHash + RightHash
	edgeNodeMaxSize     = trieutils.MaxBitArraySize + hashOrValueNodeSize // Path + Child Hash
	hashOrValueNodeSize = felt.Bytes
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// The initial idea was to differentiate between binary and edge nodes by the size of the buffer.
// However, there may be a case where the size of the buffer is the same for both binary and edge nodes.
// In this case, we need to prepend the buffer with a byte to indicate the type of the node.
// Value and hash nodes do not need this because their size is fixed.
const (
	binaryNodeType byte = iota + 1
	edgeNodeType
)

// Enc(binary) = binaryNodeType + HashNode(left) + HashNode(right)
func (n *BinaryNode) Write(buf *bytes.Buffer) error {
	if err := buf.WriteByte(binaryNodeType); err != nil {
		return err
	}

	if err := n.Left().Write(buf); err != nil {
		return err
	}

	if err := n.Right().Write(buf); err != nil {
		return err
	}

	return nil
}

// Enc(edge) = edgeNodeType + HashNode(child) + Path
func (n *EdgeNode) Write(buf *bytes.Buffer) error {
	if err := buf.WriteByte(edgeNodeType); err != nil {
		return err
	}

	if err := n.Child.Write(buf); err != nil {
		return err
	}

	if _, err := n.Path.Write(buf); err != nil {
		return err
	}

	return nil
}

// Enc(hash) = Felt
func (n *HashNode) Write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

// Enc(value) = Felt
func (n *ValueNode) Write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

// Returns the encoded bytes of a node
func EncodeNode(n Node) []byte {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := n.Write(buf); err != nil {
		panic(err)
	}

	res := make([]byte, buf.Len())
	copy(res, buf.Bytes())
	return res
}

// Decodes the encoded bytes and returns the corresponding node
func DecodeNode(blob []byte, hash felt.Felt, pathLen, maxPathLen uint8) (Node, error) {
	if len(blob) == 0 {
		return nil, errors.New("cannot decode empty blob")
	}
	if pathLen > maxPathLen {
		return nil, fmt.Errorf("node path length (%d) > max (%d)", pathLen, maxPathLen)
	}

	if n, ok := decodeHashOrValueNode(blob, pathLen, maxPathLen); ok {
		return n, nil
	}

	nodeType := blob[0]
	blob = blob[1:]

	switch nodeType {
	case binaryNodeType:
		return decodeBinaryNode(blob, hash, pathLen, maxPathLen)
	case edgeNodeType:
		return decodeEdgeNode(blob, hash, pathLen, maxPathLen)
	default:
		panic(fmt.Sprintf("unknown node type: %d", nodeType))
	}
}

func decodeHashOrValueNode(blob []byte, pathLen, maxPathLen uint8) (Node, bool) {
	if len(blob) == hashOrValueNodeSize {
		var f felt.Felt
		f.SetBytes(blob)
		if pathLen == maxPathLen {
			return &ValueNode{Felt: f}, true
		}
		return &HashNode{Felt: f}, true
	}
	return nil, false
}

func decodeBinaryNode(blob []byte, hash felt.Felt, pathLen, maxPathLen uint8) (*BinaryNode, error) {
	if len(blob) < 2*hashOrValueNodeSize {
		return nil, fmt.Errorf("invalid binary node size: %d", len(blob))
	}

	binary := &BinaryNode{}
	if !hash.IsZero() {
		binary.Flags.Hash = &HashNode{Felt: hash}
	}

	var err error
	if binary.Children[0], err = DecodeNode(blob[:hashOrValueNodeSize], hash, pathLen+1, maxPathLen); err != nil {
		return nil, err
	}
	if binary.Children[1], err = DecodeNode(blob[hashOrValueNodeSize:], hash, pathLen+1, maxPathLen); err != nil {
		return nil, err
	}
	return binary, nil
}

func decodeEdgeNode(blob []byte, hash felt.Felt, pathLen, maxPathLen uint8) (*EdgeNode, error) {
	if len(blob) > edgeNodeMaxSize || len(blob) < hashOrValueNodeSize {
		return nil, fmt.Errorf("invalid edge node size: %d", len(blob))
	}

	edge := &EdgeNode{Path: &trieutils.Path{}}
	if !hash.IsZero() {
		edge.Flags.Hash = &HashNode{Felt: hash}
	}

	var err error
	if edge.Child, err = DecodeNode(blob[:hashOrValueNodeSize], felt.Zero, pathLen, maxPathLen); err != nil {
		return nil, err
	}
	if err := edge.Path.UnmarshalBinary(blob[hashOrValueNodeSize:]); err != nil {
		return nil, err
	}

	if pathLen+edge.Path.Len() == maxPathLen {
		edge.Child = &ValueNode{Felt: edge.Child.(*HashNode).Felt}
	}
	return edge, nil
}
