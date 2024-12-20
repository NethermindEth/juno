package trie

import (
	"bytes"
	"errors"
	"fmt"

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
func (n *Node) Hash(path *Key, hashFunc hashFunc) *felt.Felt {
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
func (n *Node) HashFromParent(parentKey, nodeKey *Key, hashFunc hashFunc) *felt.Felt {
	path := path(nodeKey, parentKey)
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

func (n *Node) String() string {
	return fmt.Sprintf("Node{Value: %s, Left: %s, Right: %s, LeftHash: %s, RightHash: %s}", n.Value, n.Left, n.Right, n.LeftHash, n.RightHash)
}

// Update the receiver with non-nil fields from the `other` Node.
// If a field is non-nil in both Nodes, they must be equal, or an error is returned.
//
// This method modifies the receiver in-place and returns an error if any field conflicts are detected.
//
//nolint:gocyclo
func (n *Node) Update(other *Node) error {
	// First validate all fields for conflicts
	if n.Value != nil && other.Value != nil && !n.Value.Equal(other.Value) {
		return fmt.Errorf("conflicting Values: %v != %v", n.Value, other.Value)
	}

	if n.Left != nil && other.Left != nil && !n.Left.Equal(NilKey) && !other.Left.Equal(NilKey) && !n.Left.Equal(other.Left) {
		return fmt.Errorf("conflicting Left keys: %v != %v", n.Left, other.Left)
	}

	if n.Right != nil && other.Right != nil && !n.Right.Equal(NilKey) && !other.Right.Equal(NilKey) && !n.Right.Equal(other.Right) {
		return fmt.Errorf("conflicting Right keys: %v != %v", n.Right, other.Right)
	}

	if n.LeftHash != nil && other.LeftHash != nil && !n.LeftHash.Equal(other.LeftHash) {
		return fmt.Errorf("conflicting LeftHash: %v != %v", n.LeftHash, other.LeftHash)
	}

	if n.RightHash != nil && other.RightHash != nil && !n.RightHash.Equal(other.RightHash) {
		return fmt.Errorf("conflicting RightHash: %v != %v", n.RightHash, other.RightHash)
	}

	// After validation, perform all updates
	if other.Value != nil {
		n.Value = other.Value
	}
	if other.Left != nil && !other.Left.Equal(NilKey) {
		n.Left = other.Left
	}
	if other.Right != nil && !other.Right.Equal(NilKey) {
		n.Right = other.Right
	}
	if other.LeftHash != nil {
		n.LeftHash = other.LeftHash
	}
	if other.RightHash != nil {
		n.RightHash = other.RightHash
	}

	return nil
}
