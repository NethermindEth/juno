package trie

import (
	"encoding/binary"

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
