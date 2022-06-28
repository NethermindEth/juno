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

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/felt"
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

// commit persists the given key-value pair in storage.
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
func (t *Trie) retrieve(key []byte) (Node, bool) {
	if len(key) == 0 {
		key = []byte("root")
	}
	b, ok := t.store.Get(key)
	if !ok {
		return Node{}, false
	}
	var n Node
	if err := json.Unmarshal(b, &n); err != nil {
		// notest
		return Node{}, false
	}
	return n, true
}

// diff traverses the tree upwards from the given path (key) starting
// from the node that immediately precedes the bottom node and either
// deletes the node if it its child nodes are empty or recomputes the
// encoding and hashes otherwise.
func (t *Trie) diff(key *felt.Felt) {
	for height := t.keyLen - 1; height >= 0; height-- {
		parent := Prefix(key, height)

		leftChild, leftChildIsNotEmpty := t.retrieve(append(parent, 48 /* "0" */))
		rightChild, rightChildIsNotEmpty := t.retrieve(append(parent, 49 /* "1" */))

		if !leftChildIsNotEmpty && !rightChildIsNotEmpty {
			t.remove(parent)
		} else {
			// Overwrite the parent node.
			n := new(Node)

			// Compute its encoding.
			switch {
			case !rightChildIsNotEmpty:
				n.Encoding = Encoding{
					leftChild.Length + 1, leftChild.Path, leftChild.Bottom,
				}
			case !leftChildIsNotEmpty:
				n.Encoding = Encoding{
					rightChild.Length + 1,
					rightChild.Path.FromMont().SetBit(uint64(rightChild.Length), 1).ToMont(),
					rightChild.Bottom,
				}
			default:
				n.Encoding = Encoding{
					0, new(felt.Felt), pedersen.Digest(leftChild.Hash, rightChild.Hash),
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
func (t *Trie) Delete(key *felt.Felt) {
	// The internal representation of felt.Felt has the least significant
	// bit in the 0th position but this algorithm assumes the oppose so
	// a copy with the bits reversed is used instead.
	rev := Reversed(key, t.keyLen)

	t.remove(Prefix(rev, t.keyLen))
	t.diff(rev)
}

// Get retrieves a value from the trie with the corresponding key.
func (t *Trie) Get(key *felt.Felt) (*felt.Felt, bool) {
	// The internal representation of felt.Felt has the least significant
	// bit in the 0th position but this algorithm assumes the opposite so
	// a copy with the bits reversed is passed into the function.
	node, ok := t.retrieve(Prefix(Reversed(key, t.keyLen), t.keyLen))
	if !ok {
		return nil, false
	}
	return node.Bottom, true
}

// Put inserts a [felt.Felt] key-value pair in the trie.
func (t *Trie) Put(key, val *felt.Felt) {
	if val.CmpCompat(felt.Felt0) == 0 {
		t.Delete(key)
		return
	}

	// The internal representation of felt.Felt has the least significant
	// bit in the 0th position but this algorithm assumes the oppose so a
	// copy with the bits reversed is used instead.
	rev := Reversed(key, t.keyLen)

	leaf := Node{Encoding{0, new(felt.Felt), val}, val}
	t.commit(Prefix(rev, t.keyLen), leaf.bytes())
	t.diff(rev)
}

// Commitment returns the root hash of the trie. If the tree is empty,
// this value is nil.
func (t *Trie) Commitment() *felt.Felt {
	root, ok := t.retrieve([]byte{})
	if !ok {
		return new(felt.Felt)
	}
	return root.Hash
}
