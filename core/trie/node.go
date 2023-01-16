package trie

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
)

type ErrMalformedNode struct {
	reason string
}

func (e ErrMalformedNode) Error() string {
	return fmt.Sprintf("malformed node: %s", e.reason)
}

// A Node represents a node in the [Trie]
type Node struct {
	value *felt.Felt
	left  *bitset.BitSet
	right *bitset.BitSet
}

// Hash calculates the hash of a [Node]
func (n *Node) Hash(path *bitset.BitSet) *felt.Felt {
	if path.Len() == 0 {
		return n.value
	}

	pathWords := path.Bytes()
	if len(pathWords) > 4 {
		panic("key too long to fit in Felt")
	}

	var pathBytes [32]byte
	for idx, word := range pathWords {
		startBytes := 24 - (idx * 8)
		binary.BigEndian.PutUint64(pathBytes[startBytes:startBytes+8], word)
	}

	pathFelt := new(felt.Felt).SetBytes(pathBytes[:])

	// https://docs.starknet.io/documentation/develop/State/starknet-state/
	hash := crypto.Pedersen(n.value, pathFelt)

	pathFelt.SetUint64(uint64(path.Len()))
	return hash.Add(hash, pathFelt)
}

// Equal checks for equality of two [Node]s
func (n *Node) Equal(other *Node) bool {
	return n.value.Equal(other.value) && n.left.Equal(other.left) && n.right.Equal(n.right)
}

// MarshalBinary serializes a [Node] into a byte array
func (n *Node) MarshalBinary() ([]byte, error) {
	if n.value == nil {
		return nil, ErrMalformedNode{"cannot marshal node with nil value"}
	}

	var ret []byte
	valueB := n.value.Bytes()
	ret = append(ret, valueB[:]...)

	if n.left != nil {
		ret = append(ret, 'l')
		leftB, err := n.left.MarshalBinary()
		if err != nil {
			return nil, err
		}
		ret = append(ret, leftB...)
	}

	if n.right != nil {
		ret = append(ret, 'r')
		rightB, err := n.right.MarshalBinary()
		if err != nil {
			return nil, err
		}
		ret = append(ret, rightB...)
	}
	return ret, nil
}

// UnmarshalBinary deserializes a [Node] from a byte array
func (n *Node) UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return ErrMalformedNode{"size of input data is less than felt size"}
	}
	n.value = new(felt.Felt).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]

	stream := bytes.NewReader(data)
	for stream.Len() > 0 {
		head, err := stream.ReadByte()
		if err != nil {
			return err
		}

		var pathP **bitset.BitSet
		switch head {
		case 'l':
			pathP = &(n.left)
		case 'r':
			pathP = &(n.right)
		default:
			return ErrMalformedNode{"unknown child node prefix"}
		}
		if *pathP != nil {
			return ErrMalformedNode{"multiple children are not supported"}
		}

		*pathP = new(bitset.BitSet)
		_, err = (*pathP).ReadFrom(stream)
		if err != nil {
			return err
		}
	}

	return nil
}
