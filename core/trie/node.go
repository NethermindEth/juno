package trie

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
)

// A Node represents a node in the [Trie]
type Node struct {
	Value *felt.Felt
	Left  *bitset.BitSet
	Right *bitset.BitSet
}

// Hash calculates the hash of a [Node]
func (n *Node) Hash(path *bitset.BitSet, hashFunc hashFunc) *felt.Felt {
	if path.Len() == 0 {
		// we have to deference the Value, since the Node can released back
		// to the NodePool and be reused anytime
		hash := *n.Value
		return &hash
	}

	pathWords := path.Bytes()
	if len(pathWords) > felt.Limbs {
		panic("key too long to fit in Felt")
	}

	var pathBytes [felt.Bytes]byte
	for idx, word := range pathWords {
		startBytes := 24 - (idx * 8)
		binary.BigEndian.PutUint64(pathBytes[startBytes:startBytes+8], word)
	}

	pathFelt := new(felt.Felt).SetBytes(pathBytes[:])

	// https://docs.starknet.io/documentation/develop/State/starknet-state/
	hash := hashFunc(n.Value, pathFelt)

	pathFelt.SetUint64(uint64(path.Len()))
	return hash.Add(hash, pathFelt)
}

func (n *Node) WriteTo(buf *bytes.Buffer) (int64, error) {
	if n.Value == nil {
		return 0, errors.New("cannot marshal node with nil value")
	}

	totalBytes := int64(0)

	valueB := n.Value.Bytes()
	wrote, err := buf.Write(valueB[:])
	totalBytes += int64(wrote)
	if err != nil {
		return totalBytes, err
	}

	if n.Left != nil {
		wrote, err := n.Left.WriteTo(buf)
		totalBytes += wrote
		if err != nil {
			return totalBytes, err
		}
		wrote, err = n.Right.WriteTo(buf) // n.Right is non-nil by design
		totalBytes += wrote
		if err != nil {
			return totalBytes, err
		}
	}

	return totalBytes, nil
}

// UnmarshalBinary deserializes a [Node] from a byte array
func (n *Node) UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return errors.New("size of input data is less than felt size")
	}
	if n.Value == nil {
		n.Value = new(felt.Felt)
	}
	n.Value.SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]

	if len(data) == 0 {
		n.Left = nil
		n.Right = nil
		return nil
	}

	stream := bytes.NewReader(data)

	if n.Left == nil {
		n.Left = new(bitset.BitSet)
		n.Right = new(bitset.BitSet)
	}

	_, err := n.Left.ReadFrom(stream)
	if err != nil {
		return err
	}
	_, err = n.Right.ReadFrom(stream)
	return err
}
