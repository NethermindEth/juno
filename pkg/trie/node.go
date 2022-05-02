package trie

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
)

// encoding represents the enccoding of a node in a binary tree
// represented by the triplet (length, path, bottom).
type encoding struct {
	// Using a uint8 seems safe as the height of the largest key in the
	// protocol is 251 which is less than the highest number representable
	// by a uint8, 255.
	Length uint8    `json:"length"`
	Path   *big.Int `json:"path"`
	Bottom *big.Int `json:"bottom"`
}

// node represents a node in a binary tree.
type node struct {
	encoding
	Hash *big.Int `json:"hash"`
}

// bytes returns a JSON byte representation of a node.
func (n *node) bytes() []byte {
	b, _ := json.Marshal(n)
	return b
}

// updateHash updates the node hash.
func (n *node) updateHash() {
	if n.Length == 0 {
		n.Hash = new(big.Int).Set(n.Bottom)
	} else {
		h, _ := pedersen.Digest(n.Bottom, n.Path)
		n.Hash = h.Add(h, big.NewInt(int64(n.Length)))
	}
}
