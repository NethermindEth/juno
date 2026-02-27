package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// Trie provides access to a Merkle-Patricia trie used to store and commit state data.
type Trie interface {
	// Get returns the value stored at key in the trie.
	Get(key *felt.Felt) (felt.Felt, error)
	// Hash returns the root hash of the trie, committing all current entries.
	Hash() (felt.Felt, error)
	// HashFn returns the hash function used by the trie for node hashing.
	HashFn() crypto.HashFn
}
