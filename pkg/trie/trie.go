// Package trie implements the [Merkle-PATRICIA tree] used in the
// StarkNet protocol.
//
// The shape of the trie is independent of the key insertion or deletion
// order. In other words, the same set of keys is guaranteed to produce
// the same tree irrespective of the order of which keys are put and
// removed from the tree. This property guarantees that the commitment
// will thus remain consistent across tree that use a different
// insertion and deletion order which is not true for other linked
// structures.
//
// # Worst-case time bounds for get and put operations
//
// The get operation is subject to the structure of the underlying data
// store. For all intents and purposes this can be assumed to be
// "constant" but in reality, if for example, a hashing based symbol
// table is used as a backend to this structure, then the get operation
// will take as much time as insertions take on that data structure.
//
// Put operations on the other hand take at most 1 plus the length of
// the key database accesses which is optimal.
//
// # Space
//
// The number of links in the trie will be 2 * n * w where w is the key
// length. This however accounts for memory usage during insertion. For
// persistent storage, this number is n * w given only non-empty nodes
// are stored.
//
// [Merkle-Patricia tree]: https://docs.starknet.io/docs/State/starknet-state#merkle-patricia-tree
package trie

import (
	"encoding/json"
	"fmt"
	"math"
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

// Trie represents a binary trie.
type Trie struct {
	keyLen int
	root   node
	store  store.Storer
}

// New constructs a new binary trie.
func New(store store.Storer, keyLen int) Trie {
	return Trie{keyLen: keyLen, store: store}
}

// commit persits the given key-value pair in storage.
func (t *Trie) commit(key, val []byte) {
	t.store.Put(key, val)
}

// remove deletes a key-value pair from storage.
func (t *Trie) remove(key []byte) {
	t.store.Delete(key)
}

// retrieve gets a node from storage.
func (t *Trie) retrive(key []byte) (node, bool) {
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

// triplet encodes a given (parent) node in the trie according to the
// specification (see package documentation).
func (t *Trie) triplet(n *node, key *big.Int, height int) {
	prefix := prefix(key, height)
	left, leftIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s0", prefix)))
	right, rightIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s1", prefix)))

	// Create the following variables so that the switch can be expressed
	// in the same manner that it appears in the specification.
	leftIsEmpty, rightIsEmpty := !leftIsNotEmpty, !rightIsNotEmpty

	switch {
	case leftIsEmpty && rightIsEmpty:
		// notest
		n.encoding = encoding{0, new(big.Int), new(big.Int)}
	case !leftIsEmpty && rightIsEmpty:
		n.encoding = encoding{
			left.Length + 1, left.Path, new(big.Int).Set(left.Bottom),
		}
	case leftIsEmpty && !rightIsEmpty:
		addend := new(big.Int).SetUint64(uint64(math.Pow(2, float64(right.Length))))
		n.encoding = encoding{
			right.Length + 1,
			right.Path.Add(right.Path, addend),
			new(big.Int).Set(right.Bottom),
		}
	default:
		h, _ := pedersen.Digest(left.Hash, right.Hash)
		n.encoding = encoding{0, new(big.Int), h}
	}
}

func (t *Trie) put(n node, key, val *big.Int, height int) node {
	if n.isEmpty() {
		n = newNode()
	}

	// Reached the bottom of the tree.
	if height == t.keyLen {
		n.encoding = encoding{0, new(big.Int), val}
		n.updateHash()
		t.commit(prefix(key, height), n.bytes())
		return n
	}

	b := key.Bit(height)
	n.Next[b] = t.put(n.Next[b], key, val, height+1)
	t.triplet(&n, key, height)
	n.updateHash()
	n.clear()
	if height == 0 {
		t.commit([]byte("root"), n.bytes())
	} else {
		t.commit(prefix(key, height), n.bytes())
	}
	return n
}

// Delete removes a key-value pair from the trie.
func (t *Trie) Delete(key *big.Int) {
	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the opposite so
	// a copy with the bits reversed is passed into the function.
	t.remove(prefix(reversed(key, t.keyLen), t.keyLen))
	for height := t.keyLen - 1; height >= 0; height-- {
		curr := prefix(key, height)

		_, leftIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s0", curr)))
		_, rightIsNotEmpty := t.retrive([]byte(fmt.Sprintf("%s1", curr)))

		switch {
		case !leftIsNotEmpty && !rightIsNotEmpty:
			t.remove(curr)
		default:
			n, _ := t.retrive(curr)
			t.triplet(&n, key, height)
			n.updateHash()
			if height == 0 {
				t.commit([]byte("root"), n.bytes())
			} else {
				t.commit(curr, n.bytes())
			}
		}
	}
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
	// bit in the 0th position but this algorithm assumes the opposite so
	// a copy with the bits reversed is passed into the function.
	t.root = t.put(t.root, reversed(key, t.keyLen), val, 0)
}
