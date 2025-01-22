package trie2

import (
	"bytes"
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
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func (n *binaryNode) write(buf *bytes.Buffer) error {
	if err := n.children[0].write(buf); err != nil {
		return err
	}

	if err := n.children[1].write(buf); err != nil {
		return err
	}

	return nil
}

func (n *edgeNode) write(buf *bytes.Buffer) error {
	if err := n.child.write(buf); err != nil {
		return err
	}

	if _, err := n.path.Write(buf); err != nil {
		return err
	}

	return nil
}

func (n *hashNode) write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

func (n *valueNode) write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

func nodeToBytes(n node) []byte {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := n.write(buf); err != nil {
		panic(err)
	}

	res := make([]byte, buf.Len())
	copy(res, buf.Bytes())
	return res
}

func decodeNode(blob []byte, hash felt.Felt, pathLen, maxPathLen uint8) (node, error) {
	var (
		n      node
		err    error
		isLeaf bool
	)

	if pathLen > maxPathLen {
		return nil, fmt.Errorf("node path length (%d) is greater than max path length (%d)", pathLen, maxPathLen)
	}

	isLeaf = pathLen == maxPathLen

	switch len(blob) {
	case hashOrValueNodeSize:
		var f felt.Felt
		f.SetBytes(blob)
		if isLeaf {
			n = &valueNode{Felt: f}
		} else {
			n = &hashNode{Felt: f}
		}
	case binaryNodeSize:
		binary := &binaryNode{flags: nodeFlag{hash: &hashNode{Felt: hash}}} // cache the hash
		binary.children[0], err = decodeNode(blob[:hashOrValueNodeSize], hash, pathLen+1, maxPathLen)
		if err != nil {
			return nil, err
		}
		binary.children[1], err = decodeNode(blob[hashOrValueNodeSize:], hash, pathLen+1, maxPathLen)
		if err != nil {
			return nil, err
		}

		n = binary
	default:
		// Ensure the blob length is within the valid range for an edge node
		if len(blob) > edgeNodeMaxSize || len(blob) < hashOrValueNodeSize {
			return nil, fmt.Errorf("invalid node size: %d", len(blob))
		}

		edge := &edgeNode{
			path:  &trieutils.BitArray{},
			flags: nodeFlag{hash: &hashNode{Felt: hash}}, // cache the hash
		}
		edge.child, err = decodeNode(blob[:hashOrValueNodeSize], felt.Felt{}, pathLen, maxPathLen)
		if err != nil {
			return nil, err
		}
		if err := edge.path.UnmarshalBinary(blob[hashOrValueNodeSize:]); err != nil {
			return nil, err
		}

		// We do another path length check to see if the node is a leaf
		if pathLen+edge.path.Len() == maxPathLen {
			edge.child = &valueNode{Felt: edge.child.(*hashNode).Felt}
		}

		n = edge
	}

	return n, nil
}
