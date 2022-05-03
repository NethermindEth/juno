// Package trie implements the [Merkle-PATRICIA tree] used in the
// StarkNet protocol.
//
// The shape of the trie is independent of the key insertion or deletion
// order. In other words, the same set of keys is guaranteed to produce
// the same tree irrespective of the order of which keys are put and
// removed from the tree. This property guarantees that the commitment
// will thus remain consistent across trees that use a different
// insertion and deletion order.
//
// # Worst-case time bounds for get and put operations
//
// The get operation is subject to the structure of the underlying data
// store. For all intents and purposes this can be assumed to be
// "constant" but in reality, if for example, a hashing based symbol
// table is used as a backend to this structure, then the get operation
// will take as much time as insertions take on that data structure.
//
// Put and delete operations on the other hand take at most 1  + w
// database accesses where w represents the bit-length of the key which
// is optimal.
//
// # Space
//
// The tree only stores non-empty nodes so the space complexity is n * w
// where w is the key length.
//
// [Merkle-Patricia tree]: https://docs.starknet.io/docs/State/starknet-state#merkle-patricia-tree
package trie

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/store"
)

// A visualisation of the trie with 3-bit keys which results in a tree
// of height 2 where empty nodes are not stored in the database.
//
//                           (0,0,r)
//                           /     \
//                        0 /       \ 1
//                         /         \
//                   (2,2,1)         (2,1,1)
//                     / \             / \
//                    /   \ 1       0 /   \
//                         \         /
//                       (1,0,1) (1,1,1)
//                         / \     / \
//                      0 /   \   /   \ 1
//                       /             \
//                   (0,0,1)        (0,0,1)
//                     / \            / \
//                    /   \          /   \
//
// Put and delete operations work by first committing (or removing) the
// bottom node from the backend store and then traversing upwards to
// compute the new node encodings and hashes which result in a new tree
// commitment.

// Trie represents a binary trie.
type Trie struct {
	keyLen int
	store  store.Storer
}

// New constructs a new binary trie.
func New(store store.Storer, keyLen int) Trie {
	return Trie{keyLen: keyLen, store: store}
}

// commit persits the given key-value pair in storage.
func (t *Trie) commit(key, val []byte) {
	if bytes.Equal(key, []byte("")) {
		key = []byte("root")
	}
	t.store.Put(key, val)
}

// remove deletes a key-value pair from storage.
func (t *Trie) remove(key []byte) {
	if bytes.Equal(key, []byte("")) {
		key = []byte("root")
	}
	t.store.Delete(key)
}

// retrieve gets a node from storage and returns true if the node was
// found.
func (t *Trie) retrive(key []byte) (node, bool) {
	if bytes.Equal(key, []byte("")) {
		// notest
		key = []byte("root")
	}
	b, ok := t.store.Get(key)
	if !ok {
		return node{}, false
	}
	var n node
	if err := json.Unmarshal(b, &n); err != nil {
		// notest
		return node{}, false
	}
	return n, true
}

// diff traverses the tree upwards from the given path (key) starting
// from the node that immediately precedes the bottom node and either
// deletes the node if it its child nodes are empty or recomputes the
// encoding and hashes otherwise.
func (t *Trie) diff(key *big.Int) {
	for height := t.keyLen - 1; height >= 0; height-- {
		parent := prefix(key, height)

		_, leftChildIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s0", parent)))
		_, rightChildIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s1", parent)))

		if !leftChildIsNotEmpty && !rightChildIsNotEmpty {
			t.remove(parent)
		} else {
			t.newNodeAt(parent)
		}
	}
}

// newNodeAt creates a new parent node at the specified path. Note that
// this should only be used to create parent nodes as nodes that contain
// values are encoded differently.
func (t *Trie) newNodeAt(path []byte) {
	n := node{}
	t.triplet(&n, path)
	n.hash()
	t.commit(path, n.bytes())
}

// triplet encodes a given (parent) node in the trie according to the
// specification (see package documentation for more details).
func (t *Trie) triplet(n *node, pre []byte) {
	left, leftIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s0", pre)))
	right, rightIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s1", pre)))

	switch {
	case !rightIsNotEmpty && !leftIsNotEmpty:
		panic("attempted to encode an empty node")
	case !rightIsNotEmpty:
		n.encoding = encoding{left.Length + 1, left.Path, new(big.Int).Set(left.Bottom)}
	case !leftIsNotEmpty:
		n.encoding = encoding{
			right.Length + 1,
			right.Path.Add(
				right.Path,
				new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(right.Length)), nil),
			),
			new(big.Int).Set(right.Bottom),
		}
	default:
		h, _ := pedersen.Digest(left.Hash, right.Hash)
		n.encoding = encoding{0, new(big.Int), h}
	}
}

// Delete removes a key-value pair from the trie.
func (t *Trie) Delete(key *big.Int) {
	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the oppose so
	// a copy with the bits reversed is used instead.
	rev := reversed(key, t.keyLen)

	t.remove(prefix(rev, t.keyLen))
	t.diff(rev)
}

// Get retrieves a value from the trie with the corresponding key.
func (t *Trie) Get(key *big.Int) (*big.Int, bool) {
	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the opposite so
	// a copy with the bits reversed is passed into the function.
	node, ok := t.retrive(prefix(reversed(key, t.keyLen), t.keyLen))
	if !ok {
		return nil, false
	}
	return node.Bottom, true
}

// Put inserts a [big.Int] key-value pair in the trie.
func (t *Trie) Put(key, val *big.Int) {
	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the oppose so a
	// copy with the bits reversed is used instead.
	rev := reversed(key, t.keyLen)

	if val.Cmp(new(big.Int)) == 0 {
		t.Delete(rev)
		return
	}

	leaf := node{encoding: encoding{0, new(big.Int), val}}
	leaf.hash()
	t.commit(prefix(rev, t.keyLen), leaf.bytes())
	t.diff(rev)
}

// Commitment returns the root hash of the trie. If the tree is empty,
// this value is nil.
func (t *Trie) Commitment() *big.Int {
	root, ok := t.retrive([]byte("root"))
	if !ok {
		// notest
		return nil
	}
	return root.Hash
}
