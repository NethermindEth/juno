package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
)

// TrieReader provides read-only access to a trie. It is the
// view returned from state readers for proof generation and storage lookups.
type TrieReader interface {
	// Get returns the value stored at key in the trie. Missing keys map to felt.Zero.
	Get(key *felt.Felt) (felt.Felt, error)
	// Hash returns the root hash of the trie.
	Hash() (felt.Felt, error)
	// HashFn returns the hash function used by the trie for node hashing.
	HashFn() crypto.HashFn
}

// Trie is the mutable view of a trie.
type Trie interface {
	TrieReader
	// Update sets the value at key in the trie. A zero value deletes the key.
	Update(key, value *felt.Felt) error
}

type onTempTrieFunc func(height uint8, do func(Trie) error) error

// TrieBackend selects which trie implementation backs the temporary tries
// used during commitment hashing.
type TempTrieBackend struct {
	// Pedersen runs do on Pedersen-hashed trie.
	RunOnTempTriePedersen onTempTrieFunc
	// Poseidon runs do on Poseidon-hashed trie.
	RunOnTempTriePoseidon onTempTrieFunc
}

var (
	DeprecatedTrieBackend = TempTrieBackend{
		RunOnTempTriePedersen: func(h uint8, do func(Trie) error) error {
			return trie.RunOnTempTriePedersen(h, func(t *trie.Trie) error { return do(t) })
		},
		RunOnTempTriePoseidon: func(h uint8, do func(Trie) error) error {
			return trie.RunOnTempTriePoseidon(h, func(t *trie.Trie) error { return do(t) })
		},
	}
	TrieBackend = TempTrieBackend{
		RunOnTempTriePedersen: func(h uint8, do func(Trie) error) error {
			return trie2.RunOnTempTriePedersen(h, func(t *trie2.Trie) error { return do(t) })
		},
		RunOnTempTriePoseidon: func(h uint8, do func(Trie) error) error {
			return trie2.RunOnTempTriePoseidon(h, func(t *trie2.Trie) error { return do(t) })
		},
	}
)
