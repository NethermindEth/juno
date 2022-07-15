package trie

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/felt"
)

// Encoding represents the Encoding of a node in a binary tree
// represented by the triplet (length, path, bottom).
type Encoding struct {
	Length uint8      `json:"length"`
	Path   *felt.Felt `json:"path"`
	Bottom *felt.Felt `json:"bottom"`
}

// Node represents a Node in a binary tree.
type Node struct {
	Encoding
	Hash *felt.Felt `json:"hash"`
}

// bytes returns a JSON byte representation of a node.
func (n *Node) bytes() []byte {
	b, _ := json.Marshal(n)
	return b
}

// hash updates the node hash.
func (n *Node) hash() {
	if n.Length == 0 {
		n.Hash = n.Bottom
		return
	}
	h := pedersen.Digest(n.Bottom, n.Path)
	n.Hash = h.Add(h, new(felt.Felt).SetUint64(uint64(n.Length)))
}
