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
// Put and delete operations on the other hand take at most 1 + (w * 3)
// database accesses where w represents the bit-length of the key.
//
// # Space
//
// The tree only stores non-empty nodes so the space complexity is n * w
// where w is the key length.
//
// [Merkle-Patricia tree]: https://docs.starknet.io/docs/State/starknet-state#merkle-patricia-tree
package trie

import (
	"encoding/json"
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
	if len(key) == 0 {
		key = []byte("root")
	}
	t.store.Put(key, val)
}

// remove deletes a key-value pair from storage.
func (t *Trie) remove(key []byte) {
	if len(key) == 0 {
		key = []byte("root")
	}
	t.store.Delete(key)
}

// retrieve gets a node from storage and returns true if the node was
// found.
func (t *Trie) retrive(key []byte) (node, bool) {
	if len(key) == 0 {
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

		leftChild, leftChildIsNotEmpty := t.retrive(append(parent, 48 /* "0" */))
		rightChild, rightChildIsNotEmpty := t.retrive(append(parent, 49 /* "1" */))

		if !leftChildIsNotEmpty && !rightChildIsNotEmpty {
			t.remove(parent)
		} else {
			// Overwrite the parent node.
			n := new(node)

			// Compute its encoding.
			switch {
			case !rightChildIsNotEmpty:
				n.encoding = encoding{
					leftChild.Length + 1, leftChild.Path, new(big.Int).Set(leftChild.Bottom),
				}
			case !leftChildIsNotEmpty:
				n.encoding = encoding{
					rightChild.Length + 1,
					rightChild.Path.Add(
						rightChild.Path,
						new(big.Int).Exp(
							new(big.Int).SetUint64(2),
							new(big.Int).SetUint64(uint64(rightChild.Length)), nil)),
					new(big.Int).Set(rightChild.Bottom),
				}
			default:
				n.encoding = encoding{
					0, new(big.Int), pedersen.Digest(leftChild.Hash, rightChild.Hash),
				}
			}

			// Compute its hash.
			n.hash()

			// Commit to the database.
			t.commit(parent, n.bytes())
		}
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
	if val.Cmp(new(big.Int)) == 0 {
		t.Delete(key)
		return
	}

	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the oppose so a
	// copy with the bits reversed is used instead.
	rev := reversed(key, t.keyLen)

	leaf := node{encoding: encoding{0, new(big.Int), val}}
	leaf.hash()
	t.commit(prefix(rev, t.keyLen), leaf.bytes())
	t.diff(rev)
}

// Commitment returns the root hash of the trie. If the tree is empty,
// this value is nil.
func (t *Trie) Commitment() *big.Int {
	root, ok := t.retrive([]byte{})
	if !ok {
		return new(big.Int)
	}
	return root.Hash
}
