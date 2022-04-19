package trie

import (
	"encoding/json"
	"fmt"
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
	Next []node   `json:"-"`
}

// newNode initialises a new node with two null links.
func newNode() node {
	return node{Next: make([]node, 2)}
}

// clear sets the links in the node n to null.
func (n *node) clear() {
	n.Next = nil
}

// isEmpty returns true if the in-memory representation of a node is
// the empty node i.e. encoded by the triplet (0, 0, 0).
func (n *node) isEmpty() bool {
	return n.Next == nil
}

// updateHash updates the node hash.
func (n *node) updateHash() {
	if n.Length == 0 {
		n.Hash = new(big.Int).Set(n.Bottom)

		// DEBUG.
		digest := fmt.Sprintf("%x", n.Hash)
		if len(digest) > 3 {
			fmt.Printf("hash = %s\n", digest[len(digest)-4:])
		} else {
			fmt.Printf("hash = %s\n", digest)
		}
	} else {
		h, _ := pedersen.Digest(n.Bottom, n.Path)
		n.Hash = h.Add(h, new(big.Int).SetUint64(uint64(n.Length)))

		// DEBUG.
		digest := fmt.Sprintf("%x", n.Hash)
		fmt.Printf("hash = %s\n", digest[len(digest)-4:])
	}
	// DEBUG.
	fmt.Println()
}

// Bytes returns a JSON byte representation of a node.
func (n *node) Bytes() []byte {
	b, err := json.Marshal(n)
	if err != nil {
		// TODO: Handle properly.
		fmt.Printf("failed to marshal JSON: %v\n", err)
	}
	return b
}

// DEBUG.
// String makes [encoding] satisfy the [fmt.Stringer] interface.
func (e encoding) String() string {
	bottom := fmt.Sprintf("%x", e.Bottom)
	n := len(bottom)
	if n > 3 {
		bottom = bottom[n-4:]
	}
	return fmt.Sprintf("(%d, %d, %s)", e.Length, e.Path, bottom)
}
