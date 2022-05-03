package trie

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
)

// encoding represents the enccoding of a node in a binary tree
// represented by the triplet (length, path, bottom).
type encoding struct {
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

// hash updates the node hash.
func (n *node) hash() {
	if n.Length == 0 {
		n.Hash = new(big.Int).Set(n.Bottom)
		return
	}
	res, _ := pedersen.Digest(n.Bottom, n.Path)
	n.Hash = res.Add(res, big.NewInt(int64(n.Length)))
}
