package trie

import (
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
	Value *felt.Felt
	Left  *bitset.BitSet
	Right *bitset.BitSet
}

// Hash calculates the hash of a [Node]
func (n *Node) Hash(path *bitset.BitSet) *felt.Felt {
	if path.Len() == 0 {
		return n.Value
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
	hash := crypto.Pedersen(n.Value, pathFelt)

	pathFelt.SetUint64(uint64(path.Len()))
	return hash.Add(hash, pathFelt)
}

// Equal checks for equality of two [Node]s
func (n *Node) Equal(other *Node) bool {
	return n.Value.Equal(other.Value) && n.Left.Equal(other.Left) && n.Right.Equal(n.Right)
}
