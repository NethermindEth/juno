package trie

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

// A Node represents a node in the [Trie]
// https://docs.starknet.io/architecture-and-concepts/network-architecture/starknet-state/#trie_construction
type Node struct {
	Value     *felt.Felt
	Left      *Key
	Right     *Key
	LeftHash  *felt.Felt
	RightHash *felt.Felt
}

// Hash calculates the hash of a [Node]
func (n *Node) Hash(path *Key, hashFunc HashFunc) *felt.Felt {
	if path.Len() == 0 {
		// we have to deference the Value, since the Node can released back
		// to the NodePool and be reused anytime
		hash := *n.Value
		return &hash
	}

	pathFelt := path.Felt()
	hash := hashFunc(n.Value, &pathFelt)
	pathFelt.SetUint64(uint64(path.Len()))
	return hash.Add(hash, &pathFelt)
}

// Hash calculates the hash of a [Node]
func (n *Node) HashFromParent(parnetKey, nodeKey *Key, hashFunc HashFunc) *felt.Felt {
	path := path(nodeKey, parnetKey)
	return n.Hash(&path, hashFunc)
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
		wrote, errInner := n.Left.WriteTo(buf)
		totalBytes += wrote
		if errInner != nil {
			return totalBytes, errInner
		}
		wrote, errInner = n.Right.WriteTo(buf) // n.Right is non-nil by design
		totalBytes += wrote
		if errInner != nil {
			return totalBytes, errInner
		}
	}

	if n.LeftHash == nil && n.RightHash == nil {
		return totalBytes, nil
	} else if (n.LeftHash != nil && n.RightHash == nil) || (n.LeftHash == nil && n.RightHash != nil) {
		return totalBytes, errors.New("cannot store only one lefthash or righthash")
	}

	leftHashB := n.LeftHash.Bytes()
	wrote, err = buf.Write(leftHashB[:])
	totalBytes += int64(wrote)
	if err != nil {
		return totalBytes, err
	}

	rightHashB := n.RightHash.Bytes()
	wrote, err = buf.Write(rightHashB[:])
	totalBytes += int64(wrote)
	if err != nil {
		return totalBytes, err
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
		n.LeftHash = nil
		n.RightHash = nil
		return nil
	}

	if n.Left == nil {
		n.Left = new(Key)
		n.Right = new(Key)
	}

	if err := n.Left.UnmarshalBinary(data); err != nil {
		return err
	}
	data = data[n.Left.EncodedLen():]
	if err := n.Right.UnmarshalBinary(data); err != nil {
		return err
	}
	data = data[n.Right.EncodedLen():]

	if n.LeftHash == nil {
		n.LeftHash = new(felt.Felt)
	}
	if n.RightHash == nil {
		n.RightHash = new(felt.Felt)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) != 2*felt.Bytes {
		return errors.New("the node does not contain both left and right hash")
	}
	n.LeftHash.SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]
	n.RightHash.SetBytes(data[:felt.Bytes])
	return nil
}
