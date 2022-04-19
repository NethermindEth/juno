// Package trie implements the [Merkle-PATRICIA tree] used in the StarkNet
// protocol.
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

// Trie represents a binary trie.
type Trie struct {
	root  node
	store store.Storer
}

// TODO: Convert keyLen to a mutable variable so that it can be switched
// out for shorter keys during testing.

// keyLen describes the bit length of the keys (including leading
// zeroes) used in the trie.
const keyLen = 3

// New constructs a new binary trie.
func New(store store.Storer) Trie {
	return Trie{store: store}
}

// commit persits the given key-value pair in storage.
func (t *Trie) commit(key, val []byte) {
	// DEBUG.
	// fmt.Printf("commit: key = %s, val = %s\n\n", key, val)

	t.store.Put(key, val)
}

// retrieve gets a node from storage.
func (t *Trie) retrive(key []byte) (node, bool) {
	b, ok := t.store.Get(key)
	if !ok {
		return node{}, false
	}
	var n node
	if err := json.Unmarshal(b, &n); err != nil {
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
	if height == keyLen {
		n.encoding = encoding{0, new(big.Int), val}

		// DEBUG.
		fmt.Printf("enc = %s\n", n.encoding.String())

		n.updateHash()
		t.commit(prefix(key, height), n.Bytes())
		return n
	}

	b := key.Bit(height)
	n.Next[b] = t.put(n.Next[b], key, val, height+1)
	t.triplet(&n, key, height)

	// DEBUG.
	fmt.Printf("enc = %s\n", n.encoding.String())

	n.updateHash()
	n.clear()
	if height == 0 {
		t.commit([]byte("root"), n.Bytes())
	} else {
		t.commit(prefix(key, height), n.Bytes())
	}
	return n
}

// TODO: Implement Delete.

// Put inserts a [big.Int] key-value pair in the trie.
func (t *Trie) Put(key, val *big.Int) {
	// The internal representation of big.Int has the least significant
	// bit in the 0th position but this algorithm assumes the opposite so
	// a copy with the bits reversed is passed into the function.
	t.root = t.put(t.root, reversed(key), val, 0)
}

// Get retrieves a value from the trie with the corresponding key.
func (t *Trie) Get(key *big.Int) (*big.Int, bool) {
	node, ok := t.retrive(prefix(reversed(key), keyLen))
	if !ok {
		return nil, false
	}
	return node.Bottom, true
}
