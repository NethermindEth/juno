package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
)

// Trie provides access to a Merkle-Patricia trie used to store and commit state data.
type Trie interface {
	// Get returns the value stored at key in the trie.
	Get(key *felt.Felt) (felt.Felt, error)
	// Update sets the value at key in the trie to value.
	Update(key, value *felt.Felt) error
	// Hash returns the root hash of the trie, committing all current entries.
	Hash() (felt.Felt, error)
	// HashFn returns the hash function used by the trie for node hashing.
	HashFn() crypto.HashFn
}

type TrieBackend uint8

const (
	DeprecatedTrieBackend TrieBackend = iota
	NewTrieBackend
)

type runOnTempTrieFn func(height uint8, do func(Trie) error) error

func (e TrieBackend) Pedersen() runOnTempTrieFn {
	switch e {
	case NewTrieBackend:
		return func(h uint8, do func(Trie) error) error {
			return trie2.RunOnTempTriePedersen(h, func(t *trie2.Trie) error { return do(t) })
		}
	case DeprecatedTrieBackend:
		return func(h uint8, do func(Trie) error) error {
			return trie.RunOnTempTriePedersen(h, func(t *trie.Trie) error { return do(t) })
		}
	default:
		panic("unsupported trie backend")
	}
}

func (e TrieBackend) Poseidon() runOnTempTrieFn {
	switch e {
	case NewTrieBackend:
		return func(h uint8, do func(Trie) error) error {
			return trie2.RunOnTempTriePoseidon(h, func(t *trie2.Trie) error { return do(t) })
		}
	case DeprecatedTrieBackend:
		return func(h uint8, do func(Trie) error) error {
			return trie.RunOnTempTriePoseidon(h, func(t *trie.Trie) error { return do(t) })
		}
	default:
		panic("unsupported trie backend")
	}
}
