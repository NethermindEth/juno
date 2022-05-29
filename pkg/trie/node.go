package trie

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
)

// Encoding represents the Encoding of a node in a binary tree
// represented by the triplet (length, path, bottom).
type Encoding struct {
	Length uint8    `json:"length"`
	Path   *big.Int `json:"path"`
	Bottom *big.Int `json:"bottom"`
}

// Node represents a Node in a binary tree.
type Node struct {
	Encoding
	Hash *big.Int `json:"hash"`
}

// bytes returns a JSON byte representation of a node.
func (n *Node) bytes() []byte {
	b, _ := json.Marshal(n)
	return b
}

// hash updates the node hash.
func (n *Node) hash() {
	if n.Length == 0 {
		n.Hash = new(big.Int).Set(n.Bottom)
		return
	}
	h := pedersen.Digest(n.Bottom, n.Path)
	n.Hash = h.Add(h, new(big.Int).SetUint64(uint64(n.Length)))
}
